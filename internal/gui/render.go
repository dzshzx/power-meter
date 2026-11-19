package gui

import (
	"bytes"
	"image/png"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/dzshzx/power-meter/internal/selftest"
	"github.com/egoist/mygo/ui"
)

// Render draws the window for m at scale without opening it, in the light
// or dark appearance, for --screenshot and --dpi-preview.
func Render(m *Model, scale float32, dark bool) []byte {
	h := Height(m)
	tt := ui.NewTesterWith(func(c *ui.Context) { Build(c, m) }, Width, h, ui.TesterOptions{Dark: dark})
	tt.SetScale(scale)
	tt.Frame()
	return encode(tt.Image())
}

// SelfTest checks that the shipped binary draws the window and the tray
// icon.
func SelfTest() []selftest.Case {
	return []selftest.Case{
		{Name: "window renders in both languages", Pass: func() bool {
			previous := i18n.Preference()
			defer func() { _ = i18n.Select(previous) }()
			for _, lang := range []string{i18n.Chinese, i18n.English} {
				if i18n.Select(lang) != nil {
					return false
				}
				m := &Model{View: meter.NewSession(meter.WholeSystem).Render()}
				img, err := png.Decode(bytes.NewReader(Render(m, 1.5, false)))
				if err != nil || img.Bounds().Dx() != Width*3/2 {
					return false
				}
			}
			return true
		}},
		{Name: "tray icon renders", Pass: func() bool {
			img, err := png.Decode(bytes.NewReader(TrayIcon("42", power.AccentCharging)))
			return err == nil && img.Bounds().Dx() == TrayIconSize
		}},
	}
}
