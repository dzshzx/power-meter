//go:build windows

package sensors

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GUID_DEVICE_ENERGY_METER and the EMI IOCTLs of emi.h.
var guidDeviceEnergyMeter = windows.GUID{
	Data1: 0x45BD8344, Data2: 0x7ED6, Data3: 0x49CF,
	Data4: [8]byte{0xA4, 0x40, 0xC2, 0x76, 0xC9, 0x33, 0xB0, 0x53},
}

const (
	ioctlEMIGetVersion      = 0x00224000
	ioctlEMIGetMetadataSize = 0x00224004
	ioctlEMIGetMetadata     = 0x00224008
	ioctlEMIGetMeasurement  = 0x0022400C
	// emiMetadataMinimum is the shortest metadata worth parsing.
	emiMetadataMinimum = 68
	// emiMeasurementSize is one EMI_CHANNEL_MEASUREMENT_DATA:
	// AbsoluteEnergy (8 bytes, picowatt-hours), AbsoluteTime (8, 100 ns).
	emiMeasurementSize = 16
)

var (
	modSetupAPI                          = windows.NewLazySystemDLL("setupapi.dll")
	procSetupDiEnumDeviceInterfaces      = modSetupAPI.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW = modSetupAPI.NewProc("SetupDiGetDeviceInterfaceDetailW")
)

// spDeviceInterfaceData is SP_DEVICE_INTERFACE_DATA.
type spDeviceInterfaceData struct {
	size      uint32
	classGUID windows.GUID
	flags     uint32
	reserved  uintptr
}

// nativeEMI is the Energy Meter Interface through SetupAPI and
// DeviceIoControl.
type nativeEMI struct{}

// paths lists every present energy meter interface. Windows creates one per
// meter, and the package channel need not be on the first.
func (nativeEMI) paths() ([]string, error) {
	set, err := windows.SetupDiGetClassDevsEx(&guidDeviceEnergyMeter, "", 0,
		windows.DIGCF_PRESENT|windows.DIGCF_DEVICEINTERFACE, 0, "")
	if err != nil {
		return nil, nil
	}
	defer func() { _ = set.Close() }()

	var paths []string
	for index := uint32(0); ; index++ {
		data := spDeviceInterfaceData{size: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
		r, _, e := procSetupDiEnumDeviceInterfaces.Call(uintptr(set), 0,
			uintptr(unsafe.Pointer(&guidDeviceEnergyMeter)), uintptr(index), uintptr(unsafe.Pointer(&data)))
		if r == 0 {
			if errors.Is(e, windows.ERROR_NO_MORE_ITEMS) {
				return paths, nil
			}
			return nil, fmt.Errorf("SetupDiEnumDeviceInterfaces 失败: %d", errnoValue(e))
		}
		if path, ok := interfacePath(set, &data); ok {
			paths = append(paths, path)
		}
	}
}

func errnoValue(err error) uint32 {
	var errno windows.Errno
	if errors.As(err, &errno) {
		return uint32(errno)
	}
	return 0
}

// interfacePath reads SP_DEVICE_INTERFACE_DETAIL_DATA_W: a DWORD cbSize
// (8 on 64-bit, 6 on 32-bit, the packed size of the fixed part) followed by
// the NUL-terminated device path.
func interfacePath(set windows.DevInfo, data *spDeviceInterfaceData) (string, bool) {
	var required uint32
	_, _, _ = procSetupDiGetDeviceInterfaceDetailW.Call(uintptr(set), uintptr(unsafe.Pointer(data)),
		0, 0, uintptr(unsafe.Pointer(&required)), 0)
	if required <= 4 {
		return "", false
	}
	buffer := make([]uint64, (required+7)/8)
	cbSize := uint32(6)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		cbSize = 8
	}
	*(*uint32)(unsafe.Pointer(&buffer[0])) = cbSize
	r, _, _ := procSetupDiGetDeviceInterfaceDetailW.Call(uintptr(set), uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(required), uintptr(unsafe.Pointer(&required)), 0)
	if r == 0 {
		return "", false
	}
	path := unsafe.Slice((*uint16)(unsafe.Add(unsafe.Pointer(&buffer[0]), 4)), (required-4)/2)
	return windows.UTF16ToString(path), true
}

func openEMI(path string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
}

// channels reads a device's channel names; a device that cannot be opened
// has none.
func (nativeEMI) channels(path string) ([]string, error) {
	h, err := openEMI(path)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = windows.CloseHandle(h) }()
	return readSupportedChannels(
		func() (uint16, error) { return emiVersion(h) },
		func(version uint16) ([]string, error) { return emiChannelNames(h, version) })
}

func emiVersion(h windows.Handle) (uint16, error) {
	var buffer [2]byte
	if err := ioctl(h, ioctlEMIGetVersion, buffer[:]); err != nil {
		return 0, errors.New("IOCTL_EMI_GET_VERSION 失败")
	}
	return binary.LittleEndian.Uint16(buffer[:]), nil
}

func emiChannelNames(h windows.Handle, version uint16) ([]string, error) {
	var sizeBuffer [4]byte
	if err := ioctl(h, ioctlEMIGetMetadataSize, sizeBuffer[:]); err != nil {
		return nil, errors.New("IOCTL_EMI_GET_METADATA_SIZE 失败")
	}
	size := int32(binary.LittleEndian.Uint32(sizeBuffer[:]))
	if size <= emiMetadataMinimum {
		return nil, errors.New("EMI 元数据过短")
	}
	metadata := make([]byte, size)
	if err := ioctl(h, ioctlEMIGetMetadata, metadata); err != nil {
		return nil, errors.New("IOCTL_EMI_GET_METADATA 失败")
	}
	return parseChannelNames(version, metadata)
}

// measure reads one channel's absolute energy and time.
func (nativeEMI) measure(path string, channelCount, index int) (energy, at uint64, err error) {
	h, err := openEMI(path)
	if err != nil {
		return 0, 0, errEMIOpen
	}
	defer func() { _ = windows.CloseHandle(h) }()
	buffer := make([]byte, channelCount*emiMeasurementSize)
	if err := ioctl(h, ioctlEMIGetMeasurement, buffer); err != nil {
		return 0, 0, errors.New("IOCTL_EMI_GET_MEASUREMENT 失败")
	}
	offset := index * emiMeasurementSize
	return binary.LittleEndian.Uint64(buffer[offset:]), binary.LittleEndian.Uint64(buffer[offset+8:]), nil
}

func ioctl(h windows.Handle, code uint32, out []byte) error {
	var returned uint32
	return windows.DeviceIoControl(h, code, nil, 0, &out[0], uint32(len(out)), &returned, nil)
}
