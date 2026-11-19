// Package num formats numbers for display and reports with fixed rules, so
// readings, captions and probe reports keep the exact text of 1.5, which
// people compare across versions.
package num

import (
	"math"
	"strconv"
	"strings"
)

// Fixed formats v with exactly decimals digits after the point, as the
// invariant custom format "0.00" of .NET Framework does: v is first taken to
// 15 significant digits, then rounded to the decimals with halves away from
// zero. A result that rounds to zero carries no sign.
func Fixed(v float64, decimals int) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	}
	digits, point := significant(math.Abs(v))
	keep := point + decimals
	switch {
	case keep < 0:
		digits, point = nil, 0
	case keep < len(digits):
		roundUp := digits[keep] >= '5'
		digits = digits[:keep]
		if roundUp {
			digits, point = carry(digits, point)
		}
	}
	text := layout(digits, point, decimals)
	if v < 0 && strings.Trim(text, "0.") != "" {
		return "-" + text
	}
	return text
}

// significant returns the 15 significant decimal digits of a non-negative v
// and the position of the decimal point within them.
func significant(v float64) ([]byte, int) {
	if v == 0 {
		return nil, 0
	}
	s := strconv.FormatFloat(v, 'e', 14, 64) // d.dddddddddddddde±XX
	mantissa, exponent, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(exponent)
	return []byte(strings.Replace(mantissa, ".", "", 1)), exp + 1
}

// carry adds one unit in the last kept digit.
func carry(digits []byte, point int) ([]byte, int) {
	for i := len(digits) - 1; i >= 0; i-- {
		if digits[i] < '9' {
			digits[i]++
			return digits, point
		}
		digits[i] = '0'
	}
	return append([]byte{'1'}, digits...), point + 1
}

func layout(digits []byte, point, decimals int) string {
	digit := func(i int) byte {
		if i >= 0 && i < len(digits) {
			return digits[i]
		}
		return '0'
	}
	var b strings.Builder
	if point <= 0 {
		b.WriteByte('0')
	}
	for i := 0; i < point; i++ {
		b.WriteByte(digit(i))
	}
	if decimals > 0 {
		b.WriteByte('.')
		for i := point; i < point+decimals; i++ {
			b.WriteByte(digit(i))
		}
	}
	return b.String()
}

// Int formats a whole number as the invariant culture does.
func Int(v int) string {
	return strconv.Itoa(v)
}
