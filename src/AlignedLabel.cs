using System;
using System.Drawing;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    // GDI's default glyph-overhang padding differs with font size. Remove it
    // consistently so labels and controls share the same inset.
    internal class AlignedLabel : Label
    {
        internal const TextFormatFlags Flags = TextFormatFlags.NoPadding | TextFormatFlags.SingleLine
            | TextFormatFlags.NoPrefix;

        protected override void OnPaint(PaintEventArgs e)
        {
            TextFormatFlags flags = Flags | TextFormatFlags.VerticalCenter;
            if (TextAlign == ContentAlignment.MiddleRight)
                flags |= TextFormatFlags.Right;
            else if (TextAlign == ContentAlignment.MiddleCenter)
                flags |= TextFormatFlags.HorizontalCenter;
            TextRenderer.DrawText(e.Graphics, Text, Font, ClientRectangle, ForeColor, flags);
        }

        /// <summary>
        /// Size of the rendered content, including glyphs and smaller affixes;
        /// the UI acceptance test compares it with the bounds to find clipping.
        /// </summary>
        internal virtual Size MeasureContent()
        {
            return TextRenderer.MeasureText(Text, Font, Size.Empty, Flags);
        }

        [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential,
            CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
        private struct TextMetric
        {
            public int Height, Ascent, Descent, InternalLeading, ExternalLeading, AveCharWidth, MaxCharWidth,
                Weight, Overhang, DigitizedAspectX, DigitizedAspectY;
            public char FirstChar, LastChar, DefaultChar, BreakChar;
            public byte Italic, Underlined, StruckOut, PitchAndFamily, CharSet;
        }

        [System.Runtime.InteropServices.DllImport("gdi32.dll", CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
        private static extern bool GetTextMetrics(IntPtr deviceContext, out TextMetric metric);

        [System.Runtime.InteropServices.DllImport("gdi32.dll")]
        private static extern IntPtr SelectObject(IntPtr deviceContext, IntPtr gdiObject);

        [System.Runtime.InteropServices.DllImport("gdi32.dll")]
        private static extern bool DeleteObject(IntPtr gdiObject);

        private static readonly System.Collections.Generic.Dictionary<string, int> ascents =
            new System.Collections.Generic.Dictionary<string, int>();

        /// <summary>
        /// GDI ascent, the baseline offset TextRenderer uses. GDI+ family
        /// metrics differ for Bahnschrift's named instances.
        /// </summary>
        internal static float Ascent(Font font, Graphics graphics)
        {
            string key = font.Name + "|" + font.SizeInPoints.ToString(System.Globalization.CultureInfo.InvariantCulture)
                + "|" + font.Style + "|" + graphics.DpiY.ToString(System.Globalization.CultureInfo.InvariantCulture);
            int ascent;
            if (ascents.TryGetValue(key, out ascent)) return ascent;
            IntPtr deviceContext = graphics.GetHdc();
            IntPtr handle = font.ToHfont();
            try
            {
                IntPtr previous = SelectObject(deviceContext, handle);
                TextMetric metric;
                GetTextMetrics(deviceContext, out metric);
                SelectObject(deviceContext, previous);
                ascent = metric.Ascent;
            }
            finally
            {
                DeleteObject(handle);
                graphics.ReleaseHdc(deviceContext);
            }
            ascents[key] = ascent;
            return ascent;
        }
    }

    /// <summary>
    /// A figure such as "≈ 58.49 W": the approximation mark and unit are
    /// smaller and quieter than the number and share its baseline. An
    /// optional caption precedes the figure in the interface text font.
    /// </summary>
    internal class ReadoutLabel : AlignedLabel
    {
        internal float UnitScale = 0.6f;
        internal float PrefixScale = 1f;
        internal float CaptionScale = 0.86f;
        // Logical pixels at 96 DPI, scaled with the font.
        internal float Gap = 3f;
        internal float CaptionGap = 6f;
        internal bool SplitUnit = true;
        internal string Caption;
        internal Color UnitColor = UiTheme.Muted;
        internal Color PrefixColor = UiTheme.Faint;
        internal Color CaptionColor = UiTheme.Muted;
        // Design size of Font in points, so gaps scale with DPI.
        internal float DesignPoints = 9f;
        internal int DesignDpi = 96;

        public ReadoutLabel()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor, true);
            BackColor = Color.Transparent;
        }

        private struct Parts
        {
            public string Prefix;
            public string Number;
            public string Unit;
        }

        private Parts Split()
        {
            Parts parts = new Parts();
            string text = Text ?? "";
            if (text.StartsWith("≈ ", StringComparison.Ordinal))
            {
                parts.Prefix = "≈";
                text = text.Substring(2);
            }
            if (SplitUnit)
            {
                foreach (string unit in new string[] { " W", "%", " V", " A" })
                {
                    if (text.Length > unit.Length && text.EndsWith(unit, StringComparison.Ordinal))
                    {
                        parts.Unit = unit.Trim();
                        text = text.Substring(0, text.Length - unit.Length);
                        break;
                    }
                }
            }
            parts.Number = text;
            return parts;
        }

        private float FontScale
        {
            get { return Font.SizeInPoints / DesignPoints; }
        }

        private int Px(float logical)
        {
            return (int)Math.Round(logical * FontScale);
        }

        private Font Derived(string family, float ratio)
        {
            return new Font(family, Font.Size * ratio, FontStyle.Regular, Font.Unit);
        }

        internal override Size MeasureContent()
        {
            using (Graphics g = CreateGraphics())
            {
                int top, height;
                return new Size(Arrange(g, null, out top, out height), height);
            }
        }

        // Lays out (and optionally draws) the parts; returns the total width.
        private int Arrange(Graphics g, Point? origin, out int baselineTop, out int height)
        {
            Parts parts = Split();
            using (Font unit = Derived(Font.FontFamily.Name, UnitScale))
            using (Font prefix = Derived(Font.FontFamily.Name, PrefixScale))
            using (Font caption = Derived(UiTheme.TextFont, CaptionScale))
            {
                Size number = TextRenderer.MeasureText(g, parts.Number, Font, Size.Empty, Flags);
                float ascent = Ascent(Font, g);
                height = number.Height;
                int width = 0;
                int x = origin.HasValue ? origin.Value.X : 0;
                int y = origin.HasValue ? origin.Value.Y : 0;
                baselineTop = y;
                if (!String.IsNullOrEmpty(Caption))
                {
                    Size size = TextRenderer.MeasureText(g, Caption, caption, Size.Empty, Flags);
                    int cy = y + (int)Math.Round(ascent - Ascent(caption, g));
                    if (origin.HasValue)
                        TextRenderer.DrawText(g, Caption, caption, new Point(x + width, cy), CaptionColor, Flags);
                    width += size.Width + Px(CaptionGap);
                    height = Math.Max(height, cy - y + size.Height);
                }
                if (parts.Prefix != null)
                {
                    Size size = TextRenderer.MeasureText(g, parts.Prefix, prefix, Size.Empty, Flags);
                    int py = y + (int)Math.Round(ascent - Ascent(prefix, g));
                    if (origin.HasValue)
                        TextRenderer.DrawText(g, parts.Prefix, prefix, new Point(x + width, py), PrefixColor, Flags);
                    width += size.Width + Px(Gap);
                }
                if (origin.HasValue)
                    TextRenderer.DrawText(g, parts.Number, Font, new Point(x + width, y), ForeColor, Flags);
                width += number.Width;
                if (parts.Unit != null)
                {
                    width += Px(Gap);
                    Size size = TextRenderer.MeasureText(g, parts.Unit, unit, Size.Empty, Flags);
                    int uy = y + (int)Math.Round(ascent - Ascent(unit, g));
                    if (origin.HasValue)
                        TextRenderer.DrawText(g, parts.Unit, unit, new Point(x + width, uy), UnitColor, Flags);
                    width += size.Width;
                }
                return width;
            }
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            if (String.IsNullOrEmpty(Text)) return;
            int top, height;
            int width = Arrange(e.Graphics, null, out top, out height);
            int x = TextAlign == ContentAlignment.MiddleRight ? Width - width
                : TextAlign == ContentAlignment.MiddleCenter ? (Width - width) / 2 : 0;
            int y = (Height - height) / 2;
            Arrange(e.Graphics, new Point(Math.Max(0, x), Math.Max(0, y)), out top, out height);
        }
    }

    /// <summary>The headline figure.</summary>
    internal sealed class PowerReadout : ReadoutLabel
    {
        public PowerReadout()
        {
            UnitScale = 26f / 84f;
            PrefixScale = 34f / 84f;
            Gap = 8f;
            DesignPoints = 63f;
        }
    }
}
