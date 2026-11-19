package power

import (
	"math"
	"strings"
	"testing"
	"time"
)

func near(actual, expected float64) bool { return math.Abs(actual-expected) < 1e-9 }

// battery is the fixture reading of one pack: the state follows the flags
// and the sign of watts.
func battery(online, charging, rateAvailable bool, watts float64) *BatteryReading {
	r := NewBatteryReading()
	r.ActiveBatteryCount = 1
	switch {
	case charging:
		r.SupplyState = ExternalPowerCharging
	case watts < 0 && online:
		r.SupplyState = ExternalPowerSupplemented
	case watts < 0:
		r.SupplyState = BatteryDischarging
	case online:
		r.SupplyState = ExternalPowerIdle
	default:
		r.SupplyState = BatteryDirectionUnknown
	}
	r.RateAvailable = rateAvailable
	r.PowerWatts = watts
	return r
}

func whole(platform *Sample, b *BatteryReading) *Sample {
	return Compose(time.Now(), 0, b, nil, platform).WholeSystem
}

func TestSupplyStates(t *testing.T) {
	inconsistent := SupplyFromFlags(false, true, true)
	if inconsistent != SupplyInconsistent || SupplyFromFlags(false, true, false) != SupplyInconsistent ||
		inconsistent.Profile().StatusAvailable {
		t.Error("contradictory battery flags form one invalid state")
	}
	supplemented := SupplyFromFlags(true, false, true)
	p := supplemented.Profile()
	if supplemented != ExternalPowerSupplemented || !p.StatusAvailable || !p.PowerOnline || p.Charging ||
		!p.Discharging || p.StateText != "DISCHARGING" || p.TrayMode != "Discharging" || p.Accent != AccentDischarging {
		t.Errorf("one supply profile drives window and tray presentation: %+v", p)
	}
	cases := []struct {
		online, charging, discharging bool
		want                          SupplyState
	}{
		{true, false, false, ExternalPowerIdle},
		{true, true, false, ExternalPowerCharging},
		{false, false, true, BatteryDischarging},
		{false, false, false, BatteryDirectionUnknown},
		{true, true, true, SupplyInconsistent},
	}
	for _, c := range cases {
		if got := SupplyFromFlags(c.online, c.charging, c.discharging); got != c.want {
			t.Errorf("flags %v/%v/%v = %v, want %v", c.online, c.charging, c.discharging, got, c.want)
		}
	}
	if SupplyState(99).Profile().StateText != "BATTERY UNAVAILABLE" {
		t.Error("unknown state describes Unavailable")
	}
}

func TestWholeSystem(t *testing.T) {
	platform := FromValue(Platform, Measured, 24.0, "test", time.Second)

	// On battery the machine draws nothing through the port, so the platform
	// figure must not be presented as input power.
	r := whole(platform, battery(false, false, true, -26.5))
	if !r.Available || r.Boundary != SystemLoad || r.Kind != Measured || !near(r.Watts, 26.5) {
		t.Errorf("on battery reports measured system load: %+v", r)
	}
	if r = whole(platform, battery(false, false, false, 0)); r.Available {
		t.Error("on battery without a rate is unsupported")
	}
	r = whole(platform, battery(true, true, true, 13.27))
	if !r.Available || r.Boundary != EstimatedSystemInput || r.Kind != Estimated || !near(r.Watts, 37.27) {
		t.Errorf("on AC while charging estimates input: %+v", r)
	}
	// Battery supplementation: clamping the battery term to zero would
	// overstate input.
	if r = whole(platform, battery(true, false, true, -5)); !r.Available || !near(r.Watts, 19) {
		t.Errorf("on AC while battery supplements subtracts: %+v", r)
	}
	r = whole(platform, battery(true, false, true, -30))
	if r.Available || r.UnavailableReason != "估算结果为负，数据边界或采样窗口不一致" {
		t.Errorf("negative estimated input is rejected: %+v", r)
	}
	if r = whole(platform, battery(true, false, true, 0)); !r.Available || !near(r.Watts, 24) {
		t.Errorf("on AC with an idle battery equals platform: %+v", r)
	}
	missing := Unsupported(Platform, "驱动未安装")
	if r = whole(missing, battery(true, true, true, 13.27)); r.Available || r.UnavailableReason != "驱动未安装" {
		t.Errorf("on AC without platform power keeps its reason: %+v", r)
	}
	if r = whole(nil, battery(true, true, true, 13.27)); r.Available || r.UnavailableReason != "平台功率不可用" {
		t.Errorf("on AC without a platform sample: %+v", r)
	}
	if r = whole(platform, battery(true, true, false, 0)); r.Available {
		t.Error("on AC without a battery rate is unsupported")
	}
	if r = whole(platform, nil); r.Available || r.Boundary != SystemLoad {
		t.Error("missing battery reading is unsupported")
	}
	noBattery := NewBatteryReading()
	noBattery.StatusUnavailableReason = "本机没有活动电池"
	if r = whole(platform, noBattery); r.Available || r.UnavailableReason != "本机没有活动电池" {
		t.Errorf("no active battery preserves its status reason: %+v", r)
	}
	if r = whole(platform, NewBatteryReading()); r.UnavailableReason != "电池状态不可用" {
		t.Errorf("a status without a reason says the status is unavailable: %+v", r)
	}
}

func TestBatteryTerminal(t *testing.T) {
	s := Compose(time.Now(), 0, battery(true, true, true, 13), nil, nil).BatteryTerminal
	if !s.Available || s.Boundary != BatteryTerminal || s.Kind != Measured || !near(s.Watts, 13) {
		t.Errorf("battery terminal: %+v", s)
	}
	noRate := battery(true, true, false, 0)
	noRate.RateUnavailableReason = "电池充电速率不可用"
	if s = Compose(time.Now(), 0, noRate, nil, nil).BatteryTerminal; s.Available || s.UnavailableReason != "电池充电速率不可用" {
		t.Errorf("a missing rate keeps its reason: %+v", s)
	}
	if s = Compose(time.Now(), 0, nil, nil, nil).BatteryTerminal; s.Available || s.UnavailableReason != "电池状态不可用" {
		t.Errorf("a missing reading: %+v", s)
	}
}

func TestValidation(t *testing.T) {
	platform := FromValue(Platform, Measured, 24, "PawnIO MSR 0x64D", time.Second)
	missingEMI := Unsupported(CPUPackage, "本机没有 EMI 电表设备")
	v := ValidatePlatform(platform, missingEMI, 12)
	if !v.Available || !near(v.Watts, 24) || !strings.Contains(v.Source, "未交叉验证") {
		t.Errorf("Psys remains available without EMI cross-check: %+v", v)
	}
	emi := FromValue(CPUPackage, Measured, 12.02, "EMI", time.Second)
	if v = ValidatePlatform(platform, emi, 12); !v.Available {
		t.Error("an agreeing package figure keeps Psys")
	}
	v = ValidatePlatform(platform, emi, 24.5)
	if v.Available || v.UnavailableReason != "单位换算校验失败（MSR 24.50 W vs EMI 12.02 W）" {
		t.Errorf("a disagreeing package figure withholds Psys: %+v", v)
	}
	if v = ValidatePlatform(&Sample{Available: true, Boundary: Platform}, nil, 0); v.Source != "未交叉验证" {
		t.Errorf("a source-less reading is labelled: %+v", v)
	}
}

func TestSamples(t *testing.T) {
	if FromValue(BatteryTerminal, Measured, math.NaN(), "", 0).Available ||
		FromValue(BatteryTerminal, Measured, math.Inf(1), "", 0).Available {
		t.Error("nonfinite sensor values cannot reach readings or tray")
	}
	if Unsupported(EstimatedSystemInput, "x").Kind != Estimated || Unsupported(Platform, "x").Kind != Measured {
		t.Error("an unsupported estimate stays an estimate")
	}
	if !ExceedsPeak(-20, 10) || ExceedsPeak(10, -10) {
		t.Error("peak rule: larger magnitude wins, a tie keeps the earlier")
	}
	if Boundary(99).Label() != "未知口径" {
		t.Error("unknown boundary label")
	}
}
