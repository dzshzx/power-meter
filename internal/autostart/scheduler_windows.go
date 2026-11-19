//go:build windows

package autostart

import (
	"errors"
	"runtime"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

// Task Scheduler 2.0 constants.
const (
	// TASK_CREATE_OR_UPDATE | TASK_DONT_ADD_PRINCIPAL_ACE |
	// TASK_IGNORE_REGISTRATION_TRIGGERS. Without the second flag the
	// scheduler adds its own ACE for the principal and drops the DACL's
	// protection.
	taskRegisterFlags = 0x6 | 0x10 | 0x20
	// taskLogonInteractiveToken: run in the user's session, no password.
	taskLogonInteractiveToken = 3
	// rpcChangedMode: COM is already set up on this thread in the other
	// apartment model, which the scheduler accepts as well.
	rpcChangedMode = 0x80010106
)

// scheduler is a connection to the root folder of the Task Scheduler,
// through its scripting COM interface. No service, password or external
// executable is needed.
type scheduler struct {
	folder *ole.IDispatch
}

// withScheduler runs fn on a thread of its own with COM set up and a
// scheduler connection. The thread is never handed back: it ends with the
// goroutine, and its COM state with it. fn may take named mutexes, which
// stay on this thread.
func withScheduler(fn func(*scheduler) error) error {
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		result <- connect(fn)
	}()
	return <-result
}

func connect(fn func(*scheduler) error) error {
	switch err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); {
	case err == nil, hresult(err) == 1: // S_FALSE: already set up here
		defer ole.CoUninitialize()
	case hresult(err) != rpcChangedMode:
		return err
	}
	unknown, err := oleutil.CreateObject("Schedule.Service")
	if err != nil {
		return comError(err)
	}
	defer unknown.Release()
	service, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return comError(err)
	}
	defer service.Release()
	if _, err := call(service, "Connect"); err != nil {
		return err
	}
	root, err := call(service, "GetFolder", `\`)
	if err != nil {
		return err
	}
	defer root.Clear()
	return fn(&scheduler{folder: root.ToIDispatch()})
}

// call invokes a method and returns its result, which the caller clears.
func call(object *ole.IDispatch, method string, args ...any) (*ole.VARIANT, error) {
	v, err := oleutil.CallMethod(object, method, args...)
	return v, comError(err)
}

func callAndClear(object *ole.IDispatch, method string, args ...any) error {
	v, err := call(object, method, args...)
	if err == nil {
		_ = v.Clear()
	}
	return err
}

// task opens a registered task; found is false when there is none of that
// name.
func (s *scheduler) task(name string) (task *ole.VARIANT, found bool, err error) {
	v, err := call(s.folder, "GetTask", name)
	switch {
	case err == nil:
		return v, true, nil
	case notFound(err):
		return nil, false, nil
	}
	return nil, false, err
}

// taskXML is the registered task's XML; found is false when there is no
// task of that name.
func (s *scheduler) taskXML(name string) (text string, found bool, err error) {
	task, found, err := s.task(name)
	if !found {
		return "", false, err
	}
	defer task.Clear()
	v, err := oleutil.GetProperty(task.ToIDispatch(), "Xml")
	if err != nil {
		return "", true, comError(err)
	}
	defer v.Clear()
	return v.ToString(), true, nil
}

// register creates or replaces the task, for the user with sid and with
// the given security descriptor.
func (s *scheduler) register(name, text, sid, sddl string) error {
	return callAndClear(s.folder, "RegisterTask", name, text, int32(taskRegisterFlags), sid, nil,
		int32(taskLogonInteractiveToken), sddl)
}

func (s *scheduler) delete(name string) error {
	err := callAndClear(s.folder, "DeleteTask", name, int32(0))
	if notFound(err) {
		return nil
	}
	return err
}

// run starts the task on demand, as its logon trigger would.
func (s *scheduler) run(name string) error {
	task, found, err := s.task(name)
	if !found {
		return err
	}
	defer task.Clear()
	return callAndClear(task.ToIDispatch(), "Run", nil)
}

// security is the task's security descriptor (owner, group and DACL) in
// SDDL.
func (s *scheduler) security(name string) (string, error) {
	task, found, err := s.task(name)
	if !found {
		return "", errors.Join(err, errors.New("no task "+name))
	}
	defer task.Clear()
	const ownerGroupDACL = windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION | windows.DACL_SECURITY_INFORMATION
	v, err := call(task.ToIDispatch(), "GetSecurityDescriptor", int32(ownerGroupDACL))
	if err != nil {
		return "", err
	}
	defer v.Clear()
	return v.ToString(), nil
}

// comFailure is a COM error with the HRESULT behind it and the scheduler's
// own description.
type comFailure struct {
	code uint32
	text string
}

func (e *comFailure) Error() string { return e.text }

// comError keeps the code the scheduler reported inside a dispatch
// exception, with its description or else the system's message for the
// code.
func comError(err error) error {
	var oe *ole.OleError
	if !errors.As(err, &oe) {
		return err
	}
	code, text := uint32(oe.Code()), oe.String()
	if info, ok := oe.SubError().(ole.EXCEPINFO); ok && info.SCODE() != 0 {
		code, text = info.SCODE(), ole.NewError(uintptr(info.SCODE())).String()
		if d := strings.TrimSpace(oe.Description()); d != "" && d != "<nil>" {
			text = d
		}
	}
	return &comFailure{code: code, text: text}
}

func hresult(err error) uint32 {
	var failure *comFailure
	if errors.As(err, &failure) {
		return failure.code
	}
	var oe *ole.OleError
	if errors.As(err, &oe) {
		return uint32(oe.Code())
	}
	return 0
}

// notFound: HRESULT_FROM_WIN32 of ERROR_FILE_NOT_FOUND or
// ERROR_PATH_NOT_FOUND.
func notFound(err error) bool {
	code := hresult(err)
	return code == 0x80070002 || code == 0x80070003
}
