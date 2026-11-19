// Package gui draws the meter window, its settings popover and the tray icon
// with mygo's native UI, after COUI 17 (ColorOS 17): one rounded panel with
// rounded groups of rows on it, MiSans throughout, neutral greys, one blue
// for data and actions, amber only to warn and red only for a sensor error.
// It only shows what a meter.View says; display rules live in package meter.
// Geometry is in DIPs, which mygo scales to the window's DPI.
package gui

import (
	"github.com/dzshzx/power-meter/internal/fonts"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/egoist/mygo/ui"
)

// palette holds the colours of one appearance. The window follows the
// system's light or dark appearance.
type palette struct {
	dark      bool
	page      ui.Color // the panel, under the groups
	group     ui.Color // a rounded group of rows
	popup     ui.Color // popovers above the panel
	border    ui.Color // the panel's outline
	ink       ui.Color // primary text
	inkSoft   ui.Color // an estimated headline: a step lighter than ink
	ink2      ui.Color // captions, units, secondary values
	ink3      ui.Color // placeholders and axis labels
	faint     ui.Color // the chart's scale lines
	divider   ui.Color
	hover     ui.Color
	press     ui.Color
	accent    ui.Color // the trend, links, the switch: data and actions
	warn      ui.Color // the notice's sign
	danger    ui.Color // a sensor error
	track     ui.Color // the segmented control's track
	indicator ui.Color // its selected segment
	switchOff ui.Color
	button    ui.Color // a secondary button's fill
	shadow    ui.Color
}

func argb(v uint32) ui.Color {
	return ui.Color{A: uint8(v >> 24), R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}
}

var lightPalette = palette{
	page:      argb(0xFFF0F1F3),
	group:     argb(0xFFFFFFFF),
	popup:     argb(0xFFFFFFFF),
	border:    argb(0x14000000),
	ink:       argb(0xE6000000),
	inkSoft:   argb(0xC2000000),
	ink2:      argb(0x8F000000),
	ink3:      argb(0x5C000000),
	faint:     argb(0x12000000),
	divider:   argb(0x14000000),
	hover:     argb(0x0A000000),
	press:     argb(0x14000000),
	accent:    argb(0xFF0A72E8),
	warn:      argb(0xFFD98E04),
	danger:    argb(0xFFD93025),
	track:     argb(0x0D000000),
	indicator: argb(0xFFFFFFFF),
	switchOff: argb(0x29000000),
	button:    argb(0x0D000000),
	shadow:    argb(0x29000000),
}

var darkPalette = palette{
	dark:      true,
	page:      argb(0xFF161616),
	group:     argb(0xFF262626),
	popup:     argb(0xFF303030),
	border:    argb(0x1FFFFFFF),
	ink:       argb(0xEBFFFFFF),
	inkSoft:   argb(0xC7FFFFFF),
	ink2:      argb(0x99FFFFFF),
	ink3:      argb(0x61FFFFFF),
	faint:     argb(0x14FFFFFF),
	divider:   argb(0x1AFFFFFF),
	hover:     argb(0x0FFFFFFF),
	press:     argb(0x1AFFFFFF),
	accent:    argb(0xFF4DA6FF),
	warn:      argb(0xFFF2B33D),
	danger:    argb(0xFFFF6B5E),
	track:     argb(0x14FFFFFF),
	indicator: argb(0x2EFFFFFF),
	switchOff: argb(0x40FFFFFF),
	button:    argb(0x17FFFFFF),
	shadow:    argb(0x8C000000),
}

func paletteFor(dark bool) *palette {
	if dark {
		return &darkPalette
	}
	return &lightPalette
}

// The tray tile keeps solid colours dark enough for white figures at 16 px
// on either taskbar: there the state shows at a glance.
var (
	trayCharging    = argb(0xFF059669)
	trayDischarging = argb(0xFFD97706)
	trayNeutral     = argb(0xFF6B7280)
	appTile         = argb(0xFF059669)
	closeHover      = argb(0xFFC42B1C)
	white           = argb(0xFFFFFFFF)
)

// Fonts: MiSans in three faces, Microsoft YaHei UI where MiSans could not
// be registered, and tabular figures for numbers that change every second.
const (
	textFont   = fonts.Regular + ", Microsoft YaHei UI, Segoe UI"
	mediumFont = fonts.Medium + ", Microsoft YaHei UI, Segoe UI"
	figureFont = fonts.Semibold + ", Microsoft YaHei UI, Segoe UI"
	tabular    = "tnum"
)

// Metrics. The window is the panel and, around it, the room its shadow
// takes. Every corner is continuous (see squircle), and nested corners are
// concentric: a group inset from the panel's edge takes the panel's radius
// less the inset.
const (
	panelWidth   = 360
	panelRadius  = 32
	shadowSide   = 16
	shadowTop    = 10
	shadowBottom = 24
	inset        = 12 // the panel's edge to a group, and between groups
	groupRadius  = panelRadius - inset
	groupPadding = 18
	titleBar     = 52
	footerHeight = 46
	rowHeight    = 46
	chartHeight  = 96

	captionSize = 12
	bodySize    = 14
	tabSize     = 15
	titleSize   = 18
	headSize    = 44
	axisSize    = 11
)

// tone is a reading's colour: ink, a step lighter for an estimated
// headline, quieter for supporting text, faint without a value, and red for
// a sensor error.
func (p *palette) tone(t meter.Tone, a power.Accent) ui.Color {
	switch t {
	case meter.ToneMuted:
		return p.ink2
	case meter.ToneUnavailable:
		return p.ink3
	case meter.ToneNumericMuted:
		return p.inkSoft
	case meter.ToneAccent:
		if a == power.AccentError {
			return p.danger
		}
	}
	return p.ink
}

// trayTile is the tray icon's tile colour.
func trayTile(a power.Accent) ui.Color {
	switch a {
	case power.AccentCharging:
		return trayCharging
	case power.AccentDischarging:
		return trayDischarging
	default:
		return trayNeutral
	}
}

func withAlpha(c ui.Color, a float32) ui.Color {
	c.A = uint8(float32(c.A)*a + 0.5)
	return c
}
