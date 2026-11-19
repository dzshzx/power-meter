package gui

import (
	"strconv"
	"sync"

	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/egoist/mygo/ui"
)

// Unmodified Lucide geometry (ISC, see third_party/NOTICE.md).
const (
	iconZap           = `<path d="M15.914 4a1.5 1.5 0 0 0-2.474-1.561l-9 9A1.5 1.5 0 0 0 5.5 14h4.002a.5.5 0 0 1 .471.666L8.086 20a1.5 1.5 0 0 0 2.475 1.56l9-9A1.5 1.5 0 0 0 18.5 10h-3.997a.5.5 0 0 1-.472-.667z"/>`
	iconPin           = `<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/>`
	iconMinus         = `<path d="M5 12h14"/>`
	iconX             = `<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`
	iconPlug          = `<path d="M12 22v-5"/><path d="M15 8V2"/><path d="M17 8a1 1 0 0 1 1 1v4a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V9a1 1 0 0 1 1-1z"/><path d="M9 8V2"/>`
	iconPlugZap       = `<path d="M6.3 20.3a2.4 2.4 0 0 0 3.4 0L12 18l-6-6-2.3 2.3a2.4 2.4 0 0 0 0 3.4Z"/><path d="m2 22 3-3"/><path d="M7.5 13.5 10 11"/><path d="M10.5 16.5 13 14"/><path d="m18 3-4 4h6l-4 4"/>`
	iconBatteryMedium = `<path d="M10 14v-4"/><path d="M22 14v-4"/><path d="M6 14v-4"/><rect x="2" y="6" width="16" height="12" rx="2"/>`
	iconTriangleAlert = `<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>`
	iconShield        = `<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/>`
	iconSettings      = `<path d="M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915"/><circle cx="12" cy="12" r="3"/>`
	iconChevronDown   = `<path d="m6 9 6 6 6-6"/>`
)

var (
	iconMu    sync.Mutex
	iconCache = map[string]*ui.SVG{}
)

// icon parses a Lucide body drawn with the given stroke width; the colour
// comes from the element's text colour.
func icon(body string, strokeWidth float64) *ui.SVG {
	key := strconv.FormatFloat(strokeWidth, 'f', -1, 64) + "|" + body
	iconMu.Lock()
	defer iconMu.Unlock()
	if s, ok := iconCache[key]; ok {
		return s
	}
	doc := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">` +
		`<g fill="none" stroke="currentColor" stroke-width="` + strconv.FormatFloat(strokeWidth, 'f', -1, 64) +
		`" stroke-linecap="round" stroke-linejoin="round">` + body + `</g></svg>`
	s := ui.MustParseSVG([]byte(doc))
	iconCache[key] = s
	return s
}

func supplyIconBody(i meter.SupplyIcon) string {
	switch i {
	case meter.IconExternalPower:
		return iconPlug
	case meter.IconCharging:
		return iconPlugZap
	case meter.IconBattery:
		return iconBatteryMedium
	case meter.IconAlert:
		return iconTriangleAlert
	default:
		return ""
	}
}
