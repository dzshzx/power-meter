using System;
using System.Drawing;
using System.Windows.Forms;

namespace BatteryChargeMeter
{
    internal sealed partial class MainForm
    {
        internal const int SettingsWidth = 296;
        private GlyphButton pinButton;
        private GlyphButton settingsButton;
        private Form settingsPopup;
        private Panel settingsContent;

        private void BuildSettingsControls()
        {
            AntdUI.Config.Mode = AntdUI.TMode.Dark;
            AntdUI.Style.SetPrimary(UiTheme.Charging);
            AntdUI.Config.Animation = SystemInformation.IsMenuAnimationEnabled;

            pinButton = CaptionButton(UiIcons.Pin, "置顶", 262);
            pinButton.AutoToggle = true;
            pinButton.Toggle = TopMost;
            pinButton.ToggleChanged += delegate { TopMost = pinButton.Toggle; };

            settingsButton = new GlyphButton();
            settingsButton.IconBody = UiIcons.Settings;
            settingsButton.IconLogical = 16f;
            settingsButton.DesignHeight = 30;
            settingsButton.RadiusLogical = 4;
            settingsButton.Bounds = new Rectangle(348, 5, 34, 30);
            settingsButton.Tag = "设置";
            settingsButton.Click += delegate { OpenSettings(); };
            footerBand.Controls.Add(settingsButton);
        }

        private void CloseSettings()
        {
            Form popup = settingsPopup;
            settingsPopup = null;
            if (popup != null && !popup.IsDisposed) { popup.Close(); popup.Dispose(); }
            autostartSwitch = null;
            autostartStateLabel = null;
            if (settingsButton != null) settingsButton.Active = false;
        }

        private Rectangle SettingsBounds(int x, int y, int width, int height)
        {
            // Scale edges, not sizes, so adjacent rows never overlap after rounding.
            return DpiLayout.ScaleBounds(new Rectangle(x, y, width, height), LayoutScale);
        }

        private AlignedLabel SettingsLabel(Panel content, string text, Color color, Rectangle bounds, ContentAlignment align)
        {
            AlignedLabel label = new AlignedLabel();
            label.Text = text;
            label.Font = settingsFont;
            label.ForeColor = color;
            label.BackColor = Color.Transparent;
            label.TextAlign = align;
            label.Bounds = bounds;
            content.Controls.Add(label);
            return label;
        }

        private Font settingsFont;

        private void OpenSettings()
        {
            if (settingsPopup != null && !settingsPopup.IsDisposed) { CloseSettings(); return; }
            bool elevated = Startup.IsElevated();
            float scale = LayoutScale;
            if (settingsFont != null) settingsFont.Dispose();
            settingsFont = new Font(UiTheme.TextFont, 13f * scale, FontStyle.Regular, GraphicsUnit.Pixel);
            Panel content = new Panel();
            settingsContent = content;
            content.Size = SettingsBounds(0, 0, SettingsWidth, elevated ? 150 : 194).Size;
            content.BackColor = UiTheme.Popover;
            content.ForeColor = UiTheme.Ink;
            content.Font = settingsFont;

            SettingsLabel(content, Strings.Get("开机自启"), UiTheme.Ink, SettingsBounds(16, 6, 150, 46), ContentAlignment.MiddleLeft);
            autostartStateLabel = SettingsLabel(content, "", UiTheme.Muted, SettingsBounds(170, 6, 60, 46), ContentAlignment.MiddleRight);
            autostartSwitch = new ToggleSwitch();
            autostartSwitch.Bounds = SettingsBounds(240, 19, 40, 20);
            autostartSwitch.AccessibleName = Strings.Get("开机自启");
            autostartSwitch.Toggled += delegate
            {
                if (!updatingAutostart) ChangeAutostart(!autostartSwitch.Checked);
            };
            content.Controls.Add(autostartSwitch);
            RefreshAutostart();

            SettingsLabel(content, Strings.Get("语言"), UiTheme.Ink, SettingsBounds(16, 52, 120, 46), ContentAlignment.MiddleLeft);
            AntdUI.Select language = new AntdUI.Select();
            language.Name = "Language";
            language.Font = settingsFont;
            language.List = true;
            language.Radius = DpiLayout.ScaleValue(4, scale);
            language.BackColor = UiTheme.Field;
            language.ForeColor = UiTheme.Ink;
            language.BorderColor = UiTheme.FieldBorder;
            language.BorderHover = UiTheme.Muted;
            language.BorderActive = UiTheme.Muted;
            language.WaveSize = 0;
            language.Bounds = SettingsBounds(148, 60, 132, 30);
            language.Items.AddRange(new object[] { Strings.Get("跟随系统"), "简体中文", "English" });
            language.SelectedIndex = Strings.Preference == "zh-CN" ? 1 : Strings.Preference == "en" ? 2 : 0;
            language.SelectedIndexChanged += delegate
            {
                try
                {
                    Strings.Select(language.SelectedIndex == 1 ? "zh-CN" : language.SelectedIndex == 2 ? "en" : "system", true);
                    ApplyLanguage();
                    BeginInvoke(new MethodInvoker(CloseSettings));
                }
                catch (Exception error) { errorLabel.Text = error.Message; }
            };
            content.Controls.Add(language);

            Panel rule = new Panel();
            rule.BackColor = UiTheme.FieldBorder;
            rule.Bounds = SettingsBounds(16, 104, SettingsWidth - 32, 1);
            content.Controls.Add(rule);

            GlyphButton shield = new GlyphButton();
            shield.IconBody = UiIcons.Shield;
            shield.IconLogical = 16f;
            shield.DesignHeight = 24;
            shield.TabStop = false;
            shield.Enabled = false;
            shield.Bounds = SettingsBounds(14, 112, 20, 24);
            content.Controls.Add(shield);
            SettingsLabel(content, Strings.Get("当前权限"), UiTheme.Muted, SettingsBounds(40, 112, 130, 24), ContentAlignment.MiddleLeft);
            SettingsLabel(content, Strings.Get(elevated ? "管理员模式" : "普通模式"), UiTheme.Ink,
                SettingsBounds(172, 112, 108, 24), ContentAlignment.MiddleRight);

            if (!elevated)
            {
                ActionButton retry = new ActionButton();
                retry.Text = Strings.Get("以管理员身份重新启动");
                retry.IconBody = UiIcons.Shield;
                retry.Font = settingsFont;
                retry.DesignHeight = 32;
                retry.Fill = UiTheme.PopoverAction;
                retry.FillHover = UiTheme.PopoverActionHover;
                retry.Bounds = SettingsBounds(16, 148, SettingsWidth - 32, 32);
                retry.Click += delegate { RestartElevated(retry); };
                content.Controls.Add(retry);
            }

            AntdUI.Popover.Config config = new AntdUI.Popover.Config(settingsButton, content);
            config.Font = settingsFont;
            config.Back = UiTheme.Popover;
            config.Fore = UiTheme.Ink;
            config.Radius = 8;
            config.ArrowSize = 0;
            config.ArrowAlign = AntdUI.TAlign.TR;
            config.Padding = new Size(0, 0);
            // Bounds and fonts above already use this window's physical DPI.
            config.Dpi = 1f;
            config.OnClosed = delegate
            {
                autostartSwitch = null;
                autostartStateLabel = null;
                settingsPopup = null;
                settingsContent = null;
                settingsButton.Active = false;
            };
            settingsButton.Active = true;
            settingsPopup = AntdUI.Popover.open(config);
        }
    }
}
