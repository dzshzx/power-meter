package app

import (
	"unicode/utf16"

	"github.com/dzshzx/power-meter/internal/gui"
	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/egoist/mygo"
)

// The tray's menu: the window, the display mode, logon startup, the
// language and exit. Labels hold translation keys in the item IDs, so a
// language change relabels them.
func (a *app) buildTray() {
	radio := func(id string, mode meter.Mode) *mygo.MenuItem {
		return &mygo.MenuItem{ID: id, Label: i18n.Get(id), Type: mygo.MenuItemRadio, Checked: a.model.Mode == mode,
			Click: func(*mygo.MenuItem, *mygo.Window) { a.setMode(mode) }}
	}
	var languages []*mygo.MenuItem
	for _, l := range []struct{ value, label string }{
		{i18n.System, "跟随系统"}, {i18n.Chinese, "简体中文"}, {i18n.English, "English"},
	} {
		languages = append(languages, &mygo.MenuItem{ID: "lang:" + l.value, Label: i18n.Get(l.label), Type: mygo.MenuItemRadio,
			Checked: a.model.Language == l.value, Click: func(*mygo.MenuItem, *mygo.Window) { a.setLanguage(l.value) }})
	}
	a.menu = mygo.NewMenu([]*mygo.MenuItem{
		{ID: "显示窗口", Label: i18n.Get("显示窗口"), Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
		radio("整机功率", meter.WholeSystem),
		radio("电池端净功率", meter.Battery),
		{ID: "autostart", Label: i18n.Get("开机自启（登录后）"), Type: mygo.MenuItemCheckbox,
			Checked: a.model.Autostart.Checked, Disabled: !a.model.Autostart.Enabled,
			Click: func(item *mygo.MenuItem, _ *mygo.Window) {
				// The item toggled itself; the startup state decides.
				a.setStartup(!a.model.Autostart.Checked)
				item.SetChecked(a.model.Autostart.Checked)
			}},
		{ID: "language", Label: "语言 / Language", Submenu: languages},
		mygo.Separator(),
		{ID: "退出", Label: i18n.Get("退出"), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "PowerMeter",
		Icon:    gui.AppIcon(64),
		ToolTip: trayTooltip(i18n.AppName() + ": " + i18n.Get("正在读取")),
	})
	if err != nil {
		return
	}
	a.tray = tray
	tray.OnClick(a.show)
	// The menu shows on a right click only, so a left click restores the
	// window.
	tray.OnRightClick(func() {
		tray.SetMenu(a.menu)
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})
}

func (a *app) checkMenu() {
	if a.menu == nil {
		return
	}
	a.menu.ItemByID("整机功率").SetChecked(a.model.Mode == meter.WholeSystem)
	a.menu.ItemByID("电池端净功率").SetChecked(a.model.Mode == meter.Battery)
}

func (a *app) translateMenu() {
	if a.menu == nil {
		return
	}
	for _, key := range []string{"显示窗口", "整机功率", "电池端净功率", "退出"} {
		a.menu.ItemByID(key).SetLabel(i18n.Get(key))
	}
	a.menu.ItemByID("autostart").SetLabel(i18n.Get("开机自启（登录后）"))
	a.menu.ItemByID("lang:" + i18n.System).SetLabel(i18n.Get("跟随系统"))
	for _, l := range []string{i18n.System, i18n.Chinese, i18n.English} {
		a.menu.ItemByID("lang:" + l).SetChecked(a.model.Language == l)
	}
	a.checkMenu()
}

// updateTray shows the view's figures and text; the icon is drawn again only
// when its glyph or colour changes.
func (a *app) updateTray() {
	if a.tray == nil {
		return
	}
	v := a.model.View
	if v.TrayGlyph != "" && (v.TrayGlyph != a.glyph || v.Accent != a.accent) {
		_ = a.tray.SetIcon(gui.TrayIcon(v.TrayGlyph, v.Accent), false)
		a.glyph, a.accent = v.TrayGlyph, v.Accent
	}
	a.tray.SetToolTip(trayTooltip(v.TrayTooltip))
}

// trayTooltip fits the notification area's 63 UTF-16 units.
func trayTooltip(s string) string {
	if s == "" {
		s = i18n.AppName()
	}
	units := utf16.Encode([]rune(s))
	if len(units) <= 63 {
		return s
	}
	return string(utf16.Decode(units[:63]))
}
