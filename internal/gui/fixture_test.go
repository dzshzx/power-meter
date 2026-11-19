package gui

import (
	"math"
	"time"

	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
)

// Deterministic fixture readings, fed through power.Compose and the
// session as live readings are, so a thirty-minute trend costs one view.

var scenarios = []string{"idle", "charging", "discharging", "supplemented", "unavailable", "starting", "warming", "early"}

var scenarioSeconds = map[string]int{
	"idle": 600, "charging": 1800, "discharging": 1800, "supplemented": 60,
	"unavailable": 60, "starting": -1, "warming": 120, "early": 240,
}

func noise(tick, salt int) float64 {
	x := uint32(tick)*374761393 + uint32(salt)*668265263
	x = (x ^ (x >> 13)) * 1274126177
	return float64((x^(x>>16))&0xffff)/65535.0 - 0.5
}

// chargingScenario: the full, warm-up and partial trends record a charging
// laptop.
func chargingScenario(scenario string) bool {
	return scenario == "charging" || scenario == "early" || scenario == "warming"
}

// platformWatts: a slow drift with a busy spell peaking near tick 1200 in
// the full charging trend.
func platformWatts(scenario string, tick, last int) float64 {
	if tick == last {
		return 19.39
	}
	t := float64(tick)
	if chargingScenario(scenario) {
		w := 18.6 + 1.6*math.Sin(t/97) + 0.9*math.Sin(t/23+1) + 0.9*noise(tick, 1) +
			13.5*math.Exp(-math.Pow((t-1200)/38, 2))
		if last-tick < 60 {
			w += 0.6 * math.Sin(t*0.43)
		}
		return w
	}
	return 19.39 + math.Sin(t*0.43)*2.4
}

func batteryWatts(scenario string, tick, last int) float64 {
	t := float64(tick)
	if chargingScenario(scenario) {
		if tick == last {
			return 39.08
		}
		return 40.2 - 1.2*t/1800 + 0.25*noise(tick, 2)
	}
	switch scenario {
	case "discharging":
		if tick == last {
			return -17.50
		}
		return -(16.4 + 1.1*math.Sin(t/61) + 0.6*math.Sin(t/13) + 0.5*noise(tick, 3) +
			9.0*math.Exp(-math.Pow((t-1080)/30, 2)))
	case "supplemented":
		return -5
	}
	return 0
}

func fixtureState(scenario string) power.SupplyState {
	switch {
	case chargingScenario(scenario):
		return power.ExternalPowerCharging
	case scenario == "discharging":
		return power.BatteryDischarging
	case scenario == "supplemented":
		return power.ExternalPowerSupplemented
	}
	return power.ExternalPowerIdle
}

// fixture feeds a scenario into a fresh session and returns its last view
// with the mode it shows.
func fixture(scenario string) (*meter.View, meter.Mode) {
	mode := meter.WholeSystem
	if scenario == "discharging" {
		mode = meter.Battery
	}
	session := meter.NewSession(mode)
	last := scenarioSeconds[scenario]
	if last < 0 {
		return session.Render(), mode
	}
	end := time.Date(2026, 9, 23, 14, 38, 15, 0, time.FixedZone("", 8*3600))
	cpu := power.FromValue(power.CPUPackage, power.Measured, 10.1, "UI fixture", time.Second)
	var v *meter.View
	for tick := 0; tick <= last; tick++ {
		watts := batteryWatts(scenario, tick, last)
		b := power.NewBatteryReading()
		b.SupplyState = fixtureState(scenario)
		b.PowerWatts = watts
		// The discharge trend has an N/A spell where the firmware rate is
		// missing.
		b.RateAvailable = !(scenario == "discharging" && last >= 1800 && tick >= 300 && tick < 400)
		b.VoltageAvailable, b.CurrentAvailable = true, true
		b.VoltageVolts, b.CurrentAmps = 17.51, watts/17.51
		b.Percentage = 98
		if scenario == "discharging" {
			b.Percentage = 76
		}
		platform := power.FromValue(power.Platform, power.Measured, platformWatts(scenario, tick, last), "UI fixture", time.Second)
		if scenario == "unavailable" || (scenario == "charging" && tick >= 520 && tick < 640) {
			platform = power.Unsupported(power.Platform, power.ReasonElevationRequired)
		}
		at := end.Add(time.Duration(tick-last) * time.Second)
		v = session.Observe(power.Compose(at, float64(tick), b, cpu, platform))
	}
	return v, mode
}
