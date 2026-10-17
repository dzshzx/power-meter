using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    internal sealed partial class MainForm
    {
        public void RenderPreview(string path)
        {
            ShowPreviewWindow();
            CapturePreview(path, "DeviceDpi=" + ReadWindowDpi().ToString(CultureInfo.InvariantCulture));
            Hide();
        }

        public void RenderDpiTransitionPreview(string path, params int[] targetDpis)
        {
            if (targetDpis == null || targetDpis.Length == 0)
                throw new ArgumentException("At least one target DPI is required.", "targetDpis");

            ShowPreviewWindow();
            foreach (int targetDpi in targetDpis)
                SendDpiTransition(targetDpi);

            CapturePreview(
                path,
                "HandledDpi=" + dpiLayout.CurrentDpi.ToString(CultureInfo.InvariantCulture));
            Hide();
        }

        private void SendDpiTransition(int targetDpi)
        {
            if (targetDpi < 96 || targetDpi > 768)
                throw new ArgumentOutOfRangeException("targetDpi");

            NativeMethods.Rect suggested;
            NativeMethods.GetWindowRect(Handle, out suggested);
            float transitionScale = targetDpi / (float)Math.Max(96, dpiLayout.CurrentDpi);
            suggested.Left += 17;
            suggested.Top += 13;
            suggested.Right = suggested.Left + DpiLayout.ScaleValue(Width, transitionScale);
            suggested.Bottom = suggested.Top + DpiLayout.ScaleValue(Height, transitionScale);

            IntPtr suggestedPointer = Marshal.AllocHGlobal(Marshal.SizeOf(typeof(NativeMethods.Rect)));
            try
            {
                Marshal.StructureToPtr(suggested, suggestedPointer, false);
                long packedDpi = (long)(
                    (uint)(targetDpi & 0xffff) |
                    ((uint)(targetDpi & 0xffff) << 16));
                NativeMethods.SendMessage(
                    Handle,
                    NativeMethods.WmDpiChanged,
                    new IntPtr(packedDpi),
                    suggestedPointer);
            }
            finally
            {
                Marshal.FreeHGlobal(suggestedPointer);
            }

            Application.DoEvents();
        }

        private void ShowPreviewWindow()
        {
            StartPosition = FormStartPosition.Manual;
            Location = new Point(
                Screen.PrimaryScreen.WorkingArea.Left + 20,
                Screen.PrimaryScreen.WorkingArea.Top + 20);
            Show();
            Application.DoEvents();
            RefreshReading();
            Application.DoEvents();
        }

        private void CapturePreview(string path, string dpiMetadata)
        {
            // Paint every control first: a section that has just expanded is
            // otherwise still unpainted (black) in the redirection surface.
            Refresh();
            Application.DoEvents();
            NativeMethods.Rect rectangle;
            NativeMethods.GetWindowRect(Handle, out rectangle);
            int captureWidth = Math.Max(1, rectangle.Right - rectangle.Left);
            int captureHeight = Math.Max(1, rectangle.Bottom - rectangle.Top);
            using (Bitmap bitmap = new Bitmap(captureWidth, captureHeight))
            {
                bool printed;
                using (Graphics graphics = Graphics.FromImage(bitmap))
                {
                    IntPtr deviceContext = graphics.GetHdc();
                    try
                    {
                        printed = NativeMethods.PrintWindow(Handle, deviceContext, 2);
                    }
                    finally
                    {
                        graphics.ReleaseHdc(deviceContext);
                    }
                }

                if (!printed)
                    DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
                // GetWindowRect includes invisible resize margins. PrintWindow
                // leaves those black; publish the visible DWM frame instead.
                NativeMethods.Rect frame;
                Rectangle visible = new Rectangle(Point.Empty, bitmap.Size);
                if (NativeMethods.DwmGetWindowAttribute(Handle, 9, out frame,
                    Marshal.SizeOf(typeof(NativeMethods.Rect))) == 0)
                {
                    Rectangle bounds = Rectangle.FromLTRB(frame.Left - rectangle.Left, frame.Top - rectangle.Top,
                        frame.Right - rectangle.Left, frame.Bottom - rectangle.Top);
                    if (bounds.Width > 0 && bounds.Height > 0 && visible.Contains(bounds)) visible = bounds;
                }
                using (Bitmap framed = bitmap.Clone(visible, PixelFormat.Format32bppArgb))
                using (Bitmap shaped = RoundCorners(framed))
                    shaped.Save(path, ImageFormat.Png);
            }

            bool positionMatched = rectangle.Left == lastSuggestedPosition.X
                && rectangle.Top == lastSuggestedPosition.Y;
            File.WriteAllText(
                path + ".txt",
                String.Format(
                    CultureInfo.InvariantCulture,
                    "{0}; Form={1}x{2}; Client={3}x{4}; Content={5}x{6}; "
                        + "WindowRect={7}x{8}; FormFontPixels={9:0.###}; "
                        + "PowerFontPixels={10:0.###}; AutoScroll={11}; "
                        + "WindowPositionApplied={12}; WindowPositionMatched={13}; ErrorArea={14}x{15}",
                    dpiMetadata,
                    Width,
                    Height,
                    ClientSize.Width,
                    ClientSize.Height,
                    AutoScrollMinSize.Width,
                    AutoScrollMinSize.Height,
                    captureWidth,
                    captureHeight,
                    Font.Size,
                    powerLabel.Font.Size,
                    AutoScroll,
                    lastDpiWindowPositionApplied,
                    positionMatched,
                    errorLabel.Width,
                    errorLabel.Height));
        }

        // PrintWindow returns the square client bitmap; on Windows 11 DWM
        // shows it with 8-pixel (scaled) rounded corners, so captures match.
        private Bitmap RoundCorners(Bitmap source)
        {
            Bitmap result = new Bitmap(source.Width, source.Height, PixelFormat.Format32bppArgb);
            using (Graphics graphics = Graphics.FromImage(result))
            {
                if (!roundedCorners)
                {
                    graphics.DrawImageUnscaled(source, 0, 0);
                    return result;
                }
                graphics.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.AntiAlias;
                graphics.PixelOffsetMode = System.Drawing.Drawing2D.PixelOffsetMode.HighQuality;
                float radius = 8f * LayoutScale;
                using (System.Drawing.Drawing2D.GraphicsPath path = WidgetPath.Rounded(
                    new RectangleF(0, 0, source.Width, source.Height), radius))
                using (TextureBrush brush = new TextureBrush(source))
                    graphics.FillPath(brush, path);
            }
            return result;
        }

        /// <summary>
        /// The window with the open settings popover drawn over it where it
        /// appears, composed from the two controls' own renderings. Nothing is
        /// read from the screen, so other windows never enter the image.
        /// </summary>
        internal void CaptureWithSettings(string path)
        {
            Refresh();
            Application.DoEvents();
            NativeMethods.Rect window;
            NativeMethods.GetWindowRect(Handle, out window);
            using (Bitmap frame = new Bitmap(Math.Max(1, window.Right - window.Left), Math.Max(1, window.Bottom - window.Top)))
            {
                using (Graphics graphics = Graphics.FromImage(frame))
                {
                    IntPtr deviceContext = graphics.GetHdc();
                    try { NativeMethods.PrintWindow(Handle, deviceContext, 2); }
                    finally { graphics.ReleaseHdc(deviceContext); }
                }
                using (Bitmap shaped = RoundCorners(frame))
                {
                    Panel content = settingsContent;
                    if (content != null && !content.IsDisposed)
                    {
                        float s = LayoutScale;
                        Point button = settingsButton.PointToScreen(Point.Empty);
                        Point origin = new Point(button.X + settingsButton.Width - content.Width - window.Left,
                            button.Y - DpiLayout.ScaleValue(6, s) - content.Height - window.Top);
                        using (Bitmap panel = new Bitmap(content.Width, content.Height))
                        using (Graphics graphics = Graphics.FromImage(shaped))
                        {
                            content.DrawToBitmap(panel, new Rectangle(Point.Empty, panel.Size));
                            graphics.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.AntiAlias;
                            for (int ring = 6; ring >= 1; ring--)
                                using (System.Drawing.Drawing2D.GraphicsPath shadow = WidgetPath.Rounded(new RectangleF(
                                    origin.X - ring * s, origin.Y - ring * s + 4 * s, content.Width + 2 * ring * s, content.Height + 2 * ring * s), (8 + ring) * s))
                                using (SolidBrush brush = new SolidBrush(Color.FromArgb(18, 0, 0, 0)))
                                    graphics.FillPath(brush, shadow);
                            using (System.Drawing.Drawing2D.GraphicsPath outline = WidgetPath.Rounded(
                                new RectangleF(origin.X, origin.Y, content.Width, content.Height), 8 * s))
                            using (TextureBrush brush = new TextureBrush(panel))
                            using (Pen border = new Pen(Color.FromArgb(58, 62, 64), Math.Max(1f, s)))
                            {
                                brush.TranslateTransform(origin.X, origin.Y);
                                graphics.FillPath(brush, outline);
                                graphics.DrawPath(border, outline);
                            }
                        }
                    }
                    shaped.Save(path, ImageFormat.Png);
                }
            }
        }

        public void RenderTrayIconPreview(string path, string glyph, bool discharging)
        {
            Color color = discharging ? UiTheme.TrayDischarging : UiTheme.TrayCharging;
            using (Icon icon = CreateTextIcon(glyph, color))
            using (Bitmap bitmap = icon.ToBitmap())
                bitmap.Save(path, ImageFormat.Png);
        }
    }
}
