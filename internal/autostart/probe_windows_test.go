//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/dzshzx/power-meter/internal/winapp"
)

// A copy of this test binary stands in for the app: the live tests install
// it as the protected copy and run it as an installation. TestMain then
// acts on the app's own command line instead of running tests.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		if code, ok := runProbe(os.Args[1], os.Args[2:]); ok {
			os.Exit(code)
		}
	}
	os.Exit(m.Run())
}

func runProbe(command string, args []string) (int, bool) {
	switch command {
	case "--autostart", "--no-elevate", "--elevation-attempted":
		return probeInstance(command), true
	case "--sync-autostart":
		return RunSync(), true
	case "--remove-autostart":
		return RunRemove(), true
	case "--enable-autostart":
		return probeEnable(), true
	case "--status":
		return probeStatus(), true
	case "--ordinary-attack":
		return probeAttack(args), true
	}
	return 0, false
}

// probeInstance is the GUI route: one instance per installation; a manual
// launch restores the running one, a duplicate logon launch exits quietly.
// The running instance reports in probe-<pid>.txt beside its installation,
// records each restore request in restored-<pid>.txt, and stays until a
// file named stop appears there (or two minutes pass).
func probeInstance(mode string) int {
	identity := winapp.Identity()
	dir, _ := winapp.SplitPath(identity)
	inst, ok, err := winapp.Acquire(identity, mode == "--elevation-attempted")
	if err != nil {
		return 1
	}
	if !ok {
		if mode != "--autostart" && !winapp.RestoreExisting(identity) {
			return 3
		}
		return 0
	}
	defer inst.Close()
	pid := os.Getpid()
	err = inst.Listen(func() {
		_ = os.WriteFile(fmt.Sprintf(`%s\restored-%d.txt`, dir, pid), []byte("restored"), 0o644)
	})
	if err != nil {
		return 4
	}
	exe, _ := os.Executable()
	report := fmt.Sprintf("elevated=%v\nimage=%s\nidentity=%s\nmode=%s\n", winapp.IsElevated(), exe, identity, mode)
	if os.WriteFile(fmt.Sprintf(`%s\probe-%d.txt`, dir, pid), []byte(report), 0o644) != nil {
		return 5
	}
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if _, err := os.Stat(dir + `\stop`); err == nil {
			break
		}
	}
	return 0
}

// probeEnable turns startup on from the window's switch, confirming any
// replacement, and exits 0 when the switch then reads on.
func probeEnable() int {
	fmt.Println("outcome:", Change(true, func(title, question string) bool {
		fmt.Printf("confirm: %s | %s\n", title, strings.ReplaceAll(question, "\n", " "))
		return true
	}))
	if probeStatus() != int(SwitchOn) {
		return 1
	}
	return 0
}

// probeStatus prints the switch and exits with its value.
func probeStatus() int {
	s := ReadStatus()
	fmt.Printf("switch=%d\nnotice=%s\ntooltip=%s\n", s.Switch, s.Notice, s.Tooltip)
	return int(s.Switch)
}

// probeAttack runs under the ordinary token after an elevated run enabled
// startup for this user: every write to the protected copy, its folder, its
// ACL and the task's action must be denied, while replacing the
// installation given as the first argument (with the second) succeeds. It
// prints one line per attempt and exits 0 only when all of that holds.
func probeAttack(args []string) int {
	if winapp.IsElevated() || len(args) != 2 {
		fmt.Println("needs the ordinary token, the installation and its replacement")
		return 2
	}
	sid, err := winapp.UserSID()
	root, rootErr := DefaultRoot()
	if err != nil || rootErr != nil {
		return 2
	}
	c, err := newProtectedCopy(root, sid)
	if err != nil {
		return 2
	}
	failed := false
	for _, a := range attacks(c, TaskName(sid)) {
		err := a.try()
		denied := err != nil
		fmt.Printf("%-28s denied=%v %v\n", a.name, denied, err)
		failed = failed || !denied
	}
	data, err := os.ReadFile(args[1])
	if err == nil {
		err = os.WriteFile(args[0], data, 0o755)
	}
	fmt.Printf("%-28s replaced=%v %v\n", "replace installation", err == nil, err)
	if failed || err != nil || !exists(c.exe) {
		return 1
	}
	return 0
}

type attack struct {
	name string
	try  func() error
}

func attacks(c protectedCopy, taskName string) []attack {
	return []attack{
		{"open copy for write", func() error {
			f, err := os.OpenFile(c.exe, os.O_WRONLY, 0)
			if err == nil {
				f.Close()
			}
			return err
		}},
		{"create file beside copy", func() error { return os.WriteFile(c.dir+`\planted.txt`, []byte("planted"), 0o644) }},
		{"rename copy", func() error { return os.Rename(c.exe, c.exe+".moved") }},
		{"delete copy", func() error { return os.Remove(c.exe) }},
		{"rename copy folder", func() error { return os.Rename(c.dir, c.dir+".moved") }},
		{"add ACE to copy", func() error { return grantSelf(c.exe) }},
		{"add ACE to copy folder", func() error { return grantSelf(c.dir) }},
		{"rewrite task action", func() error { return rewriteAction(taskName) }},
	}
}

func grantSelf(path string) error {
	sid, err := winapp.UserSID()
	if err != nil {
		return err
	}
	user, err := windows.StringToSid(sid)
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeValue: windows.TrusteeValueFromSID(user)},
	}}, dacl)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

// rewriteAction re-registers the task with another program as its action,
// as an ordinary process would try to.
func rewriteAction(taskName string) error {
	sid, err := winapp.UserSID()
	if err != nil {
		return err
	}
	return withScheduler(func(s *scheduler) error {
		text, found, err := s.taskXML(taskName)
		if err != nil || !found {
			return errors.Join(err, fs.ErrNotExist)
		}
		d, err := parseTask(text)
		if err != nil {
			return err
		}
		d.Actions.Exec[0].Command = `C:\Windows\System32\whoami.exe`
		changed, err := d.marshal()
		if err != nil {
			return err
		}
		return callAndClear(s.folder, "RegisterTask", taskName, changed, int32(taskRegisterFlags), sid, nil,
			int32(taskLogonInteractiveToken), nil)
	})
}
