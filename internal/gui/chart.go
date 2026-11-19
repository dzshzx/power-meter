package gui

import (
	"math"
	"strconv"

	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/egoist/mygo/ui"
)

// Chart insets inside its element: room for the time axis below and the
// scale on the right.
const (
	chartTop    = 4
	chartRight  = 30
	chartBottom = 18
)

// chart draws the thirty-minute trend in the accent: the mean line over a
// soft area, faint scale lines and the live point. Missing readings leave
// gaps; the series and labels come from the session.
type chart struct {
	p    *palette
	view *meter.TrendView
}

func (ch chart) span(text string, color ui.Color) ui.Span {
	return ui.Span{Text: text, Font: textFont, Size: axisSize, Color: color, Features: tabular}
}

func (ch chart) draw(pt *ui.Painter, r ui.Rect) {
	plot := ui.Rect{X: r.X, Y: r.Y + chartTop, W: r.W - chartRight, H: r.H - chartTop - chartBottom}
	if plot.W <= 0 || plot.H <= 0 {
		return
	}
	px := 1 / pt.Scale()
	pt.Fill(ui.Rect{X: plot.X, Y: snap(pt, plot.Y+plot.H), W: plot.W, H: px}, ch.p.divider, 0)
	ch.axis(pt, plot)
	series := ch.view.Series
	if series == nil || !series.HasData() {
		ch.empty(pt, plot)
		return
	}
	columns := max(1, int(plot.W*pt.Scale()/2))
	buckets := series.Buckets(columns)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, b := range buckets {
		if b.HasData {
			lo, hi = math.Min(lo, b.Mean), math.Max(hi, b.Mean)
		}
	}
	pad := math.Max((hi-lo)*0.25, math.Max(0.5, math.Abs(hi)*0.03))
	ticks := meter.Ticks(lo-pad, hi+pad, 2)
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
	if latest, ok := series.Latest(); ok {
		cx, cy := x(series.Now), y(latest)
		pt.Fill(ui.Rect{X: cx - 5, Y: cy - 5, W: 10, H: 10}, ch.p.group, 5)
		pt.Fill(ui.Rect{X: cx - 3.5, Y: cy - 3.5, W: 7, H: 7}, ch.p.accent, 3.5)
	}
	if ch.view.Recorded != "" {
		s := ch.span(ch.view.Recorded, ch.p.ink3)
		w, _ := pt.MeasureText(0, s)
		pt.RichText(plot.X+plot.W-w, plot.Y-2, 0, s)
	}
}

// axis labels the window's time at its edges and thirds.
func (ch chart) axis(pt *ui.Painter, plot ui.Rect) {
	labels := ch.view.AxisLabels
	for i, text := range labels {
		s := ch.span(text, ch.p.ink3)
		w, _ := pt.MeasureText(0, s)
		lx := plot.X + plot.W*float32(i)/float32(len(labels)-1) - w/2
		switch i {
		case 0:
			lx = plot.X
		case len(labels) - 1:
			lx = plot.X + plot.W - w
		}
		pt.RichText(lx, plot.Y+plot.H+4, 0, s)
	}
}

func (ch chart) empty(pt *ui.Painter, plot ui.Rect) {
	if ch.view.Empty == "" {
		return
	}
	s := ui.Span{Text: ch.view.Empty, Font: textFont, Size: captionSize, Color: ch.p.ink3}
	w, h := pt.MeasureText(0, s)
	pt.RichText(plot.X+(plot.W-w)/2, plot.Y+(plot.H-h)/2, 0, s)
}

// scale draws faint lines at the ticks and their values on the right.
func (ch chart) scale(pt *ui.Painter, r, plot ui.Rect, ticks []float64, y func(float64) float32, px float32) {
	fractional := false
	for _, t := range ticks {
		fractional = fractional || math.Abs(t-math.Round(t)) > 1e-6
	}
	for _, t := range ticks {
		ty := snap(pt, y(t))
		if ty > plot.Y+1 && ty < plot.Y+plot.H-1 {
			pt.Fill(ui.Rect{X: plot.X, Y: ty, W: plot.W, H: px}, ch.p.faint, 0)
		}
		s := ch.span(tickLabel(t, fractional), ch.p.ink3)
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
		pt.Fill(ui.Rect{X: x(first.Seconds) - 1.6, Y: y(first.Mean) - 1.6, W: 3.2, H: 3.2}, ch.p.accent, 1.6)
		return
	}
	var line, area ui.Path
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
		} else {
			line.LineTo(bx, by)
			area.LineTo(bx, by)
		}
	}
	area.LineTo(x(buckets[run[len(run)-1]].Seconds), base).Close()
	from, to := withAlpha(ch.p.accent, 0.16), withAlpha(ch.p.accent, 0)
	if negative {
		from, to = to, from
	}
	pt.FillPathGradient(&area, ui.LinearGradient{From: from, To: to, Angle: 180})
	line.Join(ui.RoundJoin).Cap(ui.RoundCap)
	pt.StrokePath(&line, 1.6, ch.p.accent)
}

// snap puts a coordinate on a device pixel, so hairlines stay sharp.
func snap(pt *ui.Painter, v float32) float32 {
	s := pt.Scale()
	return float32(math.Floor(float64(v*s))) / s
}
