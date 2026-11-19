package num

import "testing"

func TestFixed(t *testing.T) {
	cases := []struct {
		v        float64
		decimals int
		want     string
	}{
		{0, 2, "0.00"},
		{2, 2, "2.00"},
		{-1.5, 2, "-1.50"},
		{39.08 + 27, 2, "66.08"},
		{0.004, 2, "0.00"},
		{-0.004, 2, "0.00"},
		{99.999, 2, "100.00"},
		{1234567.891, 2, "1234567.89"},
		{-20, 0, "-20"},
		{43.802 / 12.0, 3, "3.650"},
		{1e-20, 2, "0.00"},
	}
	for _, c := range cases {
		if got := Fixed(c.v, c.decimals); got != c.want {
			t.Errorf("Fixed(%v, %d) = %q, want %q", c.v, c.decimals, got, c.want)
		}
	}
}
