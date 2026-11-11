using System;
using System.Drawing;
using System.Windows.Forms;
using Xunit;
using static BatteryChargeMeter.Tests.Fixtures;

namespace BatteryChargeMeter.Tests
{
    // Paints controls into bitmaps through Control.InvokePaint, so no window
    // handle or desktop is involved. Checks read single-colour geometry (grid
    // lines, ticks, fills and the trend line), never text, whose rendering
    // depends on the machine's font settings.
    public sealed class PaintTests
    {
        private static readonly Color Accent = Color.FromArgb(18, 52, 86);
        private static readonly DateTimeOffset Time = new DateTimeOffset(2026, 11, 12, 9, 0, 0, TimeSpan.FromHours(8));

        private sealed class PaintHost : Control
        {
            internal Bitmap Render(Control target)
            {
                Bitmap bitmap = new Bitmap(target.Width, target.Height);
                using (Graphics g = Graphics.FromImage(bitmap))
                using (PaintEventArgs e = new PaintEventArgs(g, new Rectangle(Point.Empty, target.Size)))
                    InvokePaint(target, e);
                return bitmap;
            }
        }

        private static Bitmap Render(Control target)
        {
            using (PaintHost host = new PaintHost())
                return host.Render(target);
        }

        private static int CountInRow(Bitmap bitmap, int y, Color color)
        {
            int count = 0;
            for (int x = 0; x < bitmap.Width; x++)
                if (bitmap.GetPixel(x, y).ToArgb() == color.ToArgb()) count++;
            return count;
        }

        private static int CountInColumn(Bitmap bitmap, int x, int top, int bottom, Color color)
        {
            int count = 0;
            for (int y = top; y <= bottom; y++)
                if (bitmap.GetPixel(x, y).ToArgb() == color.ToArgb()) count++;
            return count;
        }

        // Twice the design size, so the 1.6-pixel trend line covers whole pixel rows.
        private static TrendChart Chart(TrendView view)
        {
            TrendChart chart = new TrendChart();
            chart.Size = new Size(TrendChart.DesignWidth * 2, TrendChart.DesignHeight * 2);
            chart.SetView(view, Accent);
            return chart;
        }

        // The pixel column of a reading in a window that starts at the first reading.
        private static int Column(RectangleF plot, double second)
        {
            return (int)(plot.Left + second / PowerTrend.WindowSeconds * plot.Width);
        }

        [Fact]
        public void GlyphButtonFillFollowsItsState()
        {
            using (GlyphButton button = new GlyphButton())
            {
                button.Size = new Size(44, 44);
                using (Bitmap idle = Render(button))
                    Check("an idle glyph button paints no fill", idle.GetPixel(22, 22).A == 0);
                button.Active = true;
                using (Bitmap active = Render(button))
                    Check("an active glyph button paints the hover fill",
                        active.GetPixel(22, 22).ToArgb() == UiTheme.ButtonHover.ToArgb());
            }
        }

        [Fact]
        public void EmptyTrendChartDrawsTicksAndGrid()
        {
            using (TrendChart chart = Chart(new MeterSession(DisplayMode.Battery).Render().Trend))
            using (Bitmap bitmap = Render(chart))
            {
                RectangleF plot = chart.Plot;
                Check("one axis tick per minute below the plot",
                    CountInRow(bitmap, (int)plot.Bottom + 2, UiTheme.TrendTick) == 31);
                Check("the empty plot shows its two inner grid lines",
                    CountInColumn(bitmap, (int)plot.Left + 4, (int)plot.Top + 2, (int)plot.Bottom - 2, UiTheme.GridLine) == 2);
            }
        }

        [Fact]
        public void TrendChartDrawsRunsGapsAndScale()
        {
            PowerSample platform = PowerSample.FromValue(PowerBoundary.Platform, MeasurementKind.Measured, 20, "fixture", TimeSpan.FromSeconds(1));
            MeterSession session = new MeterSession(DisplayMode.Battery);
            MeterView view = session.Render();
            // Four minutes of readings with no battery rate from second 100 to 129.
            for (int second = 0; second <= 240; second++)
                view = session.Observe(PowerSnapshot.Compose(Time.AddSeconds(second), second,
                    Battery(true, false, second < 100 || second >= 130, -17.5), null, platform));
            using (TrendChart chart = Chart(view.Trend))
            using (Bitmap bitmap = Render(chart))
            {
                RectangleF plot = chart.Plot;
                int top = (int)plot.Top + 2, bottom = (int)plot.Bottom - 2;
                Check("a connected run draws the trend line",
                    CountInColumn(bitmap, Column(plot, 60), top, bottom, Accent) > 0);
                Check("missing readings leave the line open",
                    CountInColumn(bitmap, Column(plot, 115), top, bottom, Accent) == 0);
                Check("scale lines cross the plot beyond the readings",
                    CountInColumn(bitmap, (int)plot.Right - 4, top, bottom, UiTheme.TrendGrid) > 0);
            }
        }
    }
}
