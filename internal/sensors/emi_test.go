package sensors

import (
	"encoding/binary"
	"errors"
	"slices"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/dzshzx/power-meter/internal/power"
)

func utf16le(s string) []byte {
	units := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[2*i:], u)
	}
	return b
}

// metadataV1 is EMI_METADATA_V1 naming one channel.
func metadataV1(name string) []byte {
	encoded := utf16le(name)
	m := make([]byte, 72+len(encoded))
	binary.LittleEndian.PutUint16(m[70:], uint16(len(encoded)))
	copy(m[72:], encoded)
	return m
}

// metadataV2 is EMI_METADATA_V2 listing the channels.
func metadataV2(names ...string) []byte {
	m := make([]byte, 68)
	binary.LittleEndian.PutUint16(m[66:], uint16(len(names)))
	for _, name := range names {
		encoded := utf16le(name)
		channel := make([]byte, 6, 6+len(encoded))
		binary.LittleEndian.PutUint16(channel[4:], uint16(len(encoded)))
		m = append(m, append(channel, encoded...)...)
	}
	return m
}

func TestEMIMetadata(t *testing.T) {
	metadata := metadataV1("CPU_PKG\x00")
	parse := func(version uint16) ([]string, error) { return parseChannelNames(version, metadata) }
	channels, err := readSupportedChannels(func() (uint16, error) { return 1, nil }, parse)
	if err != nil || !slices.Equal(channels, []string{"CPU_PKG"}) {
		t.Errorf("EMI V1 version and package metadata are accepted: %q %v", channels, err)
	}

	d := selectPackageDevice([]string{"bad-device", "good-device"}, func(path string) ([]string, error) {
		if path == "bad-device" {
			return nil, errors.New("broken metadata")
		}
		return readSupportedChannels(func() (uint16, error) { return 1, nil }, parse)
	})
	if d.path != "good-device" || len(d.selected) != 1 || len(d.discovered) != 1 ||
		!slices.Equal(d.errors, []string{"bad-device: broken metadata"}) {
		t.Errorf("bad EMI device does not hide a later package meter: %+v", d)
	}

	for _, version := range []uint16{0, 3} {
		names, err := readSupportedChannels(func() (uint16, error) { return version, nil }, parse)
		if names != nil || err != nil {
			t.Errorf("version %d is skipped without an error", version)
		}
	}
	if _, err := readSupportedChannels(func() (uint16, error) { return 0, errors.New("IOCTL_EMI_GET_VERSION 失败") }, parse); err == nil {
		t.Error("a failed version read is an error")
	}
}

func TestEMIMetadataV2(t *testing.T) {
	names, err := parseChannelNames(2, metadataV2("RAPL_Package0_PKG", "RAPL_Package0_DRAM", "RAPL_Package0_PP0 ", "RAPL_Package0_PP1"))
	want := []string{"RAPL_Package0_PKG", "RAPL_Package0_DRAM", "RAPL_Package0_PP0", "RAPL_Package0_PP1"}
	if err != nil || !slices.Equal(names, want) {
		t.Errorf("V2 lists every channel, trimming padding: %q %v", names, err)
	}
	if names, err := parseChannelNames(2, metadataV2()); err != nil || len(names) != 0 {
		t.Errorf("V2 without channels: %q %v", names, err)
	}

	truncated := metadataV2("RAPL_Package0_PKG")
	oddSize := metadataV2("AB")
	binary.LittleEndian.PutUint16(oddSize[72:], 3)
	missingChannel := metadataV2("AB")
	binary.LittleEndian.PutUint16(missingChannel[66:], 2)
	cases := []struct {
		name     string
		version  uint16
		metadata []byte
		want     error
	}{
		{"V1 shorter than its name size", 1, make([]byte, 71), errChannelTooShort},
		{"V1 empty name", 1, make([]byte, 72), errChannelOutOfBounds},
		{"V2 name past the end", 2, truncated[:len(truncated)-2], errChannelOutOfBounds},
		{"V2 odd name size", 2, oddSize, errChannelOutOfBounds},
		{"V2 fewer channels than counted", 2, missingChannel, errChannelTooShort},
		{"V2 header too short", 2, make([]byte, 67), errMetadataVersion},
		{"unknown layout", 3, metadataV2("A"), errMetadataVersion},
	}
	for _, c := range cases {
		if _, err := parseChannelNames(c.version, c.metadata); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestPackageChannel(t *testing.T) {
	cases := []struct {
		names []string
		want  int
	}{
		{[]string{"RAPL_Package0_DRAM", "RAPL_Package0_PKG"}, 1},
		{[]string{"cpu_pkg"}, 0},
		{[]string{"PKG", "_PKG_DRAM", "😀_pkg"}, 2},
		{nil, -1},
	}
	for _, c := range cases {
		if got := indexOfPackageChannel(c.names); got != c.want {
			t.Errorf("indexOfPackageChannel(%q) = %d, want %d", c.names, got, c.want)
		}
	}
}

// fakeEMI serves channel lists and measurements from tables.
type fakeEMI struct {
	devices  []string
	enumErr  error
	names    map[string][]string
	errs     map[string]error
	readings [][2]uint64
	readErr  error
}

func (f *fakeEMI) paths() ([]string, error) { return f.devices, f.enumErr }

func (f *fakeEMI) channels(path string) ([]string, error) { return f.names[path], f.errs[path] }

func (f *fakeEMI) measure(_ string, _, _ int) (uint64, uint64, error) {
	if f.readErr != nil {
		return 0, 0, f.readErr
	}
	r := f.readings[0]
	f.readings = f.readings[1:]
	return r[0], r[1], nil
}

func TestEMIDiscovery(t *testing.T) {
	rapl := []string{"RAPL_Package0_PKG", "RAPL_Package0_DRAM"}
	cases := []struct {
		name       string
		emi        *fakeEMI
		reason     string
		discovered []string
	}{
		{"no device", &fakeEMI{}, "本机没有 EMI 电表设备", nil},
		{"enumeration fails", &fakeEMI{enumErr: errors.New("SetupDiEnumDeviceInterfaces 失败: 5")},
			"EMI 初始化失败: SetupDiEnumDeviceInterfaces 失败: 5", nil},
		{"package on the second device", &fakeEMI{devices: []string{"oem", "rapl"},
			names: map[string][]string{"oem": {"DC_IN"}, "rapl": rapl}}, "", []string{"DC_IN", "RAPL_Package0_PKG", "RAPL_Package0_DRAM"}},
		{"no package channel", &fakeEMI{devices: []string{"oem"}, names: map[string][]string{"oem": {"DC_IN"}}},
			"EMI 没有 CPU 包功率通道", []string{"DC_IN"}},
		{"only unreadable devices", &fakeEMI{devices: []string{`\\?\emi#1`}, errs: map[string]error{`\\?\emi#1`: errors.New("EMI 元数据过短")}},
			`EMI 设备不可读: \\?\emi#1: EMI 元数据过短`, nil},
		{"devices without channels", &fakeEMI{devices: []string{"closed"}}, "EMI 设备未报告任何通道", nil},
	}
	for _, c := range cases {
		s := newEMISensor(c.emi)
		if s.reason != c.reason || !slices.Equal(s.discovered, c.discovered) {
			t.Errorf("%s: reason %q, discovered %q", c.name, s.reason, s.discovered)
		}
	}
}

func TestEMIRead(t *testing.T) {
	const second = 10_000_000  // 100 ns units
	const centiWattHour = 1e10 // picowatt-hours, 36 J
	fake := &fakeEMI{devices: []string{"rapl"}, names: map[string][]string{"rapl": {"RAPL_Package0_DRAM", "RAPL_Package0_PKG"}},
		readings: [][2]uint64{{0, 100}, {centiWattHour, 100 + second}, {0, 100 + 2*second}, {centiWattHour / 2, 100 + 4*second}}}
	s := newEMISensor(fake)
	if r := s.read(); r.Available || r.UnavailableReason != "正在建立基准" || r.Boundary != power.CPUPackage {
		t.Errorf("first sample sets the baseline: %+v", r)
	}
	r := s.read()
	if !r.Available || !near(r.Watts, 36) || r.Window != time.Second || r.Source != "EMI RAPL_Package0_PKG" || r.Kind != power.Measured {
		t.Errorf("second sample is average package power: %+v", r)
	}
	if r = s.read(); r.Available || r.UnavailableReason != "EMI 计数器已重置，正在重建基准" {
		t.Errorf("a counter that went backwards: %+v", r)
	}
	if r = s.read(); !r.Available || !near(r.Watts, 9) || r.Window != 2*time.Second {
		t.Errorf("the reset reading is the new baseline: %+v", r)
	}

	fake.readErr = errEMIOpen
	if r = s.read(); r.UnavailableReason != "无法打开 EMI 设备" {
		t.Errorf("open failure: %+v", r)
	}
	fake.readErr = errors.New("IOCTL_EMI_GET_MEASUREMENT 失败")
	if r = s.read(); r.UnavailableReason != "EMI 读取失败: IOCTL_EMI_GET_MEASUREMENT 失败" {
		t.Errorf("read failure: %+v", r)
	}
	if r = newEMISensor(&fakeEMI{}).read(); r.UnavailableReason != "本机没有 EMI 电表设备" {
		t.Errorf("an unavailable sensor reports its reason: %+v", r)
	}
}

func TestCounters(t *testing.T) {
	if _, _, ok := emiPower(2000, 200, 1000, 300); ok {
		t.Error("EMI counter regression requires a new baseline")
	}
	if _, _, ok := emiPower(0, 1, 10000, 600000001); ok {
		t.Error("EMI long pause discards the counter interval")
	}
	if _, _, ok := emiPower(0, 300, 10, 300); ok {
		t.Error("EMI time that did not advance gives no figure")
	}
	if watts, window, ok := emiPower(0, 0, 1e10, 50_000_000); !ok || !near(watts, 7.2) || window != 5*time.Second {
		t.Errorf("a 5 s window is still a figure: %v %v", watts, window)
	}

	var tracker raplTracker
	_, _, first := tracker.advance(1000, 2000, 0, 0.001)
	_, _, longWindow := tracker.advance(2000, 4000, 60*time.Second, 0.001)
	platform, packageWatts, resumed := tracker.advance(2100, 4200, 61*time.Second, 0.001)
	if first || longWindow || !resumed || !platform.Available || !near(platform.Watts, 0.2) || !near(packageWatts, 0.1) {
		t.Errorf("long RAPL window re-baselines the production tracker: %+v %v", platform, packageWatts)
	}
}

func TestRAPLTracker(t *testing.T) {
	var tracker raplTracker
	if p, _, ok := tracker.advance(0, 0, time.Second, 1); ok || p.UnavailableReason != "正在建立基准" {
		t.Errorf("baseline: %+v", p)
	}
	if p, _, ok := tracker.advance(10, 10, time.Second, 1); ok || p.UnavailableReason != "采样窗口为零" {
		t.Errorf("zero window: %+v", p)
	}
	if p, _, ok := tracker.advance(20, 10, 2*time.Second, 1); ok || p.UnavailableReason != "本机未启用 Psys 平台计数器" {
		t.Errorf("a Psys counter that never advances is a capability result, not zero watts: %+v", p)
	}
	if p, _, ok := tracker.advance(30, 20, 9*time.Second, 1); ok || p.UnavailableReason != "采样间隔过长，正在重新建立基准" {
		t.Errorf("long gap: %+v", p)
	}
	// 32-bit counters wrap.
	tracker = raplTracker{}
	tracker.advance(0xFFFFFFF0, 0xFFFFFF00, 0, 0.5)
	p, pkg, ok := tracker.advance(0x10, 0x100, 2*time.Second, 0.5)
	if !ok || !near(p.Watts, 512*0.5/2) || !near(pkg, 32*0.5/2) || p.Window != 2*time.Second ||
		p.Source != "PawnIO MSR 0x64D" || p.Boundary != power.Platform || p.Kind != power.Measured {
		t.Errorf("wrapped counters: %+v %v", p, pkg)
	}
}
