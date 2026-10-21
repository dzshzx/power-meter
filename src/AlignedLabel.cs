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
        private float unitScale = 0.6f;
        private float prefixScale = 1f;
        private float captionScale = 0.86f;

        internal float UnitScale
        {
            get { return unitScale; }
            set { unitScale = value; DisposeDerivedFonts(); }
        }

        internal float PrefixScale
        {
            get { return prefixScale; }
            set { prefixScale = value; DisposeDerivedFonts(); }
        }

        internal float CaptionScale
        {
            get { return captionScale; }
            set { captionScale = value; DisposeDerivedFonts(); }
        }

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

        // Smaller affix and caption fonts, created on first use and rebuilt
        // when Font (DPI) or a scale changes.
        private Font unitFont;
        private Font prefixFont;
        private Font captionFont;

        private Font UnitFont
        {
            get { return unitFont ?? (unitFont = Derived(Font.FontFamily.Name, UnitScale)); }
        }

        private Font PrefixFont
        {
            get { return prefixFont ?? (prefixFont = Derived(Font.FontFamily.Name, PrefixScale)); }
        }

        private Font CaptionFont
        {
            get { return captionFont ?? (captionFont = Derived(UiTheme.TextFont, CaptionScale)); }
        }

        private Font Derived(string family, float ratio)
        {
            return new Font(family, Font.Size * ratio, FontStyle.Regular, Font.Unit);
        }

        private void DisposeDerivedFonts()
        {
            foreach (Font font in new Font[] { unitFont, prefixFont, captionFont })
                if (font != null) font.Dispose();
            unitFont = prefixFont = captionFont = null;
        }

        protected override void OnFontChanged(EventArgs e)
        {
            DisposeDerivedFonts();
            base.OnFontChanged(e);
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing) DisposeDerivedFonts();
            base.Dispose(disposing);
        }

        // One laid-out part, positioned relative to the content's top-left.
        private struct Piece
        {
            public string Text;
            public Font Font;
            public Color Color;
            public int X;
            public int Y;
        }

        internal override Size MeasureContent()
        {
            using (Graphics g = CreateGraphics())
                return Measure(g);
        }

        /// <summary>Content size measured with a caller's device context.</summary>
        internal Size Measure(Graphics g)
        {
            int count;
            return Arrange(g, new Piece[4], out count);
        }

        // Lays out caption, prefix, number and unit on the number's baseline;
        // returns the content size.
        private Size Arrange(Graphics g, Piece[] pieces, out int count)
        {
            Parts parts = Split();
            count = 0;
            Size number = TextRenderer.MeasureText(g, parts.Number, Font, Size.Empty, Flags);
            float ascent = Ascent(Font, g);
            int height = number.Height;
            int width = 0;
            if (!String.IsNullOrEmpty(Caption))
            {
                Font caption = CaptionFont;
                Size size = TextRenderer.MeasureText(g, Caption, caption, Size.Empty, Flags);
                int cy = (int)Math.Round(ascent - Ascent(caption, g));
                pieces[count++] = new Piece { Text = Caption, Font = caption, Color = CaptionColor, X = width, Y = cy };
                width += size.Width + Px(CaptionGap);
                height = Math.Max(height, cy + size.Height);
            }
            if (parts.Prefix != null)
            {
                Font prefix = PrefixFont;
                Size size = TextRenderer.MeasureText(g, parts.Prefix, prefix, Size.Empty, Flags);
                pieces[count++] = new Piece { Text = parts.Prefix, Font = prefix, Color = PrefixColor, X = width,
                    Y = (int)Math.Round(ascent - Ascent(prefix, g)) };
                width += size.Width + Px(Gap);
            }
            pieces[count++] = new Piece { Text = parts.Number, Font = Font, Color = ForeColor, X = width, Y = 0 };
            width += number.Width;
            if (parts.Unit != null)
            {
                width += Px(Gap);
                Font unit = UnitFont;
                Size size = TextRenderer.MeasureText(g, parts.Unit, unit, Size.Empty, Flags);
                pieces[count++] = new Piece { Text = parts.Unit, Font = unit, Color = UnitColor, X = width,
                    Y = (int)Math.Round(ascent - Ascent(unit, g)) };
                width += size.Width;
            }
            return new Size(width, height);
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            if (String.IsNullOrEmpty(Text)) return;
            Piece[] pieces = new Piece[4];
            int count;
            Size content = Arrange(e.Graphics, pieces, out count);
            int x = TextAlign == ContentAlignment.MiddleRight ? Width - content.Width
                : TextAlign == ContentAlignment.MiddleCenter ? (Width - content.Width) / 2 : 0;
            int y = (Height - content.Height) / 2;
            Point origin = new Point(Math.Max(0, x), Math.Max(0, y));
            for (int index = 0; index < count; index++)
                TextRenderer.DrawText(e.Graphics, pieces[index].Text, pieces[index].Font,
                    new Point(origin.X + pieces[index].X, origin.Y + pieces[index].Y), pieces[index].Color, Flags);
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
