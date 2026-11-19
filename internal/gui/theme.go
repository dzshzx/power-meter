// Package gui draws the meter window, its settings popover and the tray icon
// with mygo's native UI, in the COUI 17 (ColorOS 17) design language:
// rounded cards on a grey or black window, COUI's label opacities, blue
// primary and semantic hues. It only shows what a meter.View says; display
// rules live in package meter. Geometry is in DIPs, which mygo scales to the
// window's DPI.
package gui

import (
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/egoist/mygo/ui"
)

// palette holds the COUI 17 colour tokens of one appearance. The window
// follows the system's light or dark appearance, as ColorOS does.
type palette struct {
	dark       bool
	background ui.Color // the window, under the cards
	card       ui.Color
	popup      ui.Color // popovers and menus above the cards
	label1     ui.Color // primary text
	label1v    ui.Color // primary text, a step quieter: estimated figures
	label2     ui.Color // secondary text
	label3     ui.Color // tertiary text and placeholders
	label4     ui.Color // quaternary: tracks and faint strokes
	divider    ui.Color
	hover      ui.Color
	press      ui.Color
	primary    ui.Color
	green      ui.Color
	orange     ui.Color
	red        ui.Color
	segment    ui.Color // the segmented control's track
	indicator  ui.Color // its selected segment
	switchOff  ui.Color
	button     ui.Color // a secondary button's fill
	shadow     ui.Color
}

func argb(v uint32) ui.Color {
	return ui.Color{A: uint8(v >> 24), R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}
}

var lightPalette = palette{
	background: argb(0xFFF0F1F2),
	card:       argb(0xFFFFFFFF),
	popup:      argb(0xFFFFFFFF),
	label1:     argb(0xE6000000),
	label1v:    argb(0xB8000000),
	label2:     argb(0x8A000000),
	label3:     argb(0x42000000),
	label4:     argb(0x14000000),
	divider:    argb(0x1F000000),
	hover:      argb(0x0F000000),
	press:      argb(0x1F000000),
	primary:    argb(0xFF0080FF),
	green:      argb(0xFF00BD13),
	orange:     argb(0xFFFF7700),
	red:        argb(0xFFDB382C),
	segment:    argb(0x0F000000),
	indicator:  argb(0xFFFFFFFF),
	switchOff:  argb(0x29000000),
	button:     argb(0xFFEBEBEB),
	shadow:     argb(0x1A000000),
}

var darkPalette = palette{
	dark:       true,
	background: argb(0xFF000000),
	card:       argb(0xFF1E1E1E),
	popup:      argb(0xFF333333),
	label1:     argb(0xE6FFFFFF),
	label1v:    argb(0xB8FFFFFF),
	label2:     argb(0x8AFFFFFF),
	label3:     argb(0x4DFFFFFF),
	label4:     argb(0x26FFFFFF),
	divider:    argb(0x1FFFFFFF),
	hover:      argb(0x1AFFFFFF),
	press:      argb(0x33FFFFFF),
	primary:    argb(0xFF1A8CFF),
	green:      argb(0xFF24B232),
	orange:     argb(0xFFF08222),
	red:        argb(0xFFFF6C61),
	segment:    argb(0x1FFFFFFF),
	indicator:  argb(0x33FFFFFF),
	switchOff:  argb(0x40FFFFFF),
	button:     argb(0x1FFFFFFF),
	shadow:     argb(0x66000000),
}

func paletteFor(dark bool) *palette {
	if dark {
		return &darkPalette
	}
	return &lightPalette
}

// The tray tile keeps solid colours dark enough for white figures at 16 px
// on either taskbar.
var (
	trayCharging    = argb(0xFF059669)
	trayDischarging = argb(0xFFD97706)
	trayNeutral     = argb(0xFF6B7280)
	appTile         = argb(0xFF059669)
	white           = argb(0xFFFFFFFF)
)

// Fonts: Segoe UI Variable where Windows 11 has it, Microsoft YaHei UI for
// Chinese, and tabular figures for numbers that change every second.
const (
	textFont   = "Segoe UI Variable Text, Microsoft YaHei UI, Segoe UI"
	figureFont = "Segoe UI Variable Display, Segoe UI Variable Text, Segoe UI"
	trayFont   = "Segoe UI"
	tabular    = "tnum"
)

// Metrics, after COUI 17's type scale and corner radii, tightened for a
// desktop window.
const (
	windowWidth   = 380
	margin        = 12 // the window's edge and the gap between cards
	cardPadding   = 16
	cardRadius    = 20
	titleBar      = 48
	segmentHeight = 36
	heroHeight    = 186
	trendHeight   = 196
	rowHeight     = 44
	footerHeight  = 40
	settingsWidth = 288

	captionSize = 12 // COUI BodyXS
	bodySize    = 14 // COUI BodyM
	titleSize   = 15
	statSize    = 18
	heroSize    = 44
	axisSize    = 10 // COUI Caption
)

// accent is the supply state's hue: green while charging, orange while the
// battery discharges, red for an error and the primary label when idle.
func (p *palette) accent(a power.Accent) ui.Color {
	switch a {
	case power.AccentCharging:
		return p.green
	case power.AccentDischarging:
		return p.orange
	case power.AccentError:
		return p.red
	default:
		return p.label1
	}
}

// chartAccent is the trend line's hue; an idle supply draws in the primary
// blue rather than a grey line.
func (p *palette) chartAccent(a power.Accent) ui.Color {
	if a == power.AccentIdle {
		return p.primary
	}
	return p.accent(a)
}

func (p *palette) tone(t meter.Tone, a power.Accent) ui.Color {
	switch t {
	case meter.ToneMuted:
		return p.label2
	case meter.ToneUnavailable:
		return p.label3
	case meter.ToneNumericMuted:
		return p.label1v
	case meter.ToneAccent:
		return p.accent(a)
	default:
		return p.label1
	}
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
