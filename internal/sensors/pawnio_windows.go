//go:build windows

package sensors

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	thirdparty "github.com/dzshzx/power-meter/third_party"
)

// pawnIOUninstallKey is where PawnIO records its installation. Its directory
// is deliberately not on PATH, and this program does not modify PATH.
const pawnIOUninstallKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO`

// newPawnIO opens PawnIO with the IntelMSR module and reads the RAPL energy
// unit. On any failure the sensor keeps the reason; a handle that was
// opened is still released by close.
func newPawnIO() *pawnIOSensor {
	s := &pawnIOSensor{}
	lib, reason := openPawnIOLib()
	if reason != "" {
		s.reason = reason
		return s
	}
	module, err := loadModule(executableDir(), thirdparty.IntelMSRFileName, thirdparty.IntelMSR)
	switch {
	case err != nil:
		s.reason = reasonPawnIOInitFailed + err.Error()
		return s
	case module == nil:
		s.reason = reasonNoModule
		return s
	}
	var handle uintptr
	r, _, _ := lib.open.Call(uintptr(unsafe.Pointer(&handle)))
	if hr := int32(r); hr < 0 {
		s.reason = describeOpenFailure(hr, elevated())
		return s
	}
	lib.handle = handle
	s.msr = lib
	if hr := lib.loadModule(module); hr < 0 {
		s.reason = reasonModuleLoad + hresult(hr)
		return s
	}
	unit, err := lib.readMSR(msrRaplPowerUnit)
	if err != nil {
		s.reason = reasonPawnIOInitFailed + err.Error()
		return s
	}
	s.unitJoules = energyUnitJoules(unit)
	s.now = time.Now
	s.start = time.Now()
	return s
}

// openPawnIOLib finds PawnIO's installation and loads its PawnIOLib.dll by
// full path.
func openPawnIOLib() (*pawnIOLib, string) {
	dir, ok := pawnIOInstallDir()
	if !ok {
		return nil, reasonNoPawnIO
	}
	path := filepath.Join(dir, "PawnIOLib.dll")
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return nil, reasonNoPawnIOLib
	}
	dll, err := windows.LoadDLL(path)
	if err != nil {
		return nil, reasonPawnIOLibLoad
	}
	lib := &pawnIOLib{}
	for _, p := range []struct {
		proc **windows.Proc
		name string
	}{{&lib.open, "pawnio_open"}, {&lib.load, "pawnio_load"}, {&lib.execute, "pawnio_execute"}, {&lib.closeProc, "pawnio_close"}} {
		if *p.proc, err = dll.FindProc(p.name); err != nil {
			return nil, reasonPawnIOLibMissing
		}
	}
	return lib, ""
}

func pawnIOInstallDir() (string, bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, pawnIOUninstallKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", false
	}
	defer func() { _ = key.Close() }()
	location, kind, err := key.GetStringValue("InstallLocation")
	if err != nil || location == "" {
		return "", false
	}
	if kind == registry.EXPAND_SZ {
		if location, err = registry.ExpandString(location); err != nil {
			return "", false
		}
	}
	if info, err := os.Stat(location); err != nil || !info.IsDir() {
		return "", false
	}
	return location, true
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// pawnIOLib is PawnIOLib.dll's C interface. Every call returns an HRESULT.
type pawnIOLib struct {
	open, load, execute, closeProc *windows.Proc
	handle                         uintptr
}

func (l *pawnIOLib) loadModule(module []byte) int32 {
	if len(module) == 0 {
		r, _, _ := l.load.Call(l.handle, 0, 0)
		return int32(r)
	}
	r, _, _ := l.load.Call(l.handle, uintptr(unsafe.Pointer(&module[0])), uintptr(len(module)))
	return int32(r)
}

var readMSRFunction = []byte("ioctl_read_msr\x00")

func (l *pawnIOLib) readMSR(msr uint32) (uint64, error) {
	in := [1]uint64{uint64(msr)}
	var out [1]uint64
	var returned uintptr
	r, _, _ := l.execute.Call(l.handle, uintptr(unsafe.Pointer(&readMSRFunction[0])),
		uintptr(unsafe.Pointer(&in[0])), 1, uintptr(unsafe.Pointer(&out[0])), 1, uintptr(unsafe.Pointer(&returned)))
	if hr := int32(r); hr < 0 {
		return 0, fmt.Errorf("MSR 0x%X 读取失败: %s", msr, hresult(hr))
	}
	return out[0], nil
}

func (l *pawnIOLib) close() {
	if l.handle != 0 {
		_, _, _ = l.closeProc.Call(l.handle)
		l.handle = 0
	}
}
