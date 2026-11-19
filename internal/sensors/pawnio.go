package sensors

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
)

// Intel RAPL registers read through the PawnIO IntelMSR module. Energy
// status counters are 32 bits wide; MSR_RAPL_POWER_UNIT bits 12:8 hold the
// energy unit as a power of one half.
const (
	msrRaplPowerUnit        = 0x606
	msrPkgEnergyStatus      = 0x611
	msrPlatformEnergyStatus = 0x64D
	raplCounterMask         = 0xFFFFFFFF
	raplEnergyUnitShift     = 8
	raplEnergyUnitFieldMask = 0x1F
)

// hresultAccessDenied is E_ACCESSDENIED, 0x80070005.
const hresultAccessDenied = -2147024891

// platformSource names the platform figure's origin.
const platformSource = "PawnIO MSR 0x64D"

// PawnIO reasons: Chinese source strings, translated where shown.
const (
	reasonNoPawnIO         = "未安装 PawnIO 驱动"
	reasonNoPawnIOLib      = "PawnIO 安装目录缺少 PawnIOLib.dll"
	reasonPawnIOLibLoad    = "无法加载 PawnIOLib.dll"
	reasonPawnIOLibMissing = "PawnIOLib.dll 不可用"
	reasonNoModule         = "缺少 IntelMSR.bin 模块"
	reasonModuleLoad       = "加载 IntelMSR 模块失败: "
	reasonPawnIOInitFailed = "PawnIO 初始化失败: "
	reasonPawnIOOpenFailed = "pawnio_open 失败: "
	reasonMSRReadFailed    = "MSR 读取失败: "
	reasonZeroWindow       = "采样窗口为零"
	reasonLongGap          = "采样间隔过长，正在重新建立基准"
	reasonPsysNotEnabled   = "本机未启用 Psys 平台计数器"
)

// msrReader reads one model-specific register through an open PawnIO
// handle with the IntelMSR module loaded.
type msrReader interface {
	readMSR(msr uint32) (uint64, error)
	close()
}

// pawnIOSensor reads Intel RAPL energy counters through the PawnIO driver.
//
// It is the only route to MSR_PLATFORM_ENERGY_STATUS (Psys), which makes an
// input-power estimate possible. The driver is a separate, user-installed,
// officially signed package; this program never installs it and reports the
// platform boundary unavailable without it.
//
// Licensing: PawnIO is GPL-2.0 with an exception for independent modules
// that talk to it solely through its device IO control interface, which is
// what PawnIOLib does. PawnIOLib and the IntelMSR module are
// LGPL-2.1-or-later and used unmodified; PawnIOLib is loaded from the
// driver's installation, and IntelMSR.bin beside the executable replaces
// the embedded module.
type pawnIOSensor struct {
	reason     string
	msr        msrReader
	unitJoules float64
	// now and start time the RAPL window on the monotonic clock.
	now     func() time.Time
	start   time.Time
	tracker raplTracker
}

func (s *pawnIOSensor) available() bool { return s.reason == "" }

// read samples the platform and package counters over one shared window.
// The package figure lets the caller cross-check the energy unit against
// EMI's independent package figure. RAPL registers carry no timestamp, so
// the window is measured here. ok is false whenever platform has no value.
func (s *pawnIOSensor) read() (platform *power.Sample, packageWatts float64, ok bool) {
	if !s.available() || s.msr == nil {
		return power.Unsupported(power.Platform, s.reason), 0, false
	}
	pkg, err := s.msr.readMSR(msrPkgEnergyStatus)
	if err != nil {
		return power.Unsupported(power.Platform, reasonMSRReadFailed+err.Error()), 0, false
	}
	psys, err := s.msr.readMSR(msrPlatformEnergyStatus)
	if err != nil {
		return power.Unsupported(power.Platform, reasonMSRReadFailed+err.Error()), 0, false
	}
	return s.tracker.advance(pkg, psys, s.now().Sub(s.start), s.unitJoules)
}

func (s *pawnIOSensor) close() {
	if s.msr != nil {
		s.msr.close()
		s.msr = nil
	}
}

// energyUnitJoules decodes MSR_RAPL_POWER_UNIT's energy status unit.
func energyUnitJoules(raw uint64) float64 {
	return math.Pow(0.5, float64((raw>>raplEnergyUnitShift)&raplEnergyUnitFieldMask))
}

// describeOpenFailure explains a failed pawnio_open. PawnIO's device admits
// only SYSTEM and administrators (D:P(A;;GA;;;SY)(A;;GA;;;BA)), so access
// denied without elevation means "run as administrator", not a fault. The
// elevation state is consulted only after the failure, so a machine whose
// administrator widened that descriptor works without a special case.
func describeOpenFailure(hr int32, elevated bool) string {
	if hr == hresultAccessDenied && !elevated {
		return power.ReasonElevationRequired
	}
	return reasonPawnIOOpenFailed + hresult(hr)
}

// hresult formats an HRESULT as reports show it: 0x and eight upper-case
// hex digits.
func hresult(hr int32) string {
	return fmt.Sprintf("0x%08X", uint32(hr))
}

// loadModule returns IntelMSR.bin from dir when a file of that name is
// there, else the embedded copy (nil when there is none). The file beside
// the executable wins so a user can swap in their own build of this
// LGPL-licensed module, as its licence requires, or test a newer upstream
// one without a rebuild.
func loadModule(dir, name string, embedded []byte) ([]byte, error) {
	beside := filepath.Join(dir, name)
	if info, err := os.Stat(beside); err == nil && info.Mode().IsRegular() {
		return os.ReadFile(beside)
	}
	if len(embedded) == 0 {
		return nil, nil
	}
	return embedded, nil
}

// raplTracker turns successive RAPL counter readings into power. The
// counters are 32 bits wide and wrap; the window comes from the caller's
// monotonic clock.
type raplTracker struct {
	elapsed   time.Duration
	pkg, psys uint64
	ok        bool
}

// advance takes the counters read at elapsed (time since the sensor
// started). The new readings always become the baseline, so after a
// resume-sized gap the very next normal sample has a figure again.
func (t *raplTracker) advance(pkg, psys uint64, elapsed time.Duration, unitJoules float64) (*power.Sample, float64, bool) {
	if !t.ok {
		t.save(pkg, psys, elapsed)
		return power.Unsupported(power.Platform, reasonBaseline), 0, false
	}
	window := elapsed - t.elapsed
	seconds := window.Seconds()
	pkgDelta := (pkg - t.pkg) & raplCounterMask
	psysDelta := (psys - t.psys) & raplCounterMask
	t.save(pkg, psys, elapsed)

	switch {
	case seconds <= 0:
		return power.Unsupported(power.Platform, reasonZeroWindow), 0, false
	case seconds > meter.MaximumGap:
		return power.Unsupported(power.Platform, reasonLongGap), 0, false
	}
	packageWatts := float64(pkgDelta) * unitJoules / seconds
	if psysDelta == 0 {
		// A counter that never advances means the board does not route the
		// platform signal: a negative capability result, not zero watts.
		return power.Unsupported(power.Platform, reasonPsysNotEnabled), 0, false
	}
	platformWatts := float64(psysDelta) * unitJoules / seconds
	return power.FromValue(power.Platform, power.Measured, platformWatts, platformSource, window), packageWatts, true
}

func (t *raplTracker) save(pkg, psys uint64, elapsed time.Duration) {
	t.pkg, t.psys, t.elapsed, t.ok = pkg, psys, elapsed, true
}
