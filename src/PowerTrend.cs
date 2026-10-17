using System;
using System.Collections.Generic;
using System.Globalization;

namespace BatteryChargeMeter
{
    /// <summary>
    /// One recorded second of the trend. Missing values are N/A readings;
    /// <see cref="BreakBefore"/> marks a capture failure or a timing gap
    /// before this reading, so the line never bridges unknown time.
    /// </summary>
    internal struct TrendPoint
    {
        public double Seconds;
        public double? Whole;
        public double? Battery;
        public bool BreakBefore;
    }

    /// <summary>
    /// The last thirty minutes of both display boundaries, recorded from the
    /// first reading regardless of the selected mode. Memory only: a restart
    /// starts an empty trend.
    /// </summary>
    internal sealed class PowerTrend
    {
        internal const double WindowSeconds = 1800;
        private readonly List<TrendPoint> points = new List<TrendPoint>();
        private double? firstSeconds;
        private bool pendingBreak;

        internal int Count { get { return points.Count; } }

        internal void Add(double seconds, PowerSample whole, PowerSample battery)
        {
            if (points.Count > 0 && seconds <= points[points.Count - 1].Seconds)
            {
                // The capture clock restarted: earlier times no longer line up.
                points.Clear();
                firstSeconds = null;
            }
            bool gap = pendingBreak
                || (points.Count > 0 && seconds - points[points.Count - 1].Seconds > PowerHistory.MaximumGap);
            pendingBreak = false;
            if (!firstSeconds.HasValue)
                firstSeconds = seconds;
            points.Add(new TrendPoint
            {
                Seconds = seconds,
                Whole = Value(whole),
                Battery = Value(battery),
                BreakBefore = gap
            });
            int drop = 0;
            while (drop < points.Count - 1 && points[drop].Seconds < seconds - WindowSeconds)
                drop++;
            if (drop > 0)
                points.RemoveRange(0, drop);
        }

        /// <summary>A failed capture: the next reading starts a new line.</summary>
        internal void Break()
        {
            if (points.Count > 0)
                pendingBreak = true;
        }

        internal TrendSeries Series(DisplayMode mode)
        {
            int count = points.Count;
            double[] seconds = new double[count];
            double?[] values = new double?[count];
            bool[] breaks = new bool[count];
            for (int i = 0; i < count; i++)
            {
                TrendPoint point = points[i];
                seconds[i] = point.Seconds;
                values[i] = mode == DisplayMode.Battery ? point.Battery : point.Whole;
                breaks[i] = point.BreakBefore;
            }
            return new TrendSeries(seconds, values, breaks, firstSeconds ?? 0);
        }

        private static double? Value(PowerSample sample)
        {
            return sample != null && sample.Available ? (double?)sample.Watts : null;
        }
    }

    /// <summary>One pixel column of the trend: the envelope and mean of its readings.</summary>
    internal struct TrendBucket
    {
        public bool HasData;
        public double Min;
        public double Max;
        public double Mean;
        // Mean time of the readings, so the line passes through where they were.
        public double Seconds;
        // False after an N/A reading, a failure or a timing gap.
        public bool ConnectedToPrevious;
    }

    /// <summary>
    /// Immutable copy of one boundary's trend. Until thirty minutes are
    /// recorded the window starts at the first reading and fills to the
    /// right; afterwards it ends at the latest reading. Values are never
    /// stretched across the window.
    /// </summary>
    internal sealed class TrendSeries
    {
        private readonly double[] seconds;
        private readonly double?[] values;
        private readonly bool[] breaks;

        internal TrendSeries(double[] seconds, double?[] values, bool[] breaks, double firstSeconds)
        {
            this.seconds = seconds;
            this.values = values;
            this.breaks = breaks;
            if (seconds.Length == 0)
            {
                Now = 0;
                Recorded = 0;
                Start = 0;
                return;
            }
            Now = seconds[seconds.Length - 1];
            Recorded = Now - firstSeconds;
            Start = Recorded >= PowerTrend.WindowSeconds ? Now - PowerTrend.WindowSeconds : firstSeconds;
        }

        internal static readonly TrendSeries Empty = new TrendSeries(new double[0], new double?[0], new bool[0], 0);

        internal double Now { get; private set; }
        internal double Start { get; private set; }
        internal double End { get { return Start + PowerTrend.WindowSeconds; } }
        internal double Recorded { get; private set; }
        internal bool Full { get { return Recorded >= PowerTrend.WindowSeconds; } }
        internal int Count { get { return seconds.Length; } }

        internal bool HasData
        {
            get
            {
                for (int i = 0; i < values.Length; i++)
                    if (values[i].HasValue && seconds[i] >= Start) return true;
                return false;
            }
        }

        /// <summary>The latest reading, if it was available.</summary>
        internal double? Latest
        {
            get { return values.Length == 0 ? null : values[values.Length - 1]; }
        }

        /// <summary>
        /// Largest magnitude in the window, keeping its sign: the same rule as
        /// the 5-minute peak statistic, so a discharge peak is the most
        /// negative reading. The earliest reading wins a tie.
        /// </summary>
        internal bool Peak(out double atSeconds, out double watts)
        {
            atSeconds = 0;
            watts = 0;
            bool found = false;
            for (int i = 0; i < values.Length; i++)
            {
                if (!values[i].HasValue || seconds[i] < Start) continue;
                if (!found || Math.Abs(values[i].Value) > Math.Abs(watts))
                {
                    found = true;
                    watts = values[i].Value;
                    atSeconds = seconds[i];
                }
            }
            return found;
        }

        /// <summary>
        /// Groups the window into <paramref name="columns"/> equal time buckets
        /// (at most one per two seconds, so 1 Hz readings never leave empty
        /// columns inside a run). Each keeps min, max and mean, so the peak
        /// survives downsampling in the envelope.
        /// </summary>
        internal TrendBucket[] Buckets(int columns)
        {
            int count = Math.Max(1, Math.Min(columns, (int)(PowerTrend.WindowSeconds / 2)));
            TrendBucket[] buckets = new TrendBucket[count];
            double[] sums = new double[count];
            double[] times = new double[count];
            int[] counts = new int[count];
            double width = PowerTrend.WindowSeconds / count;
            int lastBucket = -1;
            bool gap = false;
            for (int i = 0; i < seconds.Length; i++)
            {
                if (seconds[i] < Start) continue;
                if (breaks[i] || !values[i].HasValue)
                    gap = true;
                if (!values[i].HasValue) continue;
                double value = values[i].Value;
                int index = Math.Min(count - 1, Math.Max(0, (int)Math.Floor((seconds[i] - Start) / width)));
                if (index != lastBucket)
                {
                    // First reading of a column; it joins the previous column
                    // only if nothing missing came in between.
                    buckets[index].HasData = true;
                    buckets[index].Min = value;
                    buckets[index].Max = value;
                    buckets[index].ConnectedToPrevious = lastBucket >= 0 && !gap;
                    gap = false;
                    lastBucket = index;
                }
                else
                {
                    // A gap inside one column stays pending, so the line
                    // still breaks after this column.
                    buckets[index].Min = Math.Min(buckets[index].Min, value);
                    buckets[index].Max = Math.Max(buckets[index].Max, value);
                }
                sums[index] += value;
                times[index] += seconds[i];
                counts[index]++;
            }
            for (int i = 0; i < count; i++)
            {
                if (counts[i] == 0) continue;
                buckets[i].Mean = sums[i] / counts[i];
                buckets[i].Seconds = times[i] / counts[i];
            }
            return buckets;
        }
    }

    /// <summary>Evenly spaced, rounded axis values covering a range.</summary>
    internal static class TrendScale
    {
        internal static double[] Ticks(double min, double max, int intervals)
        {
            if (intervals < 1) intervals = 1;
            if (Double.IsNaN(min) || Double.IsNaN(max)) return new double[] { 0, 1 };
            if (max - min < 1e-9)
            {
                double pad = Math.Max(1, Math.Abs(max) * 0.05);
                min -= pad;
                max += pad;
            }
            double raw = (max - min) / intervals;
            double magnitude = Math.Pow(10, Math.Floor(Math.Log10(raw)));
            double step = magnitude;
            foreach (double factor in new double[] { 1, 2, 2.5, 5, 10 })
            {
                step = factor * magnitude;
                if (step >= raw) break;
            }
            double low = Math.Floor(min / step + 1e-9) * step;
            double high = Math.Ceiling(max / step - 1e-9) * step;
            int n = Math.Max(1, (int)Math.Round((high - low) / step));
            double[] ticks = new double[n + 1];
            for (int i = 0; i <= n; i++)
                ticks[i] = Math.Round(low + i * step, 6);
            return ticks;
        }
    }

    /// <summary>
    /// Display numbers use the typographic minus. Only presentation strings
    /// use it; CLI reports, logs and stored values keep ASCII.
    /// </summary>
    internal static class NumberText
    {
        internal const char Minus = '−';

        internal static string Format(double value, string format)
        {
            string text = value.ToString(format, CultureInfo.InvariantCulture);
            return text.StartsWith("-", StringComparison.Ordinal) ? Minus + text.Substring(1) : text;
        }
    }
}
