using System;
using System.Linq;
using Xunit;
using static BatteryChargeMeter.Tests.Fixtures;

namespace BatteryChargeMeter.Tests
{
    public sealed class TrendTests
    {
        private static readonly DateTimeOffset Time = new DateTimeOffset(2026, 10, 17, 14, 0, 0, TimeSpan.FromHours(8));

        private static PowerSample Platform(double watts)
        {
            return PowerSample.FromValue(PowerBoundary.Platform, MeasurementKind.Measured, watts, "fixture", TimeSpan.FromSeconds(1));
        }

        private static PowerSample NoPlatform()
        {
            return PowerSample.Unsupported(PowerBoundary.Platform, PawnIoSensor.ElevationRequired);
        }

        private static PowerSnapshot Snapshot(double second, double battery, PowerSample platform)
        {
            return PowerSnapshot.Compose(Time.AddSeconds(second), second,
                Battery(true, battery > 0, true, battery), null, platform);
        }

        private static PowerTrend Feed(int seconds, Func<int, double?> watts)
        {
            PowerTrend trend = new PowerTrend();
            for (int second = 0; second <= seconds; second++)
            {
                double? value = watts(second);
                trend.Add(second,
                    value.HasValue ? Value(value.Value) : PowerSample.Unsupported(PowerBoundary.BatteryTerminal, "gap"),
                    Value(1));
            }
            return trend;
        }

        [Fact]
        public void BucketsKeepThePeakInTheEnvelope()
        {
            TrendSeries series = Feed(1800, s => s == 777 ? 95 : 10 + (s % 7) * 0.1).Series(DisplayMode.WholeSystem);
            TrendBucket[] buckets = series.Buckets(330);
            double at, peak;
            Check("one bucket per column, all filled at 1 Hz", buckets.Length == 330 && buckets.All(b => b.HasData));
            Check("the envelope keeps the single-second peak", Near(buckets.Max(b => b.Max), 95));
            Check("the mean line is averaged, not the peak", buckets.Max(b => b.Mean) < 30);
            Check("the peak marker is the exact reading", series.Peak(out at, out peak) && Near(peak, 95) && Near(at, 777));
            Check("columns are capped at one per two seconds", series.Buckets(5000).Length == 900);
        }

        [Fact]
        public void NegativePeakFollowsTheStatisticRule()
        {
            Func<int, double?> watts = s => s == 1790 ? -40 : s == 1795 ? 20 : -10;
            TrendSeries series = Feed(1800, watts).Series(DisplayMode.WholeSystem);
            PowerHistory history = new PowerHistory();
            for (int second = 1501; second <= 1800; second++)
                history.Add(second, DisplayMode.WholeSystem, BatterySupplyState.BatteryDischarging, Value(watts(second).Value));
            double at, peak;
            Check("largest magnitude wins and keeps its sign",
                series.Peak(out at, out peak) && Near(peak, -40) && Near(at, 1790));
            Check("same rule as the 5 minute peak", Near(history.Peak().Value, peak));
        }

        [Fact]
        public void MissingReadingsLeaveGaps()
        {
            TrendSeries series = Feed(1800, s => s >= 600 && s < 780 ? (double?)null : 20).Series(DisplayMode.WholeSystem);
            TrendBucket[] buckets = series.Buckets(300);
            // Six seconds per column: columns 100..129 hold only N/A readings.
            Check("columns inside the gap have no data", Enumerable.Range(101, 28).All(i => !buckets[i].HasData));
            int resume = Enumerable.Range(129, 5).First(i => buckets[i].HasData);
            Check("the line restarts after the gap", !buckets[resume].ConnectedToPrevious);
            Check("no zero is drawn for N/A", buckets.Where(b => b.HasData).All(b => Near(b.Min, 20)));
            Check("ordinary neighbours stay connected", buckets[50].ConnectedToPrevious && buckets[200].ConnectedToPrevious);

            TrendSeries brief = Feed(1800, s => s == 905 ? (double?)null : 20).Series(DisplayMode.WholeSystem);
            TrendBucket[] coarse = brief.Buckets(300);
            Check("a gap inside one column still breaks the line after it",
                coarse[150].HasData && !coarse[151].ConnectedToPrevious);
        }

        [Fact]
        public void FailureBreaksWithoutClearing()
        {
            MeterSession session = new MeterSession(DisplayMode.WholeSystem);
            for (int second = 0; second < 100; second++)
                session.Observe(Snapshot(second, 30, Platform(20)));
            session.Fail("sensor failed");
            MeterView view = session.Observe(Snapshot(100, 30, Platform(20)));
            TrendBucket[] buckets = view.Trend.Series.Buckets(900);
            Check("history survives a failed capture", view.Trend.Series.Count == 101);
            Check("the reading after a failure starts a new line", !buckets[50].ConnectedToPrevious && buckets[49].ConnectedToPrevious);
        }

        [Fact]
        public void ModeChangesKeepBothSeries()
        {
            MeterSession session = new MeterSession(DisplayMode.WholeSystem);
            for (int second = 0; second < 120; second++)
                session.Observe(Snapshot(second, 30, second < 60 ? Platform(20) : NoPlatform()));
            MeterView view = session.SetMode(DisplayMode.Battery);
            Check("battery trend was recorded while whole mode was shown",
                view.Trend.Series.Count == 120 && Near(view.Trend.Series.Latest.Value, 30));
            view = session.SetMode(DisplayMode.WholeSystem);
            double at, peak;
            Check("whole trend survives a round trip and keeps its N/A tail",
                view.Trend.Series.Count == 120 && !view.Trend.Series.Latest.HasValue
                && view.Trend.Series.Peak(out at, out peak) && Near(peak, 50)
                && view.Trend.PeakLabel == Strings.Format("峰值 {0}", "≈ 50.00 W"));
            Check("elevation is offered for the elevation-gated platform reading", view.ElevationHelps);
        }

        [Fact]
        public void EarlyTrendFillsFromTheLeft()
        {
            MeterSession session = new MeterSession(DisplayMode.Battery);
            MeterView view = session.Render();
            Check("before the first reading the chart waits", !view.Trend.Series.HasData
                && view.Trend.Empty == Strings.Get("等待第一次读数") && view.Trend.Recorded == null);
            for (int second = 0; second <= 240; second++)
                view = session.Observe(Snapshot(second, -17.5, Platform(20)));
            TrendSeries series = view.Trend.Series;
            TrendBucket[] buckets = series.Buckets(300);
            int last = Array.FindLastIndex(buckets, b => b.HasData);
            Check("the window starts at the first reading and is not stretched",
                Near(series.Start, 0) && Near(series.End, 1800) && Near(series.Recorded, 240) && last == 40);
            Check("the recorded duration is shown", view.Trend.Recorded == Strings.Format("已记录 {0} 分钟", "4"));
            Check("an unfilled axis counts minutes from the first reading", view.Trend.AxisLabels[0] == "0"
                && view.Trend.AxisLabels[3] == Strings.Get("30 分"));
            Check("display figures use the typographic minus",
                view.Headline.Text == "−17.50 W" && view.PeakValue == "−17.50 W"
                && view.Trend.PeakLabel == Strings.Format("峰值 {0}", "−17.50 W")
                && NumberText.Format(-1.5, "0.00") == "−1.50" && NumberText.Format(2, "0.00") == "2.00");

            for (int second = 241; second <= 2000; second++)
                view = session.Observe(Snapshot(second, second == 100 ? -60 : -17.5, Platform(20)));
            series = view.Trend.Series;
            Check("a full window ends at the latest reading", Near(series.Start, 200) && Near(series.Now, 2000)
                && view.Trend.Recorded == null && view.Trend.AxisLabels[3] == Strings.Get("现在"));
        }

        [Fact]
        public void TrendScaleAndClock()
        {
            double[] ticks = TrendScale.Ticks(55.2, 62.7, 3);
            Check("ticks are rounded and cover the range", ticks[0] <= 55.2 && ticks[ticks.Length - 1] >= 62.7
                && ticks.All(t => Math.Abs(t * 2 - Math.Round(t * 2)) < 1e-9));
            PowerTrend trend = new PowerTrend();
            trend.Add(10, Value(1), Value(1));
            trend.Add(11, Value(1), Value(1));
            trend.Add(3, Value(1), Value(1));
            Check("a restarted capture clock starts a new trend", trend.Count == 1
                && Near(trend.Series(DisplayMode.Battery).Recorded, 0));
            trend.Add(20, Value(1), Value(1));
            Check("a timing gap breaks the line", !trend.Series(DisplayMode.Battery).Buckets(900)[8].ConnectedToPrevious);
        }
    }
}
