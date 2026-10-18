using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Drawing.Imaging;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows.Forms;

[assembly: System.Runtime.Versioning.TargetFramework(
    ".NETFramework,Version=v4.8.1",
    FrameworkDisplayName = ".NET Framework 4.8.1")]
[assembly: System.Reflection.AssemblyTitle("Power Meter")]
[assembly: System.Reflection.AssemblyProduct("Power Meter")]
[assembly: System.Runtime.CompilerServices.InternalsVisibleTo("PowerMeter.Tests")]

namespace BatteryChargeMeter
{
    internal static class NativeMethods
    {
        internal const int WmDpiChanged = 0x02E0;
        internal const int WmNcCalcSize = 0x0083;
        internal const int WmNcHitTest = 0x0084;
        internal const int WmSysCommand = 0x0112;
        internal const int WmStyleChanging = 0x007C;
        internal const int HtClient = 1;
        internal const int HtCaption = 2;
        internal const int ScMaximize = 0xF030;
        internal const int GwlStyle = -16;
        internal const int WsCaption = 0x00C00000;
        internal const int WsSysMenu = 0x00080000;
        internal const int WsMinimizeBox = 0x00020000;
        internal const int WsMaximizeBox = 0x00010000;
        internal const int WsVScroll = 0x00200000;
        internal const int WsHScroll = 0x00100000;
        internal const int SmCxVScroll = 2;
        internal const int SmCyHScroll = 3;
        internal const uint SwpNoSize = 0x0001;
        internal const uint SwpNoMove = 0x0002;
        internal const uint SwpNoZOrder = 0x0004;
        internal const uint SwpNoActivate = 0x0010;
        internal const uint SwpFrameChanged = 0x0020;
        internal const int DwmUseImmersiveDarkMode = 20;
        internal const int DwmWindowCornerPreference = 33;
        internal const int DwmBorderColor = 34;
        internal const int DwmCornerRound = 2;

        [StructLayout(LayoutKind.Sequential)]
        internal struct Rect
        {
            public int Left;
            public int Top;
            public int Right;
            public int Bottom;
        }

        [DllImport("user32.dll")]
        internal static extern bool GetWindowRect(IntPtr windowHandle, out Rect rectangle);

        [DllImport("user32.dll")]
        internal static extern uint GetDpiForWindow(IntPtr windowHandle);

        [DllImport("user32.dll")]
        internal static extern IntPtr SendMessage(
            IntPtr windowHandle,
            int message,
            IntPtr wParam,
            IntPtr lParam);

        [DllImport("user32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        internal static extern bool SetWindowPos(
            IntPtr windowHandle,
            IntPtr insertAfter,
            int x,
            int y,
            int width,
            int height,
            uint flags);

        [DllImport("user32.dll", EntryPoint = "GetWindowLong")]
        internal static extern int GetWindowLong(IntPtr windowHandle, int index);

        [DllImport("user32.dll", EntryPoint = "SetWindowLong")]
        internal static extern int SetWindowLong(IntPtr windowHandle, int index, int value);

        [DllImport("user32.dll")]
        internal static extern int GetSystemMetricsForDpi(int index, uint dpi);

        [DllImport("user32.dll")]
        internal static extern bool PrintWindow(IntPtr windowHandle, IntPtr deviceContext, uint flags);

        [DllImport("dwmapi.dll")]
        internal static extern int DwmGetWindowAttribute(IntPtr windowHandle, int attribute,
            out Rect rectangle, int size);

        [DllImport("dwmapi.dll")]
        internal static extern int DwmSetWindowAttribute(IntPtr windowHandle, int attribute,
            ref int value, int size);

        [DllImport("user32.dll")]
        internal static extern bool DestroyIcon(IntPtr iconHandle);
    }

    internal sealed partial class MainForm : Form
    {
        // Design geometry in logical pixels at 96 DPI.
        internal const int DesignWidth = 388;
        internal const int DesignHeight = 660;
        internal const int TitleBarHeight = 44;
        internal const int NoticeTop = 528;
        internal const int NoticeSectionHeight = 92;
        private const int TripTop = 340;
        private const int TripBottom = 420;
        private const int RowsTop = 428;
        private const int RowHeight = 30;

        private readonly StatusLabel stateLabel;
        private readonly ReadoutLabel updatedLabel;
        private readonly PowerReadout powerLabel;
        private readonly ReadoutLabel voltageLabel;
        private readonly ReadoutLabel currentLabel;
        private readonly ReadoutLabel percentageLabel;
        private readonly ReadoutLabel statisticsLabel;
        private readonly Label statisticsCaption;
        private readonly Label historyCaption;
        private readonly Label batteryCaption;
        private readonly ReadoutLabel historyLabel;
        private readonly NoticeLabel errorLabel;
        private readonly ActionButton elevateButton;
        private readonly BatteryCells batteryCells;
        private readonly TrendChart trendChart;
        private readonly GlyphButton minimizeButton;
        private readonly GlyphButton closeButton;
        private readonly Timer timer;
        private readonly NotifyIcon trayIcon;
        private readonly ContextMenuStrip trayMenu;
        private readonly bool trayEnabled;
        private readonly DpiLayout dpiLayout;
        private Icon generatedTrayIcon;
        private string lastTrayGlyph = "";
        private Color lastTrayColor = Color.Empty;
        private bool lastDpiWindowPositionApplied;
        private Point lastSuggestedPosition;
        private bool roundedCorners;
        private readonly Label title;
        private MeterSession session = new MeterSession(DisplayPreference.Load());
        private MeterView lastView;
        private SegmentedControl modeSegments;
        private Panel footerBand;
        private ToolStripMenuItem wholeModeItem;
        private ToolStripMenuItem batteryModeItem;
        private string elevationMessage;
        private uint restoreMessage;
        private IntPtr restoreWindow;

        public MainForm(bool enableTray) : this(enableTray, null)
        {
        }

        public MainForm(bool enableTray, string startupMessage) : this(enableTray, startupMessage, false)
        {
        }

        public MainForm(bool enableTray, string startupMessage, bool hidden)
        {
            trayEnabled = enableTray;
            startHidden = hidden;
            Text = Strings.AppName;
            AutoScaleMode = AutoScaleMode.None;
            AutoScroll = true;
            // The title bar is drawn by the window; the native caption styles
            // are restored after the handle exists (see ApplyWindowChrome).
            FormBorderStyle = FormBorderStyle.None;
            ClientSize = new Size(DesignWidth, DesignHeight);
            MaximizeBox = false;
            MinimizeBox = true;
            StartPosition = FormStartPosition.CenterScreen;
            BackColor = UiTheme.Canvas;
            ForeColor = UiTheme.Ink;
            DoubleBuffered = true;
            Font = UiTheme.Text(9f, FontStyle.Regular);
            TopMost = true;
            try
            {
                Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
            }
            catch
            {
                // A build without the embedded icon keeps the stock window icon.
            }

            modeSegments = new SegmentedControl("整机功率", "电池端");
            modeSegments.Font = UiTheme.Text(9f, FontStyle.Regular);
            modeSegments.Bounds = new Rectangle(44, 9, 136, 26);
            modeSegments.SelectedIndex = session.Mode == DisplayMode.Battery ? 1 : 0;
            modeSegments.SelectionChanged += delegate
            {
                ChangeMode(modeSegments.SelectedIndex == 1 ? DisplayMode.Battery : DisplayMode.WholeSystem);
            };
            Controls.Add(modeSegments);

            minimizeButton = CaptionButton(UiIcons.Minus, "最小化", 304);
            minimizeButton.Click += delegate { WindowState = FormWindowState.Minimized; };
            closeButton = CaptionButton(UiIcons.X, "关闭", 346);
            closeButton.IsClose = true;
            closeButton.Click += delegate { Close(); };

            title = NewLabel("估算整机输入功率", 18, 50, 168, 20, 9f, FontStyle.Regular);
            title.ForeColor = UiTheme.Muted;

            stateLabel = new StatusLabel();
            stateLabel.Text = Strings.Get("正在读取");
            stateLabel.Bounds = new Rectangle(188, 50, 182, 20);
            stateLabel.Font = UiTheme.Text(9f, FontStyle.Bold);
            stateLabel.TextAlign = ContentAlignment.MiddleRight;

            powerLabel = new PowerReadout();
            powerLabel.Text = "--.-- W";
            powerLabel.Bounds = new Rectangle(14, 70, 362, 101);
            powerLabel.Font = UiTheme.Numeric("Bahnschrift Light SemiCondensed", 63f);
            powerLabel.ForeColor = UiTheme.Charging;

            voltageLabel = Readout(18, 173, 160, 20, "Bahnschrift", 10.5f, false);
            voltageLabel.CaptionScale = 12f / 14f;
            currentLabel = Readout(180, 173, 190, 20, "Bahnschrift", 10.5f, false);
            currentLabel.CaptionScale = 12f / 14f;
            currentLabel.PrefixColor = UiTheme.Ink;

            trendChart = new TrendChart();
            trendChart.Bounds = new Rectangle(0, 198, TrendChart.DesignWidth, TrendChart.DesignHeight);
            trendChart.Font = UiTheme.Numeric("Bahnschrift", 7.5f);
            Controls.Add(trendChart);

            statisticsCaption = NewLabel("", 18, TripTop + 11, 114, 18, 9f, FontStyle.Regular);
            statisticsCaption.ForeColor = UiTheme.Muted;
            historyCaption = NewLabel("", 149, TripTop + 11, 100, 18, 9f, FontStyle.Regular);
            historyCaption.ForeColor = UiTheme.Muted;
            batteryCaption = NewLabel("电池电量", 266, TripTop + 11, 104, 18, 9f, FontStyle.Regular);
            batteryCaption.ForeColor = UiTheme.Muted;
            Controls.Add(statisticsCaption);
            Controls.Add(historyCaption);
            Controls.Add(batteryCaption);

            statisticsLabel = Readout(18, TripTop + 31, 114, 32, "Bahnschrift SemiLight", 16.5f, true);
            historyLabel = Readout(149, TripTop + 31, 100, 32, "Bahnschrift SemiLight", 16.5f, true);
            percentageLabel = Readout(266, TripTop + 31, 104, 32, "Bahnschrift SemiLight", 16.5f, true);
            foreach (ReadoutLabel statistic in new ReadoutLabel[] { statisticsLabel, historyLabel, percentageLabel })
            {
                statistic.UnitScale = 13f / 22f;
                statistic.PrefixScale = 1f;
            }

            batteryCells = new BatteryCells();
            batteryCells.Bounds = new Rectangle(266, TripTop + 65, 104, 4);
            Controls.Add(batteryCells);

            BuildPowerSourceRows();

            errorLabel = new NoticeLabel();
            errorLabel.Bounds = new Rectangle(18, NoticeTop, 352, NoticeLabel.DesignHeight);
            errorLabel.Font = UiTheme.Text(9f, FontStyle.Regular);
            errorLabel.TextChanged += delegate { UpdateDiagnosticLayout(); };

            elevateButton = new ActionButton();
            elevateButton.Tag = "以管理员身份重新启动";
            elevateButton.Text = Strings.Get("以管理员身份重新启动");
            elevateButton.IconBody = UiIcons.Shield;
            elevateButton.Font = UiTheme.Text(9f, FontStyle.Regular);
            elevateButton.Bounds = new Rectangle(41, NoticeTop + 54, 200, 28);
            elevateButton.Click += delegate { RestartElevated(elevateButton); };

            footerBand = new Panel();
            footerBand.BackColor = UiTheme.FooterBand;
            footerBand.Bounds = new Rectangle(0, NoticeTop + NoticeSectionHeight, DesignWidth, 40);

            updatedLabel = Readout(18, 1, 200, 39, "Bahnschrift", 9f, false);
            updatedLabel.Caption = Strings.Get("更新于");
            updatedLabel.CaptionScale = 1f;
            updatedLabel.CaptionColor = UiTheme.Faint;
            updatedLabel.ForeColor = UiTheme.Muted;
            updatedLabel.Text = "--:--:--";
            Controls.Remove(updatedLabel);
            footerBand.Controls.Add(updatedLabel);

            Panel footerRule = new Panel();
            footerRule.BackColor = UiTheme.Hairline;
            footerRule.Bounds = new Rectangle(0, 0, DesignWidth, 1);
            footerBand.Controls.Add(footerRule);

            BuildSettingsControls();

            Controls.Add(footerBand);
            Controls.Add(title);
            Controls.Add(stateLabel);
            Controls.Add(powerLabel);
            Controls.Add(errorLabel);
            Controls.Add(elevateButton);

            timer = new Timer();
            timer.Interval = 1000;
            timer.Tick += TimerTick;

            trayMenu = new ContextMenuStrip();
            ToolStripMenuItem showMenuItem = LocalizedMenuItem("显示窗口");
            showMenuItem.Font = new Font(showMenuItem.Font, FontStyle.Bold);
            showMenuItem.Click += delegate { RestoreFromTray(); };
            ToolStripMenuItem exitMenuItem = LocalizedMenuItem("退出");
            exitMenuItem.Click += delegate { Close(); };
            trayMenu.Items.Add(showMenuItem);
            wholeModeItem = LocalizedMenuItem("整机功率");
            batteryModeItem = LocalizedMenuItem("电池端净功率");
            wholeModeItem.Click += delegate { ChangeMode(DisplayMode.WholeSystem); };
            batteryModeItem.Click += delegate { ChangeMode(DisplayMode.Battery); };
            trayMenu.Items.Add(wholeModeItem);
            trayMenu.Items.Add(batteryModeItem);
            trayMenu.Items.Add(new ToolStripSeparator());
            trayMenu.Items.Add(exitMenuItem);

            trayIcon = new NotifyIcon();
            trayIcon.Text = Strings.AppName + ": " + Strings.Get("正在读取");
            trayIcon.Icon = Icon;
            trayIcon.ContextMenuStrip = trayMenu;
            trayIcon.Visible = false;
            trayIcon.DoubleClick += delegate { RestoreFromTray(); };

            BuildElevationControls(startupMessage);
            BuildAutostartControls();
            BuildLanguageControls();
            ChangeMode(session.Mode);

            Shown += delegate
            {
                StartMonitoring();
            };

            dpiLayout = new DpiLayout(this, ClientSize, errorLabel, NoticeSectionHeight);
            UpdateDiagnosticLayout();
        }

        private GlyphButton CaptionButton(string icon, string name, int x)
        {
            GlyphButton button = new GlyphButton();
            button.IconBody = icon;
            button.Bounds = new Rectangle(x, 0, 42, TitleBarHeight);
            button.AccessibleName = Strings.Get(name);
            button.Tag = name;
            sourceTip.SetToolTip(button, Strings.Get(name));
            Controls.Add(button);
            return button;
        }

        private ReadoutLabel Readout(int x, int y, int width, int height, string family, float points, bool splitUnit)
        {
            ReadoutLabel label = new ReadoutLabel();
            label.Bounds = new Rectangle(x, y, width, height);
            label.Font = UiTheme.Numeric(family, points);
            label.DesignPoints = points;
            label.SplitUnit = splitUnit;
            label.ForeColor = UiTheme.Ink;
            label.Text = "";
            Controls.Add(label);
            return label;
        }

        private float LayoutScale
        {
            get { return dpiLayout != null && dpiLayout.CurrentDpi > 0 ? dpiLayout.CurrentDpi / 96f : 1f; }
        }

        protected override void OnHandleCreated(EventArgs e)
        {
            base.OnHandleCreated(e);
            ApplyWindowChrome();
            dpiLayout.Apply(ReadWindowDpi());
            LayoutSourceRows();
            if (trayEnabled)
            {
                restoreMessage = GuiInstance.ListenForRestore(Handle, Startup.Identity);
                restoreWindow = Handle;
            }
        }

        /// <summary>
        /// Restores the caption, system menu and minimize styles that
        /// WinForms omits for a borderless form, then removes the non-client
        /// area in WM_NCCALCSIZE. The window keeps the native move loop,
        /// Alt+Space menu, minimize animation, taskbar behaviour and DWM
        /// shadow; Windows 11 also rounds its corners. Without
        /// WS_MAXIMIZEBOX a caption double-click does nothing.
        /// </summary>
        private const int ChromeStyles = NativeMethods.WsCaption | NativeMethods.WsSysMenu | NativeMethods.WsMinimizeBox;

        private void ApplyWindowChrome()
        {
            int style = NativeMethods.GetWindowLong(Handle, NativeMethods.GwlStyle);
            style = (style | ChromeStyles) & ~NativeMethods.WsMaximizeBox;
            NativeMethods.SetWindowLong(Handle, NativeMethods.GwlStyle, style);
            try
            {
                int dark = 1;
                NativeMethods.DwmSetWindowAttribute(Handle, NativeMethods.DwmUseImmersiveDarkMode, ref dark, sizeof(int));
                int corner = NativeMethods.DwmCornerRound;
                roundedCorners = NativeMethods.DwmSetWindowAttribute(Handle, NativeMethods.DwmWindowCornerPreference,
                    ref corner, sizeof(int)) == 0;
                int border = UiTheme.Hairline.R | (UiTheme.Hairline.G << 8) | (UiTheme.Hairline.B << 16);
                NativeMethods.DwmSetWindowAttribute(Handle, NativeMethods.DwmBorderColor, ref border, sizeof(int));
            }
            catch (DllNotFoundException)
            {
                roundedCorners = false;
            }
            NativeMethods.SetWindowPos(Handle, IntPtr.Zero, 0, 0, 0, 0,
                NativeMethods.SwpNoMove | NativeMethods.SwpNoSize | NativeMethods.SwpNoZOrder
                    | NativeMethods.SwpNoActivate | NativeMethods.SwpFrameChanged);
        }

        /// <summary>True when the drawn title bar should act as the caption.</summary>
        internal bool IsCaptionPoint(Point client)
        {
            Point scrolled = new Point(client.X, client.Y - AutoScrollPosition.Y);
            return scrolled.Y >= 0 && scrolled.Y < DpiLayout.ScaleValue(TitleBarHeight, LayoutScale)
                && client.X >= 0 && client.X < ClientSize.Width;
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            base.OnPaint(e);
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float s = LayoutScale;
            g.TranslateTransform(AutoScrollPosition.X, AutoScrollPosition.Y);

            // Application tile: Lucide zap on the icon's green.
            RectangleF tile = new RectangleF(14 * s, 14 * s, 16 * s, 16 * s);
            using (GraphicsPath path = WidgetPath.Rounded(tile, 4 * s))
            using (SolidBrush brush = new SolidBrush(UiTheme.AppTile))
                g.FillPath(brush, path);
            int glyph = (int)Math.Round(12 * s);
            UiIcons.Draw(g, UiIcons.Zap, Color.White,
                new Rectangle((int)Math.Round(tile.X + (tile.Width - glyph) / 2), (int)Math.Round(tile.Y + (tile.Height - glyph) / 2), glyph, glyph), 2.2f);

            g.SmoothingMode = SmoothingMode.None;
            using (Pen hair = new Pen(UiTheme.Hairline, Math.Max(1, (int)Math.Round(s))))
            {
                int left = DpiLayout.ScaleValue(18, s), right = DpiLayout.ScaleValue(370, s);
                int top = DpiLayout.ScaleValue(TripTop, s), bottom = DpiLayout.ScaleValue(TripBottom, s);
                g.DrawLine(hair, left, top, right, top);
                g.DrawLine(hair, left, bottom, right, bottom);
                foreach (int x in new int[] { 135, 252 })
                    g.DrawLine(hair, DpiLayout.ScaleValue(x, s), top, DpiLayout.ScaleValue(x, s), bottom);
            }

            // Dotted leaders join each supporting metric to its value.
            using (Pen dots = new Pen(Color.FromArgb(70, 74, 75), Math.Max(1f, s)))
            {
                dots.DashPattern = new float[] { 1f, 2f };
                for (int index = 0; index < sourceNames.Length; index++)
                {
                    Label name = sourceNames[index];
                    ReadoutLabel value = sourceValues[index];
                    if (name == null || !name.Visible) continue;
                    int from = name.Left + TextRenderer.MeasureText(name.Text, name.Font, Size.Empty, AlignedLabel.Flags).Width
                        + DpiLayout.ScaleValue(10, s);
                    int to = value.Right - value.MeasureContent().Width - DpiLayout.ScaleValue(10, s);
                    int y = name.Top + name.Height / 2 + DpiLayout.ScaleValue(4, s);
                    if (to > from)
                        g.DrawLine(dots, from - AutoScrollPosition.X, y - AutoScrollPosition.Y, to - AutoScrollPosition.X, y - AutoScrollPosition.Y);
                }
            }
        }

        protected override void OnHandleDestroyed(EventArgs e)
        {
            if (restoreWindow != IntPtr.Zero)
                GuiInstance.StopListening(restoreWindow, Startup.Identity);
            restoreWindow = IntPtr.Zero;
            base.OnHandleDestroyed(e);
        }

        protected override void WndProc(ref Message message)
        {
            if (restoreMessage != 0 && message.Msg == restoreMessage)
            {
                RestoreFromTray();
                message.Result = IntPtr.Zero;
                return;
            }
            if (message.Msg == NativeMethods.WmNcCalcSize && message.LParam != IntPtr.Zero)
            {
                // The whole window is client area except scroll bars (very
                // high DPI on a small screen); the title bar is drawn.
                NativeMethods.Rect proposed = (NativeMethods.Rect)Marshal.PtrToStructure(message.LParam, typeof(NativeMethods.Rect));
                int style = NativeMethods.GetWindowLong(Handle, NativeMethods.GwlStyle);
                int dpi = ReadWindowDpi();
                if ((style & NativeMethods.WsVScroll) != 0)
                    proposed.Right -= ScrollBarSize(NativeMethods.SmCxVScroll, dpi);
                if ((style & NativeMethods.WsHScroll) != 0)
                    proposed.Bottom -= ScrollBarSize(NativeMethods.SmCyHScroll, dpi);
                Marshal.StructureToPtr(proposed, message.LParam, false);
                message.Result = IntPtr.Zero;
                return;
            }
            if (message.Msg == NativeMethods.WmStyleChanging && message.WParam.ToInt64() == NativeMethods.GwlStyle
                && message.LParam != IntPtr.Zero)
            {
                // WinForms rewrites the style from its borderless CreateParams
                // on some property changes; keep the caption styles.
                int[] styles = new int[2];
                Marshal.Copy(message.LParam, styles, 0, 2);
                styles[1] = (styles[1] | ChromeStyles) & ~NativeMethods.WsMaximizeBox;
                Marshal.Copy(styles, 0, message.LParam, 2);
            }
            if (message.Msg == NativeMethods.WmSysCommand
                && (message.WParam.ToInt64() & 0xFFF0) == NativeMethods.ScMaximize)
            {
                message.Result = IntPtr.Zero;
                return;
            }
            if (message.Msg == NativeMethods.WmNcHitTest)
            {
                base.WndProc(ref message);
                if (message.Result.ToInt64() == NativeMethods.HtClient)
                {
                    long packed = message.LParam.ToInt64();
                    Point screen = new Point((short)(packed & 0xffff), (short)((packed >> 16) & 0xffff));
                    if (IsCaptionPoint(PointToClient(screen)))
                        message.Result = new IntPtr(NativeMethods.HtCaption);
                }
                return;
            }
            // WinForms gates its DpiChanged event behind app.config; handle the native
            // message so the application EXE retains per-monitor scaling.
            if (message.Msg == NativeMethods.WmDpiChanged && message.LParam != IntPtr.Zero)
            {
                CloseSettings();
                int newDpi = (int)(message.WParam.ToInt64() & 0xffff);
                NativeMethods.Rect suggested = (NativeMethods.Rect)Marshal.PtrToStructure(
                    message.LParam,
                    typeof(NativeMethods.Rect));

                lastSuggestedPosition = new Point(suggested.Left, suggested.Top);
                lastDpiWindowPositionApplied = NativeMethods.SetWindowPos(
                    Handle,
                    IntPtr.Zero,
                    suggested.Left,
                    suggested.Top,
                    suggested.Right - suggested.Left,
                    suggested.Bottom - suggested.Top,
                    NativeMethods.SwpNoZOrder | NativeMethods.SwpNoActivate);
                if (!lastDpiWindowPositionApplied)
                {
                    throw new Win32Exception(
                        Marshal.GetLastWin32Error(),
                        "Unable to apply the suggested DPI window bounds.");
                }

                dpiLayout.Apply(newDpi);
                LayoutSourceRows();
                Invalidate(true);
                message.Result = IntPtr.Zero;
                return;
            }

            base.WndProc(ref message);
        }

        protected override void OnResize(EventArgs e)
        {
            base.OnResize(e);
            if (trayEnabled && WindowState == FormWindowState.Minimized)
                BeginInvoke(new MethodInvoker(MinimizeToTray));
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                if (timer != null)
                    timer.Dispose();
                if (trayIcon != null)
                {
                    trayIcon.Visible = false;
                    trayIcon.Dispose();
                }
                if (generatedTrayIcon != null)
                    generatedTrayIcon.Dispose();
                if (trayMenu != null)
                    trayMenu.Dispose();
                CloseSettings();
                if (dpiLayout != null)
                    dpiLayout.Dispose();
                DisposePowerSources();
            }
            base.Dispose(disposing);
        }

        private static int ScrollBarSize(int metric, int dpi)
        {
            try
            {
                return NativeMethods.GetSystemMetricsForDpi(metric, (uint)dpi);
            }
            catch (EntryPointNotFoundException)
            {
                return metric == NativeMethods.SmCxVScroll
                    ? SystemInformation.VerticalScrollBarWidth : SystemInformation.HorizontalScrollBarHeight;
            }
        }

        private int ReadWindowDpi()
        {
            try
            {
                uint dpi = NativeMethods.GetDpiForWindow(Handle);
                if (dpi > 0)
                    return (int)dpi;
            }
            catch (EntryPointNotFoundException)
            {
            }

            return 96;
        }

        private Label NewLabel(string text, int x, int y, int width, int height, float size, FontStyle style)
        {
            Label label = new AlignedLabel();
            label.Tag = text;
            label.Text = Strings.Get(text);
            label.Location = new Point(x, y);
            label.Size = new Size(width, height);
            label.Font = new Font(UiTheme.TextFont, size, style, GraphicsUnit.Point);
            label.BackColor = Color.Transparent;
            label.TextAlign = ContentAlignment.MiddleLeft;
            return label;
        }

        private void TimerTick(object sender, EventArgs e)
        {
            RefreshReading();
        }

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

        private void RefreshReading()
        {
            try
            {
                ShowReading(powerSources.Capture());
            }
            catch (Exception ex)
            {
                ShowView(session.Fail(ex.Message));
            }
        }

        internal void ShowReading(PowerSnapshot snapshot)
        {
            ShowView(session.Observe(snapshot));
        }

        // Binds a view to the controls; all display rules live in MeterSession.
        private void ShowView(MeterView view)
        {
            lastView = view;
            title.Text = view.Title.Text;
            title.ForeColor = UiTheme.Muted;
            sourceTip.SetToolTip(title, view.Title.Tooltip);
            stateLabel.Text = view.Supply.Text;
            stateLabel.IconBody = UiIcons.For(view.SupplyIcon);
            stateLabel.ForeColor = view.Supply.Tone == ReadoutTone.Accent ? UiTheme.Accent(view.Accent) : UiTheme.Muted;
            powerLabel.Text = view.Headline.Text;
            powerLabel.ForeColor = UiTheme.Headline(view.Headline.Tone, view.Accent);
            sourceTip.SetToolTip(powerLabel, view.Headline.Tooltip);
            voltageLabel.Caption = view.VoltageCaption;
            voltageLabel.Text = view.Voltage;
            currentLabel.Caption = view.CurrentCaption;
            currentLabel.Text = view.Current;
            percentageLabel.Text = view.Percentage;
            batteryCells.SetValue(view.BatteryLevel, UiTheme.Accent(view.Accent), view.BatteryTone != ReadoutTone.Accent);
            trendChart.SetView(view.Trend, UiTheme.Accent(view.Accent));
            sourceTip.SetToolTip(trendChart, view.Trend.Tooltip);

            statisticsCaption.Text = view.AverageCaption;
            historyCaption.Text = view.PeakCaption;
            sourceTip.SetToolTip(statisticsCaption, view.AverageTooltip);
            sourceTip.SetToolTip(statisticsLabel, view.AverageTooltip);
            sourceTip.SetToolTip(historyCaption, view.PeakTooltip);
            sourceTip.SetToolTip(historyLabel, view.PeakTooltip);
            statisticsLabel.Text = view.AverageValue;
            historyLabel.Text = view.PeakValue;
            statisticsLabel.ForeColor = view.AverageValue.StartsWith("--", StringComparison.Ordinal) ? UiTheme.Faint : UiTheme.Ink;
            historyLabel.ForeColor = view.PeakValue.StartsWith("--", StringComparison.Ordinal) ? UiTheme.Faint : UiTheme.Ink;

            UpdatePowerSources(view);
            updatedLabel.Caption = Strings.Get("更新于");
            updatedLabel.Text = view.Updated;
            Invalidate(false);
            if (trayEnabled)
            {
                if (view.TrayGlyph != null)
                    SetTrayIcon(view.TrayGlyph, UiTheme.TrayTile(view.Accent));
                SetTrayTooltip(view.TrayTooltip);
            }
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

        private void ChangeMode(DisplayMode mode)
        {
            if (session.Mode != mode)
                DisplayPreference.Save(mode);
            MeterView view = session.SetMode(mode);
            wholeModeItem.Checked = mode == DisplayMode.WholeSystem;
            batteryModeItem.Checked = mode == DisplayMode.Battery;
            int index = mode == DisplayMode.Battery ? 1 : 0;
            if (modeSegments.SelectedIndex != index)
                modeSegments.SelectedIndex = index;
            LayoutSourceRows();
            ShowView(view);
        }

        private void BuildElevationControls(string message)
        {
            elevationMessage = Startup.IsElevated() ? null : message;
        }

        private void UpdateDiagnosticLayout()
        {
            bool expanded = !String.IsNullOrWhiteSpace(errorLabel.Text);
            errorLabel.Visible = expanded;
            sourceTip.SetToolTip(errorLabel, expanded ? errorLabel.Text : null);
            UpdateElevateAction();
            if (dpiLayout != null)
                dpiLayout.SetSectionExpanded(expanded);
            LayoutSourceRows();
        }

        // The notice offers the restart only when elevation fixes a reading
        // or the startup elevation request was declined.
        private void UpdateElevateAction()
        {
            if (elevateButton == null) return;
            bool helps = lastView != null && lastView.ElevationHelps || !String.IsNullOrEmpty(elevationMessage);
            elevateButton.Visible = errorLabel.Visible && helps && !Startup.IsElevated();
        }

        /// <summary>Shared by the notice and the settings popover.</summary>
        private void RestartElevated(Control trigger)
        {
            trigger.Enabled = false;
            ElevationResult result = Startup.TryElevate();
            if (result.Started) Close();
            else
            {
                elevationMessage = result.Message;
                RefreshDiagnostics();
                trigger.Enabled = true;
            }
        }

    }

    internal static class Program
    {
        // An elevated launch migrates an unprotected task and refreshes this
        // installation's protected copy. Failures leave the state for the
        // settings panel to report.
        private static void SynchronizeAutostart()
        {
            try
            {
                using (AutostartManager manager = AutostartManager.ForCurrentExecutable(Application.ExecutablePath))
                    manager.Synchronize(true);
            }
            catch (Exception)
            {
            }
        }

        [STAThread]
        private static void Main(string[] args)
        {
            EmbeddedLibraries.Initialize();
            Run(args);
        }

        [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.NoInlining)]
        private static void Run(string[] args)
        {
            StartupRoute route = StartupRoute.Parse(args);
            if (!route.Valid)
            {
                Environment.ExitCode = 2;
                return;
            }
            if (route.Command == "gui")
            {
                using (GuiInstance instance = new GuiInstance())
                {
                    if (!instance.Acquire(Startup.Identity, route.Handoff))
                    {
                        if (!route.StartHidden)
                            GuiInstance.RestoreExisting(Startup.Identity);
                        return;
                    }
                    ElevationResult elevation = Startup.Route(route, Startup.IsElevated(), Startup.TryElevate);
                    if (elevation.Started)
                        return;
                    if (Startup.IsElevated())
                        SynchronizeAutostart();
                    Application.EnableVisualStyles();
                    Application.SetCompatibleTextRenderingDefault(false);
                    Application.Run(new MainForm(true, elevation.Message, route.StartHidden));
                }
                return;
            }
            if (route.Command == "--remove-autostart")
            {
                // 0: removed or absent; 3: task gone, but deleting this
                // installation's protected copy needs an elevated rerun;
                // 6: removed, but a same-name task that is not this program's
                // was left unchanged (see AutostartRemoval).
                try
                {
                    using (AutostartManager manager = AutostartManager.ForCurrentExecutable(Application.ExecutablePath))
                        Environment.ExitCode = (int)manager.Disable();
                }
                catch
                {
                    Environment.ExitCode = 1;
                }
                return;
            }
            if (route.Command == "--sync-autostart")
            {
                // Elevated: apply and return 0. Ordinary token: report what an
                // elevated run would change (3 outdated copy, 4 unprotected
                // task, 5 unused copy) without requesting UAC.
                try
                {
                    using (AutostartManager manager = AutostartManager.ForCurrentExecutable(Application.ExecutablePath))
                        Environment.ExitCode = (int)manager.Synchronize(Startup.IsElevated());
                }
                catch
                {
                    Environment.ExitCode = 1;
                }
                return;
            }
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);

            switch (route.Command)
            {
                case "--screenshot":
                    using (MainForm preview = new MainForm(false))
                        preview.RenderPreview(route.Path);
                    return;
                case "--dpi-preview":
                    using (MainForm preview = new MainForm(false))
                        preview.RenderDpiTransitionPreview(route.Path, route.Dpis);
                    return;
                case "--self-test":
                    bool passed;
                    string report = PowerSelfTest.Run(out passed);
                    File.WriteAllText(route.Path, report);
                    Environment.Exit(passed ? 0 : 1);
                    return;
                case "--third-party-notices":
                    ThirdPartyNotices.WriteTo(route.Path);
                    return;
                case "--power-probe":
                    File.WriteAllText(route.Path, PowerDiagnostics.Run(route.Seconds));
                    return;
                case "--tray-preview":
                    using (MainForm preview = new MainForm(false))
                        preview.RenderTrayIconPreview(route.Path, route.TrayGlyph, route.TrayDischarging);
                    return;
            }

            Environment.ExitCode = 2;
        }
    }
}
