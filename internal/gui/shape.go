package gui

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// smoothing is how far each corner's curve reaches along the sides beyond
// a circular arc of the same radius, as a fraction of the radius: the
// continuous corners of ColorOS and iOS, whose curvature grows from the
// straight side instead of jumping to the arc's.
const smoothing = 0.6

// curve is the shape of a rectangle's continuous corners: each starts p
// before its corner along one side and ends p after it along the next,
// through a cubic, a circular arc of radius R as one cubic, and another
// cubic. It is the cubic, arc, cubic construction Figma uses for corner
// smoothing.
type curve struct{ p, b, c, d, arcLen, R, arc float64 }

func curveOf(r ui.Rect, radius float32) curve {
	limit := float64(min(r.W, r.H) / 2)
	R := math.Min(float64(radius), limit)
	s := smoothing
	p := (1 + s) * R
	if p > limit {
		s = math.Max(limit/R-1, 0)
		p = limit
	}
	arc := 90 * (1 - s)
	arcLen := math.Sin(rad(arc/2)) * R * math.Sqrt2
	alpha := (90 - arc) / 2
	p34 := R * math.Tan(rad(alpha/2))
	beta := 45 * s
	c := p34 * math.Cos(rad(beta))
	d := c * math.Tan(rad(beta))
	b := (p - arcLen - c - d) / 3
	return curve{p: p, b: b, c: c, d: d, arcLen: arcLen, R: R, arc: arc}
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }

// corner is a corner k of a rectangle, with u the way along the side that
// comes into it and v the way along the side that leaves it, clockwise.
type corner struct{ kx, ky, ux, uy, vx, vy float64 }

// at is the point u along the side into the corner (back from it when
// negative) and v along the side out of it.
func (k corner) at(u, v float64) (float64, float64) {
	return k.kx + u*k.ux + v*k.vx, k.ky + u*k.uy + v*k.vy
}

// pt is at as a path takes it.
func (k corner) pt(u, v float64) (float32, float32) {
	x, y := k.at(u, v)
	return float32(x), float32(y)
}

// cornersOf are r's corners clockwise from the top right.
func cornersOf(r ui.Rect) [4]corner {
	x0, y0, x1, y1 := float64(r.X), float64(r.Y), float64(r.X+r.W), float64(r.Y+r.H)
	return [4]corner{
		{x1, y0, 1, 0, 0, 1},   // top right: along the top, then down
		{x1, y1, 0, 1, -1, 0},  // bottom right
		{x0, y1, -1, 0, 0, -1}, // bottom left
		{x0, y0, 0, -1, 1, 0},  // top left
	}
}

// trace adds the curve round k to path, which stands at k.at(-p, 0); it
// ends at k.at(0, p).
func (cv curve) trace(path *ui.Path, k corner) {
	p, b, c, d, arcLen := cv.p, cv.b, cv.c, cv.d, cv.arcLen
	a := 2 * b
	// The cubic from the side into the arc.
	c1x, c1y := k.pt(-p+a, 0)
	c2x, c2y := k.pt(-p+a+b, 0)
	ax, ay := k.at(-p+a+b+c, d)
	f := func(v float64) float32 { return float32(v) }
	path.CubeTo(c1x, c1y, c2x, c2y, f(ax), f(ay))
	// The circular arc, as one cubic.
	ex, ey := k.at(-p+a+b+c+arcLen, d+arcLen)
	cx, cy := k.at(-cv.R, cv.R)
	k4 := 4.0 / 3 * math.Tan(rad(cv.arc)/4)
	t1x, t1y := tangent(ax-cx, ay-cy, k.ux+k.vx, k.uy+k.vy)
	t2x, t2y := tangent(ex-cx, ey-cy, k.ux+k.vx, k.uy+k.vy)
	path.CubeTo(f(ax+k4*t1x), f(ay+k4*t1y), f(ex-k4*t2x), f(ey-k4*t2y), f(ex), f(ey))
	// The cubic from the arc into the next side.
	c3x, c3y := k.pt(-p+a+b+c+arcLen+d, d+arcLen+c)
	c4x, c4y := k.pt(0, p-a)
	endx, endy := k.pt(0, p)
	path.CubeTo(c3x, c3y, c4x, c4y, endx, endy)
}

// tangent is the radius vector (x, y) turned a quarter, to point the way
// the path goes (along dx, dy).
func tangent(x, y, dx, dy float64) (float64, float64) {
	tx, ty := -y, x
	if tx*dx+ty*dy < 0 {
		tx, ty = -tx, -ty
	}
	return tx, ty
}

// squircle adds a rectangle with continuous corners of radius to path,
// clockwise from the top-left corner's end.
func squircle(path *ui.Path, r ui.Rect, radius float32) {
	cv := curveOf(r, radius)
	ks := cornersOf(r)
	path.MoveTo(ks[3].pt(0, cv.p))
	for _, k := range ks {
		path.LineTo(k.pt(-cv.p, 0))
		cv.trace(path, k)
	}
	path.Close()
}

// The window draws its squircles in pieces. A path is drawn through a mask
// of its bounds, kept in the atlas the process shares with the glyphs,
// which grows to twice the masks it holds and never shrinks: the panel's
// fill and outline as whole paths grow it to 4096×4096, 16 MB, and even a
// side's edge as a path, a pixel wide, takes a shelf of the atlas as tall.
// So only the corners are paths, each of the square its curve takes; the
// pixels between them are rectangles, which take no mask, those the
// straight edges cover in part drawn with the alpha of how much they
// cover. The pieces meet on pixel boundaries, so each pixel is covered by
// one of them, as the whole path would cover it.

// grid is where a squircle's pieces meet, in DIPs on device pixels: the
// corners' squares end at the seams xl, xr, yt and yb, and the edges' part
// pixels at ix0, ix1, iy0 and iy1.
type grid struct {
	x0, y0, x1, y1     float32
	xl, xr, yt, yb     float32
	ix0, ix1, iy0, iy1 float32
}

// gridOf places the seams of r, whose corners take p along each side, at
// scale device pixels a DIP; false when the corners leave no whole pixel
// between them.
func gridOf(r ui.Rect, p float64, scale float32) (grid, bool) {
	s := float64(scale)
	up := func(v float64) float32 { return float32(math.Ceil(v*s-1e-4) / s) }
	down := func(v float64) float32 { return float32(math.Floor(v*s+1e-4) / s) }
	x0, y0, x1, y1 := float64(r.X), float64(r.Y), float64(r.X+r.W), float64(r.Y+r.H)
	g := grid{
		x0: r.X, y0: r.Y, x1: r.X + r.W, y1: r.Y + r.H,
		xl: up(x0 + p), xr: down(x1 - p), yt: up(y0 + p), yb: down(y1 - p),
		ix0: up(x0), ix1: down(x1), iy0: up(y0), iy1: down(y1),
	}
	return g, g.xl <= g.xr && g.yt <= g.yb
}

// seams are how far corner i's square reaches along the side into it and
// along the side out of it (cornersOf's order).
func (g grid) seams(i int) (into, out float64) {
	right, top := float64(g.x1-g.xr), float64(g.yt-g.y0)
	bottom, left := float64(g.y1-g.yb), float64(g.xl-g.x0)
	switch i {
	case 0:
		return right, top
	case 1:
		return bottom, right
	case 2:
		return left, bottom
	}
	return top, left
}

// piece adds corner k's piece of the outline to path: from into back along
// the side into it, round its curve, to out along the side out of it.
func (cv curve) piece(path *ui.Path, k corner, into, out float64) {
	path.MoveTo(k.pt(-into, 0))
	path.LineTo(k.pt(-cv.p, 0))
	cv.trace(path, k)
	path.LineTo(k.pt(0, out))
}

// fillSquircle fills r with continuous corners of radius, as squircle
// outlines it.
func fillSquircle(pt *ui.Painter, r ui.Rect, radius float32, color ui.Color) {
	cv := curveOf(r, radius)
	g, ok := gridOf(r, cv.p, pt.Scale())
	if !ok {
		var path ui.Path
		squircle(&path, r, radius)
		pt.FillPath(&path, color)
		return
	}
	for i, k := range cornersOf(r) {
		into, out := g.seams(i)
		var path ui.Path
		cv.piece(&path, k, into, out)
		path.LineTo(k.pt(-into, out))
		path.Close()
		pt.FillPath(&path, color)
	}
	// Whole pixels: the middle, from the top's to the bottom's, and the
	// sides between the corners.
	fillPixels(pt, g.xl, g.iy0, g.xr, g.iy1, color)
	fillPixels(pt, g.ix0, g.yt, g.xl, g.yb, color)
	fillPixels(pt, g.xr, g.yt, g.ix1, g.yb, color)
	// The edges' pixels the shape covers in part.
	fillBand(pt, g.xl, g.y0, g.xr, g.iy0, color)
	fillBand(pt, g.xl, g.iy1, g.xr, g.y1, color)
	fillBand(pt, g.x0, g.yt, g.ix0, g.yb, color)
	fillBand(pt, g.ix1, g.yt, g.x1, g.yb, color)
}

// strokeSquircle outlines r with continuous corners of radius, width DIPs
// wide on the outline squircle makes, in the pieces fillSquircle fills.
func strokeSquircle(pt *ui.Painter, r ui.Rect, radius, width float32, color ui.Color) {
	cv := curveOf(r, radius)
	g, ok := gridOf(r, cv.p, pt.Scale())
	if !ok {
		var path ui.Path
		squircle(&path, r, radius)
		pt.StrokePath(&path, width, color)
		return
	}
	for i, k := range cornersOf(r) {
		into, out := g.seams(i)
		var path ui.Path
		path.Cap(ui.ButtCap)
		cv.piece(&path, k, into, out)
		pt.StrokePath(&path, width, color)
	}
	// The sides between the corners, as their strokes would cover them.
	h := width / 2
	fillBand(pt, g.xl, g.y0-h, g.xr, g.y0+h, color)
	fillBand(pt, g.xl, g.y1-h, g.xr, g.y1+h, color)
	fillBand(pt, g.x0-h, g.yt, g.x0+h, g.yb, color)
	fillBand(pt, g.x1-h, g.yt, g.x1+h, g.yb, color)
}

// fillPixels fills a rectangle on pixel boundaries, without a mask.
func fillPixels(pt *ui.Painter, x0, y0, x1, y1 float32, color ui.Color) {
	if x1 > x0 && y1 > y0 {
		pt.Fill(ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, color, 0)
	}
}

// fillBand fills a rectangle off pixel boundaries without a mask: in
// rectangles of whole pixels, each with the alpha of how much of its
// pixels the band covers.
func fillBand(pt *ui.Painter, x0, y0, x1, y1 float32, color ui.Color) {
	s := pt.Scale()
	for _, x := range spans(x0, x1, s) {
		for _, y := range spans(y0, y1, s) {
			if cover := x.cover * y.cover; cover > 1e-3 && x.to > x.from && y.to > y.from {
				pt.Fill(ui.Rect{X: x.from, Y: y.from, W: x.to - x.from, H: y.to - y.from}, color.Alpha(cover), 0)
			}
		}
	}
}

// span is a run of device pixels, in DIPs, and how much of each a range
// covers.
type span struct{ from, to, cover float32 }

// spans are the pixels of v0 to v1 at scale device pixels a DIP: the one
// it covers in part at each end and the whole ones between.
func spans(v0, v1, scale float32) [3]span {
	s := float64(scale)
	a, b := float64(v0)*s, float64(v1)*s
	px := func(v float64) float32 { return float32(v / s) }
	lo, hi := math.Ceil(a-1e-4), math.Floor(b+1e-4)
	if hi < lo {
		// Within one pixel.
		p := math.Floor(a)
		return [3]span{{px(p), px(p + 1), float32(b - a)}}
	}
	return [3]span{
		{px(lo - 1), px(lo), float32(lo - a)},
		{px(lo), px(hi), 1},
		{px(hi), px(hi + 1), float32(b - hi)},
	}
}

// shape fills an element's box with continuous corners, as its background.
func shape(color ui.Color, radius float32) func(*ui.Painter, ui.Rect) {
	return func(pt *ui.Painter, r ui.Rect) {
		fillSquircle(pt, r, radius, color)
	}
}

// outlined fills an element's box with continuous corners and outlines it
// with a hairline, as a popover's surface.
func outlined(fill, line ui.Color, radius float32) func(*ui.Painter, ui.Rect) {
	return func(pt *ui.Painter, r ui.Rect) {
		fillSquircle(pt, r, radius, fill)
		strokeSquircle(pt, r, radius, 1/pt.Scale(), line)
	}
}
