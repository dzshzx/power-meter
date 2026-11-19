package autostart

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

const (
	testSID       = "S-1-5-21-111-222-333-1001"
	installedPath = `C:\Test & 测试\Meter 1.2.1.exe`
	testRoot      = `C:\Program Files\Power Meter\autostart`
)

func protectedExe(sid string) string { return testRoot + `\` + sid + `\PowerMeter.exe` }

// registered is the XML of a task in the registered shape, after edit.
func registered(t *testing.T, target string, edit func(*taskDefinition)) string {
	t.Helper()
	d := definition(target, testSID)
	if edit != nil {
		edit(&d)
	}
	text, err := d.marshal()
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// inspect reads a task as if this user's protected copy was taken from
// identity.
func inspect(t *testing.T, text, identity string) state {
	t.Helper()
	s, err := inspectTask(text, identity, testSID, protectedExe(testSID), identity, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRegisteredShapeRoundtrips(t *testing.T) {
	text := registered(t, protectedExe(testSID), nil)
	s := inspect(t, text, installedPath)
	if !s.exists || !s.enabled || !s.thisCopy || !s.protected || s.executable != installedPath ||
		s.target != protectedExe(testSID) || s.repairReason != "" || s.registration != text {
		t.Errorf("logon task roundtrips the protected copy path and elevated interactive user policy: %+v", s)
	}
	d, err := parseTask(text)
	if err != nil {
		t.Fatal(err)
	}
	exec := d.Actions.Exec[0]
	if exec.WorkingDirectory != testRoot+`\`+testSID || exec.Arguments != "--autostart" ||
		*d.Settings.DisallowStartIfOnBatteries || *d.Settings.StopIfGoingOnBatteries ||
		*d.Settings.MultipleInstancesPolicy != "IgnoreNew" ||
		!strings.Contains(text, "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>") ||
		!strings.Contains(text, `xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task"`) {
		t.Errorf("registered settings: %+v\n%s", d, text)
	}
	other, _ := inspectTask(text, `C:\Other\Meter.exe`, testSID, protectedExe(testSID), installedPath, nil)
	noSource, _ := inspectTask(text, installedPath, testSID, protectedExe(testSID), "", nil)
	if other.thisCopy || noSource.thisCopy || other.executable != installedPath || noSource.executable != protectedExe(testSID) {
		t.Errorf("other copies are distinguished by the protected copy's recorded source: %+v %+v", other, noSource)
	}
}

// The XML special characters in a path survive the round trip.
func TestEscapedPathsRoundtrip(t *testing.T) {
	path := `C:\A & <B> "C" 'D'\测试\PowerMeter.exe`
	text := registered(t, path, nil)
	if !strings.Contains(text, "&amp;") || !strings.Contains(text, "&lt;B&gt;") {
		t.Errorf("path is escaped: %s", text)
	}
	if s := inspect(t, text, path); s.target != path || !s.thisCopy || s.protected {
		t.Errorf("escaped path reads back: %+v", s)
	}
}

// What the scheduler stores is UTF-16 text declared as such.
func TestSchedulerDeclarationIsRead(t *testing.T) {
	text := `<?xml version="1.0" encoding="UTF-16"?>` + "\r\n" + registered(t, protectedExe(testSID), nil)
	if s := inspect(t, text, installedPath); !s.enabled {
		t.Errorf("declared UTF-16 text reads: %+v", s)
	}
	if _, err := inspectTask("<Task><Settings>", installedPath, testSID, protectedExe(testSID), installedPath, nil); err == nil || errors.Is(err, errForeignTask) {
		t.Errorf("malformed XML is a read failure, not a foreign task: %v", err)
	}
}

func TestUnprotectedTaskNeedsMigration(t *testing.T) {
	s := inspect(t, registered(t, installedPath, nil), installedPath)
	if !s.exists || !s.thisCopy || s.protected || s.enabled || s.repairReason == "" || s.executable != installedPath {
		t.Errorf("elevated task running a user-writable file is never reported as working startup: %+v", s)
	}
	other := inspect(t, registered(t, `C:\Other\PowerMeter.exe`, nil), installedPath)
	if other.thisCopy || other.executable != `C:\Other\PowerMeter.exe` {
		t.Errorf("an unprotected task of another path is another installation's: %+v", other)
	}
}

func TestDisabledTaskReadsDisabled(t *testing.T) {
	text := registered(t, protectedExe(testSID), func(d *taskDefinition) { d.Settings.Enabled = ptr(false) })
	if inspect(t, text, installedPath).enabled {
		t.Error("disabled task is read as disabled")
	}
}

// Each shape this program never registers is a foreign task (a distinct
// error, so removal can leave it and still finish).
func TestForeignShapeIsNeverAdopted(t *testing.T) {
	edits := map[string]func(*taskDefinition){
		"source":       func(d *taskDefinition) { d.RegistrationInfo.Source = "someone-else" },
		"arguments":    func(d *taskDefinition) { d.Actions.Exec[0].Arguments = "--other" },
		"no arguments": func(d *taskDefinition) { d.Actions.Exec[0].Arguments = "" },
		"second action": func(d *taskDefinition) {
			d.Actions.Exec = append(d.Actions.Exec, taskExec{Command: `C:\Other\Tool.exe`})
		},
		"other action": func(d *taskDefinition) { d.Actions.Other = []anyElement{{XMLName: xml.Name{Local: "ShowMessage"}}} },
		"principal":    func(d *taskDefinition) { d.Principals.Principal[0].UserID = ptr("S-1-5-21-111-222-333-1002") },
		"group":        func(d *taskDefinition) { d.Principals.Principal[0].UserID = nil },
		"no principal": func(d *taskDefinition) { d.Principals.Principal = nil },
	}
	for name, edit := range edits {
		text := registered(t, protectedExe(testSID), edit)
		if _, err := inspectTask(text, installedPath, testSID, protectedExe(testSID), installedPath, nil); !errors.Is(err, errForeignTask) {
			t.Errorf("%s: %v, want a foreign task", name, err)
		}
	}
}

func TestAccountNamesNormalizeToTheUser(t *testing.T) {
	account := func(name string) string {
		if strings.EqualFold(name, `PC\Me`) {
			return testSID
		}
		return ""
	}
	named := registered(t, protectedExe(testSID), func(d *taskDefinition) {
		d.Principals.Principal[0].UserID = ptr(`PC\Me`)
		d.Triggers.Logon[0].UserID = ptr(`pc\me`)
	})
	s, err := inspectTask(named, installedPath, testSID, protectedExe(testSID), installedPath, account)
	if err != nil || !s.enabled {
		t.Errorf("scheduler account-name normalization preserves exact user identity: %+v %v", s, err)
	}
	if _, err := inspectTask(named, installedPath, testSID, protectedExe(testSID), installedPath, nil); !errors.Is(err, errForeignTask) {
		t.Errorf("an account name that does not map is not this user: %v", err)
	}
	otherLogon := registered(t, protectedExe(testSID), func(d *taskDefinition) { d.Triggers.Logon[0].UserID = ptr("S-1-5-21-111-222-333-1002") })
	if s, err := inspectTask(otherLogon, installedPath, testSID, protectedExe(testSID), installedPath, account); err != nil || s.enabled || s.repairReason == "" {
		t.Errorf("different logon account cannot be treated as current-user startup: %+v %v", s, err)
	}
}

func TestChangedPolicyNeedsRepair(t *testing.T) {
	edits := map[string]func(*taskDefinition){
		"battery start": func(d *taskDefinition) { d.Settings.DisallowStartIfOnBatteries = ptr(true) },
		"battery stop":  func(d *taskDefinition) { d.Settings.StopIfGoingOnBatteries = ptr(true) },
		"idle":          func(d *taskDefinition) { d.Settings.RunOnlyIfIdle = ptr(true) },
		"network":       func(d *taskDefinition) { d.Settings.RunOnlyIfNetworkAvailable = ptr(true) },
		"time limit":    func(d *taskDefinition) { d.Settings.ExecutionTimeLimit = ptr("PT1H") },
		"instances":     func(d *taskDefinition) { d.Settings.MultipleInstancesPolicy = ptr("Parallel") },
		"run level":     func(d *taskDefinition) { d.Principals.Principal[0].RunLevel = ptr("LeastPrivilege") },
		"logon type":    func(d *taskDefinition) { d.Principals.Principal[0].LogonType = ptr("Password") },
		"trigger":       func(d *taskDefinition) { d.Triggers.Logon[0].Enabled = ptr(false) },
		"no trigger":    func(d *taskDefinition) { d.Triggers.Logon = nil },
		"any user":      func(d *taskDefinition) { d.Triggers.Logon[0].UserID = nil },
		// Missing elements take the schema defaults.
		"default battery start": func(d *taskDefinition) { d.Settings.DisallowStartIfOnBatteries = nil },
		"default battery stop":  func(d *taskDefinition) { d.Settings.StopIfGoingOnBatteries = nil },
		"default time limit":    func(d *taskDefinition) { d.Settings.ExecutionTimeLimit = nil },
		"default run level":     func(d *taskDefinition) { d.Principals.Principal[0].RunLevel = nil },
	}
	for name, edit := range edits {
		s := inspect(t, registered(t, protectedExe(testSID), edit), installedPath)
		if s.enabled || s.repairReason == "" || !s.thisCopy {
			t.Errorf("changed task policy requires explicit repair: %s: %+v", name, s)
		}
	}
	defaults := registered(t, protectedExe(testSID), func(d *taskDefinition) {
		d.Settings.Enabled, d.Settings.RunOnlyIfIdle, d.Settings.RunOnlyIfNetworkAvailable = nil, nil, nil
		d.Settings.MultipleInstancesPolicy, d.Triggers.Logon[0].Enabled = nil, nil
	})
	if s := inspect(t, defaults, installedPath); !s.enabled {
		t.Errorf("elements whose defaults are the registered values may be missing: %+v", s)
	}
}

func TestZeroDuration(t *testing.T) {
	for text, want := range map[string]bool{
		"PT0S": true, "P0D": true, "PT0H0M0S": true, " PT0.000S ": true, "P0DT0S": true,
		"PT72H": false, "PT1S": false, "P": false, "PT": false, "": false, "0": false, "-PT0S": false, "PT0X": false,
	} {
		if zeroDuration(text) != want {
			t.Errorf("zeroDuration(%q) != %v", text, want)
		}
	}
}

func TestStaleConfirmationIsRejected(t *testing.T) {
	confirmed := inspect(t, registered(t, protectedExe(testSID), nil), installedPath)
	repointed := inspect(t, registered(t, `C:\Third copy\Meter.exe`, nil), installedPath)
	reowned, _ := inspectTask(registered(t, protectedExe(testSID), nil), installedPath, testSID, protectedExe(testSID), `C:\Third copy\Meter.exe`, nil)
	same := inspect(t, registered(t, protectedExe(testSID), nil), installedPath)
	if unchanged(&confirmed, &repointed) || unchanged(&confirmed, &reowned) || !unchanged(&confirmed, &same) {
		t.Error("a confirmation covers the task definition and the copy's recorded owner")
	}
	if p := enable(true, &confirmed, &reowned); p.refusal != changedElsewhere {
		t.Errorf("a stale confirmation is refused: %+v", p)
	}
}

func TestWithCopy(t *testing.T) {
	if s := withCopy(state{}, `C:\Old\PowerMeter.exe`, installedPath); s.otherCopy != `C:\Old\PowerMeter.exe` {
		t.Errorf("another installation's copy is recorded: %+v", s)
	}
	if s := withCopy(state{}, strings.ToUpper(installedPath), installedPath); s.otherCopy != "" {
		t.Errorf("this installation's copy is not another's: %+v", s)
	}
	if s := withCopy(state{}, "", installedPath); s.otherCopy != "" {
		t.Errorf("no copy: %+v", s)
	}
}

func TestTaskNames(t *testing.T) {
	if TaskName(testSID) != "PowerMeter.Logon."+testSID || taskSecurity(testSID) != "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGXSD;;;"+testSID+")" {
		t.Error("task name and security descriptor use the new names")
	}
}
