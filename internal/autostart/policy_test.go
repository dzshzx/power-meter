package autostart

import (
	"strings"
	"testing"

	"github.com/dzshzx/power-meter/internal/i18n"
)

// fact builds facts from the flags the decision table reads.
func fact(exists, thisCopy, isProtected, enabled, foreign, copyOwned, copyMatches, elevated bool) facts {
	return facts{
		task:        state{exists: exists, thisCopy: thisCopy, protected: isProtected, enabled: enabled},
		foreign:     foreign,
		copyOwned:   copyOwned,
		copyMatches: copyMatches,
		elevated:    elevated,
	}
}

// steps names what a plan does, in execution order.
func steps(p plan) string {
	var s []string
	for _, step := range []struct {
		on   bool
		name string
	}{
		{p.refusal != "", "refuse"},
		{p.deleteTask, "delete"},
		{p.installCopy, "install"},
		{p.registerTask, "register"},
		{p.restartIfStopped, "restart"},
		{p.removeCopy, "remove"},
	} {
		if step.on {
			s = append(s, step.name)
		}
	}
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, "+")
}

func TestSynchronize(t *testing.T) {
	fromCopy := fact(true, true, false, true, false, true, false, true)
	fromCopy.runningFromCopy = true
	unprotected := fact(true, true, false, true, false, false, false, true)
	unprotectedOrdinary := unprotected
	unprotectedOrdinary.elevated = false
	cases := []struct {
		name  string
		facts facts
		apply bool
		sync  syncResult
		steps string
	}{
		{"protected copy process never synchronizes", fromCopy, true, syncCurrent, "none"},
		{"report-only sync names an unprotected task without acting", unprotected, false, syncUnprotected, "none"},
		{"elevated sync migrates an unprotected task to the protected copy", unprotected, true, syncCurrent, "install+register+restart"},
		{"sync without elevation refuses to write the copy", unprotectedOrdinary, true, syncUnprotected, "refuse"},
		{"matching protected copy is current", fact(true, true, true, true, false, true, true, true), true, syncCurrent, "none"},
		{"report-only sync names an outdated copy", fact(true, true, true, false, false, true, false, false), false, syncStaleCopy, "none"},
		{"outdated copy of disabled startup is refreshed without restart", fact(true, true, true, false, false, true, false, true), true, syncCurrent, "install"},
		{"outdated copy of enabled startup is refreshed and restarted", fact(true, true, true, true, false, true, false, true), true, syncCurrent, "install+restart"},
		{"report-only sync names an unused copy", fact(false, false, false, false, false, true, false, false), false, syncOrphanCopy, "none"},
		{"elevated sync removes an unused copy", fact(false, false, false, false, false, true, false, true), true, syncCurrent, "remove"},
		{"an owned copy behind a foreign task is unused", fact(false, false, false, false, true, true, false, false), false, syncOrphanCopy, "none"},
		{"elevated sync removes the copy behind a foreign task", fact(false, false, false, false, true, true, false, true), true, syncCurrent, "remove"},
		{"ordinary sync succeeds when current", fact(true, true, true, true, false, true, true, false), true, syncCurrent, "none"},
		{"ordinary sync refuses to refresh", fact(true, true, true, true, false, true, false, false), true, syncStaleCopy, "refuse"},
		{"ordinary sync refuses to remove", fact(false, false, false, false, false, true, false, false), true, syncOrphanCopy, "refuse"},
		{"nothing to do", fact(false, false, false, false, false, false, false, false), true, syncCurrent, "none"},
		{"another installation's startup is left alone", fact(true, false, true, true, false, false, false, true), true, syncCurrent, "none"},
	}
	for _, c := range cases {
		p := synchronize(c.facts, c.apply)
		if p.sync != c.sync || steps(p) != c.steps {
			t.Errorf("%s: sync %d steps %s, want %d %s", c.name, p.sync, steps(p), c.sync, c.steps)
		}
		if steps(p) == "refuse" && p.refusal != elevationRequiredForCopy {
			t.Errorf("%s: refusal %q", c.name, p.refusal)
		}
	}
}

func TestDisable(t *testing.T) {
	cases := []struct {
		name    string
		facts   facts
		removal removal
		steps   string
	}{
		{"elevated removal deletes the task and the copy", fact(true, true, true, true, false, true, true, true), removed, "delete+remove"},
		{"ordinary removal deletes the task and defers the copy", fact(true, true, true, true, false, true, true, false), removalCopyNeedsElevation, "delete"},
		{"removal keeps a foreign task and still removes the copy", fact(false, false, false, false, true, true, false, true), removalForeignTaskKept, "remove"},
		{"elevation takes precedence over a foreign task", fact(false, false, false, false, true, true, false, false), removalCopyNeedsElevation, "none"},
		{"a foreign task without a copy", fact(false, false, false, false, true, false, false, false), removalForeignTaskKept, "none"},
		{"removal never deletes another installation's task", fact(true, false, true, true, false, false, false, false), removed, "none"},
		{"ordinary removal disarms an unprotected task without a copy", fact(true, true, false, true, false, false, false, false), removed, "delete"},
	}
	for _, c := range cases {
		p := disable(c.facts)
		if p.removal != c.removal || steps(p) != c.steps {
			t.Errorf("%s: removal %d steps %s, want %d %s", c.name, p.removal, steps(p), c.removal, c.steps)
		}
	}
}

func TestEnable(t *testing.T) {
	absent := state{}
	other := state{exists: true, registration: "<Task/>", executable: `C:\Other\PowerMeter.exe`}
	otherCopy := state{otherCopy: `C:\Other\PowerMeter.exe`}
	movedCopy := state{otherCopy: `C:\Third\PowerMeter.exe`}
	cases := []struct {
		name             string
		elevated         bool
		expected, actual *state
		want             string
	}{
		{"enabling requires elevation", false, &absent, nil, "refuse"},
		{"a changed task invalidates the confirmation", true, &absent, &other, "refuse"},
		{"no confirmation, no change", true, nil, &absent, "refuse"},
		{"a changed copy owner invalidates the confirmation", true, &otherCopy, &movedCopy, "refuse"},
		{"a confirmed replacement installs and registers", true, &other, &other, "install+register+restart"},
		{"a confirmed copy replacement installs and registers", true, &otherCopy, &otherCopy, "install+register+restart"},
	}
	for _, c := range cases {
		if got := steps(enable(c.elevated, c.expected, c.actual)); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	if enable(false, &absent, nil).refusal != ElevationRequired || enable(true, &absent, &other).refusal != changedElsewhere {
		t.Error("refusals say why")
	}
}

func TestReplaces(t *testing.T) {
	cases := []struct {
		name  string
		state state
		want  string
	}{
		{"nothing to replace", state{}, ""},
		{"own startup", state{exists: true, thisCopy: true, executable: `C:\Me.exe`}, ""},
		{"another installation's task", state{exists: true, executable: `C:\Other.exe`}, `C:\Other.exe`},
		{"another installation's copy without a task of this name", state{otherCopy: `C:\Old\PowerMeter.exe`}, `C:\Old\PowerMeter.exe`},
		{"an own unprotected task over another installation's copy", state{exists: true, thisCopy: true, otherCopy: `C:\Old.exe`}, `C:\Old.exe`},
	}
	for _, c := range cases {
		if got := replaces(c.state); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDescribe(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Select(i18n.System) })
	_ = i18n.Select(i18n.Chinese)
	absent := state{}
	other := state{exists: true, registration: "<Task/>", executable: `C:\Other\PowerMeter.exe`}
	if s := describe(absent, true, "PowerMeter.Logon.S-1"); s.Switch != SwitchOff || s.Notice != foreignTaskNotice+"PowerMeter.Logon.S-1" {
		t.Errorf("switch shows a foreign task as off with where to delete it: %+v", s)
	}
	working := state{exists: true, thisCopy: true, protected: true, enabled: true}
	repair := state{exists: true, thisCopy: true, repairReason: "repair"}
	enabledRepair := state{exists: true, thisCopy: true, enabled: true, repairReason: "repair"}
	if s := describe(working, false, "t"); s.Switch != SwitchOn || s.Notice != "" {
		t.Errorf("switch reflects working startup: %+v", s)
	}
	if s := describe(repair, false, "t"); s.Switch != SwitchOff || s.Notice != "repair" {
		t.Errorf("switch asks for repair: %+v", s)
	}
	if s := describe(enabledRepair, false, "t"); s.Switch != SwitchOn || s.Notice != "repair" {
		t.Errorf("enabled startup still asks for repair: %+v", s)
	}
	if s := describe(other, false, "t"); !strings.Contains(s.Tooltip, other.executable) || s.Switch != SwitchOff {
		t.Errorf("switch names another installation's startup: %+v", s)
	}
	if s := describe(state{otherCopy: `C:\Old\PowerMeter.exe`}, false, "t"); !strings.Contains(s.Tooltip, `C:\Old\PowerMeter.exe`) {
		t.Errorf("switch names another installation's copy: %+v", s)
	}
	if s := describe(absent, false, "t"); s.Switch != SwitchOff || s.Notice != "" || strings.Contains(s.Tooltip, "：") {
		t.Errorf("absent startup is off with the plain tooltip: %+v", s)
	}
	if s := readFailed("x"); s.Switch != SwitchUnknown || s.Tooltip != "" || s.Notice != "自启状态读取失败：x" {
		t.Errorf("read failures are unknown with the reason: %+v", s)
	}
}

func TestOutcome(t *testing.T) {
	if !strings.Contains(outcome(false, removalCopyNeedsElevation), "下次以管理员身份启动") ||
		outcome(false, removalForeignTaskKept) != outcome(false, removed) ||
		outcome(true, removed) == outcome(false, removed) {
		t.Error("switch outcome names deferred copy removal")
	}
}

// Every notice the package shows is in the language table, and a foreign
// task's notice translates with the task name kept verbatim.
func TestNoticesTranslate(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Select(i18n.System) })
	_ = i18n.Select(i18n.English)
	texts := []string{
		ElevationRequired, elevationRequiredForCopy, changedElsewhere,
		outcome(true, removed), outcome(false, removed), outcome(false, removalCopyNeedsElevation),
		errForeignTask.Error(), errBusy.Error(),
		repairReason(false, true, true), repairReason(true, true, false),
	}
	for _, text := range texts {
		if i18n.Diagnostic(text) == text {
			t.Errorf("no English text for %q", text)
		}
	}
	name := "PowerMeter.Logon.S-1-5-21-1"
	for _, text := range []string{describe(state{}, true, name).Notice, readFailedNotice + name, changeFailed + name} {
		if got := i18n.Diagnostic(text); got == text || !strings.HasSuffix(got, name) {
			t.Errorf("Diagnostic(%q) = %q", text, got)
		}
	}
	title, question := confirmation(`C:\A\PowerMeter.exe`, `D:\B\PowerMeter.exe`)
	if title != "Replace startup application" || !strings.Contains(question, `C:\A\PowerMeter.exe`) || !strings.Contains(question, `D:\B\PowerMeter.exe`) {
		t.Errorf("confirmation: %q %q", title, question)
	}
	if s := describe(state{}, false, name); s.Tooltip != i18n.Get("以当前 Windows 用户登录后，从仅管理员可写的受保护副本以管理员权限运行并留在托盘。移动程序后需重新启用。") || !strings.HasPrefix(s.Tooltip, "Run elevated") {
		t.Errorf("tooltip follows the language: %q", s.Tooltip)
	}
}
