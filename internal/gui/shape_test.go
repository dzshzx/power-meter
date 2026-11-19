package gui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSquircle(t *testing.T) {
	for _, tc := range []struct {
		r      ui.Rect
		radius float32
	}{
		{ui.Rect{X: 10, Y: 20, W: 300, H: 120}, 20},
		{ui.Rect{X: 0, Y: 0, W: 40, H: 40}, 20}, // a circle's worth: no room to smooth
		{ui.Rect{X: 0, Y: 0, W: 160, H: 30}, 15},
	} {
		var path ui.Path
		squircle(&path, tc.r, tc.radius)
		b := path.Bounds()
		if math.Abs(float64(b.X-tc.r.X)) > 0.01 || math.Abs(float64(b.Y-tc.r.Y)) > 0.01 ||
			math.Abs(float64(b.W-tc.r.W)) > 0.01 || math.Abs(float64(b.H-tc.r.H)) > 0.01 {
			t.Errorf("%+v radius %v: bounds %+v", tc.r, tc.radius, b)
		}
		cx, cy := tc.r.X+tc.r.W/2, tc.r.Y+tc.r.H/2
		if !path.Contains(cx, cy) || !path.Contains(cx, tc.r.Y+0.5) || !path.Contains(tc.r.X+0.5, cy) {
			t.Errorf("%+v: the middle and the sides' midpoints are inside", tc.r)
		}
		for _, k := range [][2]float32{{tc.r.X + 1, tc.r.Y + 1}, {tc.r.X + tc.r.W - 1, tc.r.Y + tc.r.H - 1}} {
			if path.Contains(k[0], k[1]) {
				t.Errorf("%+v: corner %v is outside", tc.r, k)
			}
		}
	}
}
