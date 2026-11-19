package num

import (
	"math"
	"testing"
)

func TestFixed(t *testing.T) {
	cases := []struct {
		v        float64
		decimals int
		want     string
	}{
		{0, 2, "0.00"},
		{2, 2, "2.00"},
		{-1.5, 2, "-1.50"},
		{66.08, 2, "66.08"},
		{39.08 + 27, 2, "66.08"},
		{2.675, 2, "2.68"}, // 15 digits first, then halves away from zero
		{-2.675, 2, "-2.68"},
		{0.005, 2, "0.01"},
		{0.004, 2, "0.00"},
		{-0.004, 2, "0.00"},
		{9.995, 2, "10.00"},
		{99.999, 2, "100.00"},
		{1234567.891, 2, "1234567.89"},
		{0.0001234, 2, "0.00"},
		{-20, 0, "-20"},
		{-10, 0, "-10"},
		{17.5, 0, "18"},
		{43.802 / 12.0, 2, "3.65"},
		{1e-20, 2, "0.00"},
		{math.NaN(), 2, "NaN"},
	}
	for _, c := range cases {
		if got := Fixed(c.v, c.decimals); got != c.want {
			t.Errorf("Fixed(%v, %d) = %q, want %q", c.v, c.decimals, got, c.want)
		}
	}
}
