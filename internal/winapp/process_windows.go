//go:build windows

package winapp

import (
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/dzshzx/power-meter/internal/cli"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	user32            = windows.NewLazySystemDLL("user32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteExW")
	procAllowSetFg    = user32.NewProc("AllowSetForegroundWindow")
)

// IsElevated reports whether the process token holds the Administrators
// group enabled: true after UAC elevation, false for the filtered token of
// an ordinary launch.
func IsElevated() bool {
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	member, err := windows.Token(0).IsMember(admins)
	return err == nil && member
}

// UserSID is the SID of the user the process runs as, such as
// "S-1-5-21-…-1001". An elevation with another administrator's credentials
// runs as that user.
func UserSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

var identity = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	sid, err := UserSID()
	if err != nil {
		return exe
	}
	return IdentityFor(exe, sid)
})

// Identity is the installation this process represents: its own
// executable, or for the protected logon copy the installation it was
// copied from (IdentityFor). The single-instance lock and the restore
// request use it, so a manual launch restores the running logon instance.
func Identity() string { return identity() }

// TryElevate starts this executable again through the UAC prompt, marked
// with --elevation-attempted so the elevated copy waits for this process's
// single-instance lock instead of asking again. Cancelling the prompt or a
// failed start keeps the ordinary window with the reason.
func TryElevate() cli.ElevationResult {
	return cli.Launch(func() (bool, error) {
		exe, err := os.Executable()
		if err != nil {
			return false, err
		}
		return runas(exe, cli.ElevationAttempted)
	})
}

// shellExecuteInfo is SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	size          uint32
	mask          uint32
	window        windows.HWND
	verb          *uint16
	file          *uint16
	parameters    *uint16
	directory     *uint16
	show          int32
	instApp       windows.Handle
	idList        uintptr
	class         *uint16
	keyClass      windows.Handle
	hotKey        uint32
	iconOrMonitor windows.Handle
	process       windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskFlagDDEWait    = 0x00000100
	seeMaskFlagNoUI       = 0x00000400
)

// runas starts exe elevated. A declined prompt is ERROR_CANCELLED (1223).
func runas(exe, parameters string) (bool, error) {
	dir, _ := SplitPath(exe)
	var info shellExecuteInfo
	var err error
	for _, field := range []struct {
		dst  **uint16
		text string
	}{{&info.verb, "runas"}, {&info.file, exe}, {&info.parameters, parameters}, {&info.directory, dir}} {
		if *field.dst, err = windows.UTF16PtrFromString(field.text); err != nil {
			return false, err
		}
	}
	info.mask = seeMaskNoCloseProcess | seeMaskFlagDDEWait | seeMaskFlagNoUI
	info.show = windows.SW_SHOWNORMAL
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errno, isErrno := callErr.(syscall.Errno); isErrno && errno != 0 {
			return false, errno
		}
		return false, nil
	}
	if info.process == 0 {
		return false, nil
	}
	_ = windows.CloseHandle(info.process)
	return true, nil
}
