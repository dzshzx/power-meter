package settings

import (
	"testing"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
)

func TestDefaultsAndRoundTrip(t *testing.T) {
	if testing.Short() || !isolated {
		t.Skip("the Windows registry is the user's; run the round trip off Windows")
	}
	if Mode() != meter.WholeSystem || Language() != i18n.System {
		t.Fatal("a fresh install shows whole-system power in the system language")
	}
	if err := SaveMode(meter.Battery); err != nil || Mode() != meter.Battery {
		t.Error("the battery mode round-trips")
	}
	if err := SaveLanguage(i18n.English); err != nil || Language() != i18n.English {
		t.Error("the language round-trips")
	}
	if err := write(language, "fr"); err != nil || Language() != i18n.System {
		t.Error("an unknown language falls back to the system's")
	}
}
