//go:build !windows

package sensors

import (
	"errors"

	"github.com/dzshzx/power-meter/internal/power"
)

// reasonWindowsOnly is every source's reason off Windows.
const reasonWindowsOnly = "此数据源仅在 Windows 上可用"

// New returns sources whose every figure is unavailable: the battery, EMI
// and PawnIO interfaces exist only on Windows.
func New() *Sources {
	battery := func() (*power.BatteryReading, error) { return nil, errors.New(reasonWindowsOnly) }
	return newSources(battery, &emiSensor{reason: reasonWindowsOnly, packageIndex: -1},
		&pawnIOSensor{reason: reasonWindowsOnly})
}

func elevated() bool { return false }
