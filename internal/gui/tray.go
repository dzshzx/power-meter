package gui

import (
	"bytes"
	"image"
	"image/png"

	"github.com/dzshzx/power-meter/internal/fonts"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/egoist/mygo/ui"
)

// TrayIconSize is the tray image's edge in pixels; Windows scales it to the
// notification area's size.
const TrayIconSize = 64

// TrayIcon draws the tray image: the reading's figures in white on a tile
// of the supply's colour.
func TrayIcon(glyph string, accent power.Accent) []byte {
	ensureFonts()
	return encode(ui.Render(func(c *ui.Context) {
		transparent(c)
		ui.Column(c).Fill().Background(ui.Transparent).Draw(func(pt *ui.Painter, r ui.Rect) {
			drawTrayIcon(pt, r, glyph, trayTile(accent))
		})
	}, TrayIconSize, TrayIconSize, 1))
}

// AppIcon draws the app's mark at size pixels: the window's title, the
// installer and the tray before the first reading use it.
func AppIcon(size int) []byte {
	return encode(ui.Render(func(c *ui.Context) {
		transparent(c)
		ui.Column(c).Fill().Background(ui.Transparent).Draw(func(pt *ui.Painter, r ui.Rect) {
			drawAppMark(pt, r)
		})
	}, size, size, 1))
}

func drawTrayIcon(pt *ui.Painter, r ui.Rect, glyph string, tile ui.Color) {
	inset := r.W * 3 / 64
	box := ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}
	pt.Fill(box, tile, r.W*15/64)
	size := r.W * 34 / 64
	if len([]rune(glyph)) > 2 {
		size = r.W * 26 / 64
	}
	s := ui.Span{Text: glyph, Font: figureFont, Size: size, Color: white}
	w, h := pt.MeasureText(0, s)
	pt.RichText(r.X+(r.W-w)/2, r.Y+(r.H-h)/2+r.H/64, 0, s)
}

// drawAppMark is a white battery on a green rounded tile, as the program's
// icon.
func drawAppMark(pt *ui.Painter, r ui.Rect) {
	inset := r.W / 32
	box := ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}
	pt.Fill(box, appTile, box.W*14/60)
	glyph := box.W * 0.72
	pt.Icon(icon(iconBatteryMedium, 2), ui.Rect{X: r.X + (r.W-glyph)/2, Y: r.Y + (r.H-glyph)/2, W: glyph, H: glyph}, white)
}

// ensureFonts registers MiSans before the first drawing; the faces fall
// back to the system's fonts if that fails.
func ensureFonts() { _ = fonts.Register() }

// transparent clears the window background, so an icon's corners show
// what is behind it.
func transparent(c *ui.Context) {
	t := *c.Theme()
	t.Background = ui.Transparent
	c.SetTheme(&t)
}

func encode(img *image.RGBA) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}
