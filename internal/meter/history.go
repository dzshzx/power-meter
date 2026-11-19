// Package meter turns snapshots into what the window and tray show: the
// statistics, the thirty-minute trend and the localized view. Every display
// rule lives here, so it can be checked without a window.
package meter

import (
	"math"
	"strings"

	"github.com/dzshzx/power-meter/internal/num"
	"github.com/dzshzx/power-meter/internal/power"
)

// Mode is the boundary the headline, tray and statistics show.
type Mode int

const (
	WholeSystem Mode = iota
	Battery
)

// String is the stored name of the mode.
func (m Mode) String() string {
	if m == Battery {
		return "Battery"
	}
	return "WholeSystem"
}

// ParseMode reads a stored mode; anything but "Battery" is the default
// whole-system mode.
func ParseMode(stored string) Mode {
	if stored == "Battery" {
		return Battery
	}
	return WholeSystem
}

// Statistics windows, in seconds. The captions' translation keys spell out
// 10 and 5 minutes ("10 分钟均值", "/10分", "/5分"), so changing a window
// also means new keys.
const (
	MaximumGap    = 5.0
	AverageWindow = 600.0
	PeakWindow    = 300.0
	retention     = AverageWindow
)

type timedPower struct {
	seconds float64
	watts   float64
	ok      bool
}

// History is the selected boundary's recent readings for the 10-minute
// time-weighted mean and the 5-minute peak. A mode or supply-state change,
// time going backwards or a gap over MaximumGap starts it again.
type History struct {
	points []timedPower
	mode   Mode
	supply power.SupplyState
}

// Clear drops every reading.
func (h *History) Clear() { h.points = h.points[:0] }

// Len is the number of readings held.
func (h *History) Len() int { return len(h.points) }

// Add records one reading taken at seconds on the capture clock.
func (h *History) Add(seconds float64, mode Mode, supply power.SupplyState, sample *power.Sample) {
	if n := len(h.points); n > 0 {
		last := h.points[n-1].seconds
		if h.mode != mode || h.supply != supply || seconds <= last || seconds-last > MaximumGap {
			h.Clear()
		}
	}
	h.mode, h.supply = mode, supply
	watts, ok := sample.Value()
	h.points = append(h.points, timedPower{seconds: seconds, watts: watts, ok: ok})
	// Retain one predecessor for clipping the first weighted interval.
	drop := 0
	for len(h.points)-drop > 1 && h.points[drop+1].seconds <= seconds-retention {
		drop++
	}
	h.points = h.points[drop:]
}

// Duration is how much of a window the readings cover.
func (h *History) Duration(window float64) float64 {
	n := len(h.points)
	if n < 2 {
		return 0
	}
	return math.Min(window, h.points[n-1].seconds-h.points[0].seconds)
}

// Average is the 10-minute time-weighted mean and the seconds it covers. A
// left-held reading counts until the next one; an interval touching a missing
// reading is excluded rather than claiming a value through unknown time.
func (h *History) Average() (mean float64, ok bool, coverage float64) {
	n := len(h.points)
	if n < 2 || !h.points[n-1].ok {
		return 0, false, 0
	}
	cutoff := h.points[n-1].seconds - AverageWindow
	energy := 0.0
	for i := 1; i < n; i++ {
		left, right := h.points[i-1], h.points[i]
		duration := right.seconds - math.Max(left.seconds, cutoff)
		if duration <= 0 || !left.ok || !right.ok {
			continue
		}
		energy += left.watts * duration
		coverage += duration
	}
	if coverage <= 0 {
		return 0, false, coverage
	}
	return energy / coverage, true, coverage
}

// Peak is the largest-magnitude reading of the last 5 minutes, with its sign.
func (h *History) Peak() (float64, bool) {
	n := len(h.points)
	if n == 0 || !h.points[n-1].ok {
		return 0, false
	}
	cutoff := h.points[n-1].seconds - PeakWindow
	peak, found := 0.0, false
	for _, p := range h.points {
		if p.seconds >= cutoff && p.ok && (!found || power.ExceedsPeak(p.watts, peak)) {
			peak, found = p.watts, true
		}
	}
	return peak, found
}

// Minus is the typographic minus that display numbers use. CLI reports, logs
// and stored values keep ASCII.
const Minus = "−"

// Number formats a display number with decimals digits and the
// typographic minus.
func Number(v float64, decimals int) string {
	text := num.Fixed(v, decimals)
	if rest, ok := strings.CutPrefix(text, "-"); ok {
		return Minus + rest
	}
	return text
}
