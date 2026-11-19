package sensors

import (
	"math"
	"testing"
	"time"

	"github.com/dzshzx/power-meter/internal/power"
)

func near(actual, expected float64) bool { return math.Abs(actual-expected) < 1e-9 }

// battery is the fixture reading of one pack: the state follows the flags
// and the sign of watts.
func battery(online, charging, rateAvailable bool, watts float64) *power.BatteryReading {
	r := power.NewBatteryReading()
	r.ActiveBatteryCount = 1
	switch {
	case charging:
		r.SupplyState = power.ExternalPowerCharging
	case watts < 0 && online:
		r.SupplyState = power.ExternalPowerSupplemented
	case watts < 0:
		r.SupplyState = power.BatteryDischarging
	case online:
		r.SupplyState = power.ExternalPowerIdle
	default:
		r.SupplyState = power.BatteryDirectionUnknown
	}
	r.RateAvailable = rateAvailable
	r.PowerWatts = watts
	return r
}

func TestRates(t *testing.T) {
	const unknown = math.MaxUint32
	cases := []struct {
		name                       string
		state                      power.SupplyState
		charge, discharge, voltage uint32
		rate                       bool
		watts                      float64
		reason                     string
		voltageOK, currentOK       bool
		volts, amps                float64
	}{
		{name: "charging, charge rate known", state: power.ExternalPowerCharging, charge: 43802, voltage: 12000,
			rate: true, watts: 43.802, voltageOK: true, currentOK: true, volts: 12, amps: 43.802 / 12},
		// The rate for the direction in effect is unknown, so nothing may be
		// presented as a measurement.
		{name: "charging, only discharge rate known", state: power.ExternalPowerCharging, charge: unknown,
			reason: "电池充电速率不可用"},
		{name: "discharging, discharge rate known", state: power.BatteryDischarging, discharge: 25000,
			rate: true, watts: -25},
		{name: "discharging, only charge rate known", state: power.BatteryDischarging, discharge: unknown,
			reason: "电池放电速率不可用"},
		{name: "supplementing on AC uses the discharge rate", state: power.ExternalPowerSupplemented,
			discharge: 5000, voltage: 15000, rate: true, watts: -5, voltageOK: true, currentOK: true, volts: 15, amps: 5.0 / 15},
		// Neither direction while external power is present is a real zero.
		{name: "online idle battery reports a known zero", state: power.ExternalPowerIdle, charge: unknown,
			discharge: unknown, rate: true},
		// Offline without a direction: calling that a measured zero would
		// fabricate a whole-system reading.
		{name: "offline battery without a direction is unavailable", state: power.BatteryDirectionUnknown,
			charge: unknown, discharge: unknown, voltage: 12000, reason: "离线状态下电池方向不明", voltageOK: true, volts: 12},
		{name: "contradictory flags have no rate", state: power.SupplyInconsistent, charge: 1000, discharge: 1000,
			reason: "电池供电标志互相矛盾"},
		{name: "no status has no rate", state: power.SupplyUnavailable, reason: "电池速率不可用"},
		{name: "unknown voltage", state: power.ExternalPowerCharging, charge: 1000, voltage: unknown, rate: true, watts: 1},
		{name: "zero voltage is unknown", state: power.ExternalPowerCharging, charge: 1000, rate: true, watts: 1},
	}
	for _, c := range cases {
		r := power.NewBatteryReading()
		r.SupplyState = c.state
		resolveRate(r, c.charge, c.discharge)
		resolveElectricalDetails(r, c.voltage)
		if r.RateAvailable != c.rate || !near(r.PowerWatts, c.watts) || r.RateUnavailableReason != c.reason ||
			r.VoltageAvailable != c.voltageOK || r.CurrentAvailable != c.currentOK ||
			!near(r.VoltageVolts, c.volts) || !near(r.CurrentAmps, c.amps) {
			t.Errorf("%s: %+v", c.name, r)
		}
	}
}

func TestBatteryAggregation(t *testing.T) {
	first := battery(false, false, true, -10)
	second := battery(false, false, true, -15)
	first.Percentage, second.Percentage = 12, 88
	combined := combineReadings([]*power.BatteryReading{first, second})
	assignSystemPercentage(combined, 64)
	if !combined.StatusAvailable() || combined.ActiveBatteryCount != 2 || !combined.RateAvailable ||
		!combined.Discharging() || combined.Percentage != 64 || !near(combined.PowerWatts, -25) ||
		combined.VoltageAvailable || combined.CurrentAvailable {
		t.Errorf("two active batteries aggregate terminal power: %+v", combined)
	}

	unavailable := combineReadings(nil)
	assignSystemPercentage(unavailable, 64)
	if unavailable.Profile().StateText != "BATTERY UNAVAILABLE" || unavailable.Percentage != -1 ||
		unavailable.StatusUnavailableReason != "本机没有活动电池" || unavailable.RateUnavailableReason != "电池状态不可用" {
		t.Errorf("missing active battery is not labelled on battery: %+v", unavailable)
	}

	single := battery(true, true, true, 10)
	single.ActiveBatteryCount = 0
	if r := combineReadings([]*power.BatteryReading{single}); r != single || r.ActiveBatteryCount != 1 {
		t.Errorf("one pack is the reading itself: %+v", r)
	}
	if r := combineReadings([]*power.BatteryReading{nil}); r.StatusAvailable() || r.StatusUnavailableReason != "电池状态不可用" {
		t.Errorf("one pack without a reading: %+v", r)
	}
	assignSystemPercentage(nil, 50)
	inconsistent := battery(true, true, true, 1)
	inconsistent.SupplyState = power.SupplyInconsistent
	for _, pct := range []int{-1, 101} {
		if assignSystemPercentage(single, pct); single.Percentage != -1 {
			t.Errorf("percentage %d is unknown", pct)
		}
	}
	if assignSystemPercentage(inconsistent, 50); inconsistent.Percentage != -1 {
		t.Error("a reading without a status has no percentage")
	}
}

func TestBatteryAggregationWithoutRates(t *testing.T) {
	charging := battery(true, true, true, 10)
	noRate := battery(true, true, false, 0)
	noRate.RateUnavailableReason = "电池充电速率不可用"
	combined := combineReadings([]*power.BatteryReading{charging, noRate})
	if combined.RateAvailable || combined.RateUnavailableReason != "电池充电速率不可用" ||
		combined.SupplyState != power.ExternalPowerCharging {
		t.Errorf("a pack without a rate keeps its reason and the shared direction: %+v", combined)
	}

	idle := battery(true, false, false, 0)
	combined = combineReadings([]*power.BatteryReading{charging, idle})
	if combined.RateAvailable || combined.RateUnavailableReason != "至少一块电池速率不可用" ||
		combined.SupplyState != power.ExternalPowerDirectionUnknown {
		t.Errorf("packs without a rate in different directions leave the direction unknown: %+v", combined)
	}

	offline := battery(false, false, false, 0)
	discharging := battery(false, false, true, -3)
	combined = combineReadings([]*power.BatteryReading{offline, discharging})
	if combined.RateAvailable || combined.SupplyState != power.BatteryDirectionUnknown {
		t.Errorf("offline packs in different directions leave the direction unknown: %+v", combined)
	}
}

func TestBatteryAggregationInconsistent(t *testing.T) {
	cases := []struct {
		name   string
		packs  []*power.BatteryReading
		rate   bool
		reason string
	}{
		{"packs disagreeing on external power are inconsistent",
			[]*power.BatteryReading{battery(true, true, true, 10), battery(false, false, true, -5)}, false, "多块电池报告的供电状态不一致"},
		{"a missing pack status makes the aggregate inconsistent",
			[]*power.BatteryReading{battery(false, false, true, -5), nil}, false, "至少一块电池状态不可用"},
		{"aggregate power flowing in without external power is inconsistent",
			[]*power.BatteryReading{battery(false, false, true, 3), battery(false, false, true, 2)}, false, "聚合后的电池功率方向与供电状态不一致"},
	}
	for _, c := range cases {
		combined := combineReadings(c.packs)
		if combined.SupplyState != power.SupplyInconsistent || combined.RateAvailable != c.rate ||
			combined.StatusUnavailableReason != c.reason {
			t.Errorf("%s: %+v", c.name, combined)
		}
	}
}

func TestNoActiveBatteryKeepsItsReason(t *testing.T) {
	platform := power.FromValue(power.Platform, power.Measured, 24, "test", time.Second)
	noBattery := combineReadings(nil)
	whole := power.Compose(time.Now(), 0, noBattery, nil, platform).WholeSystem
	if whole.Available || whole.UnavailableReason != noBattery.StatusUnavailableReason {
		t.Errorf("no active battery preserves its status reason: %+v", whole)
	}
}

func TestBatteryReadingFromWMI(t *testing.T) {
	r := batteryReading([]wmiBattery{{PowerOnline: true, Charging: true, ChargeRate: 27980, DischargeRate: 0, Voltage: 16500}}, 57)
	if r.SupplyState != power.ExternalPowerCharging || !r.RateAvailable || !near(r.PowerWatts, 27.98) ||
		!near(r.VoltageVolts, 16.5) || r.Percentage != 57 || len(r.RawReadings) != 1 ||
		r.RawReadings[0] != "ChargeRate_mW=27980; DischargeRate_mW=0; Voltage_mV=16500; PowerOnline=True; Charging=True; Discharging=False (4294967295=unknown)" {
		t.Errorf("one charging pack: %+v", r)
	}

	r = batteryReading([]wmiBattery{{Charging: true, Discharging: true, ChargeRate: unknownRate, DischargeRate: unknownRate, Voltage: unknownRate}}, 57)
	if r.StatusAvailable() || r.StatusUnavailableReason != "电池供电标志互相矛盾" || r.Percentage != -1 ||
		r.RawReadings[0] != "ChargeRate_mW=4294967295; DischargeRate_mW=4294967295; Voltage_mV=4294967295; PowerOnline=False; Charging=True; Discharging=True (4294967295=unknown)" {
		t.Errorf("contradictory flags: %+v", r)
	}

	r = batteryReading([]wmiBattery{
		{Discharging: true, DischargeRate: 10000, Voltage: 15000},
		{Discharging: true, DischargeRate: 7000, Voltage: 14000},
	}, 40)
	if r.ActiveBatteryCount != 2 || !near(r.PowerWatts, -17) || len(r.RawReadings) != 2 || r.Percentage != 40 {
		t.Errorf("two discharging packs keep both raw readings: %+v", r)
	}
}

func TestWMIValues(t *testing.T) {
	uints := []struct {
		in   any
		want uint32
	}{
		{nil, unknownRate},
		{int32(-1), unknownRate}, // CIM uint32 arrives as a signed VARIANT
		{int32(28388), 28388},
		{uint32(15462), 15462},
		{int64(-5), unknownRate},
		{int64(1 << 33), unknownRate},
		{uint8(7), 7},
		{"4294967295", unknownRate},
		{"12", 12},
		{"x", unknownRate},
		{3.5, unknownRate},
	}
	for _, c := range uints {
		if got := wmiUint32(c.in); got != c.want {
			t.Errorf("wmiUint32(%#v) = %d, want %d", c.in, got, c.want)
		}
	}
	bools := []struct {
		in   any
		want bool
	}{
		{nil, false}, {true, true}, {false, false}, {int32(1), true}, {int32(0), false}, {"True", true}, {"no", false}, {1.0, false},
	}
	for _, c := range bools {
		if got := wmiBool(c.in); got != c.want {
			t.Errorf("wmiBool(%#v) = %v, want %v", c.in, got, c.want)
		}
	}
}
