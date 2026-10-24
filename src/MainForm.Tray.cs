using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Drawing.Imaging;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    internal sealed partial class MainForm
    {
        private void MinimizeToTray()
        {
            ShowInTaskbar = false;
            Hide();
        }

        private void RestoreFromTray()
        {
            startHidden = false;
            ShowInTaskbar = true;
            Show();
            WindowState = FormWindowState.Normal;
            Activate();
        }

        private void SetTrayTooltip(string value)
        {
            if (String.IsNullOrEmpty(value))
                value = "Net battery terminal power";
            trayIcon.Text = value.Length <= 63 ? value : value.Substring(0, 63);
        }

        private void SetTrayIcon(string glyph, Color tileColor)
        {
            if (String.Equals(glyph, lastTrayGlyph, StringComparison.Ordinal) && tileColor.ToArgb() == lastTrayColor.ToArgb())
                return;

            Icon nextIcon = CreateTextIcon(glyph, tileColor);
            Icon previousIcon = generatedTrayIcon;
            generatedTrayIcon = nextIcon;
            trayIcon.Icon = nextIcon;
            lastTrayGlyph = glyph;
            lastTrayColor = tileColor;

            if (previousIcon != null)
                previousIcon.Dispose();
        }

        private Icon CreateTextIcon(string glyph, Color tileColor)
        {
            using (Bitmap bitmap = new Bitmap(64, 64, PixelFormat.Format32bppArgb))
            using (Graphics graphics = Graphics.FromImage(bitmap))
            {
                graphics.Clear(Color.Transparent);
                graphics.SmoothingMode = SmoothingMode.AntiAlias;
                graphics.PixelOffsetMode = PixelOffsetMode.HighQuality;
                graphics.TextRenderingHint = System.Drawing.Text.TextRenderingHint.AntiAliasGridFit;

                // A solid tile keeps the watt glyph readable on any taskbar theme.
                using (GraphicsPath tile = WidgetPath.Rounded(new Rectangle(3, 3, 58, 58), 15))
                using (SolidBrush tileBrush = new SolidBrush(tileColor))
                    graphics.FillPath(tileBrush, tile);

                float fontSize = glyph.Length <= 2 ? 34f : 26f;
                using (Font font = new Font("Segoe UI", fontSize, FontStyle.Bold, GraphicsUnit.Pixel))
                using (StringFormat format = new StringFormat())
                {
                    format.Alignment = StringAlignment.Center;
                    format.LineAlignment = StringAlignment.Center;
                    graphics.DrawString(glyph, font, Brushes.White, new RectangleF(0f, 1f, 64f, 63f), format);
                }

                IntPtr iconHandle = bitmap.GetHicon();
                try
                {
                    using (Icon temporary = Icon.FromHandle(iconHandle))
                        return (Icon)temporary.Clone();
                }
                finally
                {
                    NativeMethods.DestroyIcon(iconHandle);
                }
            }
        }
    }
}
