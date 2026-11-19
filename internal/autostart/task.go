package autostart

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/dzshzx/power-meter/internal/winapp"
)

const (
	taskPrefix = "PowerMeter.Logon."
	// taskOwner marks the one task shape this program registers.
	taskOwner     = "PowerMeter.Logon.v1"
	taskArguments = "--autostart"
	// taskSecurityPrefix: full control for SYSTEM and administrators; the
	// same user's ordinary token may read, run and delete the task
	// (including per-user uninstall), but cannot rewrite its elevated
	// action. The protected DACL suppresses the scheduler's automatic extra
	// principal ACE. The action itself lives in an admin-only folder.
	taskSecurityPrefix = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGXSD;;;"
	taskNamespace      = "http://schemas.microsoft.com/windows/2004/02/mit/task"
)

// TaskName is the logon task of the user with this SID.
func TaskName(sid string) string { return taskPrefix + sid }

func taskSecurity(sid string) string { return taskSecurityPrefix + sid + ")" }

var (
	// errForeignTask: a task with this program's name whose structure is not
	// one this program registers (edited by the user or another program). It
	// is never modified; removal treats it as not this program's startup.
	errForeignTask = errors.New("同名任务不属于本程序，未修改。")
	// errBusy: another process holds the startup lock.
	errBusy = errors.New("另一份程序正在修改自启，请稍后重试。")
)

// taskDefinition is the Task Scheduler XML, as far as this program writes or
// reads it. Optional elements are pointers, so a missing one takes the
// schema default when read.
type taskDefinition struct {
	XMLName          xml.Name         `xml:"Task"`
	Version          string           `xml:"version,attr,omitempty"`
	Namespace        string           `xml:"xmlns,attr,omitempty"`
	RegistrationInfo taskRegistration `xml:"RegistrationInfo"`
	Triggers         taskTriggers     `xml:"Triggers"`
	Principals       taskPrincipals   `xml:"Principals"`
	Settings         taskSettings     `xml:"Settings"`
	Actions          taskActions      `xml:"Actions"`
}

type taskRegistration struct {
	Source string `xml:"Source,omitempty"`
}

type taskTriggers struct {
	Logon []logonTrigger `xml:"LogonTrigger"`
	Other []anyElement   `xml:",any"`
}

type logonTrigger struct {
	Enabled *bool   `xml:"Enabled,omitempty"`
	UserID  *string `xml:"UserId,omitempty"`
}

type taskPrincipals struct {
	Principal []taskPrincipal `xml:"Principal"`
}

type taskPrincipal struct {
	ID        string  `xml:"id,attr,omitempty"`
	UserID    *string `xml:"UserId,omitempty"`
	LogonType *string `xml:"LogonType,omitempty"`
	RunLevel  *string `xml:"RunLevel,omitempty"`
}

type taskSettings struct {
	MultipleInstancesPolicy    *string       `xml:"MultipleInstancesPolicy,omitempty"`
	DisallowStartIfOnBatteries *bool         `xml:"DisallowStartIfOnBatteries,omitempty"`
	StopIfGoingOnBatteries     *bool         `xml:"StopIfGoingOnBatteries,omitempty"`
	AllowHardTerminate         *bool         `xml:"AllowHardTerminate,omitempty"`
	StartWhenAvailable         *bool         `xml:"StartWhenAvailable,omitempty"`
	RunOnlyIfNetworkAvailable  *bool         `xml:"RunOnlyIfNetworkAvailable,omitempty"`
	IdleSettings               *idleSettings `xml:"IdleSettings,omitempty"`
	AllowStartOnDemand         *bool         `xml:"AllowStartOnDemand,omitempty"`
	Enabled                    *bool         `xml:"Enabled,omitempty"`
	Hidden                     *bool         `xml:"Hidden,omitempty"`
	RunOnlyIfIdle              *bool         `xml:"RunOnlyIfIdle,omitempty"`
	WakeToRun                  *bool         `xml:"WakeToRun,omitempty"`
	ExecutionTimeLimit         *string       `xml:"ExecutionTimeLimit,omitempty"`
	Priority                   *int          `xml:"Priority,omitempty"`
}

type idleSettings struct {
	StopOnIdleEnd *bool `xml:"StopOnIdleEnd,omitempty"`
	RestartOnIdle *bool `xml:"RestartOnIdle,omitempty"`
}

type taskActions struct {
	Context string       `xml:"Context,attr,omitempty"`
	Exec    []taskExec   `xml:"Exec"`
	Other   []anyElement `xml:",any"`
}

type taskExec struct {
	Command          string `xml:"Command"`
	Arguments        string `xml:"Arguments,omitempty"`
	WorkingDirectory string `xml:"WorkingDirectory,omitempty"`
}

// anyElement counts elements of kinds this program never writes.
type anyElement struct {
	XMLName xml.Name
}

func ptr[T any](v T) *T { return &v }

func valueOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// definition is the one task shape this program registers: a logon trigger
// and an elevated interactive principal for the user, battery-, idle- and
// network-independent, unlimited, single-instance, running target.
func definition(target, sid string) taskDefinition {
	dir, _ := winapp.SplitPath(target)
	return taskDefinition{
		Version:          "1.2",
		Namespace:        taskNamespace,
		RegistrationInfo: taskRegistration{Source: taskOwner},
		Triggers:         taskTriggers{Logon: []logonTrigger{{Enabled: ptr(true), UserID: ptr(sid)}}},
		Principals: taskPrincipals{Principal: []taskPrincipal{{
			ID: "User", UserID: ptr(sid), LogonType: ptr("InteractiveToken"), RunLevel: ptr("HighestAvailable"),
		}}},
		Settings: taskSettings{
			MultipleInstancesPolicy:    ptr("IgnoreNew"),
			DisallowStartIfOnBatteries: ptr(false),
			StopIfGoingOnBatteries:     ptr(false),
			AllowHardTerminate:         ptr(true),
			StartWhenAvailable:         ptr(true),
			RunOnlyIfNetworkAvailable:  ptr(false),
			IdleSettings:               &idleSettings{StopOnIdleEnd: ptr(false), RestartOnIdle: ptr(false)},
			AllowStartOnDemand:         ptr(true),
			Enabled:                    ptr(true),
			Hidden:                     ptr(false),
			RunOnlyIfIdle:              ptr(false),
			WakeToRun:                  ptr(false),
			ExecutionTimeLimit:         ptr("PT0S"),
			Priority:                   ptr(7),
		},
		Actions: taskActions{Context: "User", Exec: []taskExec{{Command: target, Arguments: taskArguments, WorkingDirectory: dir}}},
	}
}

func (d taskDefinition) marshal() (string, error) {
	b, err := xml.MarshalIndent(d, "", "  ")
	return string(b), err
}

func parseTask(text string) (taskDefinition, error) {
	var d taskDefinition
	dec := xml.NewDecoder(strings.NewReader(text))
	// The scheduler declares UTF-16, the encoding of the string it hands
	// over; here the text is already decoded.
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	err := dec.Decode(&d)
	return d, err
}

// accountSID maps an account name the scheduler wrote instead of a SID
// (DOMAIN\user) to its SID, or "" when it does not map.
type accountSID func(name string) string

func matchesUser(value, sid string, account accountSID) bool {
	if strings.EqualFold(value, sid) {
		return true
	}
	return strings.TrimSpace(value) != "" && account != nil && strings.EqualFold(account(value), sid)
}

// inspectTask reads a registered task for the installation identity,
// given this user's protected copy and its recorded source. A task in a
// shape this program never registers is errForeignTask.
func inspectTask(text, identity, sid, protectedExe, protectedSource string, account accountSID) (state, error) {
	d, err := parseTask(text)
	if err != nil {
		return state{}, err
	}
	exec, ok := ownShape(d, sid, account)
	if !ok {
		return state{}, errForeignTask
	}
	target := exec.Command
	policy := hasStartupPolicy(d, sid, account)
	settings := hasStartupSettings(d.Settings)
	isProtected := protectedExe != "" && winapp.SamePath(target, protectedExe)
	s := state{
		exists:       true,
		target:       target,
		executable:   target,
		protected:    isProtected,
		registration: text,
		repairReason: repairReason(policy, settings, isProtected),
	}
	if isProtected {
		s.thisCopy = protectedSource != "" && winapp.SamePath(protectedSource, identity)
		if protectedSource != "" {
			s.executable = protectedSource
		}
	} else {
		s.thisCopy = winapp.SamePath(target, identity)
	}
	// An elevated task that runs a user-writable file is never reported as
	// working startup, even if its policy is intact.
	s.enabled = valueOr(d.Settings.Enabled, true) && policy && settings && isProtected
	return s, nil
}

// withCopy records the owner of this user's protected copy when it is not
// this installation.
func withCopy(s state, copySource, identity string) state {
	if copySource != "" && !winapp.SamePath(copySource, identity) {
		s.otherCopy = copySource
	}
	return s
}

func ownShape(d taskDefinition, sid string, account accountSID) (taskExec, bool) {
	if d.RegistrationInfo.Source != taskOwner || len(d.Principals.Principal) == 0 ||
		!matchesUser(valueOr(d.Principals.Principal[0].UserID, ""), sid, account) ||
		len(d.Actions.Exec) != 1 || len(d.Actions.Other) != 0 ||
		d.Actions.Exec[0].Arguments != taskArguments {
		return taskExec{}, false
	}
	return d.Actions.Exec[0], true
}

func hasStartupPolicy(d taskDefinition, sid string, account accountSID) bool {
	p := d.Principals.Principal[0]
	if len(d.Triggers.Logon) == 0 {
		return false
	}
	logon := d.Triggers.Logon[0]
	return valueOr(p.RunLevel, "LeastPrivilege") == "HighestAvailable" &&
		valueOr(p.LogonType, "") == "InteractiveToken" &&
		matchesUser(valueOr(logon.UserID, ""), sid, account) && valueOr(logon.Enabled, true)
}

func hasStartupSettings(s taskSettings) bool {
	return !valueOr(s.DisallowStartIfOnBatteries, true) &&
		!valueOr(s.StopIfGoingOnBatteries, true) &&
		!valueOr(s.RunOnlyIfIdle, false) &&
		!valueOr(s.RunOnlyIfNetworkAvailable, false) &&
		zeroDuration(valueOr(s.ExecutionTimeLimit, "PT72H")) &&
		valueOr(s.MultipleInstancesPolicy, "IgnoreNew") == "IgnoreNew"
}

// zeroDuration reports whether an XML duration such as PT0S is zero, which
// for a time limit means unlimited.
func zeroDuration(text string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(text), "P")
	digits := false
	for _, r := range rest {
		switch {
		case r == '0':
			digits = true
		case r == '.' || strings.ContainsRune("YMWDHST", r):
		default:
			return false
		}
	}
	return ok && digits
}

func repairReason(policy, settings, isProtected bool) string {
	switch {
	case !policy || !settings:
		return "自启任务设置已变化（权限、触发条件或运行限制），请重新勾选以修复。"
	case !isProtected:
		return "自启仍指向普通权限可替换的程序文件，请以管理员身份重新勾选，改用受保护副本。"
	}
	return ""
}
