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

// squircle adds a rectangle with continuous corners of radius to path,
// clockwise from the top-left corner's end. Its corner is the cubic, arc,
// cubic construction Figma uses for corner smoothing.
func squircle(path *ui.Path, r ui.Rect, radius float32) {
	limit := float64(min(r.W, r.H) / 2)
	R := math.Min(float64(radius), limit)
	s := smoothing
	p := (1 + s) * R
	if p > limit {
		s = math.Max(limit/R-1, 0)
		p = limit
	}
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	arc := 90 * (1 - s)
	arcLen := math.Sin(rad(arc/2)) * R * math.Sqrt2
	alpha := (90 - arc) / 2
	p34 := R * math.Tan(rad(alpha/2))
	beta := 45 * s
	c := p34 * math.Cos(rad(beta))
	d := c * math.Tan(rad(beta))
	b := (p - arcLen - c - d) / 3
	a := 2 * b

	x0, y0, x1, y1 := float64(r.X), float64(r.Y), float64(r.X+r.W), float64(r.Y+r.H)
	corners := []struct{ kx, ky, ux, uy, vx, vy float64 }{
		{x1, y0, 1, 0, 0, 1},   // top right: along the top, then down
		{x1, y1, 0, 1, -1, 0},  // bottom right
		{x0, y1, -1, 0, 0, -1}, // bottom left
		{x0, y0, 0, -1, 1, 0},  // top left
	}
	pt := func(x, y float64) (float32, float32) { return float32(x), float32(y) }
	last := corners[3]
	path.MoveTo(pt(last.kx+p*last.vx, last.ky+p*last.vy))
	for _, k := range corners {
		at := func(u, v float64) (float64, float64) {
			return k.kx + u*k.ux + v*k.vx, k.ky + u*k.uy + v*k.vy
		}
		sx, sy := at(-p, 0)
		path.LineTo(pt(sx, sy))
		// The cubic from the side into the arc.
		c1x, c1y := at(-p+a, 0)
		c2x, c2y := at(-p+a+b, 0)
		ax, ay := at(-p+a+b+c, d)
		path.CubeTo(float32(c1x), float32(c1y), float32(c2x), float32(c2y), float32(ax), float32(ay))
		// The circular arc, as one cubic.
		ex, ey := at(-p+a+b+c+arcLen, d+arcLen)
		cx, cy := at(-R, R)
		k4 := 4.0 / 3 * math.Tan(rad(arc)/4)
		t1x, t1y := tangent(ax-cx, ay-cy, k.ux+k.vx, k.uy+k.vy)
		t2x, t2y := tangent(ex-cx, ey-cy, k.ux+k.vx, k.uy+k.vy)
		path.CubeTo(float32(ax+k4*t1x), float32(ay+k4*t1y), float32(ex-k4*t2x), float32(ey-k4*t2y), float32(ex), float32(ey))
		// The cubic from the arc into the next side.
		c3x, c3y := at(-p+a+b+c+arcLen+d, d+arcLen+c)
		c4x, c4y := at(0, p-a)
		endx, endy := at(0, p)
		path.CubeTo(float32(c3x), float32(c3y), float32(c4x), float32(c4y), float32(endx), float32(endy))
	}
	path.Close()
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

// shape fills an element's box with continuous corners, as its background.
func shape(color ui.Color, radius float32) func(*ui.Painter, ui.Rect) {
	return func(pt *ui.Painter, r ui.Rect) {
		var path ui.Path
		squircle(&path, r, radius)
		pt.FillPath(&path, color)
	}
}

// outlined fills an element's box with continuous corners and outlines it
// with a hairline, as a popover's surface.
func outlined(fill, line ui.Color, radius float32) func(*ui.Painter, ui.Rect) {
	return func(pt *ui.Painter, r ui.Rect) {
		var path ui.Path
		squircle(&path, r, radius)
		pt.FillPath(&path, fill)
		pt.StrokePath(&path, 1/pt.Scale(), line)
	}
}
