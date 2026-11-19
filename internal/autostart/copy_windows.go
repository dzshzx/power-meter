//go:build windows

package autostart

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/dzshzx/power-meter/internal/winapp"
)

// The elevated logon task must only execute files that the signed-in
// user's ordinary token cannot replace. Enabling startup copies the running
// executable, plus a user-supplied IntelMSR.bin beside it, into a per-user
// folder below Program Files whose explicit, non-inherited DACL grants write
// access only to SYSTEM and Administrators.
//
// Invariants:
//   - Only an elevated process writes here, and it copies only its own
//     running image; it never copies a file named by another process.
//   - A process running from a protected copy never copies from its
//     recorded source, so logon startup cannot pick up a replaced
//     user-writable file.
//   - source.txt records which installation owns the copy. It is only used
//     for ownership decisions and instance naming, never as a file to run or
//     copy.

const (
	moduleName    = "IntelMSR.bin"
	retiredMarker = ".retired-"
	// folderSecurity: owned by Administrators; SYSTEM and Administrators
	// full control, Users read and execute; protected from inheritance and
	// inherited by everything inside. An owner can always rewrite a DACL,
	// so no object here may be owned by the user, whatever the default-owner
	// policy says.
	folderSecurity = "O:BAD:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1200a9;;;BU)"
)

var (
	errNotElevated  = errors.New(elevationRequiredForCopy)
	errNotOwnImage  = errors.New("只能从正在运行的程序本身创建自启副本。")
	errCopyWrite    = errors.New("自启副本写入后的校验失败。")
	errCopyRemove   = errors.New("自启副本删除后的校验失败。")
	errStageVerify  = errors.New("自启副本写入校验失败。")
	errStageChanged = errors.New("读取自启副本来源时文件被截断。")
)

// DefaultRoot is <Program Files>\Power Meter\autostart. Program Files comes
// from the known-folder API, not the user-controllable %ProgramFiles%.
func DefaultRoot() (string, error) {
	programFiles, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		return "", err
	}
	return filepath.Join(programFiles, "Power Meter", "autostart"), nil
}

// protectedCopy is one user's copy: <root>\<SID>\PowerMeter.exe.
type protectedCopy struct {
	root string
	dir  string
	exe  string
}

func newProtectedCopy(root, sid string) (protectedCopy, error) {
	full, ok := winapp.FullPath(root)
	programFiles, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		return protectedCopy{}, err
	}
	// The root and its parent receive an explicit DACL, so both must be
	// application folders directly below Program Files.
	parent, _ := winapp.SplitPath(full)
	grandparent, _ := winapp.SplitPath(parent)
	if !ok || !winapp.SamePath(grandparent, programFiles) {
		return protectedCopy{}, errors.New(`protected copy root must be <Program Files>\<folder>\<folder>`)
	}
	dir := full + `\` + sid
	return protectedCopy{root: full, dir: dir, exe: dir + `\` + winapp.ExecutableName}, nil
}

func (c protectedCopy) module() string     { return c.dir + `\` + moduleName }
func (c protectedCopy) sourceFile() string { return c.dir + `\` + winapp.SourceName }

// source is the installation recorded with the copy, "" when there is none.
func (c protectedCopy) source() string { return winapp.CopySource(c.dir) }

func (c protectedCopy) ownedBy(identity string) bool {
	s := c.source()
	return s != "" && winapp.SamePath(s, identity)
}

func (c protectedCopy) isSelf(image string) bool { return winapp.SamePath(image, c.exe) }

// matches reports whether the copy holds exactly the executable and the
// optional module beside image.
func (c protectedCopy) matches(image string) bool {
	if !exists(c.exe) || !sameBytes(image, c.exe) {
		return false
	}
	dir, _ := winapp.SplitPath(image)
	sourceModule := dir + `\` + moduleName
	has := exists(sourceModule)
	if has != exists(c.module()) {
		return false
	}
	return !has || sameBytes(sourceModule, c.module())
}

// install copies image, which must be the installation identity itself, and
// returns how many running logon instances it stopped, so the caller can
// restart them through the scheduler.
func (c protectedCopy) install(image, identity string) (int, error) {
	if !winapp.IsElevated() {
		return 0, errNotElevated
	}
	if c.isSelf(image) {
		return 0, nil
	}
	if !winapp.SamePath(image, identity) {
		return 0, errNotOwnImage
	}
	parent, _ := winapp.SplitPath(c.root)
	for _, dir := range []string{parent, c.root, c.dir} {
		if err := protectFolder(dir); err != nil {
			return 0, err
		}
	}
	c.removeRetired()
	staged, err := c.stageAll(image, identity)
	defer func() { staged.discard() }()
	if err != nil {
		return 0, err
	}
	stopped := c.stopInstances()
	if err := c.swapIn(&staged); err != nil {
		return stopped, err
	}
	if !c.matches(image) || !c.ownedBy(identity) {
		return stopped, errCopyWrite
	}
	return stopped, nil
}

// staging holds verified files beside their targets until they are swapped
// in; a name left set is a leftover to retire.
type staging struct{ exe, module, source string }

func (s staging) discard() {
	for _, path := range []string{s.exe, s.module, s.source} {
		if path != "" && exists(path) {
			_ = retire(path)
		}
	}
}

// stageAll writes and verifies every file first, before anything running
// is touched.
func (c protectedCopy) stageAll(image, identity string) (staging, error) {
	var s staging
	var err error
	if s.exe, err = c.stage(image, nil); err != nil {
		return s, err
	}
	dir, _ := winapp.SplitPath(image)
	if sourceModule := dir + `\` + moduleName; exists(sourceModule) {
		if s.module, err = c.stage(sourceModule, nil); err != nil {
			return s, err
		}
	}
	s.source, err = c.stage(c.sourceFile(), []byte(identity))
	return s, err
}

func (c protectedCopy) swapIn(s *staging) error {
	if err := swap(s.exe, c.exe); err != nil {
		return err
	}
	s.exe = ""
	if s.module != "" {
		if err := swap(s.module, c.module()); err != nil {
			return err
		}
		s.module = ""
	} else if exists(c.module()) {
		if err := retire(c.module()); err != nil {
			return err
		}
	}
	if err := swap(s.source, c.sourceFile()); err != nil {
		return err
	}
	s.source = ""
	return nil
}

// stage writes a new file beside the copy, owned by Administrators, from
// the file at source or from data when given, and verifies what landed.
func (c protectedCopy) stage(source string, data []byte) (string, error) {
	if data == nil {
		var err error
		if data, err = readWhole(source); err != nil {
			return "", err
		}
	}
	_, name := winapp.SplitPath(source)
	staged := c.dir + `\` + name + retiredMarker + newID() + ".new"
	if err := writeNew(staged, data); err != nil {
		return "", err
	}
	if err := ownByAdministrators(staged); err != nil {
		_ = retire(staged)
		return "", err
	}
	if sum, err := hashFile(staged); err != nil || sum != sha256.Sum256(data) {
		_ = retire(staged)
		return "", errors.Join(errStageVerify, err)
	}
	return staged, nil
}

// readWhole reads a file that must not change while it is read.
func readWhole(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != info.Size() {
		return nil, errStageChanged
	}
	return data, nil
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

// remove deletes the copy and the application folders it leaves empty;
// other users' copies keep theirs.
func (c protectedCopy) remove() error {
	if !winapp.IsElevated() {
		return errNotElevated
	}
	if !exists(c.dir) {
		return nil
	}
	c.stopInstances()
	for _, path := range []string{c.exe, c.module(), c.sourceFile()} {
		if exists(path) {
			if err := retire(path); err != nil {
				return err
			}
		}
	}
	c.removeRetired()
	if exists(c.exe) || exists(c.sourceFile()) {
		return errCopyRemove
	}
	parent, _ := winapp.SplitPath(c.root)
	for _, dir := range []string{c.dir, c.root, parent} {
		// Remove only deletes an empty folder; stop at the first that
		// still holds something.
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// swap puts staged in place of target. A running target is renamed away
// first, which Windows allows.
func swap(staged, target string) error {
	if exists(target) {
		if err := retire(target); err != nil {
			return err
		}
	}
	return os.Rename(staged, target)
}

// retire deletes a file now if possible, otherwise renames it aside and
// deletes it at the next reboot and on every later install or removal.
func retire(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	retired := path
	if !strings.Contains(path, retiredMarker) {
		retired = path + retiredMarker + newID()
		if err := os.Rename(path, retired); err != nil {
			return err
		}
	}
	if os.Remove(retired) != nil {
		// Best effort; the next install or removal deletes it otherwise.
		if p, err := windows.UTF16PtrFromString(retired); err == nil {
			_ = windows.MoveFileEx(p, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
		}
	}
	return nil
}

func (c protectedCopy) removeRetired() {
	matches, _ := filepath.Glob(filepath.Join(c.dir, "*"+retiredMarker+"*"))
	for _, path := range matches {
		_ = os.Remove(path)
	}
}

// stopInstances ends the processes running the copy, other than this one,
// and returns how many it stopped.
func (c protectedCopy) stopInstances() int {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snapshot)
	self := windows.GetCurrentProcessId()
	stopped := 0
	entry := windows.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err := windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID != self && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), winapp.ExecutableName) &&
			c.stop(entry.ProcessID) {
			stopped++
		}
	}
	return stopped
}

func (c protectedCopy) stop(pid uint32) bool {
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.PROCESS_TERMINATE | windows.SYNCHRONIZE
	h, err := windows.OpenProcess(access, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	if !winapp.SamePath(processImage(h), c.exe) || windows.TerminateProcess(h, 1) != nil {
		return false
	}
	_, _ = windows.WaitForSingleObject(h, 10000)
	return true
}

func processImage(h windows.Handle) string {
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

// protectFolder creates the folder with folderSecurity, or applies it to
// the folder that is there.
func protectFolder(path string) error {
	sd, err := windows.SecurityDescriptorFromString(folderSecurity)
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	sa := &windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(*sa))
	err = windows.CreateDirectory(p, sa)
	if err == nil || !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner, nil, dacl, nil)
}

func ownByAdministrators(path string) error {
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, admins, nil, nil, nil)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hashFile(path string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func sameBytes(a, b string) bool {
	ha, errA := hashFile(a)
	hb, errB := hashFile(b)
	return errA == nil && errB == nil && bytes.Equal(ha[:], hb[:])
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
