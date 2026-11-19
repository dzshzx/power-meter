//go:build windows

package winapp

import (
	"errors"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	instancePrefix = `Local\PowerMeter.`
	restorePrefix  = `Local\PowerMeter.Restore.`
	// handoffWait is how long the elevated copy waits for the process that
	// started it to let go of the lock.
	handoffWait = 15 * time.Second
	// asfwAny lets whichever process the restore reaches take the
	// foreground.
	asfwAny = ^uintptr(0)
)

// errRestore: the restore request could not be set up.
var errRestore = errors.New("无法初始化窗口恢复通知。")

// objectSecurity grants the user full access to a named object, at medium
// integrity: an ordinary launch must be able to open what an elevated
// instance created, so it can wait on the lock or ask for the window.
func objectSecurity(sid string) (*windows.SecurityAttributes, error) {
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + sid + ")S:(ML;;NW;;;ME)")
	if err != nil {
		return nil, err
	}
	sa := &windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(*sa))
	return sa, nil
}

// Instance is this installation's single-instance lock. A Windows mutex
// belongs to the thread that took it, so a dedicated thread holds it until
// Close.
type Instance struct {
	key     string
	sid     string
	release chan struct{}
	done    chan struct{}
	closed  sync.Once

	mu       sync.Mutex
	stop     windows.Handle
	listened chan struct{}
}

// Acquire takes the lock of the installation identity for this user. ok is
// false when another instance holds it: an ordinary launch then calls
// RestoreExisting and exits, a logon launch (--autostart) exits quietly.
// The elevated copy started by TryElevate passes handoff and waits up to 15
// seconds for the process that started it to exit.
func Acquire(identity string, handoff bool) (inst *Instance, ok bool, err error) {
	sid, err := UserSID()
	if err != nil {
		return nil, false, err
	}
	inst = &Instance{key: instanceKey(sid, identity), sid: sid, release: make(chan struct{}), done: make(chan struct{})}
	type taken struct {
		owned bool
		err   error
	}
	result := make(chan taken, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(inst.done)
		h, owned, err := lockMutex(instancePrefix+inst.key, sid, handoff)
		result <- taken{owned, err}
		if h == 0 {
			return
		}
		if owned {
			<-inst.release
			_ = windows.ReleaseMutex(h)
		}
		_ = windows.CloseHandle(h)
	}()
	r := <-result
	if r.err != nil || !r.owned {
		<-inst.done
		return nil, false, r.err
	}
	return inst, true, nil
}

// lockMutex opens or creates the named mutex and waits for it on the
// calling thread, which then owns it.
func lockMutex(name, sid string, handoff bool) (windows.Handle, bool, error) {
	sa, err := objectSecurity(sid)
	if err != nil {
		return 0, false, err
	}
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}
	h, err := windows.CreateMutex(sa, false, n)
	if h == 0 {
		// A higher-integrity holder can deny an ordinary launch a handle;
		// its lock still prevents duplicates.
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) && !handoff {
			return 0, false, nil
		}
		return 0, false, err
	}
	var wait time.Duration
	if handoff {
		wait = handoffWait
	}
	event, err := windows.WaitForSingleObject(h, uint32(wait.Milliseconds()))
	switch event {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		return h, true, nil
	case uint32(windows.WAIT_TIMEOUT):
		return h, false, nil
	}
	return h, false, err
}

// Listen calls onRestore each time another launch of this installation asks
// for the window (RestoreExisting), on a goroutine of its own: post to the
// UI thread from there. Only this parameter-free request crosses integrity
// levels, so an ordinary launch can restore an elevated instance; nothing
// else is exposed.
func (i *Instance) Listen(onRestore func()) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.listened != nil {
		return nil
	}
	sa, err := objectSecurity(i.sid)
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(restorePrefix + i.key)
	if err != nil {
		return err
	}
	request, err := windows.CreateEvent(sa, 0, 0, name)
	if request == 0 {
		return errors.Join(errRestore, err)
	}
	stop, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		_ = windows.CloseHandle(request)
		return errors.Join(errRestore, err)
	}
	i.stop, i.listened = stop, make(chan struct{})
	go func() {
		defer close(i.listened)
		defer windows.CloseHandle(request)
		for {
			event, err := windows.WaitForMultipleObjects([]windows.Handle{stop, request}, false, windows.INFINITE)
			if err != nil || event != windows.WAIT_OBJECT_0+1 {
				return
			}
			onRestore()
		}
	}()
	return nil
}

// Close stops listening and releases the lock. The process exiting
// releases it too.
func (i *Instance) Close() {
	i.closed.Do(func() {
		i.mu.Lock()
		if i.listened != nil {
			_ = windows.SetEvent(i.stop)
			<-i.listened
			_ = windows.CloseHandle(i.stop)
		}
		i.mu.Unlock()
		close(i.release)
		<-i.done
	})
}

// RestoreExisting asks the running instance of identity to show its window,
// for up to two seconds while that instance is still starting. It reports
// whether an instance took the request.
func RestoreExisting(identity string) bool {
	sid, err := UserSID()
	if err != nil {
		return false
	}
	name, err := windows.UTF16PtrFromString(restorePrefix + instanceKey(sid, identity))
	if err != nil {
		return false
	}
	for range 20 {
		if request, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name); err == nil {
			// This launch was just started by the user, so it may hand
			// the foreground on; the instance shows its window next.
			_, _, _ = procAllowSetFg.Call(asfwAny)
			err = windows.SetEvent(request)
			_ = windows.CloseHandle(request)
			if err == nil {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// WithLock runs fn while holding the per-user named mutex Local\<name>,
// waiting up to wait for another holder. held is false when the wait ran
// out and fn did not run. The calling goroutine stays on one thread for
// the duration, as the mutex requires.
func WithLock(name string, wait time.Duration, fn func() error) (held bool, err error) {
	sid, err := UserSID()
	if err != nil {
		return false, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	sa, err := objectSecurity(sid)
	if err != nil {
		return false, err
	}
	n, err := windows.UTF16PtrFromString(`Local\` + name)
	if err != nil {
		return false, err
	}
	h, err := windows.CreateMutex(sa, false, n)
	if h == 0 {
		return false, err
	}
	defer windows.CloseHandle(h)
	event, err := windows.WaitForSingleObject(h, uint32(wait.Milliseconds()))
	switch event {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
	case uint32(windows.WAIT_TIMEOUT):
		return false, nil
	default:
		return false, err
	}
	defer windows.ReleaseMutex(h)
	return true, fn()
}
