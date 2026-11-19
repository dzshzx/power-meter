package sensors

import (
	"fmt"
	"strings"
	"time"

	"github.com/dzshzx/power-meter/internal/num"
	"github.com/dzshzx/power-meter/internal/power"
)

// probeNewline ends every report line, as Windows text files do.
const probeNewline = "\r\n"

// Probe discovers the sources, takes one baseline sample and then one
// sample a second for seconds seconds (at least one), and returns the
// --power-probe report. Source availability is a per-machine hardware and
// firmware fact, so users and bug reports see the discovery result directly
// rather than inferring it from a greyed-out label. Reasons stay in their
// Chinese source form.
func Probe(seconds int) string {
	src := New()
	defer src.Close()
	return probe(src, max(seconds, 1), elevated(), time.Sleep)
}

func probe(src *Sources, seconds int, elevated bool, sleep func(time.Duration)) string {
	var b strings.Builder
	line := func(text string) { b.WriteString(text + probeNewline) }
	line("Power source discovery")
	line("  Elevated token: " + boolText(elevated))
	line("  Timestamp is application observation time, not a BMS timestamp.")
	line("  Battery firmware window / BMS internal timestamp: unknown.")
	line("  CPU EMI cross-check validates energy units, not Psys rail coverage or total accuracy.")
	line("  CPU package (EMI): " + src.CPUPackageStatus())
	line("  Platform (PawnIO): " + src.PlatformStatus())
	line("  EMI channels: " + describeList(src.EMIChannels()))
	line("  EMI discovery errors: " + describeList(src.EMIDiscoveryErrors()))
	line("")

	// The energy counters are cumulative, so one tick establishes the
	// baselines before any figure exists. It is taken outside the reported
	// window so that seconds is the number of rows.
	sleep(time.Second)
	src.Capture()

	line(columns("t", "Battery_W", "CpuPkg_W", "Platform_W", "WholeSystem_W"))
	for range seconds {
		sleep(time.Second)
		s := src.Capture()
		line(describeBattery(s))
		for _, raw := range s.Battery.RawReadings {
			line("  raw battery: " + raw)
		}
		line(columns(num.Fixed(s.ElapsedSeconds, 3)+"s", describe(s.BatteryTerminal), describe(s.CPUPackage),
			describe(s.Platform), describeWholeSystem(s.WholeSystem)))
	}
	return b.String()
}

func columns(t, battery, cpu, platform, whole string) string {
	return fmt.Sprintf("%-7s %-14s %-14s %-14s %s", t, battery, cpu, platform, whole)
}

// probeTimeLayout is the round-trip form of a local time with its offset.
const probeTimeLayout = "2006-01-02T15:04:05.0000000-07:00"

func describeBattery(s *power.Snapshot) string {
	b := s.Battery
	voltage, current := "n/a", "n/a"
	if b.VoltageAvailable {
		voltage = num.Fixed(b.VoltageVolts, 3) + " V"
	}
	if b.CurrentAvailable {
		current = "≈ " + num.Fixed(b.CurrentAmps, 3) + " A"
	}
	return fmt.Sprintf("timestamp=%s; elapsed=%ss; supply=%s; active_batteries=%d; battery_voltage=%s; battery_current_estimate=%s",
		s.Timestamp.Format(probeTimeLayout), num.Fixed(s.ElapsedSeconds, 3), b.SupplyState, b.ActiveBatteryCount, voltage, current)
}

func describeList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

// describeWholeSystem names the boundary beside the value, because the
// whole-system boundary changes with the supply state and a fixed column
// heading would imply the wrong one.
func describeWholeSystem(s *power.Sample) string {
	value := describe(s)
	if !s.IsAvailable() {
		return value
	}
	return value + " (" + s.Boundary.Label() + ")"
}

func describe(s *power.Sample) string {
	switch {
	case s == nil:
		return "n/a"
	case !s.Available:
		return "n/a (" + s.UnavailableReason + ")"
	}
	prefix := ""
	if s.Kind == power.Estimated {
		prefix = "≈ "
	}
	window := "unknown"
	if s.Window > 0 {
		window = num.Fixed(s.Window.Seconds(), 3) + "s"
	}
	return prefix + num.Fixed(s.Watts, 2) + " [available=true; window=" + window + "; source=" + s.Source + "]"
}
