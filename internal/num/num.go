// Package num formats numbers for display and reports.
package num

import (
	"strconv"
	"strings"
)

// Fixed formats v with exactly decimals digits after the point, rounded to
// the nearest. A result that rounds to zero carries no sign.
func Fixed(v float64, decimals int) string {
	text := strconv.FormatFloat(v, 'f', decimals, 64)
	if rest, ok := strings.CutPrefix(text, "-"); ok && strings.Trim(rest, "0.") == "" {
		return rest
	}
	return text
}
