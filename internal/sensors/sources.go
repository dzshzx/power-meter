// Package sensors reads the machine's power sources and reports what this
// machine can and cannot measure. Terms follow GLOSSARY.md; which Windows
// interfaces can supply which boundary is in
// docs/research/system-input-power.md.
//
// Three sources feed one power.Snapshot per tick:
//
//   - the battery terminal, from WMI root\WMI BatteryStatus (all active
//     packs combined) and the system's charge percentage;
//   - the CPU package, from the Energy Meter Interface (EMI), which needs no
//     driver or elevation;
//   - the platform (Intel Psys, MSR 0x64D), through the separately installed
//     PawnIO driver and its IntelMSR module, which needs an elevated token.
//     Its energy unit is cross-checked against EMI by power.ValidatePlatform.
//
// Every source is discovered at runtime and degrades to an unavailable
// reason, never to a substituted number. Use:
//
//	src := sensors.New()
//	defer src.Close()
//	snapshot := src.Capture() // once per second
//
// Probe produces the --power-probe report. Off Windows, New returns sources
// whose every figure is unavailable, so callers build on any platform.
package sensors

import (
	"slices"
	"sync"
	"time"

	"github.com/dzshzx/power-meter/internal/power"
)

// StatusAvailable is the status of a source that was found and opened.
const StatusAvailable = "可用"

// Sources owns the optional power sources and captures one snapshot per
// tick. Its methods are safe for concurrent use.
type Sources struct {
	mu      sync.Mutex
	battery func() (*power.BatteryReading, error)
	emi     *emiSensor
	pawnIO  *pawnIOSensor
	start   time.Time
	now     func() time.Time
}

func newSources(battery func() (*power.BatteryReading, error), emi *emiSensor, pawnIO *pawnIOSensor) *Sources {
	return &Sources{battery: battery, emi: emi, pawnIO: pawnIO, start: time.Now(), now: time.Now}
}

// Capture reads every source once and composes the snapshot. A battery
// query that fails becomes an unavailable battery reading with the error.
func (s *Sources) Capture() *power.Snapshot {
	battery, err := s.battery()
	if err != nil {
		battery = power.NewBatteryReading()
		battery.StatusUnavailableReason = err.Error()
		battery.RateUnavailableReason = err.Error()
	}
	return s.read(battery)
}

// read samples EMI and PawnIO, validates the platform figure's energy unit
// against EMI and composes the snapshot with battery.
func (s *Sources) read(battery *power.BatteryReading) *power.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	timestamp := s.now()
	elapsed := timestamp.Sub(s.start).Seconds()
	cpuPackage := s.emi.read()
	platform, msrPackageWatts, ok := s.pawnIO.read()
	if ok {
		platform = power.ValidatePlatform(platform, cpuPackage, msrPackageWatts)
	}
	return power.Compose(timestamp, elapsed, battery, cpuPackage, platform)
}

// CPUPackageStatus is StatusAvailable or why EMI cannot supply CPU package
// power.
func (s *Sources) CPUPackageStatus() string { return status(s.emi.reason) }

// PlatformStatus is StatusAvailable or why PawnIO cannot supply platform
// power; power.ReasonElevationRequired means restarting elevated fixes it.
func (s *Sources) PlatformStatus() string { return status(s.pawnIO.reason) }

func status(reason string) string {
	if reason == "" {
		return StatusAvailable
	}
	return reason
}

// EMIChannels lists the EMI channels of the device in use, or of every
// device inspected when none carried a package channel. A machine exposing
// only RAPL channels cannot report input power directly, and this list is
// how a report shows that.
func (s *Sources) EMIChannels() []string { return slices.Clone(s.emi.discovered) }

// EMIDiscoveryErrors lists the EMI devices that could not be read, each as
// "path: error".
func (s *Sources) EMIDiscoveryErrors() []string { return slices.Clone(s.emi.errors) }

// Close releases the PawnIO handle. Capture must not be called afterwards.
func (s *Sources) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pawnIO.close()
}
