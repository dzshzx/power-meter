// Package power holds the power figures, their boundaries and supply states,
// and the one place a tick's snapshot is composed. Terms follow GLOSSARY.md.
package power

import (
	"math"
	"time"
)

// Boundary is the electrical boundary a figure is measured across. Every
// value shown carries its boundary, because sensors on different boundaries
// are not interchangeable.
type Boundary int

const (
	BatteryTerminal Boundary = iota
	CPUPackage
	Platform
	SystemLoad
	EstimatedSystemInput
)

// Label is the boundary's Chinese caption, which is also its translation key.
func (b Boundary) Label() string {
	switch b {
	case BatteryTerminal:
		return "电池端净功率"
	case CPUPackage:
		return "CPU 包功率"
	case Platform:
		return "平台功率"
	case SystemLoad:
		return "系统负载功率"
	case EstimatedSystemInput:
		return "估算整机输入功率"
	default:
		return "未知口径"
	}
}

// Kind says whether a figure came off a hardware counter or was derived from
// others. An estimate is never promoted to Measured.
type Kind int

const (
	Measured Kind = iota
	Estimated
)

// Sample is one power figure plus the provenance needed to label it
// honestly. An unavailable sample carries the reason instead of a value;
// callers never substitute a neighbouring boundary's number. A nil *Sample
// means no figure was produced at all.
type Sample struct {
	Available         bool
	UnavailableReason string
	Watts             float64
	Boundary          Boundary
	Kind              Kind
	Source            string
	Window            time.Duration
}

// FromValue is an available figure, or an unavailable one when watts is not
// finite.
func FromValue(boundary Boundary, kind Kind, watts float64, source string, window time.Duration) *Sample {
	if math.IsNaN(watts) || math.IsInf(watts, 0) {
		return Unsupported(boundary, "采样结果不是有限数值")
	}
	return &Sample{Available: true, Boundary: boundary, Kind: kind, Watts: watts, Source: source, Window: window}
}

// Unsupported is a figure that has no value, with the reason.
func Unsupported(boundary Boundary, reason string) *Sample {
	kind := Measured
	if boundary == EstimatedSystemInput {
		kind = Estimated
	}
	return &Sample{Boundary: boundary, Kind: kind, UnavailableReason: reason}
}

// Value is the reading in watts, and false when there is none to record.
func (s *Sample) Value() (float64, bool) {
	if s == nil || !s.Available {
		return 0, false
	}
	return s.Watts, true
}

// IsAvailable reports whether s holds a value.
func (s *Sample) IsAvailable() bool {
	return s != nil && s.Available
}

// ExceedsPeak is the peak rule shared by the 5-minute statistic and the
// trend: the largest magnitude wins and keeps its sign, so a discharge peak is
// the most negative reading; an equal magnitude keeps the earlier one.
func ExceedsPeak(candidate, peak float64) bool {
	return math.Abs(candidate) > math.Abs(peak)
}

// ReasonElevationRequired is the platform reason that restarting as
// administrator fixes.
const ReasonElevationRequired = "需要以管理员身份运行才能读取平台功率"
