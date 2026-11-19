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

// figureText shows a reading at size in color, with its sign and unit in
// the palette's quieter labels.
func figureText(c *ui.Context, p *palette, text string, size float32, color ui.Color, weight int) ui.Element {
	f := splitFigure(text)
	row := ui.Row(c).AlignItems(ui.Baseline).Gap(size * 0.08).FontFeatures(tabular)
	row.Children(func() {
		if f.prefix != "" {
			ui.Text(c, f.prefix).Font(figureFont).LineHeight(1.15).FontSize(size * 0.55).TextColor(p.label3).SingleLine()
		}
		ui.Text(c, f.number).Font(figureFont).LineHeight(1.15).FontSize(size).FontWeight(weight).TextColor(color).SingleLine()
		if f.unit != "" {
			ui.Text(c, f.unit).Font(figureFont).LineHeight(1.15).FontSize(max(size*0.42, captionSize)).FontWeight(500).
				TextColor(p.label2).SingleLine().Margin(0, 0, 0, size*0.06)
		}
	})
	return row
}

// card is a COUI card: a rounded surface on the window background.
func card(c *ui.Context, p *palette) ui.Element {
	return ui.Column(c).Background(p.card).Radius(cardRadius)
}

// iconButton is COUI's circular icon button: a press highlight and no
// border. danger turns the hover red, for the window's close button.
func iconButton(c *ui.Context, p *palette, body string, label string, size float32, danger bool) ui.Element {
	b := ui.ButtonBase(c).Size(size, size).Radius(size / 2).Justify(ui.Center).AlignItems(ui.Center).
		Label(label).Tooltip(label).TextColor(p.label1).FocusRing(false)
	switch {
	case danger && b.Pressed():
		b.Background(withAlpha(p.red, 0.8)).TextColor(white)
	case danger && b.Hovered():
		b.Background(p.red).TextColor(white)
	case b.Pressed():
		b.Background(p.press)
	case b.Hovered():
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, icon(body, 2)).Size(18, 18)
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
