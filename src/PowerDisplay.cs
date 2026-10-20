using System;
using System.Collections.Generic;
using Microsoft.Win32;

namespace BatteryChargeMeter
{
    internal enum DisplayMode
    {
        WholeSystem,
        Battery
    }

    internal static class DisplayPreference
    {
        private const string Key = @"Software\BatteryChargeMeter";
        internal static DisplayMode Load()
        {
            return Load(delegate
            {
                using (RegistryKey key = Registry.CurrentUser.OpenSubKey(Key))
                    return key == null ? null : key.GetValue("DisplayMode");
            });
        }

        internal static DisplayMode Load(Func<object> read)
        {
            try
            {
                return String.Equals(read() as string, "Battery", StringComparison.Ordinal)
                    ? DisplayMode.Battery : DisplayMode.WholeSystem;
            }
            catch
            {
                return DisplayMode.WholeSystem;
            }
        }

        internal static void Save(DisplayMode mode)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.CreateSubKey(Key))
                    key.SetValue("DisplayMode", mode.ToString(), RegistryValueKind.String);
            }
            catch
            {
                // Read-only profiles retain the selection for this session.
            }
        }
    }

    internal sealed class TimedPower
    {
        internal double Seconds;
        internal double? Watts;
    }

    internal sealed class PowerHistory
    {
        internal const double MaximumGap = 5.0;
        // Both statistics' windows. The captions' translation keys in
        // MeterSession.Statistics spell out 10 and 5 minutes ("10 分钟均值",
        // "/10分", "/5分"), so changing a window here also means new keys.
        internal const double AverageWindow = 600;
        internal const double PeakWindow = 300;
        private const double Retention = AverageWindow > PeakWindow ? AverageWindow : PeakWindow;
        private readonly List<TimedPower> points = new List<TimedPower>();
        private DisplayMode mode;
        private BatterySupplyState supply;
        internal IList<TimedPower> Points
        {
            get { return points.AsReadOnly(); }
        }

        internal void Clear()
        {
            points.Clear();
        }

        internal void Add(double seconds, DisplayMode nextMode, BatterySupplyState nextSupply, PowerSample sample)
        {
            if (points.Count > 0 && (mode != nextMode || supply != nextSupply
                || seconds <= points[points.Count - 1].Seconds
                || seconds - points[points.Count - 1].Seconds > MaximumGap))
                Clear();
            mode = nextMode;
            supply = nextSupply;
            points.Add(new TimedPower
            {
                Seconds = seconds,
                Watts = PowerSample.WattsOrNull(sample)
            });
            // Retain one predecessor for clipping the first weighted interval.
            while (points.Count > 1 && points[1].Seconds <= seconds - Retention)
                points.RemoveAt(0);
        }

        internal double Duration(double window)
        {
            return points.Count < 2 ? 0 : Math.Min(window, points[points.Count - 1].Seconds - points[0].Seconds);
        }

        internal double? Average(out double coverage)
        {
            coverage = 0;
            if (points.Count < 2 || !points[points.Count - 1].Watts.HasValue)
                return null;
            double cutoff = points[points.Count - 1].Seconds - AverageWindow;
            double energy = 0;
            // Left-held observations; an interval touching a missing reading is
            // excluded rather than claiming a value throughout an unknown gap.
            for (int i = 1; i < points.Count; i++)
            {
                TimedPower left = points[i - 1];
                TimedPower right = points[i];
                double duration = right.Seconds - Math.Max(left.Seconds, cutoff);
                if (duration <= 0 || !left.Watts.HasValue || !right.Watts.HasValue)
                    continue;
                energy += left.Watts.Value * duration;
                coverage += duration;
            }
            return coverage > 0 ? (double?)(energy / coverage) : null;
        }

        internal double? Peak()
        {
            if (points.Count == 0 || !points[points.Count - 1].Watts.HasValue)
                return null;
            double cutoff = points[points.Count - 1].Seconds - PeakWindow;
            double? peak = null;
            foreach (TimedPower point in points)
            {
                if (point.Seconds >= cutoff && point.Watts.HasValue
                    && (!peak.HasValue || PowerSample.ExceedsPeak(point.Watts.Value, peak.Value)))
                    peak = point.Watts;
            }
            return peak;
        }
    }
}
