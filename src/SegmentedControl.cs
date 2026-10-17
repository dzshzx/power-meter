using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    /// <summary>
    /// Two-option segmented switch for the display mode. The tray menu remains
    /// the parallel path; <see cref="DpiLayout"/> manages <see cref="Font"/>,
    /// so painting reuses it instead of creating unscaled fonts.
    /// </summary>
    internal sealed class SegmentedControl : Panel
    {
        private readonly string[] items;
        private int selectedIndex;
        private int hoveredIndex = -1;

        internal event EventHandler SelectionChanged;

        internal SegmentedControl(string first, string second)
        {
            items = new string[] { first, second };
            DoubleBuffered = true;
            BackColor = UiTheme.Canvas;
            Cursor = Cursors.Hand;
            SetStyle(ControlStyles.Selectable, true);
            TabStop = true;
            AccessibleName = "功率显示口径";
            AccessibleRole = AccessibleRole.List;
        }

        internal int SelectedIndex
        {
            get { return selectedIndex; }
            set
            {
                if (value == selectedIndex)
                    return;
                selectedIndex = value;
                Invalidate();
            }
        }

        protected override void OnMouseDown(MouseEventArgs e)
        {
            base.OnMouseDown(e);
            if (e.Button != MouseButtons.Left)
                return;
            Focus();
            int index = e.X >= Width / 2 ? 1 : 0;
            SelectItem(index);
        }

        private void SelectItem(int index)
        {
            if (index == selectedIndex)
                return;
            SelectedIndex = index;
            EventHandler handler = SelectionChanged;
            if (handler != null)
                handler(this, EventArgs.Empty);
        }

        protected override bool IsInputKey(Keys keyData)
        {
            Keys key = keyData & Keys.KeyCode;
            return key == Keys.Left || key == Keys.Right || base.IsInputKey(keyData);
        }

        protected override void OnKeyDown(KeyEventArgs e)
        {
            base.OnKeyDown(e);
            if (e.KeyCode == Keys.Left || e.KeyCode == Keys.Right || e.KeyCode == Keys.Space)
            {
                SelectItem(e.KeyCode == Keys.Left ? 0 : e.KeyCode == Keys.Right ? 1 : 1 - selectedIndex);
                e.Handled = true;
                e.SuppressKeyPress = true;
            }
        }

        protected override void OnGotFocus(EventArgs e) { base.OnGotFocus(e); Invalidate(); }
        protected override void OnLostFocus(EventArgs e) { base.OnLostFocus(e); Invalidate(); }

        protected override void OnMouseMove(MouseEventArgs e)
        {
            base.OnMouseMove(e);
            int index = ClientRectangle.Contains(e.Location) ? (e.X >= Width / 2 ? 1 : 0) : -1;
            if (hoveredIndex == index) return;
            hoveredIndex = index;
            Invalidate();
        }

        protected override void OnMouseLeave(EventArgs e)
        {
            base.OnMouseLeave(e);
            hoveredIndex = -1;
            Invalidate();
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            base.OnPaint(e);
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float scale = Height / 26f;
            using (SolidBrush back = new SolidBrush(UiTheme.Canvas))
                g.FillRectangle(back, ClientRectangle);
            using (GraphicsPath track = WidgetPath.Rounded(new RectangleF(0, 0, Width, Height), 5 * scale))
            using (SolidBrush trackBrush = new SolidBrush(UiTheme.SegmentTrack))
                g.FillPath(trackBrush, track);

            int pad = Math.Max(1, (int)Math.Round(2 * scale));
            int pillWidth = Width / 2 - pad;
            if (hoveredIndex >= 0 && hoveredIndex != selectedIndex)
            {
                int hoverX = hoveredIndex == 0 ? pad : Width - pad - pillWidth;
                using (GraphicsPath hover = WidgetPath.Rounded(new RectangleF(hoverX, pad, pillWidth, Height - 2 * pad), 3 * scale))
                using (SolidBrush hoverBrush = new SolidBrush(UiTheme.SegmentHover))
                    g.FillPath(hoverBrush, hover);
            }
            int pillX = selectedIndex == 0 ? pad : Width - pad - pillWidth;
            Rectangle segment = new Rectangle(pillX, pad, pillWidth, Height - 2 * pad);
            using (GraphicsPath pill = WidgetPath.Rounded(new RectangleF(segment.X, segment.Y, segment.Width, segment.Height), 3 * scale))
            using (SolidBrush pillBrush = new SolidBrush(UiTheme.SegmentOn))
                g.FillPath(pillBrush, pill);
            if (Focused && ShowFocusCues)
                ControlPaint.DrawFocusRectangle(g, Rectangle.Inflate(segment, -pad, -pad), UiTheme.Muted, Color.Transparent);

            using (Font strong = new Font(Font, FontStyle.Bold))
            {
                for (int i = 0; i < items.Length; i++)
                {
                    Rectangle cell = i == 0 ? new Rectangle(pad, 0, pillWidth, Height) : new Rectangle(Width - pad - pillWidth, 0, pillWidth, Height);
                    TextRenderer.DrawText(
                        g,
                        Strings.Get(items[i]),
                        i == selectedIndex ? strong : Font,
                        cell,
                        i == selectedIndex || i == hoveredIndex ? UiTheme.Ink : UiTheme.Muted,
                        TextFormatFlags.HorizontalCenter | TextFormatFlags.VerticalCenter
                            | TextFormatFlags.NoPadding | TextFormatFlags.SingleLine);
                }
            }
        }
    }
}
