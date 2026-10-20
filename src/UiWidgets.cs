using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Globalization;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    /// <summary>
    /// Unmodified Lucide geometry (ISC, see third_party/NOTICE.md; compact SVG
    /// arc flags are written out in full). The stroke colour is written into
    /// the SVG because AntdUI tints SVGs with a root fill.
    /// </summary>
    internal static class UiIcons
    {
        internal const string Zap = "<path d=\"M15.914 4a1.5 1.5 0 0 0-2.474-1.561l-9 9A1.5 1.5 0 0 0 5.5 14h4.002a.5.5 0 0 1 .471.666L8.086 20a1.5 1.5 0 0 0 2.475 1.56l9-9A1.5 1.5 0 0 0 18.5 10h-3.997a.5.5 0 0 1-.472-.667z\"/>";
        internal const string Pin = "<path d=\"M12 17v5\"/><path d=\"M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z\"/>";
        internal const string Minus = "<path d=\"M5 12h14\"/>";
        internal const string X = "<path d=\"M18 6 6 18\"/><path d=\"m6 6 12 12\"/>";
        internal const string Plug = "<path d=\"M12 22v-5\"/><path d=\"M15 8V2\"/><path d=\"M17 8a1 1 0 0 1 1 1v4a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V9a1 1 0 0 1 1-1z\"/><path d=\"M9 8V2\"/>";
        internal const string PlugZap = "<path d=\"M6.3 20.3a2.4 2.4 0 0 0 3.4 0L12 18l-6-6-2.3 2.3a2.4 2.4 0 0 0 0 3.4Z\"/><path d=\"m2 22 3-3\"/><path d=\"M7.5 13.5 10 11\"/><path d=\"M10.5 16.5 13 14\"/><path d=\"m18 3-4 4h6l-4 4\"/>";
        internal const string BatteryMedium = "<path d=\"M10 14v-4\"/><path d=\"M22 14v-4\"/><path d=\"M6 14v-4\"/><rect x=\"2\" y=\"6\" width=\"16\" height=\"12\" rx=\"2\"/>";
        internal const string TriangleAlert = "<path d=\"m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3\"/><path d=\"M12 9v4\"/><path d=\"M12 17h.01\"/>";
        internal const string Shield = "<path d=\"M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z\"/>";
        internal const string Settings = "<path d=\"M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915\"/><circle cx=\"12\" cy=\"12\" r=\"3\"/>";

        private static readonly Dictionary<string, Bitmap> cache = new Dictionary<string, Bitmap>();

        internal static string Svg(string body, Color stroke, float strokeWidth)
        {
            return "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"24\" height=\"24\" viewBox=\"0 0 24 24\">"
                + "<g fill=\"none\" stroke=\"" + UiTheme.Hex(stroke) + "\" stroke-width=\""
                + strokeWidth.ToString("0.##", CultureInfo.InvariantCulture)
                + "\" stroke-linecap=\"round\" stroke-linejoin=\"round\">" + body + "</g></svg>";
        }

        internal static string For(SupplyIcon icon)
        {
            switch (icon)
            {
                case SupplyIcon.ExternalPower: return Plug;
                case SupplyIcon.Charging: return PlugZap;
                case SupplyIcon.Battery: return BatteryMedium;
                case SupplyIcon.Alert: return TriangleAlert;
                default: return null;
            }
        }

        /// <summary>Rendered glyphs are few (icon x colour x size) and reused each tick.</summary>
        internal static Bitmap Render(string body, Color stroke, int size, float strokeWidth)
        {
            size = Math.Max(4, size);
            string key = stroke.ToArgb().ToString(CultureInfo.InvariantCulture) + "|" + size.ToString(CultureInfo.InvariantCulture)
                + "|" + strokeWidth.ToString(CultureInfo.InvariantCulture) + "|" + body;
            Bitmap bitmap;
            if (!cache.TryGetValue(key, out bitmap))
            {
                bitmap = AntdUI.SvgExtend.SvgToBmp(Svg(body, stroke, strokeWidth), size, size, null);
                cache[key] = bitmap;
            }
            return bitmap;
        }

        internal static void Draw(Graphics g, string body, Color stroke, Rectangle bounds, float strokeWidth)
        {
            if (body == null) return;
            int size = Math.Min(bounds.Width, bounds.Height);
            g.DrawImage(Render(body, stroke, size, strokeWidth),
                bounds.X + (bounds.Width - size) / 2, bounds.Y + (bounds.Height - size) / 2, size, size);
        }
    }

    /// <summary>Supply-state text with its Lucide glyph, right-aligned.</summary>
    internal sealed class StatusLabel : AlignedLabel
    {
        private string iconBody;

        public StatusLabel()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor, true);
            BackColor = Color.Transparent;
        }

        internal string IconBody
        {
            get { return iconBody; }
            set
            {
                if (value == iconBody) return;
                iconBody = value;
                Invalidate();
            }
        }

        private int IconSize { get { return (int)Math.Round(Font.Size * 15f / 12f * (Font.Unit == GraphicsUnit.Point ? 96f / 72f : 1f)); } }
        private int IconGap { get { return Math.Max(2, IconSize * 6 / 15); } }

        internal override Size MeasureContent()
        {
            Size text = TextRenderer.MeasureText(Text, Font, Size.Empty, Flags);
            int icon = iconBody == null ? 0 : IconSize + IconGap;
            return new Size(icon + text.Width, Math.Max(text.Height, iconBody == null ? 0 : IconSize));
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            if (String.IsNullOrEmpty(Text)) return;
            Size content = MeasureContent();
            int x = Math.Max(0, Width - content.Width);
            if (iconBody != null)
            {
                int size = IconSize;
                UiIcons.Draw(e.Graphics, iconBody, ForeColor, new Rectangle(x, (Height - size) / 2, size, size), 1.8f);
                x += size + IconGap;
            }
            TextRenderer.DrawText(e.Graphics, Text, Font, new Rectangle(x, 0, Width - x, Height), ForeColor,
                Flags | TextFormatFlags.VerticalCenter);
        }
    }

    /// <summary>
    /// Flat glyph button for the title bar and footer: hover fill, a lit glyph
    /// while toggled or active, and the system red hover for Close.
    /// </summary>
    internal sealed class GlyphButton : Control, IButtonControl
    {
        private bool hovered;
        private bool pressed;
        private bool toggle;
        private bool active;
        internal string IconBody;
        internal bool IsClose;
        internal bool AutoToggle;
        // Glyph size and corner radius in logical pixels for DesignHeight.
        internal float IconLogical = 15f;
        internal float StrokeWidth = 1.5f;
        internal int DesignHeight = 44;
        internal int RadiusLogical;
        internal event EventHandler ToggleChanged;

        public GlyphButton()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor, true);
            BackColor = Color.Transparent;
            Cursor = Cursors.Default;
            TabStop = true;
            AccessibleRole = AccessibleRole.PushButton;
        }

        internal bool Toggle
        {
            get { return toggle; }
            set
            {
                if (toggle == value) return;
                toggle = value;
                Invalidate();
                EventHandler handler = ToggleChanged;
                if (handler != null) handler(this, EventArgs.Empty);
            }
        }

        internal bool Active
        {
            get { return active; }
            set { if (active != value) { active = value; Invalidate(); } }
        }

        public DialogResult DialogResult { get; set; }

        public void NotifyDefault(bool value)
        {
        }

        public void PerformClick()
        {
            OnClick(EventArgs.Empty);
        }

        protected override void OnClick(EventArgs e)
        {
            if (AutoToggle) Toggle = !Toggle;
            base.OnClick(e);
        }

        protected override void OnMouseEnter(EventArgs e) { base.OnMouseEnter(e); hovered = true; Invalidate(); }
        protected override void OnMouseLeave(EventArgs e) { base.OnMouseLeave(e); hovered = false; pressed = false; Invalidate(); }
        protected override void OnMouseDown(MouseEventArgs e) { base.OnMouseDown(e); pressed = e.Button == MouseButtons.Left; Invalidate(); }
        protected override void OnMouseUp(MouseEventArgs e) { base.OnMouseUp(e); pressed = false; Invalidate(); }

        protected override bool IsInputKey(Keys keyData)
        {
            return (keyData & Keys.KeyCode) == Keys.Space || base.IsInputKey(keyData);
        }

        protected override void OnKeyDown(KeyEventArgs e)
        {
            base.OnKeyDown(e);
            if (e.KeyCode == Keys.Space || e.KeyCode == Keys.Enter) { PerformClick(); e.Handled = true; }
        }

        protected override void OnGotFocus(EventArgs e) { base.OnGotFocus(e); Invalidate(); }
        protected override void OnLostFocus(EventArgs e) { base.OnLostFocus(e); Invalidate(); }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float scale = Height / (float)DesignHeight;
            Color fill = Color.Empty;
            if (IsClose && (hovered || pressed))
                fill = pressed ? UiTheme.ClosePressed : UiTheme.CloseHover;
            else if (pressed)
                fill = UiTheme.ButtonActive;
            else if (hovered || active)
                fill = UiTheme.ButtonHover;
            if (!fill.IsEmpty)
            {
                using (GraphicsPath path = WidgetPath.Rounded(new Rectangle(0, 0, Width, Height), RadiusLogical * scale))
                using (SolidBrush brush = new SolidBrush(fill))
                    g.FillPath(brush, path);
            }
            Color glyph = IsClose && (hovered || pressed) ? Color.White
                : toggle || active || hovered ? UiTheme.Ink : UiTheme.Muted;
            int size = (int)Math.Round(IconLogical * scale);
            UiIcons.Draw(g, IconBody, glyph, new Rectangle((Width - size) / 2, (Height - size) / 2, size, size), StrokeWidth);
            if (Focused && ShowFocusCues)
                ControlPaint.DrawFocusRectangle(g, Rectangle.Inflate(ClientRectangle, -2, -2), UiTheme.Muted, Color.Transparent);
        }
    }

    /// <summary>Flat text button with an optional leading glyph.</summary>
    internal sealed class ActionButton : Control, IButtonControl
    {
        private bool hovered;
        internal string IconBody;
        internal Color Fill = UiTheme.ActionFill;
        internal Color FillHover = UiTheme.ActionFillHover;
        internal int DesignHeight = 28;

        public ActionButton()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor, true);
            BackColor = Color.Transparent;
            ForeColor = UiTheme.Ink;
            TabStop = true;
            AccessibleRole = AccessibleRole.PushButton;
        }

        public DialogResult DialogResult { get; set; }
        public void NotifyDefault(bool value) { }
        public void PerformClick() { if (Enabled) OnClick(EventArgs.Empty); }

        protected override void OnMouseEnter(EventArgs e) { base.OnMouseEnter(e); hovered = true; Invalidate(); }
        protected override void OnMouseLeave(EventArgs e) { base.OnMouseLeave(e); hovered = false; Invalidate(); }
        protected override void OnEnabledChanged(EventArgs e) { base.OnEnabledChanged(e); Invalidate(); }
        protected override void OnTextChanged(EventArgs e) { base.OnTextChanged(e); Invalidate(); }

        protected override void OnKeyDown(KeyEventArgs e)
        {
            base.OnKeyDown(e);
            if (e.KeyCode == Keys.Space || e.KeyCode == Keys.Enter) { PerformClick(); e.Handled = true; }
        }

        internal Size MeasureContent()
        {
            float scale = Height / (float)DesignHeight;
            Size text = TextRenderer.MeasureText(Text, Font, Size.Empty, AlignedLabel.Flags);
            int icon = IconBody == null ? 0 : (int)Math.Round(20 * scale);
            return new Size(text.Width + icon + (int)Math.Round(24 * scale), text.Height);
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float scale = Height / (float)DesignHeight;
            using (GraphicsPath path = WidgetPath.Rounded(new Rectangle(0, 0, Width, Height), 4 * scale))
            using (SolidBrush brush = new SolidBrush(hovered && Enabled ? FillHover : Fill))
                g.FillPath(brush, path);
            Color fore = Enabled ? ForeColor : UiTheme.Faint;
            Size text = TextRenderer.MeasureText(g, Text, Font, Size.Empty, AlignedLabel.Flags);
            int iconSize = (int)Math.Round(14 * scale);
            int gap = (int)Math.Round(6 * scale);
            int total = text.Width + (IconBody == null ? 0 : iconSize + gap);
            int x = Math.Max(0, (Width - total) / 2);
            if (IconBody != null)
            {
                UiIcons.Draw(g, IconBody, fore, new Rectangle(x, (Height - iconSize) / 2, iconSize, iconSize), 1.8f);
                x += iconSize + gap;
            }
            TextRenderer.DrawText(g, Text, Font, new Rectangle(x, 0, Width - x, Height), fore,
                AlignedLabel.Flags | TextFormatFlags.VerticalCenter);
            if (Focused && ShowFocusCues)
                ControlPaint.DrawFocusRectangle(g, Rectangle.Inflate(ClientRectangle, -2, -2), UiTheme.Muted, Color.Transparent);
        }
    }

    /// <summary>On/off switch; the owner applies the requested state.</summary>
    internal sealed class ToggleSwitch : Control
    {
        private CheckState state;
        internal event EventHandler Toggled;

        public ToggleSwitch()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor, true);
            BackColor = Color.Transparent;
            Cursor = Cursors.Hand;
            TabStop = true;
            AccessibleRole = AccessibleRole.CheckButton;
        }

        internal CheckState CheckState
        {
            get { return state; }
            set { if (state != value) { state = value; Invalidate(); } }
        }

        internal bool Checked { get { return state == CheckState.Checked; } }

        protected override void OnClick(EventArgs e)
        {
            base.OnClick(e);
            Focus();
            EventHandler handler = Toggled;
            if (handler != null) handler(this, EventArgs.Empty);
        }

        protected override void OnKeyDown(KeyEventArgs e)
        {
            base.OnKeyDown(e);
            if (e.KeyCode == Keys.Space) { OnClick(EventArgs.Empty); e.Handled = true; }
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float s = Height / 20f;
            RectangleF track = new RectangleF(0.5f, 0.5f, Width - 1f, Height - 1f);
            bool on = state == CheckState.Checked;
            using (GraphicsPath path = WidgetPath.Rounded(track, track.Height / 2))
            using (SolidBrush brush = new SolidBrush(on ? UiTheme.Charging : UiTheme.CellTrack))
                g.FillPath(brush, path);
            float knob = 12 * s;
            float x = on ? Width - 4 * s - knob : state == CheckState.Indeterminate ? (Width - knob) / 2 : 4 * s;
            using (SolidBrush brush = new SolidBrush(on ? UiTheme.ToggleKnob : UiTheme.Muted))
                g.FillEllipse(brush, x, (Height - knob) / 2, knob, knob);
            if (Focused && ShowFocusCues)
                ControlPaint.DrawFocusRectangle(g, ClientRectangle, UiTheme.Muted, Color.Transparent);
        }
    }

    /// <summary>Ten-segment battery level.</summary>
    internal sealed class BatteryCells : Control
    {
        private int level = -1;
        private Color fill = UiTheme.Charging;
        private bool quiet;

        public BatteryCells()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor, true);
            BackColor = Color.Transparent;
        }

        internal void SetValue(int value, Color color, bool idle)
        {
            level = value;
            fill = color;
            quiet = idle;
            Invalidate();
        }

        internal int LitCells
        {
            get { return level < 0 ? 0 : Math.Min(10, (int)Math.Round(level / 10.0, MidpointRounding.AwayFromZero)); }
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float s = Height / 4f;
            float gap = 2 * s;
            float width = (Width - 9 * gap) / 10f;
            int lit = LitCells;
            using (SolidBrush track = new SolidBrush(UiTheme.CellTrack))
            using (SolidBrush on = new SolidBrush(UiTheme.Alpha(fill, quiet ? 140 : 216)))
            {
                for (int i = 0; i < 10; i++)
                {
                    RectangleF cell = new RectangleF(i * (width + gap), 0, width, Height);
                    using (GraphicsPath path = WidgetPath.Rounded(cell, Math.Max(1f, s)))
                        g.FillPath(i < lit ? on : track, path);
                }
            }
        }
    }

    /// <summary>
    /// Diagnostic notice: a hairline, the alert glyph and up to two wrapped
    /// lines; each line's boundary name is bold. The complete text is the
    /// tooltip when lines are cut.
    /// </summary>
    internal sealed class NoticeLabel : Control
    {
        internal const int DesignHeight = 52;
        internal int VisibleLineCount { get; private set; }
        internal int LinesNeeded { get; private set; }

        public NoticeLabel()
        {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
                | ControlStyles.SupportsTransparentBackColor | ControlStyles.ResizeRedraw, true);
            BackColor = Color.Transparent;
            ForeColor = UiTheme.ErrorRed;
        }

        protected override void OnTextChanged(EventArgs e) { base.OnTextChanged(e); Invalidate(); }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            float s = Height / (float)DesignHeight;
            using (Pen hair = new Pen(UiTheme.Hairline))
                g.DrawLine(hair, 0, 0, Width, 0);
            if (String.IsNullOrEmpty(Text)) return;
            int icon = (int)Math.Round(15 * s);
            int top = (int)Math.Round(12 * s);
            UiIcons.Draw(g, UiIcons.TriangleAlert, ForeColor, new Rectangle(0, top + (int)Math.Round(2 * s), icon, icon), 1.8f);
            int left = icon + (int)Math.Round(8 * s);
            int lineHeight = (int)Math.Round(19 * s);
            int maxLines = Math.Max(1, (Height - top) / lineHeight);
            int used = 0;
            int needed = 0;
            using (Font bold = new Font(Font, FontStyle.Bold))
            {
                foreach (string line in Text.Replace("\r", "").Split('\n'))
                {
                    if (line.Length == 0) continue;
                    int colon = line.IndexOf(": ", StringComparison.Ordinal);
                    string label = colon > 0 ? line.Substring(0, colon) + (Strings.IsChinese ? "：" : ": ") : "";
                    string rest = colon > 0 ? line.Substring(colon + 2) : line;
                    int labelWidth = label.Length == 0 ? 0 : TextRenderer.MeasureText(g, label, bold, Size.Empty, AlignedLabel.Flags).Width;
                    int restWidth = Math.Max(1, Width - left - labelWidth);
                    Size restSize = TextRenderer.MeasureText(g, rest, Font, new Size(restWidth, Int32.MaxValue),
                        TextFormatFlags.NoPadding | TextFormatFlags.WordBreak | TextFormatFlags.NoPrefix);
                    int lines = Math.Max(1, (restSize.Height + lineHeight / 3) / Math.Max(1, Font.Height));
                    needed += lines;
                    if (used >= maxLines) continue;
                    int available = maxLines - used;
                    int shown = Math.Min(lines, available);
                    int y = top + used * lineHeight;
                    if (label.Length > 0)
                        TextRenderer.DrawText(g, label, bold, new Rectangle(left, y, labelWidth, lineHeight), ForeColor,
                            AlignedLabel.Flags | TextFormatFlags.VerticalCenter);
                    TextRenderer.DrawText(g, rest, Font, new Rectangle(left + labelWidth, y + (lineHeight - Font.Height) / 2,
                        restWidth, shown * lineHeight), ForeColor,
                        TextFormatFlags.NoPadding | TextFormatFlags.WordBreak | TextFormatFlags.NoPrefix
                            | TextFormatFlags.EndEllipsis);
                    used += shown;
                }
            }
            VisibleLineCount = used;
            LinesNeeded = needed;
        }
    }

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
                using (Pen tick = new Pen(UiTheme.SegmentOn, 1f))
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
            using (Pen grid = new Pen(UiTheme.SegmentTrack, 1f))
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
