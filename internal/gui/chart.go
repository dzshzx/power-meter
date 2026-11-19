package gui

import (
	"math"
	"strconv"

	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/egoist/mygo/ui"
)

// Chart insets inside its element: room for the axis labels below and the
// scale on the right.
const (
	chartLeft   = 2
	chartTop    = 6
	chartRight  = 34
	chartBottom = 22
)

// chart draws the thirty-minute trend: the mean line over a faint min-max
// envelope and a soft area, the peak marker and the live reading. Missing
// readings leave gaps; the series and labels come from the session.
type chart struct {
	p      *palette
	view   *meter.TrendView
	accent ui.Color
}

func (ch chart) span(text string, size float32, color ui.Color) ui.Span {
	return ui.Span{Text: text, Font: textFont, Size: size, Color: color}
}

func (ch chart) draw(pt *ui.Painter, r ui.Rect) {
	plot := ui.Rect{X: r.X + chartLeft, Y: r.Y + chartTop, W: r.W - chartLeft - chartRight, H: r.H - chartTop - chartBottom}
	if plot.W <= 0 || plot.H <= 0 {
		return
	}
	px := 1 / pt.Scale()
	ch.axis(pt, plot, px)
	series := ch.view.Series
	if series == nil || !series.HasData() {
		ch.empty(pt, plot, px)
		return
	}
	columns := max(1, int(plot.W*pt.Scale()/2))
	buckets := series.Buckets(columns)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, b := range buckets {
		if b.HasData {
			lo, hi = math.Min(lo, b.Min), math.Max(hi, b.Max)
		}
	}
	pad := math.Max((hi-lo)*0.2, math.Max(0.5, math.Abs(hi)*0.02))
	ticks := meter.Ticks(lo-pad, hi+pad, 3)
	lo, hi = ticks[0], ticks[len(ticks)-1]
	x := func(t float64) float32 {
		return plot.X + float32((t-series.Start)/meter.TrendWindow)*plot.W
	}
	y := func(v float64) float32 {
		return plot.Y + plot.H - float32((v-lo)/(hi-lo))*plot.H
	}
	ch.scale(pt, r, plot, ticks, y, px)
	pt.Clip(ui.Rect{X: plot.X, Y: plot.Y - 4, W: plot.W + 1, H: plot.H + 8}, 0, func() {
		for _, run := range connectedRuns(buckets) {
			ch.run(pt, plot, buckets, run, x, y, hi <= 0)
		}
	})
	if at, peak, ok := series.Peak(); ok {
		ch.peak(pt, plot, x(at), y(peak))
	}
	if latest, ok := series.Latest(); ok {
		cx, cy := x(series.Now), y(latest)
		pt.Fill(ui.Rect{X: cx - px/2, Y: plot.Y, W: px, H: plot.H}, withAlpha(ch.accent, 0.35), 0)
		pt.Fill(ui.Rect{X: cx - 4.5, Y: cy - 4.5, W: 9, H: 9}, ch.p.card, 4.5)
		var ring ui.Path
		ring.Circle(cx, cy, 3.5)
		pt.StrokePath(&ring, 2, ch.accent)
	}
	if ch.view.Recorded != "" {
		s := ch.span(ch.view.Recorded, axisSize, ch.p.label3)
		w, _ := pt.MeasureText(0, s)
		pt.RichText(plot.X+plot.W-w-4, plot.Y+2, 0, s)
	}
}

// axis draws the grid: thirds of the window, minute ticks and the time
// labels below.
func (ch chart) axis(pt *ui.Painter, plot ui.Rect, px float32) {
	for i := 0; i <= 3; i++ {
		gx := snap(pt, plot.X+plot.W*float32(i)/3)
		pt.Fill(ui.Rect{X: gx, Y: plot.Y, W: px, H: plot.H}, ch.p.label4, 0)
	}
	pt.Fill(ui.Rect{X: plot.X, Y: snap(pt, plot.Y+plot.H), W: plot.W, H: px}, ch.p.divider, 0)
	for minute := 0; minute <= 30; minute += 5 {
		tx := snap(pt, plot.X+plot.W*float32(minute)/30)
		pt.Fill(ui.Rect{X: tx, Y: plot.Y + plot.H, W: px, H: 3}, ch.p.divider, 0)
	}
	labels := ch.view.AxisLabels
	for i, label := range labels {
		s := ch.span(label, axisSize, ch.p.label3)
		w, _ := pt.MeasureText(0, s)
		lx := plot.X + plot.W*float32(i)/float32(len(labels)-1) - w/2
		switch i {
		case 0:
			lx = plot.X
		case len(labels) - 1:
			lx = plot.X + plot.W - w
		}
		pt.RichText(lx, plot.Y+plot.H+6, 0, s)
	}
}

func (ch chart) empty(pt *ui.Painter, plot ui.Rect, px float32) {
	for i := 1; i < 3; i++ {
		pt.Fill(ui.Rect{X: plot.X, Y: snap(pt, plot.Y+plot.H*float32(i)/3), W: plot.W, H: px}, ch.p.label4, 0)
	}
	if ch.view.Empty == "" {
		return
	}
	s := ch.span(ch.view.Empty, captionSize, ch.p.label2)
	w, h := pt.MeasureText(0, s)
	box := ui.Rect{X: plot.X + (plot.W-w)/2 - 12, Y: plot.Y + (plot.H-h)/2 - 6, W: w + 24, H: h + 12}
	pt.Fill(box, ch.p.card, box.H/2)
	pt.RichText(box.X+12, box.Y+6, 0, s)
}

// scale draws the horizontal grid at the ticks and their values on the
// right.
func (ch chart) scale(pt *ui.Painter, r, plot ui.Rect, ticks []float64, y func(float64) float32, px float32) {
	fractional := false
	for _, t := range ticks {
		fractional = fractional || math.Abs(t-math.Round(t)) > 1e-6
	}
	for _, t := range ticks {
		ty := snap(pt, y(t))
		if ty > plot.Y+1 && ty < plot.Y+plot.H-1 {
			var line ui.Path
			line.MoveTo(plot.X, ty+px/2).LineTo(plot.X+plot.W, ty+px/2).Dash(0, 3, 3)
			pt.StrokePath(&line, px, ch.p.divider)
		}
		s := ch.span(tickLabel(t, fractional), axisSize, ch.p.label3)
		w, h := pt.MeasureText(0, s)
		pt.RichText(r.X+r.W-w, ty-h/2, 0, s)
	}
}

func tickLabel(v float64, fractional bool) string {
	text := strconv.FormatFloat(v, 'f', 0, 64)
	if fractional {
		text = strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
	}
	if text == "-0" {
		text = "0"
	}
	if text != "" && text[0] == '-' {
		text = meter.Minus + text[1:]
	}
	return text
}

// connectedRuns groups the columns with data into runs the line connects.
func connectedRuns(buckets []meter.TrendBucket) [][]int {
	var runs [][]int
	for i, b := range buckets {
		if !b.HasData {
			continue
		}
		if len(runs) == 0 || !b.ConnectedToPrevious {
			runs = append(runs, nil)
		}
		runs[len(runs)-1] = append(runs[len(runs)-1], i)
	}
	return runs
}

func (ch chart) run(pt *ui.Painter, plot ui.Rect, buckets []meter.TrendBucket, run []int,
	x, y func(float64) float32, negative bool) {
	first := buckets[run[0]]
	if len(run) == 1 {
		pt.Fill(ui.Rect{X: x(first.Seconds) - 1.75, Y: y(first.Mean) - 1.75, W: 3.5, H: 3.5}, ch.accent, 1.75)
		return
	}
	var line, area, band ui.Path
	base := plot.Y + plot.H
	if negative {
		base = plot.Y
	}
	for k, i := range run {
		b := buckets[i]
		bx, by := x(b.Seconds), y(b.Mean)
		if k == 0 {
			line.MoveTo(bx, by)
			area.MoveTo(bx, base).LineTo(bx, by)
			band.MoveTo(bx, y(b.Max))
		} else {
			line.LineTo(bx, by)
			area.LineTo(bx, by)
			band.LineTo(bx, y(b.Max))
		}
	}
	last := buckets[run[len(run)-1]]
	area.LineTo(x(last.Seconds), base).Close()
	for k := len(run) - 1; k >= 0; k-- {
		b := buckets[run[k]]
		band.LineTo(x(b.Seconds), y(b.Min))
	}
	band.Close()
	from, to := withAlpha(ch.accent, 0.22), withAlpha(ch.accent, 0)
	if negative {
		from, to = to, from
	}
	pt.FillPathGradient(&area, ui.LinearGradient{From: from, To: to, Angle: 180})
	pt.FillPath(&band, withAlpha(ch.accent, 0.16))
	line.Join(ui.RoundJoin).Cap(ui.RoundCap)
	pt.StrokePath(&line, 1.75, ch.accent)
}

func (ch chart) peak(pt *ui.Painter, plot ui.Rect, px, py float32) {
	pt.Fill(ui.Rect{X: px - 3, Y: py - 3, W: 6, H: 6}, ch.accent, 3)
	if ch.view.PeakLabel == "" {
		return
	}
	s := ch.span(ch.view.PeakLabel, axisSize, ch.p.label2)
	w, h := pt.MeasureText(0, s)
	ty := py - h - 8
	if ty < plot.Y {
		ty = py + 6
	}
	tx := max(plot.X+2, min(plot.X+plot.W-w-8, px-w/2-3))
	box := ui.Rect{X: tx, Y: ty, W: w + 6, H: h + 2}
	pt.Fill(box, withAlpha(ch.p.card, 0.85), 4)
	pt.RichText(tx+3, ty+1, 0, s)
}

// snap puts a coordinate on a device pixel, so hairlines stay sharp.
func snap(pt *ui.Painter, v float32) float32 {
	s := pt.Scale()
	return float32(math.Floor(float64(v*s))) / s
}
