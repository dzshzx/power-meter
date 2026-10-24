using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Globalization;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    /// <summary>
    /// Thirty-minute trend. The session supplies the series and labels; this
    /// control picks one bucket per pixel column and draws the mean line with
    /// a faint min-max envelope, a soft glow, the peak marker and the live
    /// cursor. Missing readings leave gaps.
    /// </summary>
    internal sealed class TrendChart : Control
    {
        internal const int DesignWidth = 388;
        internal const int DesignHeight = 128;
        private TrendView view;
        private Color accent = UiTheme.Charging;

        public TrendChart()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor | ControlStyles.ResizeRedraw, true);
            BackColor = Color.Transparent;
        }

        internal TrendView View { get { return view; } }

        internal void SetView(TrendView next, Color color)
        {
            view = next;
            accent = color;
            Invalidate();
        }

        private float S { get { return Height / (float)DesignHeight; } }

        /// <summary>Plot area in client pixels.</summary>
        internal RectangleF Plot
        {
            get
            {
                float s = S;
                return RectangleF.FromLTRB(18 * s, 8 * s, Width - 40 * s, 104 * s);
            }
        }

        internal int Columns { get { return Math.Max(1, (int)Plot.Width); } }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            g.PixelOffsetMode = PixelOffsetMode.HighQuality;
            float s = S;
            RectangleF plot = Plot;
            TrendSeries series = view == null ? TrendSeries.Empty : view.Series;
            bool hasData = series.HasData;

            using (Pen grid = new Pen(UiTheme.GridLine, 1f))
            {
                for (int i = 0; i <= 3; i++)
                {
                    float x = (float)Math.Round(plot.Left + plot.Width * i / 3f) + 0.5f;
                    g.DrawLine(grid, x, plot.Top, x, plot.Bottom);
                }
                g.DrawLine(grid, plot.Left, plot.Top + 0.5f, plot.Right, plot.Top + 0.5f);
                g.DrawLine(grid, plot.Left, plot.Bottom - 0.5f, plot.Right, plot.Bottom - 0.5f);
            }

            TextFormatFlags flags = AlignedLabel.Flags;
            if (view != null && view.AxisLabels != null)
            {
                using (Pen tick = new Pen(UiTheme.TrendTick, 1f))
                    for (int minute = 0; minute <= 30; minute++)
                    {
                        float x = (float)Math.Round(plot.Left + plot.Width * minute / 30f) + 0.5f;
                        g.DrawLine(tick, x, plot.Bottom, x, plot.Bottom + 3 * s);
                    }
                int labelTop = (int)Math.Round(plot.Bottom + 6 * s);
                for (int i = 0; i < view.AxisLabels.Length; i++)
                {
                    string label = view.AxisLabels[i];
                    Size size = TextRenderer.MeasureText(g, label, Font, Size.Empty, flags);
                    float x = plot.Left + plot.Width * i / (view.AxisLabels.Length - 1f);
                    int left = i == 0 ? (int)plot.Left : i == view.AxisLabels.Length - 1 ? (int)plot.Right - size.Width
                        : (int)Math.Round(x - size.Width / 2f);
                    TextRenderer.DrawText(g, label, Font, new Point(left, labelTop), UiTheme.Faint, flags);
                }
            }

            if (!hasData)
            {
                using (Pen grid = new Pen(UiTheme.GridLine, 1f))
                    for (int i = 1; i < 3; i++)
                    {
                        float y = (float)Math.Round(plot.Top + plot.Height * i / 3f) + 0.5f;
                        g.DrawLine(grid, plot.Left, y, plot.Right, y);
                    }
                string empty = view == null ? "" : view.Empty;
                if (!String.IsNullOrEmpty(empty))
                {
                    using (Font text = new Font(UiTheme.TextFont, Font.Size * 1.2f, FontStyle.Regular, Font.Unit))
                    {
                        Size size = TextRenderer.MeasureText(g, empty, text, Size.Empty, flags);
                        Rectangle box = new Rectangle((int)(plot.Left + (plot.Width - size.Width) / 2) - (int)(10 * s),
                            (int)(plot.Top + (plot.Height - size.Height) / 2) - (int)(3 * s),
                            size.Width + (int)(20 * s), size.Height + (int)(6 * s));
                        using (SolidBrush back = new SolidBrush(UiTheme.Canvas))
                            g.FillRectangle(back, box);
                        TextRenderer.DrawText(g, empty, text, box, UiTheme.Faint, flags
                            | TextFormatFlags.HorizontalCenter | TextFormatFlags.VerticalCenter);
                    }
                }
                return;
            }

            TrendBucket[] buckets = series.Buckets(Columns);
            double min = Double.MaxValue, max = Double.MinValue;
            foreach (TrendBucket bucket in buckets)
            {
                if (!bucket.HasData) continue;
                min = Math.Min(min, bucket.Min);
                max = Math.Max(max, bucket.Max);
            }
            double pad = Math.Max((max - min) * 0.2, Math.Max(0.5, Math.Abs(max) * 0.02));
            double[] ticks = TrendScale.Ticks(min - pad, max + pad, 3);
            double lo = ticks[0], hi = ticks[ticks.Length - 1];
            Func<double, float> X = t => plot.Left + (float)((t - series.Start) / PowerTrend.WindowSeconds) * plot.Width;
            Func<double, float> Y = v => plot.Bottom - (float)((v - lo) / (hi - lo)) * plot.Height;

            bool fractional = false;
            foreach (double tick in ticks)
                if (Math.Abs(tick - Math.Round(tick)) > 1e-6) fractional = true;
            using (Pen grid = new Pen(UiTheme.TrendGrid, 1f))
                foreach (double tick in ticks)
                {
                    float y = (float)Math.Round(Y(tick)) + 0.5f;
                    if (y > plot.Top + 1 && y < plot.Bottom - 1)
                        g.DrawLine(grid, plot.Left, y, plot.Right, y);
                    string label = NumberText.Format(tick, fractional ? "0.#" : "0");
                    Size size = TextRenderer.MeasureText(g, label, Font, Size.Empty, flags);
                    TextRenderer.DrawText(g, label, Font,
                        new Point(Width - (int)Math.Round(10 * s) - size.Width, (int)Math.Round(y - size.Height / 2f)),
                        UiTheme.Faint, flags);
                }

            // Runs of connected columns, ending at the live reading.
            List<List<int>> runs = new List<List<int>>();
            for (int i = 0; i < buckets.Length; i++)
            {
                if (!buckets[i].HasData) continue;
                if (runs.Count == 0 || !buckets[i].ConnectedToPrevious)
                    runs.Add(new List<int>());
                runs[runs.Count - 1].Add(i);
            }
            bool negative = hi <= 0;
            float baseline = negative ? plot.Top : plot.Bottom;
            Region clip = g.Clip;
            g.SetClip(RectangleF.Inflate(plot, 0, 4 * s));
            foreach (List<int> run in runs)
            {
                PointF[] line = new PointF[run.Count];
                PointF[] envelope = new PointF[run.Count * 2];
                for (int k = 0; k < run.Count; k++)
                {
                    TrendBucket bucket = buckets[run[k]];
                    float x = X(bucket.Seconds);
                    line[k] = new PointF(x, Y(bucket.Mean));
                    envelope[k] = new PointF(x, Y(bucket.Max));
                    envelope[run.Count * 2 - 1 - k] = new PointF(x, Y(bucket.Min));
                }
                if (line.Length == 1)
                {
                    using (SolidBrush dot = new SolidBrush(accent))
                        g.FillEllipse(dot, line[0].X - 1.6f * s, line[0].Y - 1.6f * s, 3.2f * s, 3.2f * s);
                    continue;
                }
                using (GraphicsPath area = new GraphicsPath())
                {
                    area.AddLines(line);
                    area.AddLine(line[line.Length - 1], new PointF(line[line.Length - 1].X, baseline));
                    area.AddLine(new PointF(line[line.Length - 1].X, baseline), new PointF(line[0].X, baseline));
                    area.CloseFigure();
                    RectangleF bounds = RectangleF.FromLTRB(plot.Left, plot.Top - 1, plot.Right, plot.Bottom + 1);
                    using (LinearGradientBrush gradient = new LinearGradientBrush(bounds,
                        UiTheme.Alpha(accent, negative ? 0 : 64), UiTheme.Alpha(accent, negative ? 64 : 0), LinearGradientMode.Vertical))
                        g.FillPath(gradient, area);
                }
                using (SolidBrush band = new SolidBrush(UiTheme.Alpha(accent, 40)))
                    g.FillPolygon(band, envelope);
                foreach (float[] layer in new float[][] { new float[] { 7f, 14 }, new float[] { 4.5f, 26 }, new float[] { 3f, 44 } })
                    using (Pen glow = new Pen(UiTheme.Alpha(accent, (int)layer[1]), layer[0] * s))
                    {
                        glow.LineJoin = LineJoin.Round;
                        glow.StartCap = LineCap.Round;
                        glow.EndCap = LineCap.Round;
                        g.DrawLines(glow, line);
                    }
                using (Pen pen = new Pen(accent, 1.6f * s))
                {
                    pen.LineJoin = LineJoin.Round;
                    g.DrawLines(pen, line);
                }
            }
            g.Clip = clip;

            double peakAt, peak;
            if (series.Peak(out peakAt, out peak))
            {
                PointF point = new PointF(X(peakAt), Y(peak));
                using (SolidBrush dot = new SolidBrush(accent))
                    g.FillEllipse(dot, point.X - 2.8f * s, point.Y - 2.8f * s, 5.6f * s, 5.6f * s);
                if (!String.IsNullOrEmpty(view.PeakLabel))
                {
                    Size size = TextRenderer.MeasureText(g, view.PeakLabel, Font, Size.Empty, flags);
                    bool above = point.Y - size.Height - 6 * s >= plot.Top;
                    float y = above ? point.Y - size.Height - 5 * s : point.Y + 5 * s;
                    float x = Math.Max(plot.Left + 2 * s, Math.Min(plot.Right - size.Width - 2 * s, point.X - size.Width / 2f));
                    Rectangle box = new Rectangle((int)x - (int)(3 * s), (int)y - (int)s, size.Width + (int)(6 * s), size.Height + (int)(2 * s));
                    using (SolidBrush back = new SolidBrush(UiTheme.Alpha(UiTheme.Canvas, 200)))
                        g.FillRectangle(back, box);
                    TextRenderer.DrawText(g, view.PeakLabel, Font, new Point((int)x, (int)y), UiTheme.Muted, flags);
                }
            }

            double? latest = series.Latest;
            if (latest.HasValue)
            {
                float x = X(series.Now);
                float y = Y(latest.Value);
                using (Pen cursor = new Pen(UiTheme.Alpha(accent, 90), 1f))
                    g.DrawLine(cursor, x, plot.Top, x, plot.Bottom);
                float r = 4f * s;
                using (SolidBrush back = new SolidBrush(UiTheme.Canvas))
                    g.FillEllipse(back, x - r, y - r, 2 * r, 2 * r);
                using (Pen ring = new Pen(accent, 1.6f * s))
                    g.DrawEllipse(ring, x - r, y - r, 2 * r, 2 * r);
            }

            if (view != null && !String.IsNullOrEmpty(view.Recorded))
            {
                using (Font text = new Font(UiTheme.TextFont, Font.Size * 1.1f, FontStyle.Regular, Font.Unit))
                    TextRenderer.DrawText(g, view.Recorded, text,
                        new Point((int)(plot.Right - 6 * s) - TextRenderer.MeasureText(g, view.Recorded, text, Size.Empty, flags).Width,
                            (int)(plot.Top + 5 * s)), UiTheme.Faint, flags);
            }
        }
    }
}
