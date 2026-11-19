//go:build windows

package sensors

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dzshzx/power-meter/internal/power"
)

// The live tests read this machine's sensors and run only with
// POWERMETER_LIVE=1, so builds and CI never depend on the hardware.
func live(t *testing.T) {
	t.Helper()
	if os.Getenv("POWERMETER_LIVE") != "1" {
		t.Skip("set POWERMETER_LIVE=1 to read this machine's sensors")
	}
}

// TestLiveProbe runs --power-probe for POWERMETER_PROBE_SECONDS (default
// 10) and writes the report to POWERMETER_PROBE_OUT when set.
func TestLiveProbe(t *testing.T) {
	live(t)
	seconds := 10
	if text := os.Getenv("POWERMETER_PROBE_SECONDS"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil {
			t.Fatal(err)
		}
		seconds = n
	}
	report := Probe(seconds)
	if out := os.Getenv("POWERMETER_PROBE_OUT"); out != "" {
		if err := os.WriteFile(out, []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if rows := strings.Count(report, "\ntimestamp="); rows != seconds {
		t.Errorf("%d rows for %d seconds:\n%s", rows, seconds, report)
	}
	t.Log("\n" + report)
}

// TestLiveCapture checks that every source reports either a figure or a
// reason once the baselines exist.
func TestLiveCapture(t *testing.T) {
	live(t)
	src := New()
	defer src.Close()
	t.Logf("EMI: %s; PawnIO: %s; elevated: %v", src.CPUPackageStatus(), src.PlatformStatus(), elevated())
	src.Capture()
	s := src.Capture()
	for name, sample := range map[string]*power.Sample{
		"battery": s.BatteryTerminal, "cpu": s.CPUPackage, "platform": s.Platform, "whole": s.WholeSystem,
	} {
		if !sample.Available && sample.UnavailableReason == "" {
			t.Errorf("%s has neither a figure nor a reason", name)
		}
		t.Logf("%s: %s", name, describe(sample))
	}
}
