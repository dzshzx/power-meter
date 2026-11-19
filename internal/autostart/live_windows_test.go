//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/dzshzx/power-meter/internal/winapp"
)

// live skips a test that writes system state (scheduled tasks, Program
// Files) unless POWERMETER_LIVE=1, and requires an elevated token: these
// tests never raise UAC.
func live(t *testing.T) {
	t.Helper()
	if os.Getenv("POWERMETER_LIVE") != "1" {
		t.Skip("set POWERMETER_LIVE=1 to register tasks and write below Program Files")
	}
	if !winapp.IsElevated() {
		t.Fatal("live tests need an elevated token; they never request UAC")
	}
}

func currentSID(t *testing.T) string {
	t.Helper()
	sid, err := winapp.UserSID()
	if err != nil {
		t.Fatal(err)
	}
	return sid
}

func TestProtectedRootMustBeBelowProgramFiles(t *testing.T) {
	root, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(root, `\Power Meter\autostart`) {
		t.Errorf("default root: %s", root)
	}
	if c, err := newProtectedCopy(root, testSID); err != nil || c.exe != root+`\`+testSID+`\PowerMeter.exe` {
		t.Errorf("the default root is accepted: %+v %v", c, err)
	}
	parent, _ := winapp.SplitPath(root)
	for _, bad := range []string{parent, root + `\deeper`, os.TempDir() + `\Power Meter\autostart`, `autostart`} {
		if _, err := newProtectedCopy(bad, testSID); err == nil {
			t.Errorf("root %q accepted", bad)
		}
	}
}

// The scheduler may store the user as DOMAIN\user; the real account lookup
// maps it back to this user's SID.
func TestAccountNamesNormalizeToTheCurrentUser(t *testing.T) {
	sid := currentSID(t)
	user, err := windows.StringToSid(sid)
	if err != nil {
		t.Fatal(err)
	}
	account, domain, _, err := user.LookupAccount("")
	if err != nil {
		t.Fatal(err)
	}
	name := domain + `\` + account
	d := definition(protectedExe(sid), sid)
	d.Principals.Principal[0].UserID = ptr(name)
	d.Triggers.Logon[0].UserID = ptr(name)
	text, err := d.marshal()
	if err != nil {
		t.Fatal(err)
	}
	s, err := inspectTask(text, installedPath, sid, protectedExe(sid), installedPath, lookupAccount)
	if err != nil || !s.enabled {
		t.Errorf("account name %s reads as this user: %+v %v", name, s, err)
	}
	if lookupAccount(`NO-SUCH-DOMAIN\no-such-user`) != "" {
		t.Error("an unknown account maps to nothing")
	}
}

// fixture is an isolated installation set: copies of this test binary in a
// temporary folder, a protected root of their own below Program Files and a
// task name of their own, so the real startup is never touched.
type fixture struct {
	t        *testing.T
	sid      string
	id       string
	dir      string
	task     string
	parent   string
	root     string
	exe      string // the shared protected copy
	a, b, c  *manager
	pathA    string
	pathB    string
	pathC    string
	original [32]byte
}

func newFixture(t *testing.T) *fixture {
	sid := currentSID(t)
	f := &fixture{t: t, sid: sid, id: newID()}
	f.dir = filepath.Join(os.TempDir(), "pm-autostart-"+f.id)
	programFiles, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.parent = filepath.Join(programFiles, "Power Meter AutostartTest "+f.id)
	f.root = filepath.Join(f.parent, "autostart")
	f.exe = filepath.Join(f.root, sid, "PowerMeter.exe")
	f.task = "PowerMeter.Test." + f.id
	t.Cleanup(f.cleanup)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.pathA = filepath.Join(f.dir, "版本 A & space", "PowerMeter.exe")
	f.pathB = filepath.Join(f.dir, "copy B", "Battery Meter.exe")
	f.pathC = filepath.Join(f.dir, "copy C", "Battery Meter.exe")
	for _, p := range []string{f.pathA, f.pathB, f.pathC} {
		copyFile(t, self, p)
	}
	if f.original, err = hashFile(f.pathA); err != nil {
		t.Fatal(err)
	}
	f.a, f.b, f.c = f.manager(f.pathA), f.manager(f.pathB), f.manager(f.pathC)
	return f
}

func (f *fixture) manager(path string) *manager {
	m, err := newManager(path, f.sid, f.task, f.root, true)
	if err != nil {
		f.t.Fatal(err)
	}
	return m
}

func (f *fixture) cleanup() {
	_ = os.WriteFile(filepath.Join(filepath.Dir(f.pathA), "stop"), nil, 0o644)
	c := protectedCopy{root: f.root, dir: filepath.Dir(f.exe), exe: f.exe}
	c.stopInstances()
	_ = withScheduler(func(s *scheduler) error { return s.delete(f.task) })
	_ = os.RemoveAll(f.parent)
	_ = os.RemoveAll(f.dir)
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(to), 0o755)
	}
	if err == nil {
		err = os.WriteFile(to, data, 0o755)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) read(m *manager) state {
	f.t.Helper()
	s, err := m.Read()
	if err != nil {
		f.t.Fatalf("read: %v", err)
	}
	return s
}

func (f *fixture) enable(m *manager) {
	f.t.Helper()
	if err := m.Enable(f.read(m)); err != nil {
		f.t.Fatalf("enable: %v", err)
	}
}

func (f *fixture) taskXML() string {
	f.t.Helper()
	var text string
	err := withScheduler(func(s *scheduler) error {
		var err error
		text, _, err = s.taskXML(f.task)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return text
}

func (f *fixture) registerXML(text string) {
	f.t.Helper()
	err := withScheduler(func(s *scheduler) error {
		return callAndClear(s.folder, "RegisterTask", f.task, text, int32(taskRegisterFlags), f.sid, nil,
			int32(taskLogonInteractiveToken), nil)
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) command() string {
	f.t.Helper()
	d, err := parseTask(f.taskXML())
	if err != nil || len(d.Actions.Exec) == 0 {
		f.t.Fatalf("task action: %v", err)
	}
	return d.Actions.Exec[0].Command
}

func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}

// TestLiveIntegration registers, reads, replaces and removes real tasks and
// protected copies under isolated names.
func TestLiveIntegration(t *testing.T) {
	live(t)
	f := newFixture(t)
	check(t, !f.read(f.a).exists, "a unique test task starts absent")
	f.unprotectedTaskIsMigrated()
	absent := f.read(f.a)
	f.enableRegistersTheProtectedShape()
	f.replacementNeedsConfirmation(absent)
	f.anotherInstallationsCopyNeedsConfirmation()
	f.foreignTaskIsLeftAlone(absent)
	f.driftIsRepaired()
	f.schedulerRunsTheProtectedCopy()
	f.handoffWaitsForTheLock()
	if r, err := f.a.Disable(); err != nil || r != removed {
		t.Fatalf("final removal: %d %v", r, err)
	}
	_, statErr := os.Stat(f.parent)
	check(t, !f.read(f.a).exists && errors.Is(statErr, os.ErrNotExist), "final removal reads back absent task, copy and empty folders: %v", statErr)
}

// A task in this program's shape that runs the user-writable file is
// reported as needing migration and moved to the protected copy.
func (f *fixture) unprotectedTaskIsMigrated() {
	t := f.t
	text, err := definition(f.pathA, f.sid).marshal()
	if err != nil {
		t.Fatal(err)
	}
	f.registerXML(text)
	s := f.read(f.a)
	check(t, s.thisCopy && !s.protected && !s.enabled && s.repairReason != "", "a task running the user-writable file needs migration: %+v", s)
	r, err := f.a.Synchronize(false)
	check(t, err == nil && r == syncUnprotected, "report-only synchronization flags the unprotected task: %d %v", r, err)
	r, err = f.a.Synchronize(true)
	check(t, err == nil && r == syncCurrent && f.command() == f.exe, "elevated synchronization migrates the task to the protected copy: %d %v %s", r, err, f.command())
	if _, err := f.a.Disable(); err != nil {
		t.Fatal(err)
	}
	check(t, !f.read(f.a).exists && !exists(f.exe), "elevated disable removes task and protected copy")
}

func (f *fixture) enableRegistersTheProtectedShape() {
	t := f.t
	f.enable(f.a)
	s := f.read(f.a)
	check(t, s.enabled && s.thisCopy && s.protected && winapp.SamePath(s.executable, f.pathA), "register and read back the elevated logon task: %+v", s)
	sum, err := hashFile(f.exe)
	check(t, err == nil && f.command() == f.exe && sum == f.original, "the task runs a byte-identical copy in the admin-only folder")
	f.enable(f.a)
	check(t, f.read(f.a).enabled, "an idempotent update reads back enabled")
	d, err := parseTask(f.taskXML())
	if err != nil {
		t.Fatal(err)
	}
	p := d.Principals.Principal[0]
	check(t, valueOr(p.RunLevel, "") == "HighestAvailable" && valueOr(p.LogonType, "") == "InteractiveToken" && strings.EqualFold(valueOr(p.UserID, ""), f.sid),
		"explicit user SID, highest token, no stored password: %+v", p)
	check(t, hasStartupSettings(d.Settings) && valueOr(d.Settings.MultipleInstancesPolicy, "") == "IgnoreNew",
		"battery, idle, network, time-limit and instance settings: %s", f.taskXML())
	f.checkTaskSecurity()
	for _, folder := range []string{f.parent, f.root, filepath.Dir(f.exe)} {
		checkProtected(t, folder, true)
	}
	for _, file := range []string{f.exe, filepath.Join(filepath.Dir(f.exe), "source.txt")} {
		checkProtected(t, file, false)
	}
}

// checkTaskSecurity: a protected DACL in which SYSTEM and Administrators
// have full control and the user may only read, run (GR GX, stored as
// 0x1200a9) and delete (SD, 0x10000); no ACE the scheduler adds itself.
func (f *fixture) checkTaskSecurity() {
	var sddl string
	err := withScheduler(func(s *scheduler) error {
		var err error
		sddl, err = s.security(f.task)
		return err
	})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Logf("task security: %s", sddl)
	_, dacl, _ := strings.Cut(sddl, "D:")
	flags, aces, _ := strings.Cut(dacl, "(")
	check(f.t, strings.HasPrefix(flags, "P") && "("+aces == "(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1300a9;;;"+f.sid+")",
		"task ACL lets the user only read, run and delete: %s", sddl)
}

// checkProtected: owned by Administrators, protected DACL, writable only by
// SYSTEM and Administrators.
func checkProtected(t *testing.T, path string, folder bool) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	text := sd.String()
	check(t, owner.IsWellKnown(windows.WinBuiltinAdministratorsSid), "%s owned by Administrators: %s", path, text)
	check(t, !folder || control&windows.SE_DACL_PROTECTED != 0, "%s DACL does not inherit: %s", path, text)
	for _, ace := range strings.Split(text, "(")[1:] {
		fields := strings.Split(strings.TrimSuffix(ace, ")"), ";")
		check(t, len(fields) == 6, "%s ACE %q", path, ace)
		trustee := fields[5]
		check(t, !grantsWrite(fields[2]) || trustee == "SY" || trustee == "BA", "%s grants write only to SYSTEM and Administrators: %s", path, text)
	}
	check(t, strings.Contains(text, ";0x1200a9;;;BU)"), "%s lets Users read and execute: %s", path, text)
}

// grantsWrite reports whether SDDL rights include writing, deleting or
// changing the security of a file.
func grantsWrite(rights string) bool {
	if mask, err := strconv.ParseUint(strings.TrimPrefix(rights, "0x"), 16, 32); strings.HasPrefix(rights, "0x") && err == nil {
		const writes = 0x2 | 0x4 | 0x10 | 0x40 | 0x100 | 0x10000 | 0x40000 | 0x80000 | 0x10000000 | 0x40000000
		return mask&writes != 0
	}
	for _, r := range []string{"FA", "FW", "GA", "GW", "WD", "WO", "SD", "WP", "CC", "DC"} {
		for i := 0; i+2 <= len(rights); i += 2 {
			if rights[i:i+2] == r {
				return true
			}
		}
	}
	return false
}

func (f *fixture) replacementNeedsConfirmation(absent state) {
	t := f.t
	err := f.b.Enable(absent)
	check(t, err != nil && f.read(f.a).thisCopy, "another copy cannot silently repoint the task: %v", err)
	confirmedA := f.read(f.b)
	check(t, replaces(confirmedA) != "" && winapp.SamePath(replaces(confirmedA), f.pathA), "B is asked to replace A: %+v", confirmedA)
	f.enable(f.c)
	err = f.b.Enable(confirmedA)
	check(t, err != nil && f.read(f.c).thisCopy, "a stale A-to-B confirmation cannot overwrite concurrent copy C: %v", err)
	f.enable(f.b)
	if _, err := f.a.Disable(); err != nil {
		t.Fatal(err)
	}
	check(t, f.read(f.b).thisCopy && exists(f.exe), "old installation cleanup preserves another copy's task and protected copy")
	if _, err := f.b.Disable(); err != nil {
		t.Fatal(err)
	}
	check(t, !f.read(f.b).exists && !exists(f.exe), "own task and protected copy deletion reads back absent")
}

// A protected copy of another installation without a task of this name
// (another program's task may run it) is not overwritten silently.
func (f *fixture) anotherInstallationsCopyNeedsConfirmation() {
	t := f.t
	f.enable(f.a)
	if err := withScheduler(func(s *scheduler) error { return s.delete(f.task) }); err != nil {
		t.Fatal(err)
	}
	s := f.read(f.b)
	check(t, !s.exists && winapp.SamePath(replaces(s), f.pathA), "B sees A's copy as a replacement: %+v", s)
	check(t, describe(s, false, f.task).Tooltip != describe(state{}, false, f.task).Tooltip, "the switch tooltip names A's copy")
	err := f.b.Enable(state{})
	check(t, err != nil && f.c.copy.ownedBy(f.pathA), "an unconfirmed enable leaves A's copy: %v", err)
	r, err := f.b.Synchronize(false)
	check(t, err == nil && r == syncCurrent, "B leaves A's copy alone: %d %v", r, err)
	r, err = f.a.Synchronize(false)
	check(t, err == nil && r == syncOrphanCopy, "A reports its copy as unused: %d %v", r, err)
	r, err = f.a.Synchronize(true)
	check(t, err == nil && r == syncCurrent && !exists(f.exe), "A removes its unused copy: %d %v", r, err)
}

// A same-name task in a shape this program never registers is left alone:
// reading reports it, enabling refuses, and disabling still removes this
// installation's copy and reports the kept task (exit code 6).
func (f *fixture) foreignTaskIsLeftAlone(absent state) {
	t := f.t
	f.enable(f.a)
	foreign := strings.NewReplacer("<Arguments>--autostart</Arguments>", "<Arguments>--not-power-meter</Arguments>",
		"HighestAvailable", "LeastPrivilege").Replace(f.taskXML())
	f.registerXML(foreign)
	foreign = f.taskXML()
	_, err := f.a.Read()
	check(t, errors.Is(err, errForeignTask), "a foreign same-name task is reported as not this program's: %v", err)
	err = f.a.Enable(absent)
	check(t, err != nil && f.taskXML() == foreign, "enabling never overwrites a foreign same-name task: %v", err)
	status, err := f.a.Describe()
	check(t, err == nil && status.Switch == SwitchOff && strings.HasSuffix(status.Notice, f.task), "the switch is off and says where to delete it: %+v %v", status, err)
	r, err := f.a.Synchronize(false)
	check(t, err == nil && r == syncOrphanCopy, "report-only synchronization treats the owned copy as unused beside a foreign task: %d %v", r, err)
	rm, err := f.a.Disable()
	check(t, err == nil && rm == removalForeignTaskKept, "disable reports the kept foreign task: %d %v", rm, err)
	check(t, f.taskXML() == foreign && !exists(f.exe), "disable leaves the foreign task unchanged and removes the protected copy")
	rm, err = f.a.Disable()
	check(t, err == nil && rm == removalForeignTaskKept, "repeated disable stays finishable: %d %v", rm, err)
	if err := withScheduler(func(s *scheduler) error { return s.delete(f.task) }); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) driftIsRepaired() {
	t := f.t
	f.enable(f.a)
	f.registerXML(strings.Replace(f.taskXML(), "<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>",
		"<DisallowStartIfOnBatteries>true</DisallowStartIfOnBatteries>", 1))
	drifted := f.read(f.a)
	check(t, !drifted.enabled && drifted.repairReason != "", "real task policy drift is reported as needing repair: %+v", drifted)
	f.enable(f.a)
	check(t, f.read(f.a).enabled, "re-enable repairs drift and reads back enabled")
}

// files waits until pattern matches want files in the folder of copy A
// and returns their contents by name.
func (f *fixture) files(pattern string, want int) map[string]string {
	f.t.Helper()
	dir := filepath.Dir(f.pathA)
	var found map[string]string
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		found = map[string]string{}
		for _, m := range matches {
			data, _ := os.ReadFile(m)
			found[filepath.Base(m)] = string(data)
		}
		if len(found) == want {
			return found
		}
	}
	f.t.Fatalf("expected %d files %s, found %v", want, pattern, found)
	return nil
}

// probes waits for the instance reports.
func (f *fixture) probes(want int) map[string]string { return f.files("probe-*.txt", want) }

func (f *fixture) resetProbes() {
	dir := filepath.Dir(f.pathA)
	_ = os.WriteFile(filepath.Join(dir, "stop"), nil, 0o644)
	c := protectedCopy{root: f.root, dir: filepath.Dir(f.exe), exe: f.exe}
	c.stopInstances()
	time.Sleep(500 * time.Millisecond)
	for _, pattern := range []string{"probe-*.txt", "restored-*.txt", "stop"} {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

func (f *fixture) runTask() {
	f.t.Helper()
	if err := withScheduler(func(s *scheduler) error { return s.run(f.task) }); err != nil {
		f.t.Fatal(err)
	}
}

// Replacing the user-writable executable, as any ordinary process can,
// leaves the elevated scheduler launch on the unchanged protected copy.
func (f *fixture) schedulerRunsTheProtectedCopy() {
	t := f.t
	f.resetProbes()
	backup := filepath.Join(f.dir, "copy-a-backup.exe")
	copyFile(t, f.pathA, backup)
	copyFile(t, `C:\Windows\System32\whoami.exe`, f.pathA)
	f.runTask()
	reports := f.probes(1)
	for _, r := range reports {
		check(t, strings.Contains(r, "elevated=true") && strings.Contains(r, "image="+f.exe) && strings.Contains(r, "mode=--autostart"),
			"the scheduler runs the protected copy, elevated, with --autostart: %s", r)
	}
	sum, err := hashFile(f.exe)
	check(t, err == nil && sum == f.original, "the protected copy is the original build after the source was replaced")
	copyFile(t, backup, f.pathA)
	f.runTask()
	time.Sleep(time.Second)
	f.probes(1)
	duplicate := exec.Command(f.pathA, "--autostart")
	check(t, duplicate.Run() == nil, "a duplicate logon launch exits cleanly")
	manual := exec.Command(f.pathA, "--no-elevate")
	check(t, manual.Run() == nil, "a manual launch hands over to the running instance")
	f.files("restored-*.txt", 1)
	f.probes(1)
	f.resetProbes()
}

// The elevated copy started by the elevation request waits for the
// process that started it, and the scheduler does not add a second one.
func (f *fixture) handoffWaitsForTheLock() {
	t := f.t
	lease, ok, err := winapp.Acquire(f.pathA, false)
	check(t, err == nil && ok, "parent owns the lock: %v", err)
	child := exec.Command(f.pathA, "--elevation-attempted")
	if err := child.Start(); err != nil {
		lease.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	time.Sleep(time.Second)
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(f.pathA), "probe-*.txt"))
	check(t, len(matches) == 0, "the handoff child waits for the parent's lock")
	lease.Close()
	f.probes(1)
	f.runTask()
	time.Sleep(2 * time.Second)
	f.probes(1)
	f.resetProbes()
	select {
	case err := <-done:
		check(t, err == nil, "the handoff child exits cleanly: %v", err)
	case <-time.After(10 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("the handoff child did not stop")
	}
	fmt.Println("handoff checked")
}
