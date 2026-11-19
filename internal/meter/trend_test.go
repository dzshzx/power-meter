package meter

import (
	"math"
	"testing"
	"time"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/power"
)

var trendTime = time.Date(2026, 10, 17, 14, 0, 0, 0, time.FixedZone("", 8*3600))

func trendSnapshot(second, batteryWatts float64, platform *power.Sample) *power.Snapshot {
	return power.Compose(trendTime.Add(time.Duration(second)*time.Second), second,
		battery(true, batteryWatts > 0, true, batteryWatts), nil, platform)
}

func noPlatform() *power.Sample {
	return power.Unsupported(power.Platform, power.ReasonElevationRequired)
}

// feed records seconds 0..seconds of the whole boundary; a false second is
// an N/A reading.
func feed(seconds int, watts func(int) (float64, bool)) *Trend {
	var trend Trend
	for second := 0; second <= seconds; second++ {
		s := gap()
		if w, ok := watts(second); ok {
			s = value(w)
		}
		trend.Add(float64(second), s, value(1))
	}
	return &trend
}

func TestBucketsKeepThePeakInTheEnvelope(t *testing.T) {
	series := feed(1800, func(s int) (float64, bool) {
		if s == 777 {
			return 95, true
		}
		return 10 + float64(s%7)*0.1, true
	}).Series(WholeSystem)
	buckets := series.Buckets(330)
	all, maxPeak, maxMean := true, math.Inf(-1), math.Inf(-1)
	for _, b := range buckets {
		all = all && b.HasData
		maxPeak = math.Max(maxPeak, b.Max)
		maxMean = math.Max(maxMean, b.Mean)
	}
	check(t, "one bucket per column, all filled at 1 Hz", len(buckets) == 330 && all)
	check(t, "the envelope keeps the single-second peak", near(maxPeak, 95))
	check(t, "the mean line is averaged, not the peak", maxMean < 30)
	at, peak, ok := series.Peak()
	check(t, "the peak marker is the exact reading", ok && near(peak, 95) && near(at, 777))
	check(t, "columns are capped at one per two seconds", len(series.Buckets(5000)) == 900)
}

func TestNegativePeakFollowsTheStatisticRule(t *testing.T) {
	watts := func(s int) (float64, bool) {
		switch s {
		case 1790:
			return -40, true
		case 1795:
			return 20, true
		}
		return -10, true
	}
	series := feed(1800, watts).Series(WholeSystem)
	var h History
	for second := 1501; second <= 1800; second++ {
		w, _ := watts(second)
		h.Add(float64(second), WholeSystem, power.BatteryDischarging, value(w))
	}
	at, peak, ok := series.Peak()
	check(t, "largest magnitude wins and keeps its sign", ok && near(peak, -40) && near(at, 1790))
	statistic, _ := h.Peak()
	check(t, "same rule as the 5 minute peak", near(statistic, peak))
}

func TestMissingReadingsLeaveGaps(t *testing.T) {
	series := feed(1800, func(s int) (float64, bool) { return 20, s < 600 || s >= 780 }).Series(WholeSystem)
	buckets := series.Buckets(300)
	// Six seconds per column: columns 100..129 hold only N/A readings.
	inside := true
	for i := 101; i < 129; i++ {
		inside = inside && !buckets[i].HasData
	}
	check(t, "columns inside the gap have no data", inside)
	resume := 129
	for !buckets[resume].HasData {
		resume++
	}
	check(t, "the line restarts after the gap", !buckets[resume].ConnectedToPrevious)
	noZero := true
	for _, b := range buckets {
		if b.HasData {
			noZero = noZero && near(b.Min, 20)
		}
	}
	check(t, "no zero is drawn for N/A", noZero)
	check(t, "ordinary neighbours stay connected", buckets[50].ConnectedToPrevious && buckets[200].ConnectedToPrevious)

	brief := feed(1800, func(s int) (float64, bool) { return 20, s != 905 }).Series(WholeSystem)
	coarse := brief.Buckets(300)
	check(t, "a gap inside one column still breaks the line after it", coarse[150].HasData && !coarse[151].ConnectedToPrevious)
}

func TestFailureBreaksWithoutClearing(t *testing.T) {
	session := NewSession(WholeSystem)
	for second := 0; second < 100; second++ {
		session.Observe(trendSnapshot(float64(second), 30, platformOf(20)))
	}
	session.Fail("sensor failed")
	v := session.Observe(trendSnapshot(100, 30, platformOf(20)))
	buckets := v.Trend.Series.Buckets(900)
	check(t, "history survives a failed capture", v.Trend.Series.Count() == 101)
	check(t, "the reading after a failure starts a new line", !buckets[50].ConnectedToPrevious && buckets[49].ConnectedToPrevious)
}

func TestModeChangesKeepBothSeries(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		session := NewSession(WholeSystem)
		for second := 0; second < 120; second++ {
			p := platformOf(20)
			if second >= 60 {
				p = noPlatform()
			}
			session.Observe(trendSnapshot(float64(second), 30, p))
		}
		v := session.SetMode(Battery)
		latest, ok := v.Trend.Series.Latest()
		check(t, "battery trend was recorded while whole mode was shown",
			v.Trend.Series.Count() == 120 && ok && near(latest, 30))
		v = session.SetMode(WholeSystem)
		_, latestOK := v.Trend.Series.Latest()
		_, peak, peakOK := v.Trend.Series.Peak()
		check(t, "whole trend survives a round trip and keeps its N/A tail",
			v.Trend.Series.Count() == 120 && !latestOK && peakOK && near(peak, 50) &&
				v.Trend.PeakLabel == i18n.Format("峰值 {0}", "≈ 50.00 W"))
		check(t, "elevation is offered for the elevation-gated platform reading", v.ElevationHelps)
		check(t, "an unavailable whole trend says so", v.Trend.Empty == i18n.Get("整机功率不可用，暂无走势"))
	})
}

func TestEarlyTrendFillsFromTheLeft(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		session := NewSession(Battery)
		v := session.Render()
		check(t, "before the first reading the chart waits",
			!v.Trend.Series.HasData() && v.Trend.Empty == i18n.Get("等待第一次读数") && v.Trend.Recorded == "" &&
				v.Trend.AxisLabels[0] == i18n.Get("−30 分"))
		for second := 0; second <= 240; second++ {
			v = session.Observe(trendSnapshot(float64(second), -17.5, platformOf(20)))
		}
		series := v.Trend.Series
		buckets := series.Buckets(300)
		last := -1
		for i, b := range buckets {
			if b.HasData {
				last = i
			}
		}
		check(t, "the window starts at the first reading and is not stretched",
			near(series.Start, 0) && near(series.End(), 1800) && near(series.Recorded, 240) && last == 40)
		check(t, "the recorded duration is shown", v.Trend.Recorded == i18n.Format("已记录 {0} 分钟", "4"))
		check(t, "an unfilled axis counts minutes from the first reading",
			v.Trend.AxisLabels[0] == "0" && v.Trend.AxisLabels[3] == i18n.Get("30 分"))
		check(t, "display figures use the typographic minus",
			v.Headline.Text == "−17.50 W" && v.PeakValue == "−17.50 W" &&
				v.Trend.PeakLabel == i18n.Format("峰值 {0}", "−17.50 W") &&
				Number(-1.5, 2) == "−1.50" && Number(2, 2) == "2.00")
		check(t, "the battery trend's empty text names the battery", v.Trend.Empty == i18n.Get("电池端功率不可用，暂无走势"))

		for second := 241; second <= 2000; second++ {
			v = session.Observe(trendSnapshot(float64(second), -17.5, platformOf(20)))
		}
		series = v.Trend.Series
		check(t, "a full window ends at the latest reading",
			near(series.Start, 200) && near(series.Now, 2000) && v.Trend.Recorded == "" &&
				v.Trend.AxisLabels[3] == i18n.Get("现在") && v.Trend.AxisLabels[1] == "−20")
	})
}

func TestTrendScaleAndClock(t *testing.T) {
	ticks := Ticks(55.2, 62.7, 3)
	rounded := true
	for _, tick := range ticks {
		rounded = rounded && math.Abs(tick*2-math.Round(tick*2)) < 1e-9
	}
	check(t, "ticks are rounded and cover the range", ticks[0] <= 55.2 && ticks[len(ticks)-1] >= 62.7 && rounded)
	flat := Ticks(5, 5, 4)
	check(t, "a flat range is padded", flat[0] < 5 && flat[len(flat)-1] > 5)
	var trend Trend
	trend.Add(10, value(1), value(1))
	trend.Add(11, value(1), value(1))
	trend.Add(3, value(1), value(1))
	check(t, "a restarted capture clock starts a new trend",
		trend.Count() == 1 && near(trend.Series(Battery).Recorded, 0))
	trend.Add(20, value(1), value(1))
	check(t, "a timing gap breaks the line", !trend.Series(Battery).Buckets(900)[8].ConnectedToPrevious)
}
