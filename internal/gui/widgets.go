package gui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// figure is a reading split for typesetting: the approximation sign and the
// unit are set smaller and quieter than the number, on its baseline.
type figure struct {
	prefix, number, unit string
}

// splitFigure splits "≈ 19.39 W", "−17.50 W", "98%", "N/A" and "--".
func splitFigure(text string) figure {
	f := figure{number: text}
	if rest, ok := strings.CutPrefix(f.number, "≈ "); ok {
		f.prefix, f.number = "≈", rest
	}
	if i := strings.LastIndexByte(f.number, ' '); i > 0 {
		f.number, f.unit = f.number[:i], f.number[i+1:]
	} else if rest, ok := strings.CutSuffix(f.number, "%"); ok && rest != "" {
		f.number, f.unit = rest, "%"
	}
	return f
}

// figureText sets a reading: the number at size in font and color, the sign
// and the unit in the regular face at unitSize, quieter, on one baseline.
func figureText(c *ui.Context, p *palette, text string, size, unitSize float32, font string, color ui.Color) ui.Element {
	f := splitFigure(text)
	gap := max(unitSize*0.3, 3)
	return ui.Row(c).AlignItems(ui.Baseline).FontFeatures(tabular).Children(func() {
		if f.prefix != "" {
			ui.Text(c, f.prefix).Font(textFont).FontSize(unitSize).TextColor(p.ink3).SingleLine().Margin(0, gap, 0, 0)
		}
		ui.Text(c, f.number).Font(font).FontSize(size).LineHeight(1.15).TextColor(color).SingleLine()
		if f.unit != "" {
			ui.Text(c, f.unit).Font(textFont).FontSize(unitSize).TextColor(p.ink2).SingleLine().Margin(0, 0, 0, gap)
		}
	})
}

// label is a line of text in the regular face.
func label(c *ui.Context, text string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, text).Font(textFont).FontSize(size).TextColor(color).SingleLine()
}

// group is a COUI card: a group of rows on the panel, its continuous
// corners concentric with the panel's.
func group(c *ui.Context, p *palette) ui.Element {
	return ui.Column(c).Draw(shape(p.group, groupRadius))
}

// iconButton is a round icon button: a press highlight and no border. on
// keeps it highlighted, as a toggle that is on; close turns its hover red,
// as Windows does for the close button.
func iconButton(c *ui.Context, p *palette, body string, name string, on, close bool) ui.Element {
	const size = 30
	b := ui.ButtonBase(c).Size(size, size).Radius(size / 2).Justify(ui.Center).AlignItems(ui.Center).
		Label(name).Tooltip(name).TextColor(p.ink2).FocusRing(false)
	switch {
	case close && b.Pressed():
		b.Background(withAlpha(closeHover, 0.8)).TextColor(white)
	case close && b.Hovered():
		b.Background(closeHover).TextColor(white)
	case b.Pressed():
		b.Background(p.press).TextColor(p.ink)
	case on:
		b.Background(p.track).TextColor(p.ink)
	case b.Hovered():
		b.Background(p.hover).TextColor(p.ink)
	}
	b.Children(func() {
		ui.Icon(c, icon(body, 1.75)).Size(16, 16)
	})
	return b
}

// hairline is a divider one device pixel thick.
func hairline(c *ui.Context, p *palette) ui.Element {
	return ui.Column(c).Height(1).Draw(func(pt *ui.Painter, r ui.Rect) {
		h := 1 / pt.Scale()
		pt.Fill(ui.Rect{X: r.X, Y: r.Y + (r.H-h)/2, W: r.W, H: h}, p.divider, 0)
	})
}
