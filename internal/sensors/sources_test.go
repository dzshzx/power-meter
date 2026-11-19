package sensors

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/power"
)

// clockEMI is a package meter drawing a constant power on a fake clock.
type clockEMI struct {
	clock       *fakeClock
	start       time.Time
	pWhPerSec   float64
	unreadables []string
}

func (e *clockEMI) paths() ([]string, error) { return append(e.unreadables, "rapl"), nil }

func (e *clockEMI) channels(path string) ([]string, error) {
	if path != "rapl" {
		return nil, errors.New("IOCTL_EMI_GET_VERSION 失败")
	}
	return []string{"RAPL_Package0_PKG", "RAPL_Package0_DRAM"}, nil
}

func (e *clockEMI) measure(string, int, int) (uint64, uint64, error) {
	d := e.clock.t.Sub(e.start)
	return uint64(d.Seconds() * e.pWhPerSec), uint64(d / 100), nil
}

// fixture builds sources on a fake clock: EMI reads 36 W of CPU package
// power, and battery packs are read afresh on every capture.
func fixture(clock *fakeClock, packs []wmiBattery, pawn *pawnIOSensor) *Sources {
	emi := newEMISensor(&clockEMI{clock: clock, start: clock.now(), pWhPerSec: 1e10})
	s := newSources(func() (*power.BatteryReading, error) { return batteryReading(packs, 57), nil }, emi, pawn)
	s.now, s.start = clock.now, clock.now()
	return s
}

var charging = []wmiBattery{{PowerOnline: true, Charging: true, ChargeRate: 27980, DischargeRate: 0, Voltage: 16500}}

func TestCaptureValidatesPlatform(t *testing.T) {
	clock := newClock()
	src := fixture(clock, charging, pawnSensor(&fakeMSR{pkgStep: 36 * 16384, psysStep: 45 * 16384}, clock))
	src.Capture()
	clock.sleep(time.Second)
	s := src.Capture()
	if !s.Platform.Available || s.Platform.Source != "PawnIO MSR 0x64D" || !near(s.WholeSystem.Watts, 72.98) ||
		s.WholeSystem.Boundary != power.EstimatedSystemInput || !near(s.ElapsedSeconds, 1) || !s.Timestamp.Equal(clock.now()) {
		t.Errorf("agreeing package figures keep Psys: %+v %+v", s.Platform, s.WholeSystem)
	}

	clock = newClock()
	src = fixture(clock, charging, pawnSensor(&fakeMSR{pkgStep: 72 * 16384, psysStep: 45 * 16384}, clock))
	src.Capture()
	clock.sleep(time.Second)
	s = src.Capture()
	if s.Platform.Available || s.Platform.UnavailableReason != "单位换算校验失败（MSR 72.00 W vs EMI 36.00 W）" {
		t.Errorf("a wrong energy unit withholds Psys: %+v", s.Platform)
	}

	clock = newClock()
	src = fixture(clock, charging, pawnSensor(&fakeMSR{pkgStep: 36 * 16384, psysStep: 45 * 16384}, clock))
	src.emi = &emiSensor{reason: "本机没有 EMI 电表设备", packageIndex: -1}
	src.Capture()
	clock.sleep(time.Second)
	if s = src.Capture(); !s.Platform.Available || s.Platform.Source != "PawnIO MSR 0x64D（EMI 未交叉验证）" {
		t.Errorf("Psys without EMI stays available, labelled: %+v", s.Platform)
	}
}

func TestCaptureBatteryError(t *testing.T) {
	src := newSources(func() (*power.BatteryReading, error) { return nil, errors.New("WMI 不可用") },
		&emiSensor{reason: "本机没有 EMI 电表设备", packageIndex: -1}, &pawnIOSensor{reason: "未安装 PawnIO 驱动"})
	s := src.Capture()
	if s.Battery.StatusAvailable() || s.Battery.StatusUnavailableReason != "WMI 不可用" ||
		s.Battery.RateUnavailableReason != "WMI 不可用" || s.Battery.Percentage != -1 ||
		s.WholeSystem.UnavailableReason != "WMI 不可用" {
		t.Errorf("a failed battery query is an unavailable reading with the error: %+v", s.Battery)
	}
	if src.CPUPackageStatus() != "本机没有 EMI 电表设备" || src.PlatformStatus() != "未安装 PawnIO 驱动" {
		t.Error("statuses carry the reasons")
	}
	src.Close()
}

func TestStatuses(t *testing.T) {
	clock := newClock()
	src := fixture(clock, charging, pawnSensor(&fakeMSR{}, clock))
	if src.CPUPackageStatus() != StatusAvailable || src.PlatformStatus() != "可用" {
		t.Error("available sources say so")
	}
	channels := src.EMIChannels()
	channels[0] = "changed"
	if src.EMIChannels()[0] != "RAPL_Package0_PKG" || len(src.EMIDiscoveryErrors()) != 0 {
		t.Error("callers get copies of the lists")
	}
}

func TestProbeReport(t *testing.T) {
	clock := newClock()
	src := fixture(clock, charging, pawnSensor(&fakeMSR{pkgStep: 36 * 16384, psysStep: 45 * 16384}, clock))
	got := probe(src, 2, true, clock.sleep)
	const battery = "27.98 [available=true; window=unknown; source=WMI BatteryStatus；固件采样窗口与内部时间戳未知]"
	const rest = " 36.00 [available=true; window=1.000s; source=EMI RAPL_Package0_PKG]" +
		" 45.00 [available=true; window=1.000s; source=PawnIO MSR 0x64D]" +
		" ≈ 72.98 [available=true; window=1.000s; source=平台功率 + 电池端净功率] (估算整机输入功率)"
	const raw = "  raw battery: ChargeRate_mW=27980; DischargeRate_mW=0; Voltage_mV=16500; PowerOnline=True; Charging=True; Discharging=False (4294967295=unknown)"
	want := strings.Join([]string{
		"Power source discovery",
		"  Elevated token: True",
		"  Timestamp is application observation time, not a BMS timestamp.",
		"  Battery firmware window / BMS internal timestamp: unknown.",
		"  CPU EMI cross-check validates energy units, not Psys rail coverage or total accuracy.",
		"  CPU package (EMI): 可用",
		"  Platform (PawnIO): 可用",
		"  EMI channels: RAPL_Package0_PKG, RAPL_Package0_DRAM",
		"  EMI discovery errors: (none)",
		"",
		"t       Battery_W      CpuPkg_W       Platform_W     WholeSystem_W",
		"timestamp=2026-11-19T20:00:02.0000000+08:00; elapsed=2.000s; supply=ExternalPowerCharging; active_batteries=1; battery_voltage=16.500 V; battery_current_estimate=≈ 1.696 A",
		raw,
		"2.000s  " + battery + rest,
		"timestamp=2026-11-19T20:00:03.0000000+08:00; elapsed=3.000s; supply=ExternalPowerCharging; active_batteries=1; battery_voltage=16.500 V; battery_current_estimate=≈ 1.696 A",
		raw,
		"3.000s  " + battery + rest,
		"",
	}, "\n")
	if got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}
}

func TestProbeReportUnavailable(t *testing.T) {
	clock := newClock()
	discharging := []wmiBattery{{Discharging: true, ChargeRate: 0, DischargeRate: 17080, Voltage: 15462}}
	src := fixture(clock, discharging, &pawnIOSensor{reason: power.ReasonElevationRequired})
	src.emi = newEMISensor(&clockEMI{clock: clock, start: clock.now(), pWhPerSec: 1e10, unreadables: []string{`\\?\emi#2`}})
	clock.t = clock.t.Add(100 * time.Millisecond)
	lines := strings.Split(probe(src, 1, false, clock.sleep), "\n")
	want := map[int]string{
		1:  "  Elevated token: False",
		6:  "  Platform (PawnIO): 需要以管理员身份运行才能读取平台功率",
		7:  "  EMI channels: RAPL_Package0_PKG, RAPL_Package0_DRAM",
		8:  `  EMI discovery errors: \\?\emi#2: IOCTL_EMI_GET_VERSION 失败`,
		11: "timestamp=2026-11-19T20:00:02.1000000+08:00; elapsed=2.100s; supply=BatteryDischarging; active_batteries=1; battery_voltage=15.462 V; battery_current_estimate=≈ 1.105 A",
		13: "2.100s  -17.08 [available=true; window=unknown; source=WMI BatteryStatus；固件采样窗口与内部时间戳未知]" +
			" 36.00 [available=true; window=1.000s; source=EMI RAPL_Package0_PKG] n/a (需要以管理员身份运行才能读取平台功率)" +
			" 17.08 [available=true; window=unknown; source=电池端放电功率] (系统负载功率)",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d:\n%q\nwant:\n%q", i, lines[i], w)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		sample *power.Sample
		whole  bool
		want   string
	}{
		{nil, false, "n/a"},
		{nil, true, "n/a"},
		{power.Unsupported(power.Platform, "未安装 PawnIO 驱动"), false, "n/a (未安装 PawnIO 驱动)"},
		{power.Unsupported(power.EstimatedSystemInput, "平台功率不可用"), true, "n/a (平台功率不可用)"},
		{power.FromValue(power.CPUPackage, power.Measured, 0.004, "EMI X", 1500*time.Microsecond), false,
			"0.00 [available=true; window=0.002s; source=EMI X]"},
		{power.FromValue(power.SystemLoad, power.Measured, 17.085, "电池端放电功率", 0), true,
			"17.09 [available=true; window=unknown; source=电池端放电功率] (系统负载功率)"},
	}
	for _, c := range cases {
		got := describe(c.sample)
		if c.whole {
			got = describeWholeSystem(c.sample)
		}
		if got != c.want {
			t.Errorf("describe(%+v) = %q, want %q", c.sample, got, c.want)
		}
	}
	if describeList(nil) != "(none)" || describeList([]string{"a", "b"}) != "a, b" {
		t.Error("lists")
	}
}

// Every reason a source can give has an English text, so the window shows it
// in either language.
func TestReasonsTranslate(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Select(i18n.System) })
	if err := i18n.Select(i18n.English); err != nil {
		t.Fatal(err)
	}
	reasons := []string{
		reasonFlagsConflict, reasonNoActiveBattery, reasonStatusUnavailable, reasonPackStatus, reasonPacksDisagree,
		reasonPackRate, reasonAggregateDirection, reasonChargeRate, reasonDischargeRate, reasonOfflineDirection,
		reasonRateUnavailable, reasonNoEMIDevice, reasonNoPackage, reasonEMIUnreadable + "x", reasonNoChannels,
		reasonEMIInitFailed + "x", reasonEMIOpenFailed, reasonEMIReadFailed + "x", reasonBaseline, reasonEMICounterReset,
		errMetadataVersion.Error(), errChannelTooShort.Error(), errChannelOutOfBounds.Error(),
		reasonNoPawnIO, reasonNoPawnIOLib, reasonPawnIOLibLoad, reasonPawnIOLibMissing, reasonNoModule,
		reasonModuleLoad + hresult(-1), reasonPawnIOInitFailed + "x", describeOpenFailure(-1, false),
		reasonMSRReadFailed + "x", reasonZeroWindow, reasonLongGap, reasonPsysNotEnabled, power.ReasonElevationRequired,
		"此数据源仅在 Windows 上可用",
	}
	for _, r := range reasons {
		if i18n.Diagnostic(r) == r {
			t.Errorf("%q has no English text", r)
		}
	}
}
