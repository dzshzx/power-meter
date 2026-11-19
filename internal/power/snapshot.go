package power

import (
	"fmt"
	"math"
	"time"

	"github.com/dzshzx/power-meter/internal/num"
)

// Snapshot is one tick's worth of power figures, each carrying its boundary.
// It is built only by Compose, so the battery terminal and whole-system
// figures follow the same rules in production and in tests.
type Snapshot struct {
	Timestamp       time.Time
	ElapsedSeconds  float64
	Battery         *BatteryReading
	BatteryTerminal *Sample
	CPUPackage      *Sample
	Platform        *Sample
	// WholeSystem is measured system load on battery and estimated system
	// input on external power: read its Boundary rather than assuming either.
	WholeSystem *Sample
}

// Compose builds a snapshot. The platform sample must already be
// cross-validated (or unsupported); validation needs the MSR package reading
// and belongs to capture.
func Compose(timestamp time.Time, elapsedSeconds float64, battery *BatteryReading, cpuPackage, platform *Sample) *Snapshot {
	return &Snapshot{
		Timestamp:       timestamp,
		ElapsedSeconds:  elapsedSeconds,
		Battery:         battery,
		BatteryTerminal: batterySample(battery),
		CPUPackage:      cpuPackage,
		Platform:        platform,
		WholeSystem:     deriveWholeSystem(platform, battery),
	}
}

func batterySample(battery *BatteryReading) *Sample {
	switch {
	case battery == nil:
		return Unsupported(BatteryTerminal, "电池状态不可用")
	case !battery.StatusAvailable():
		return Unsupported(BatteryTerminal, battery.StatusUnavailableReason)
	case !battery.RateAvailable:
		return Unsupported(BatteryTerminal, battery.RateUnavailableReason)
	}
	return FromValue(BatteryTerminal, Measured, battery.PowerWatts, "WMI BatteryStatus；固件采样窗口与内部时间戳未知", 0)
}

// deriveWholeSystem answers the whole-machine question, which is a different
// question in each supply state and therefore a different boundary.
//
// On battery every watt the machine uses leaves the battery terminals, so
// total consumption is measured outright; input power is zero by definition.
//
// On external power input is estimated as platform power plus the signed
// battery power. An adapter at its limit lets the battery supplement the
// load, and clamping that term to zero would overstate what the port
// supplies. The sum still omits charging-path and conversion losses, so it
// reads low and stays an estimate.
func deriveWholeSystem(platform *Sample, battery *BatteryReading) *Sample {
	if battery == nil {
		return Unsupported(SystemLoad, "电池状态不可用")
	}
	if !battery.StatusAvailable() {
		reason := battery.StatusUnavailableReason
		if reason == "" {
			reason = "电池状态不可用"
		}
		return Unsupported(SystemLoad, reason)
	}
	if !battery.PowerOnline() {
		if !battery.RateAvailable {
			return Unsupported(SystemLoad, "电池放电速率不可用")
		}
		return FromValue(SystemLoad, Measured, -battery.PowerWatts, "电池端放电功率", 0)
	}
	if platform == nil {
		return Unsupported(EstimatedSystemInput, "平台功率不可用")
	}
	if !platform.Available {
		return Unsupported(EstimatedSystemInput, platform.UnavailableReason)
	}
	if !battery.RateAvailable {
		return Unsupported(EstimatedSystemInput, "电池速率不可用")
	}
	estimated := platform.Watts + battery.PowerWatts
	if math.IsNaN(estimated) || math.IsInf(estimated, 0) {
		return Unsupported(EstimatedSystemInput, "估算结果不是有限数值")
	}
	if estimated < 0 {
		return Unsupported(EstimatedSystemInput, "估算结果为负，数据边界或采样窗口不一致")
	}
	return FromValue(EstimatedSystemInput, Estimated, estimated, "平台功率 + 电池端净功率", platform.Window)
}

// A wrong energy-unit assumption shows up as a large multiplicative error,
// so the tolerance only has to catch that while staying loose enough for the
// ordinary skew between two sensors sampling slightly different windows.
const (
	absoluteToleranceWatts = 1.5
	relativeTolerance      = 0.08
)

// ValidatePlatform confirms the RAPL energy unit by checking the MSR package
// figure against the EMI package figure, which comes from a wholly separate
// interface. A disagreement withholds the value. Missing EMI is not a Psys
// failure: the reading stays available, labelled as not cross-validated.
func ValidatePlatform(platform, emiPackage *Sample, msrPackageWatts float64) *Sample {
	if !emiPackage.IsAvailable() {
		validated := *platform
		if validated.Source == "" {
			validated.Source = "未交叉验证"
		} else {
			validated.Source += "（EMI 未交叉验证）"
		}
		return &validated
	}
	difference := math.Abs(msrPackageWatts - emiPackage.Watts)
	tolerance := math.Max(absoluteToleranceWatts, math.Abs(emiPackage.Watts)*relativeTolerance)
	if difference > tolerance {
		return Unsupported(Platform, fmt.Sprintf("单位换算校验失败（MSR %s W vs EMI %s W）",
			num.Fixed(msrPackageWatts, 2), num.Fixed(emiPackage.Watts, 2)))
	}
	return platform
}
