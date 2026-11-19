//go:build windows

package sensors

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"

	"github.com/dzshzx/power-meter/internal/power"
)

// COM results CoInitializeEx returns for a thread that already has an
// apartment: the same model (still needs a matching uninitialize), or a
// different one (use it as it is).
const (
	sFalse           = 1
	rpcEChangedMode  = 0x80010106
	batteryNamespace = `root\WMI`
	batteryQuery     = "SELECT * FROM BatteryStatus"
)

// readBattery queries every active BatteryStatus instance and combines them
// with the system's charge percentage.
func readBattery() (*power.BatteryReading, error) {
	packs, err := queryBatteryStatus()
	if err != nil {
		return nil, err
	}
	return batteryReading(packs, systemPercentage()), nil
}

// queryBatteryStatus runs the WMI query on this goroutine's locked thread,
// joining the thread's COM apartment or creating a multithreaded one.
func queryBatteryStatus() ([]wmiBattery, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err == nil {
		defer ole.CoUninitialize()
	} else {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) {
			return nil, err
		}
		switch oleErr.Code() {
		case sFalse:
			defer ole.CoUninitialize()
		case rpcEChangedMode:
		default:
			return nil, err
		}
	}

	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, err
	}
	defer unknown.Release()
	locator, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, err
	}
	defer locator.Release()
	serviceVariant, err := oleutil.CallMethod(locator, "ConnectServer", ".", batteryNamespace)
	if err != nil {
		return nil, err
	}
	defer func() { _ = serviceVariant.Clear() }()
	resultVariant, err := oleutil.CallMethod(serviceVariant.ToIDispatch(), "ExecQuery", batteryQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resultVariant.Clear() }()

	var packs []wmiBattery
	err = oleutil.ForEach(resultVariant.ToIDispatch(), func(v *ole.VARIANT) error {
		defer func() { _ = v.Clear() }()
		if pack, active := batteryInstance(v.ToIDispatch()); active {
			packs = append(packs, pack)
		}
		return nil
	})
	return packs, err
}

// batteryInstance reads one BatteryStatus instance; active is false only
// when the instance has an Active property that is false.
func batteryInstance(item *ole.IDispatch) (pack wmiBattery, active bool) {
	if v, ok := property(item, "Active"); ok && !wmiBool(v) {
		return pack, false
	}
	value := func(name string) any { v, _ := property(item, name); return v }
	return wmiBattery{
		PowerOnline:   wmiBool(value("PowerOnline")),
		Charging:      wmiBool(value("Charging")),
		Discharging:   wmiBool(value("Discharging")),
		ChargeRate:    wmiUint32(value("ChargeRate")),
		DischargeRate: wmiUint32(value("DischargeRate")),
		Voltage:       wmiUint32(value("Voltage")),
	}, true
}

// property is a WMI property's value, and false when the instance has no
// such property.
func property(item *ole.IDispatch, name string) (any, bool) {
	v, err := oleutil.GetProperty(item, name)
	if err != nil {
		return nil, false
	}
	defer func() { _ = v.Clear() }()
	return v.Value(), true
}

var procGetSystemPowerStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")

// systemPowerStatus is SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	acLineStatus        byte
	batteryFlag         byte
	batteryLifePercent  byte
	systemStatusFlag    byte
	batteryLifeTime     uint32
	batteryFullLifeTime uint32
}

// systemPercentage is the operating system's aggregate charge percentage,
// rather than any single pack's, or -1 when unknown.
func systemPercentage() int {
	var s systemPowerStatus
	if r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s))); r == 0 {
		return -1
	}
	if s.batteryLifePercent > 100 {
		return -1
	}
	return int(s.batteryLifePercent)
}
