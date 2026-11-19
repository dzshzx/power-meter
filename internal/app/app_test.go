package app

import "testing"

func TestFitTop(t *testing.T) {
	cases := []struct{ y, wh, top, height, want int }{
		{100, 536, 0, 552, 16},  // runs past the bottom: moves up
		{-54, 536, 0, 552, 0},   // starts above the top (centred while taller): moves down
		{10, 536, 0, 552, 10},   // inside: stays
		{300, 700, 40, 552, 40}, // taller than the area: the title bar stays visible
	}
	for _, c := range cases {
		if got := fitTop(c.y, c.wh, c.top, c.height); got != c.want {
			t.Errorf("fitTop(%d, %d, %d, %d) = %d, want %d", c.y, c.wh, c.top, c.height, got, c.want)
		}
	}
}

func TestFitHeight(t *testing.T) {
	cases := []struct{ content, work, want int }{
		{585, 980, 585}, // 175% on the notebook: everything fits
		{585, 552, 536}, // 300%: the work area less the margin, and the content scrolls
		{585, 601, 585}, // exactly the margin to spare
		{585, 602, 585},
		{585, 0, 585}, // no work area known: leave the content height
	}
	for _, c := range cases {
		if got := fitHeight(c.content, c.work); got != c.want {
			t.Errorf("fitHeight(%d, %d) = %d, want %d", c.content, c.work, got, c.want)
		}
	}
}
