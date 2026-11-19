package gui

import (
	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

const settingsRow = 48

// languages are the language choices: the system's, then each language in
// its own name.
var languages = []struct{ value, key string }{
	{i18n.System, "跟随系统"},
	{i18n.Chinese, "简体中文"},
	{i18n.English, "English"},
}

func languageName(value string) string {
	for _, l := range languages {
		if l.value == value {
			return i18n.Get(l.key)
		}
	}
	return i18n.Get("跟随系统")
}

// settings is the popover above the settings button: COUI preference rows
// on a popup surface.
func settings(c *ui.Context, p *palette, m *Model, anchor ui.Element) {
	ui.PopoverBase(c.Key("settings"), anchor, &m.SettingsOpen, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 8, 0).Width(settingsWidth).
			Padding(4, 0, 4, 0).Radius(16).Background(p.popup).Shadow(0, 8, 32, 0, p.shadow).Font(textFont).
			TextColor(p.label1)
		autostartRow(c, p, m)
		settingsDivider(c, p)
		languageRow(c, p, m)
		settingsDivider(c, p)
		ui.Row(c).Height(settingsRow).Padding(0, cardPadding, 0, cardPadding).AlignItems(ui.Center).Children(func() {
			ui.Text(c, i18n.Get("当前权限")).FontSize(bodySize).SingleLine()
			ui.Spacer(c)
			mode := "普通模式"
			if m.Elevated {
				mode = "管理员模式"
			}
			ui.Text(c, i18n.Get(mode)).FontSize(bodySize).TextColor(p.label2).SingleLine()
		})
		if !m.Elevated {
			restart := ui.ButtonBase(c).Height(36).Margin(4, cardPadding, 10, cardPadding).Radius(18).Justify(ui.Center).
				AlignItems(ui.Center).Background(p.button).TextColor(p.primary).FocusRing(false)
			if restart.Pressed() {
				restart.Background(p.button.Over(p.background).Mix(p.label1, 0.12))
			} else if restart.Hovered() {
				restart.Background(p.button.Mix(p.label1, 0.06))
			}
			restart.Children(func() {
				ui.Icon(c, icon(iconShield, 2.2)).Size(15, 15).Margin(0, 6, 0, 0)
				ui.Text(c, i18n.Get("以管理员身份重新启动")).FontSize(bodySize).FontWeight(500).SingleLine()
			})
			if restart.Clicked() && m.Actions.Elevate != nil {
				m.SettingsOpen = false
				m.Actions.Elevate()
			}
		}
	})
}

func settingsDivider(c *ui.Context, p *palette) {
	ui.Column(c).Padding(0, cardPadding, 0, cardPadding).Children(func() { hairline(c, p) })
}

func autostartRow(c *ui.Context, p *palette, m *Model) {
	on := m.Autostart.Checked
	row := ui.SwitchBase(c.Key("autostart"), &on).Height(settingsRow).Padding(0, cardPadding, 0, cardPadding).
		AlignItems(ui.Center).Disabled(!m.Autostart.Enabled).DimDisabled(false).FocusRing(false).Tooltip(m.Autostart.Tooltip).
		Label(i18n.Get("开机自启"))
	row.Children(func() {
		label := ui.Text(c, i18n.Get("开机自启")).FontSize(bodySize).SingleLine()
		if !m.Autostart.Enabled {
			label.TextColor(p.label3)
		}
		ui.Spacer(c)
		switchTrack(c, p, m.Autostart.Checked, m.Autostart.Enabled)
	})
	if on != m.Autostart.Checked && m.Actions.SetAutostart != nil {
		m.Actions.SetAutostart(on)
	}
}

// switchTrack is COUI's switch: a 44×24 track and an 18 DIP thumb.
func switchTrack(c *ui.Context, p *palette, on, enabled bool) {
	track := p.switchOff
	if on {
		track = p.primary
	}
	opacity := float32(1)
	if !enabled {
		opacity = 0.4
	}
	ui.Column(c).Size(44, 24).Opacity(opacity).Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(r, track, r.H/2)
		x := r.X + 3
		if on {
			x = r.X + r.W - 21
		}
		thumb := ui.Rect{X: x, Y: r.Y + 3, W: 18, H: 18}
		pt.Shadow(thumb, 9, 0, 1, 2.3, 0, withAlpha(p.shadow, 0.6))
		pt.Fill(thumb, white, 9)
	})
}

func languageRow(c *ui.Context, p *palette, m *Model) {
	chosen := m.Language
	sel := ui.SelectBase(c.Key("language"), &chosen)
	sel.Trigger.Height(settingsRow).Padding(0, 10, 0, cardPadding).AlignItems(ui.Center).FocusRing(false).
		Label(i18n.Get("语言") + " / Language")
	if sel.Trigger.Hovered() {
		sel.Trigger.Background(p.hover)
	}
	sel.Trigger.Children(func() {
		ui.Text(c, i18n.Get("语言")).FontSize(bodySize).SingleLine()
		ui.Spacer(c)
		ui.Text(c, languageName(m.Language)).FontSize(bodySize).TextColor(p.label2).SingleLine()
		ui.Icon(c, icon(iconChevronDown, 2)).Size(16, 16).TextColor(p.label3).Margin(0, 0, 0, 4)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(4, 0, 0, 0).Padding(4).Radius(14).Background(p.popup).Shadow(0, 8, 28, 0, p.shadow).
			Width(160).Font(textFont).TextColor(p.label1)
		for _, l := range languages {
			item := sel.Item(l.value).Height(40).Padding(0, 12, 0, 12).Radius(10).AlignItems(ui.Center).FocusRing(false)
			if item.Highlighted() {
				item.Background(p.hover)
			}
			item.Children(func() {
				text := ui.Text(c, i18n.Get(l.key)).FontSize(bodySize).SingleLine().Grow(1)
				if l.value == m.Language {
					text.TextColor(p.primary).FontWeight(500)
				}
			})
		}
	})
	if chosen != m.Language && m.Actions.SetLanguage != nil {
		m.Actions.SetLanguage(chosen)
	}
}
