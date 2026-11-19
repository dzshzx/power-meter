package meter

import (
	"math"

	"github.com/dzshzx/power-meter/internal/power"
)

// TrendWindow is the trend's span in seconds.
const TrendWindow = 1800.0

// trendPoint is one recorded second. A missing value is an N/A reading;
// breakBefore marks a capture failure or a timing gap before it, so the line
// never bridges unknown time.
type trendPoint struct {
	seconds     float64
	whole       float64
	wholeOK     bool
	battery     float64
	batteryOK   bool
	breakBefore bool
}

// Trend is the last thirty minutes of both display boundaries, recorded from
// the first reading whatever the selected mode. It lives in memory only: a
// restart starts an empty trend.
type Trend struct {
	points       []trendPoint
	firstSeconds float64
	hasFirst     bool
	pendingBreak bool
}

// Count is the number of readings held.
func (t *Trend) Count() int { return len(t.points) }

// Add records one reading of both boundaries.
func (t *Trend) Add(seconds float64, whole, battery *power.Sample) {
	if n := len(t.points); n > 0 && seconds <= t.points[n-1].seconds {
		// The capture clock restarted: earlier times no longer line up.
		t.points = t.points[:0]
		t.hasFirst = false
	}
	gap := t.pendingBreak
	if n := len(t.points); n > 0 && seconds-t.points[n-1].seconds > MaximumGap {
		gap = true
	}
	t.pendingBreak = false
	if !t.hasFirst {
		t.firstSeconds, t.hasFirst = seconds, true
	}
	p := trendPoint{seconds: seconds, breakBefore: gap}
	p.whole, p.wholeOK = whole.Value()
	p.battery, p.batteryOK = battery.Value()
	t.points = append(t.points, p)
	drop := 0
	for drop < len(t.points)-1 && t.points[drop].seconds < seconds-TrendWindow {
		drop++
	}
	t.points = t.points[drop:]
}

// Break marks a failed capture: the next reading starts a new line.
func (t *Trend) Break() {
	if len(t.points) > 0 {
		t.pendingBreak = true
	}
}

// Series copies one boundary's trend.
func (t *Trend) Series(mode Mode) *TrendSeries {
	n := len(t.points)
	s := &TrendSeries{seconds: make([]float64, n), values: make([]float64, n), ok: make([]bool, n), breaks: make([]bool, n)}
	for i, p := range t.points {
		s.seconds[i] = p.seconds
		if mode == Battery {
			s.values[i], s.ok[i] = p.battery, p.batteryOK
		} else {
			s.values[i], s.ok[i] = p.whole, p.wholeOK
		}
		s.breaks[i] = p.breakBefore
	}
	if n > 0 {
		first := 0.0
		if t.hasFirst {
			first = t.firstSeconds
		}
		s.Now = s.seconds[n-1]
		s.Recorded = s.Now - first
		s.Start = first
		if s.Recorded >= TrendWindow {
			s.Start = s.Now - TrendWindow
		}
	}
	return s
}

// TrendBucket is one pixel column of the trend: the envelope and mean of its
// readings.
type TrendBucket struct {
	HasData bool
	Min     float64
	Max     float64
	Mean    float64
	// Seconds is the readings' mean time, so the line passes where they were.
	Seconds float64
	// ConnectedToPrevious is false after an N/A reading, a failure or a gap.
	ConnectedToPrevious bool
}

// TrendSeries is an immutable copy of one boundary's trend. Until thirty
// minutes are recorded the window starts at the first reading and fills to
// the right; afterwards it ends at the latest reading. Values are never
// stretched across the window.
type TrendSeries struct {
	seconds  []float64
	values   []float64
	ok       []bool
	breaks   []bool
	Now      float64
	Start    float64
	Recorded float64
}

// End is the right edge of the window.
func (s *TrendSeries) End() float64 { return s.Start + TrendWindow }

// Full reports whether thirty minutes are recorded.
func (s *TrendSeries) Full() bool { return s.Recorded >= TrendWindow }

// Count is the number of readings, available or not.
func (s *TrendSeries) Count() int { return len(s.seconds) }

// HasData reports whether the window holds any available reading.
func (s *TrendSeries) HasData() bool {
	for i := range s.values {
		if s.ok[i] && s.seconds[i] >= s.Start {
			return true
		}
	}
	return false
}

// Latest is the latest reading, if it was available.
func (s *TrendSeries) Latest() (float64, bool) {
	n := len(s.values)
	if n == 0 {
		return 0, false
	}
	return s.values[n-1], s.ok[n-1]
}

// Peak is the largest magnitude in the window with its sign and time: the
// same rule as the 5-minute peak. The earliest reading wins a tie.
func (s *TrendSeries) Peak() (at, watts float64, found bool) {
	for i, v := range s.values {
		if !s.ok[i] || s.seconds[i] < s.Start {
			continue
		}
		if !found || power.ExceedsPeak(v, watts) {
			at, watts, found = s.seconds[i], v, true
		}
	}
	return at, watts, found
}

// Buckets groups the window into equal time columns (at most one per two
// seconds, so 1 Hz readings never leave empty columns inside a run). Each
// keeps min, max and mean, so the peak survives downsampling in the
// envelope.
func (s *TrendSeries) Buckets(columns int) []TrendBucket {
	count := max(1, min(columns, int(TrendWindow/2)))
	buckets := make([]TrendBucket, count)
	sums := make([]float64, count)
	times := make([]float64, count)
	counts := make([]int, count)
	width := TrendWindow / float64(count)
	last := -1
	gap := false
	for i, at := range s.seconds {
		if at < s.Start {
			continue
		}
		if s.breaks[i] || !s.ok[i] {
			gap = true
		}
		if !s.ok[i] {
			continue
		}
		v := s.values[i]
		index := min(count-1, max(0, int(math.Floor((at-s.Start)/width))))
		b := &buckets[index]
		if index != last {
			// The first reading of a column joins the previous column only if
			// nothing missing came in between.
			*b = TrendBucket{HasData: true, Min: v, Max: v, ConnectedToPrevious: last >= 0 && !gap}
			gap = false
			last = index
		} else {
			// A gap inside one column stays pending, so the line still breaks
			// after this column.
			b.Min = math.Min(b.Min, v)
			b.Max = math.Max(b.Max, v)
		}
		sums[index] += v
		times[index] += at
		counts[index]++
	}
	for i := range buckets {
		if counts[i] > 0 {
			buckets[i].Mean = sums[i] / float64(counts[i])
			buckets[i].Seconds = times[i] / float64(counts[i])
		}
	}
	return buckets
}

// Ticks returns evenly spaced, rounded axis values covering min to max.
func Ticks(lo, hi float64, intervals int) []float64 {
	intervals = max(intervals, 1)
	if math.IsNaN(lo) || math.IsNaN(hi) {
		return []float64{0, 1}
	}
	if hi-lo < 1e-9 {
		pad := math.Max(1, math.Abs(hi)*0.05)
		lo -= pad
		hi += pad
	}
	raw := (hi - lo) / float64(intervals)
	magnitude := math.Pow(10, math.Floor(math.Log10(raw)))
	step := magnitude
	for _, factor := range []float64{1, 2, 2.5, 5, 10} {
		step = factor * magnitude
		if step >= raw {
			break
		}
	}
	low := math.Floor(lo/step+1e-9) * step
	high := math.Ceil(hi/step-1e-9) * step
	n := max(1, int(math.Round((high-low)/step)))
	ticks := make([]float64, n+1)
	for i := range ticks {
		ticks[i] = math.RoundToEven((low+float64(i)*step)*1e6) / 1e6
	}
	return ticks
}
