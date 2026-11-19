//go:build windows

package autostart

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"

	"github.com/dzshzx/power-meter/internal/winapp"
)

var (
	errRegisterVerify = errors.New("自启任务写入后的读取校验失败。")
	errDeleteVerify   = errors.New("自启任务删除后的读取校验失败。")
)

// manager gathers facts about one installation's startup and carries out
// the plans the policy makes from them.
type manager struct {
	// image is the running file; identity the installation it represents.
	// They differ only when running from the protected copy.
	image    string
	identity string
	sid      string
	taskName string
	copy     protectedCopy
	elevated bool
}

func newManager(image, sid, taskName, root string, elevated bool) (*manager, error) {
	full, ok := winapp.FullPath(image)
	if !ok {
		return nil, errors.New("not a fully qualified path: " + image)
	}
	c, err := newProtectedCopy(root, sid)
	if err != nil {
		return nil, err
	}
	return &manager{
		image:    full,
		identity: winapp.IdentityFor(full, sid),
		sid:      sid,
		taskName: taskName,
		copy:     c,
		elevated: elevated,
	}, nil
}

func forCurrentProcess() (*manager, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	sid, err := winapp.UserSID()
	if err != nil {
		return nil, err
	}
	root, err := DefaultRoot()
	if err != nil {
		return nil, err
	}
	return newManager(exe, sid, TaskName(sid), root, winapp.IsElevated())
}

// lookupAccount maps DOMAIN\user, as the scheduler may store a logon
// trigger's user, to its SID.
func lookupAccount(name string) string {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return ""
	}
	return sid.String()
}

// read is the task as this installation sees it. A foreign task is
// errForeignTask.
func (m *manager) read(s *scheduler) (state, error) {
	source := m.copy.source()
	text, found, err := s.taskXML(m.taskName)
	if err != nil {
		return state{}, err
	}
	var st state
	if found {
		if st, err = inspectTask(text, m.identity, m.sid, m.copy.exe, source, lookupAccount); err != nil {
			return state{}, err
		}
	}
	return withCopy(st, source, m.identity), nil
}

// readOwn is read with a foreign task as absent: it is not this program's
// startup, never runs this installation's copy as far as cleanup is
// concerned, and is never touched. The copy lives below Program Files, so
// removing it cannot let an ordinary process plant a file where such a task
// points.
func (m *manager) readOwn(s *scheduler) (st state, foreign bool, err error) {
	st, err = m.read(s)
	if errors.Is(err, errForeignTask) {
		return withCopy(state{}, m.copy.source(), m.identity), true, nil
	}
	return st, false, err
}

func (m *manager) gather(s *scheduler, forSync bool) (facts, error) {
	f := facts{elevated: m.elevated, runningFromCopy: !winapp.SamePath(m.image, m.identity)}
	// A protected copy never synchronizes, so it reads nothing.
	if forSync && f.runningFromCopy {
		return f, nil
	}
	var err error
	if f.task, f.foreign, err = m.readOwn(s); err != nil {
		return f, err
	}
	f.copyOwned = m.copy.ownedBy(m.identity)
	// Only synchronization compares file contents. Removal must never
	// depend on reading files beside the user-writable installation, so
	// uninstall and the in-app switch always have a way out.
	f.copyMatches = forSync && f.task.exists && f.task.thisCopy && f.task.protected && m.copy.matches(m.image)
	return f, nil
}

func (m *manager) execute(s *scheduler, p plan) error {
	if p.refusal != "" {
		return errors.New(p.refusal)
	}
	if p.deleteTask {
		if err := m.deleteTask(s); err != nil {
			return err
		}
	}
	stopped := 0
	if p.installCopy {
		var err error
		if stopped, err = m.copy.install(m.image, m.identity); err != nil {
			return err
		}
	}
	if p.registerTask {
		if err := m.register(s); err != nil {
			return err
		}
	}
	if p.restartIfStopped && stopped > 0 {
		// Best effort: otherwise the instance returns at the next sign-in.
		_ = s.run(m.taskName)
	}
	if p.removeCopy {
		return m.copy.remove()
	}
	return nil
}

// deleteTask deletes this installation's task and reads it back.
func (m *manager) deleteTask(s *scheduler) error {
	if err := s.delete(m.taskName); err != nil {
		return err
	}
	after, _, err := m.readOwn(s)
	if err != nil {
		return err
	}
	if after.thisCopy {
		return errDeleteVerify
	}
	return nil
}

// register writes the task for the protected copy, which must already hold
// this installation's files, and reads it back.
func (m *manager) register(s *scheduler) error {
	text, err := definition(m.copy.exe, m.sid).marshal()
	if err != nil {
		return err
	}
	if err := s.register(m.taskName, text, m.sid, taskSecurity(m.sid)); err != nil {
		return err
	}
	after, err := m.read(s)
	if err != nil {
		return err
	}
	if !after.thisCopy || !after.enabled || !after.protected {
		return errRegisterVerify
	}
	return nil
}

// mutate runs a change under the per-user startup lock, so two copies never
// change startup at once.
func (m *manager) mutate(change func(*scheduler) error) error {
	return withScheduler(func(s *scheduler) error {
		held, err := winapp.WithLock("PowerMeter.Autostart."+m.taskName, 5*time.Second, func() error { return change(s) })
		if err == nil && !held {
			return errBusy
		}
		return err
	})
}

// Read is the task as this installation sees it.
func (m *manager) Read() (st state, err error) {
	err = withScheduler(func(s *scheduler) error {
		st, err = m.read(s)
		return err
	})
	return st, err
}

// Enable registers this installation's startup if it still is what the
// user confirmed (expected, from Read).
func (m *manager) Enable(expected state) error {
	return m.mutate(func(s *scheduler) error {
		// Refuse before reading when not elevated, as the window does.
		var actual *state
		if m.elevated {
			st, err := m.read(s)
			if err != nil {
				return err
			}
			actual = &st
		}
		return m.execute(s, enable(m.elevated, &expected, actual))
	})
}

// Disable deletes this installation's task and, when elevated, its
// protected copy. A foreign task is left unchanged and reported.
func (m *manager) Disable() (r removal, err error) {
	err = m.mutate(func(s *scheduler) error {
		f, err := m.gather(s, false)
		if err != nil {
			return err
		}
		p := disable(f)
		r = p.removal
		return m.execute(s, p)
	})
	return r, err
}

// Synchronize brings the protected copy in line with this installation. A
// protected copy never synchronizes, because its source is user-writable.
func (m *manager) Synchronize(apply bool) (r syncResult, err error) {
	err = m.mutate(func(s *scheduler) error {
		f, err := m.gather(s, true)
		if err != nil {
			return err
		}
		p := synchronize(f, apply)
		r = p.sync
		return m.execute(s, p)
	})
	return r, err
}

// Describe is the switch state. It reads only the task and the copy's
// recorded owner, because the window refreshes it on every activation.
func (m *manager) Describe() (Status, error) {
	var status Status
	err := withScheduler(func(s *scheduler) error {
		st, foreign, err := m.readOwn(s)
		status = describe(st, foreign, m.taskName)
		return err
	})
	return status, err
}

// RunSync is --sync-autostart. Elevated, it applies and returns 0. Under an
// ordinary token it only reports what an elevated run would change: 3 an
// outdated copy, 4 a task that runs a user-writable file, 5 an unused copy.
// 1 is a failure. It never asks for UAC.
func RunSync() int {
	m, err := forCurrentProcess()
	if err != nil {
		return 1
	}
	r, err := m.Synchronize(m.elevated)
	if err != nil {
		return 1
	}
	return int(r)
}

// RunRemove is --remove-autostart, for the uninstaller: 0 removed or
// absent; 3 the task is gone but deleting the protected copy needs an
// elevated rerun; 6 removed, but a same-name task that is not this
// program's was left unchanged; 1 a failure. It never asks for UAC.
func RunRemove() int {
	m, err := forCurrentProcess()
	if err != nil {
		return 1
	}
	r, err := m.Disable()
	if err != nil {
		return 1
	}
	return int(r)
}

// SyncOnStart is for an elevated GUI launch: it migrates a task that runs
// a user-writable file, refreshes this installation's protected copy and
// removes an unused one. Failures leave the state for the settings panel
// to report.
func SyncOnStart() {
	if m, err := forCurrentProcess(); err == nil && m.elevated {
		_, _ = m.Synchronize(true)
	}
}

// ReadStatus is the switch for the settings panel and the tray menu. It
// never fails: a read failure is SwitchUnknown with the reason in Notice.
func ReadStatus() Status {
	m, err := forCurrentProcess()
	if err == nil {
		var s Status
		if s, err = m.Describe(); err == nil {
			return s
		}
	}
	return readFailed(err.Error())
}

// Change turns this installation's startup on or off and returns the notice
// for the diagnostics area (a Chinese source string, shown through
// i18n.Diagnostic), or "" when the user declined a replacement. done is true
// when startup now stands as asked: the notice then confirms the change
// rather than reporting a problem.
//
// Enabling needs an elevated process; otherwise it returns the notice that
// asks for the "restart as administrator" button. When enabling would
// replace another installation's startup (its task, or the protected copy
// it recorded), Change first calls confirm with a localized title and
// question and goes on only on true. confirm runs on the calling goroutine;
// a nil confirm declines.
func Change(enable bool, confirm func(title, question string) bool) (notice string, done bool) {
	m, err := forCurrentProcess()
	if err != nil {
		return changeFailed + err.Error(), false
	}
	if !enable {
		r, err := m.Disable()
		if err != nil {
			return changeFailed + err.Error(), false
		}
		return outcome(false, r), true
	}
	return m.change(confirm)
}

func (m *manager) change(confirm func(title, question string) bool) (string, bool) {
	// Refuse before asking about replacement.
	if !m.elevated {
		return changeFailed + ElevationRequired, false
	}
	st, err := m.Read()
	if err != nil {
		return changeFailed + err.Error(), false
	}
	if other := replaces(st); other != "" && (confirm == nil || !confirm(confirmation(other, m.identity))) {
		return "", false
	}
	if err := m.Enable(st); err != nil {
		return changeFailed + err.Error(), false
	}
	return outcome(true, removed), true
}
