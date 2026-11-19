package meter

import (
	"math"
	"testing"
	"time"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/power"
)

func near(actual, expected float64) bool { return math.Abs(actual-expected) < 1e-9 }

func check(t *testing.T, name string, condition bool) {
	t.Helper()
	if !condition {
		t.Error(name)
	}
}

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

func value(w float64) *power.Sample {
	return power.FromValue(power.BatteryTerminal, power.Measured, w, "test", 0)
}

func gap() *power.Sample { return power.Unsupported(power.BatteryTerminal, "gap") }

func platformOf(w float64) *power.Sample {
	return power.FromValue(power.Platform, power.Measured, w, "fixture", time.Second)
}

func snapshotAt(b *power.BatteryReading, platform *power.Sample, elapsed float64) *power.Snapshot {
	return power.Compose(time.Now(), elapsed, b, nil, platform)
}

// eachLanguage runs a test once in Chinese and once in English, so a rule
// never depends on the language.
func eachLanguage(t *testing.T, test func(t *testing.T)) {
	for _, language := range []string{i18n.Chinese, i18n.English} {
		t.Run(language, func(t *testing.T) {
			if err := i18n.Select(language); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = i18n.Select(i18n.System) })
			test(t)
		})
	}
}
