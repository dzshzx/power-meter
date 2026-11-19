package sensors

import (
	"encoding/binary"
	"errors"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
)

// EMI reasons and errors: Chinese source strings, translated where shown.
const (
	reasonNoEMIDevice     = "本机没有 EMI 电表设备"
	reasonNoPackage       = "EMI 没有 CPU 包功率通道"
	reasonEMIUnreadable   = "EMI 设备不可读: "
	reasonNoChannels      = "EMI 设备未报告任何通道"
	reasonEMIInitFailed   = "EMI 初始化失败: "
	reasonEMIOpenFailed   = "无法打开 EMI 设备"
	reasonEMIReadFailed   = "EMI 读取失败: "
	reasonBaseline        = "正在建立基准"
	reasonEMICounterReset = "EMI 计数器已重置，正在重建基准"
)

var (
	errEMIOpen            = errors.New(reasonEMIOpenFailed)
	errMetadataVersion    = errors.New("不支持的 EMI 元数据版本")
	errChannelTooShort    = errors.New("EMI 通道元数据过短")
	errChannelOutOfBounds = errors.New("EMI 通道名越界")
)

const (
	packageChannelSuffix = "_PKG"
	// Offsets into the emi.h metadata layouts (see parseChannelNames).
	metadataV1NameSize = 70
	metadataV2Header   = 68
	channelNameSize    = 4
)

// emiDevices is the Energy Meter Interface as the sensor uses it: the
// device paths of GUID_DEVICE_ENERGY_METER, a device's channel names (nil
// when the device cannot be opened or has an unknown version), and one
// channel's absolute energy (picowatt-hours) and time (100 ns units).
type emiDevices interface {
	paths() ([]string, error)
	channels(path string) ([]string, error)
	measure(path string, channelCount, index int) (energy, time uint64, err error)
}

// emiSensor reads CPU package power from the first EMI device that carries
// a package channel. EMI needs no driver install and no elevation.
//
// EMI reports a monotonic energy counter, not power, so a figure exists only
// from the second sample on. The device's own timestamps time the window, so
// this process being descheduled does not distort it.
type emiSensor struct {
	devices      emiDevices
	reason       string
	path         string
	channels     []string
	packageIndex int
	// discovered lists the channels of the device in use, or of every
	// device inspected when none carried a package channel.
	discovered []string
	errors     []string
	counter    emiCounter
}

func newEMISensor(devices emiDevices) *emiSensor {
	s := &emiSensor{devices: devices, packageIndex: -1}
	paths, err := devices.paths()
	if err != nil {
		s.reason = reasonEMIInitFailed + err.Error()
		return s
	}
	if len(paths) == 0 {
		s.reason = reasonNoEMIDevice
		return s
	}
	d := selectPackageDevice(paths, devices.channels)
	s.discovered, s.errors = d.discovered, d.errors
	if d.path != "" {
		s.path, s.channels = d.path, d.selected
		s.packageIndex = indexOfPackageChannel(d.selected)
	}
	switch {
	case s.packageIndex >= 0:
	case len(s.discovered) > 0:
		s.reason = reasonNoPackage
	case len(s.errors) > 0:
		s.reason = reasonEMIUnreadable + s.errors[0]
	default:
		s.reason = reasonNoChannels
	}
	return s
}

// emiDiscovery is the device chosen for the package channel, if any, with
// every channel and error met on the way.
type emiDiscovery struct {
	path       string
	selected   []string
	discovered []string
	errors     []string
}

// selectPackageDevice walks the devices in order until one has a package
// channel. A machine with an OEM meter beside the generic RAPL one exposes
// several devices, and an unreadable one must not hide a later meter.
func selectPackageDevice(paths []string, channels func(string) ([]string, error)) emiDiscovery {
	var d emiDiscovery
	for _, path := range paths {
		names, err := channels(path)
		if err != nil {
			d.errors = append(d.errors, path+": "+err.Error())
			continue
		}
		d.discovered = append(d.discovered, names...)
		if indexOfPackageChannel(names) >= 0 {
			d.path, d.selected = path, names
			return d
		}
	}
	return d
}

// readSupportedChannels reads the channel names of a device whose metadata
// version this parser knows (1 or 2), and nothing for any other version.
func readSupportedChannels(version func() (uint16, error), names func(uint16) ([]string, error)) ([]string, error) {
	v, err := version()
	if err != nil {
		return nil, err
	}
	if v != 1 && v != 2 {
		return nil, nil
	}
	return names(v)
}

func indexOfPackageChannel(names []string) int {
	for i, name := range names {
		n := len(name) - len(packageChannelSuffix)
		if n >= 0 && strings.EqualFold(name[n:], packageChannelSuffix) {
			return i
		}
	}
	return -1
}

// parseChannelNames reads the two metadata layouts of emi.h.
//
// EMI_METADATA_V1: MeasurementUnit (4 bytes), HardwareOEM (32),
// HardwareModel (32), HardwareRevision (2), MeteredHardwareNameSize (2),
// MeteredHardwareName: one channel.
//
// EMI_METADATA_V2: HardwareOEM (32), HardwareModel (32), HardwareRevision
// (2), ChannelCount (2), then per channel MeasurementUnit (4), ChannelNameSize
// (2), ChannelName.
func parseChannelNames(version uint16, metadata []byte) ([]string, error) {
	if version == 1 {
		name, _, err := channelName(metadata, metadataV1NameSize)
		if err != nil {
			return nil, err
		}
		return []string{name}, nil
	}
	if version != 2 || len(metadata) < metadataV2Header {
		return nil, errMetadataVersion
	}
	count := int(binary.LittleEndian.Uint16(metadata[metadataV2Header-2:]))
	names := make([]string, 0, count)
	offset := metadataV2Header
	for range count {
		name, next, err := channelName(metadata, offset+channelNameSize)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		offset = next
	}
	return names, nil
}

// channelName decodes the UTF-16 name whose 2-byte size sits at sizeOffset
// and returns the offset just past it.
func channelName(metadata []byte, sizeOffset int) (string, int, error) {
	if sizeOffset < 0 || sizeOffset+2 > len(metadata) {
		return "", 0, errChannelTooShort
	}
	size := int(binary.LittleEndian.Uint16(metadata[sizeOffset:]))
	start := sizeOffset + 2
	if size < 2 || size%2 != 0 || start+size > len(metadata) {
		return "", 0, errChannelOutOfBounds
	}
	units := make([]uint16, size/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(metadata[start+2*i:])
	}
	return strings.TrimRight(string(utf16.Decode(units)), "\x00 "), start + size, nil
}

// available reports whether the sensor found a package channel.
func (s *emiSensor) available() bool { return s.reason == "" }

// read takes one package sample.
func (s *emiSensor) read() *power.Sample {
	if !s.available() {
		return power.Unsupported(power.CPUPackage, s.reason)
	}
	energy, at, err := s.devices.measure(s.path, len(s.channels), s.packageIndex)
	switch {
	case errors.Is(err, errEMIOpen):
		return power.Unsupported(power.CPUPackage, reasonEMIOpenFailed)
	case err != nil:
		return power.Unsupported(power.CPUPackage, reasonEMIReadFailed+err.Error())
	}
	return s.counter.advance(energy, at, "EMI "+s.channels[s.packageIndex])
}

// emiCounter turns successive absolute readings into package power.
type emiCounter struct {
	energy, time uint64
	ok           bool
}

func (c *emiCounter) advance(energy, at uint64, source string) *power.Sample {
	if !c.ok {
		c.energy, c.time, c.ok = energy, at, true
		return power.Unsupported(power.CPUPackage, reasonBaseline)
	}
	oldEnergy, oldTime := c.energy, c.time
	c.energy, c.time = energy, at
	watts, window, ok := emiPower(oldEnergy, oldTime, energy, at)
	if !ok {
		return power.Unsupported(power.CPUPackage, reasonEMICounterReset)
	}
	return power.FromValue(power.CPUPackage, power.Measured, watts, source, window)
}

// emiPower is the average power between two readings. A counter or clock
// that went backwards, or a gap longer than the statistics tolerate, gives
// no figure; the caller starts again from the newer reading.
func emiPower(oldEnergy, oldTime, energy, at uint64) (float64, time.Duration, bool) {
	if energy < oldEnergy || at <= oldTime {
		return 0, 0, false
	}
	ticks := at - oldTime
	seconds := float64(ticks) / 1e7
	if seconds > meter.MaximumGap {
		return 0, 0, false
	}
	watts := float64(energy-oldEnergy) / 1e12 * 3600 / seconds
	return watts, time.Duration(ticks) * 100, true
}
