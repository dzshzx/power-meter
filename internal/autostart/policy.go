package autostart

import (
	"strings"

	"github.com/dzshzx/power-meter/internal/i18n"
)

// state is what a read of the logon task and this user's protected copy
// found. A foreign same-name task reads as absent.
type state struct {
	exists   bool
	enabled  bool
	thisCopy bool
	// protected: the task runs this user's admin-only protected copy.
	protected bool
	// executable is the installation that owns startup (the protected
	// copy's source); target is the file the task actually executes.
	executable   string
	target       string
	registration string
	repairReason string
	// otherCopy is the installation recorded in this user's protected copy
	// when that is not this installation. Enabling overwrites that copy,
	// which another installation's task may still run.
	otherCopy string
}

// syncResult is what --sync-autostart reports; the values are its exit
// codes.
type syncResult int

const (
	syncCurrent syncResult = 0
	// syncStaleCopy: this installation's task runs an outdated protected copy.
	syncStaleCopy syncResult = 3
	// syncOrphanCopy: no task runs this installation's protected copy.
	syncOrphanCopy syncResult = 5
)

// removal is what --remove-autostart reports; the values are its exit
// codes (1 is a failure, 2 invalid arguments).
type removal int

const (
	removed removal = 0
	// removalCopyNeedsElevation: this installation's task is gone, but
	// deleting its protected copy needs an elevated rerun.
	removalCopyNeedsElevation removal = 3
	// removalForeignTaskKept: this installation's startup is removed; a
	// same-name task that fails the ownership check was left unchanged for
	// the user to delete.
	removalForeignTaskKept removal = 6
)

// facts is what the manager read before deciding.
type facts struct {
	task    state
	foreign bool
	// runningFromCopy: this process is itself the protected copy, whose
	// source is user-writable, so it never synchronizes from it.
	runningFromCopy bool
	// copyOwned: the copy's recorded source is this installation.
	copyOwned bool
	// copyMatches: the copy holds exactly this installation's files.
	copyMatches bool
	elevated    bool
}

// plan is a decision, carried out by the manager in this order: delete the
// task, install the copy, register the task, restart stopped instances,
// remove the copy. A refusal performs nothing.
type plan struct {
	sync         syncResult
	removal      removal
	deleteTask   bool
	installCopy  bool
	registerTask bool
	// restartIfStopped: restart logon instances the copy installation stopped.
	restartIfStopped bool
	removeCopy       bool
	refusal          string
}

// Status is how the settings panel and the tray menu present startup: On
// checks the switch. Notice is a Chinese source string for the diagnostics
// area, shown through i18n.Diagnostic; Tooltip is already in the current
// language.
type Status struct {
	On      bool
	Notice  string
	Tooltip string
}

// Refusals and notices: Chinese source strings, shown through
// i18n.Diagnostic.
const (
	ElevationRequired        = "请先点击下方“以管理员身份重新启动”，再勾选开机自启。"
	elevationRequiredForCopy = "写入或删除自启副本需要管理员权限。"
	changedElsewhere         = "自启任务已被另一份程序修改，请重新读取并确认后再试。"
	// foreignTaskNotice is a lookup key ending in {0}; the task name
	// follows verbatim.
	foreignTaskNotice = "同名自启任务不属于本程序，已保留未改；如不再需要，请在任务计划程序库中手工删除："
	readFailedNotice  = "自启状态读取失败："
	changeFailed      = "自启设置未完成："
)

// synchronize refreshes an outdated copy and removes a copy no task uses.
// Without apply it only reports what an elevated run would change.
func synchronize(f facts, apply bool) plan {
	var p plan
	if f.runningFromCopy {
		return p
	}
	t := f.task
	runsCopy := t.exists && t.thisCopy && t.protected
	var needed syncResult
	switch {
	case runsCopy && f.copyMatches:
		needed = syncCurrent
	case runsCopy:
		needed = syncStaleCopy
	case f.copyOwned:
		needed = syncOrphanCopy
	default:
		needed = syncCurrent
	}
	p.sync = needed
	if !apply || needed == syncCurrent {
		return p
	}
	if !f.elevated {
		p.refusal = elevationRequiredForCopy
		return p
	}
	p.sync = syncCurrent
	if needed == syncStaleCopy {
		p.installCopy, p.restartIfStopped = true, t.enabled
	} else {
		p.removeCopy = true
	}
	return p
}

// disable deletes this installation's task and, when elevated, its copy. A
// foreign task is left unchanged and reported; removalCopyNeedsElevation
// takes precedence because the elevated rerun reports the foreign task
// again.
func disable(f facts) plan {
	p := plan{deleteTask: f.task.exists && f.task.thisCopy, removal: removed}
	if f.foreign {
		p.removal = removalForeignTaskKept
	}
	switch {
	case !f.copyOwned:
	case !f.elevated:
		p.removal = removalCopyNeedsElevation
	default:
		p.removeCopy = true
	}
	return p
}

// enable registers this installation's startup. Every protected task has the
// same action, so the owner recorded with the copy is part of what the user
// confirmed; actual is nil when the manager did not read (not elevated).
func enable(elevated bool, expected, actual *state) plan {
	switch {
	case !elevated:
		return plan{refusal: ElevationRequired}
	case !unchanged(expected, actual):
		return plan{refusal: changedElsewhere}
	}
	return plan{installCopy: true, registerTask: true, restartIfStopped: true}
}

// unchanged reports whether startup still is what the user confirmed: the
// same task definition, owner and protected-copy owner.
func unchanged(expected, actual *state) bool {
	return expected != nil && actual != nil && expected.exists == actual.exists &&
		expected.registration == actual.registration &&
		strings.EqualFold(expected.executable, actual.executable) &&
		strings.EqualFold(expected.otherCopy, actual.otherCopy)
}

// replaces is the other installation whose startup enabling would replace:
// the owner of a same-name task of another installation, or the source of a
// protected copy this installation does not own. "" means enabling needs no
// confirmation.
func replaces(s state) string {
	if s.exists && !s.thisCopy {
		return s.executable
	}
	return s.otherCopy
}

// confirmation is the question the window asks before enabling replaces
// another installation's startup, in the current language.
func confirmation(other, identity string) (title, question string) {
	return i18n.Get("更换自启程序"), i18n.Format("自启当前指向：\n{0}\n\n更换为：\n{1}？", other, identity)
}

// describe is the switch for a read state. A foreign task shows as off,
// is left alone, and the notice says where to delete it.
func describe(task state, foreign bool, taskName string) Status {
	s := Status{On: task.thisCopy && task.enabled}
	switch {
	case foreign:
		s.Notice = foreignTaskNotice + taskName
	case task.thisCopy && task.repairReason != "":
		s.Notice = task.repairReason
	}
	if other := replaces(task); other != "" {
		s.Tooltip = i18n.Format("当前自启指向：{0}；勾选可确认更换。", other)
	} else {
		s.Tooltip = i18n.Get("以当前 Windows 用户登录后，从仅管理员可写的受保护副本以管理员权限运行并留在托盘。移动程序后需重新启用。")
	}
	return s
}

// readFailed is the switch when startup could not be read: off, with the
// reason. Changing it reads startup again first.
func readFailed(reason string) Status {
	return Status{Notice: readFailedNotice + reason}
}

// outcome is the notice after the in-app switch changed startup.
func outcome(enabled bool, r removal) string {
	switch {
	case enabled:
		return "已启用开机自启：登录后以管理员权限运行受保护副本，在托盘显示。"
	case r == removalCopyNeedsElevation:
		return "已关闭此程序的开机自启；受保护副本将在下次以管理员身份启动时删除。"
	}
	return "已关闭此程序的开机自启。"
}
