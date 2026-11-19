package gui

import (
	"strings"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/egoist/mygo/ui"
)

// Autostart is the settings switch: whether startup is on, whether the
// switch can change it now, and why not.
type Autostart struct {
	Checked bool
	Enabled bool
	Tooltip string
}

// Actions are the window's requests. The app carries them out and updates
// the model; a nil action is ignored.
type Actions struct {
	SetMode      func(meter.Mode)
	SetPinned    func(bool)
	Minimize     func()
	Close        func()
	SetAutostart func(bool)
	SetLanguage  func(string)
	Elevate      func()
}

// Model is what the window shows: the meter's view and the app's state
// around it.
type Model struct {
	View     *meter.View
	Mode     meter.Mode
	Pinned   bool
	Elevated bool
	// Language is the saved preference: i18n.System, Chinese or English.
	Language  string
	Autostart Autostart
	// ElevationNotice says why the window runs without elevation, and
	// StartupNotice reports on logon startup; empty when there is nothing
	// to say. Both show above the view's diagnostics.
	ElevationNotice string
	StartupNotice   string
	Actions         Actions
	// SettingsOpen shows the settings popover.
	SettingsOpen bool
}

// notice is the text of the notice card, one line per message; empty hides
// the card.
func (m *Model) notice() string {
	var lines []string
	for _, line := range append([]string{m.ElevationNotice, m.StartupNotice}, m.View.Diagnostics...) {
		if line != "" {
			lines = append(lines, i18n.Diagnostic(line))
		}
	}
	return strings.Join(lines, "\n")
}

// offersElevation: restarting as administrator would help, and this window
// is not elevated.
func (m *Model) offersElevation() bool {
	return !m.Elevated && (m.View.ElevationHelps || m.ElevationNotice != "")
}

// Fixed section heights: the window is sized from them, not measured.
const (
	noticeHeight       = 56 // two lines of text
	noticeActionHeight = 24
	rowsCount          = 3
)

// panelHeight is the panel's height for m: the notice group adds to it.
func panelHeight(m *Model) int {
	h := titleBar + heroHeight + inset + rowsCount*rowHeight + footerHeight
	if m.notice() != "" {
		h += inset + noticeHeight
		if m.offersElevation() {
			h += noticeActionHeight
		}
	}
	return h
}

// Height is the window's content height for m: the panel and the room for
// its shadow.
func Height(m *Model) int { return shadowTop + panelHeight(m) + shadowBottom }

// Width is the window's content width: the panel and the room for its
// shadow.
const Width = shadowSide + panelWidth + shadowSide

// Panel is the panel's rectangle in the window, in DIPs: the part that
// takes the mouse. The rest is the shadow, which lets clicks through.
func Panel(m *Model) (x, y, w, h int) {
	return shadowSide, shadowTop, panelWidth, panelHeight(m)
}

// Build builds the window for m; the app gives it to mygo as the window's
// view. The window is transparent: the panel rounds its own corners and
// draws its own shadow.
func Build(c *ui.Context, m *Model) {
	ensureFonts()
	p := paletteFor(c.Theme().Dark)
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted, t.Accent, t.Font = ui.Transparent, p.ink, p.ink2, p.accent, textFont
	t.Inverse, t.InverseText = p.popup, p.ink
	c.SetTheme(&t)
	ui.Column(c).Fill().Background(ui.Transparent).Padding(shadowTop, shadowSide, shadowBottom, shadowSide).Children(func() {
		ui.Column(c).Grow(1).Background(p.page).Radius(panelRadius).Border(1, p.border).
			Shadow(0, 8, 22, 0, p.shadow).Font(textFont).TextColor(p.ink).Children(func() {
			titleBarView(c, p, m)
			ui.Scroll(c.Key("page")).Grow(1).Children(func() {
				ui.Column(c).Padding(0, inset, 0, inset).Gap(inset).Children(func() {
					hero(c, p, m)
					rows(c, p, m)
					if text := m.notice(); text != "" {
						notice(c, p, m, text)
					}
				})
			})
			footer(c, p, m)
		})
	})
}

func titleBarView(c *ui.Context, p *palette, m *Model) {
	ui.Row(c).Height(titleBar).Padding(0, 12, 0, 22).AlignItems(ui.Center).Gap(4).DragWindow().Children(func() {
		ui.Text(c, i18n.AppName()).Font(mediumFont).FontSize(titleSize).TextColor(p.ink).SingleLine()
		ui.Spacer(c)
		pin := iconButton(c, p, iconPin, i18n.Get("置顶"), m.Pinned, false).Checked(m.Pinned)
		if pin.Clicked() && m.Actions.SetPinned != nil {
			m.Actions.SetPinned(!m.Pinned)
		}
		if iconButton(c, p, iconMinus, i18n.Get("最小化"), false, false).Clicked() && m.Actions.Minimize != nil {
			m.Actions.Minimize()
		}
		if iconButton(c, p, iconX, i18n.Get("关闭"), false, true).Clicked() && m.Actions.Close != nil {
			m.Actions.Close()
		}
	})
}

// segments is COUI's capsule segmented control choosing the headline's
// boundary.
func segments(c *ui.Context, p *palette, m *Model) {
	const h = 28
	selected := int(m.Mode)
	parts := ui.SegmentedBase(c.Key("mode"), &selected, 2)
	parts.Track.Height(h).Width(144).Padding(2).Radius(h / 2).Background(p.track).Label(i18n.Get("功率显示口径")).
		Children(func() {
			for i, key := range []string{"整机功率", "电池端"} {
				seg := parts.Segment(i).Grow(1).Basis(0).Radius(h/2 - 2).Justify(ui.Center).AlignItems(ui.Center).FocusRing(false)
				font, color := textFont, p.ink2
				switch {
				case i == selected:
					seg.Background(p.indicator)
					if !p.dark {
						seg.Shadow(0, 1, 3, 0, argb(0x1A000000))
					}
					font, color = mediumFont, p.ink
				case seg.Hovered():
					color = p.ink
				}
				seg.Children(func() {
					ui.Text(c, i18n.Get(key)).Font(font).FontSize(13).TextColor(color).SingleLine()
				})
			}
		})
	if next := meter.Mode(selected); next != m.Mode && m.Actions.SetMode != nil {
		m.Actions.SetMode(next)
	}
}

// hero is the first group: what the headline measures and the boundary
// switch, the headline, the supply state with the battery's figures, and
// the trend with its statistics.
func hero(c *ui.Context, p *palette, m *Model) {
	v := m.View
	group(c, p).Height(heroHeight).Padding(14, groupPadding, 14, groupPadding).Children(func() {
		ui.Row(c).MinHeight(28).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, v.Title.Text).Font(textFont).FontSize(captionSize).LineHeight(1.3).TextColor(p.ink2).
				MaxLines(2).Ellipsis("…").Grow(1).Shrink(1).Tooltip(v.Title.Tooltip)
			segments(c, p, m)
		})
		ui.Column(c).Margin(6, 0, 0, 0).Children(func() {
			figureText(c, p, v.Headline.Text, headSize, 18, figureFont, p.tone(v.Headline.Tone, v.Accent)).
				Tooltip(v.Headline.Tooltip)
		})
		supply(c, p, v)
		battery := strings.Join([]string{v.Voltage, v.Current, v.Percentage}, " · ")
		label(c, battery, captionSize, p.ink2).FontFeatures(tabular).Margin(2, 0, 0, 20).
			Tooltip(v.VoltageCaption + " · " + v.CurrentCaption + " · " + i18n.Get("电池电量"))
		view := v.Trend
		ui.Column(c).Height(chartHeight).Margin(14, 0, 0, 0).Tooltip(view.Tooltip).
			Draw(chart{p: p, view: &view}.draw)
		ui.Spacer(c)
		ui.Row(c).Children(func() {
			statistic(c, p, v.AverageCaption, v.AverageValue, v.AverageTone, v.AverageTooltip)
			statistic(c, p, v.PeakCaption, v.PeakValue, v.PeakTone, v.PeakTooltip)
		})
	})
}

// supply is the supply state: its glyph and words, quiet; red only for a
// sensor error.
func supply(c *ui.Context, p *palette, v *meter.View) {
	color := p.ink2
	if v.Accent == power.AccentError {
		color = p.danger
	}
	ui.Row(c).Height(20).Margin(2, 0, 0, 0).AlignItems(ui.Center).Gap(6).Tooltip(v.Supply.Tooltip).Children(func() {
		if body := supplyIconBody(v.SupplyIcon); body != "" {
			ui.Icon(c, icon(body, 1.75)).Size(14, 14).TextColor(color)
		}
		label(c, v.Supply.Text, captionSize, color).Shrink(1).Ellipsis("…")
	})
}

// statistic is a caption over its figure, half the group's width.
func statistic(c *ui.Context, p *palette, caption, value string, tone meter.Tone, tooltip string) {
	ui.Column(c).Grow(1).Basis(0).Gap(2).Tooltip(tooltip).Children(func() {
		label(c, caption, captionSize, p.ink2).Ellipsis("…")
		figureText(c, p, value, 15, captionSize, mediumFont, p.tone(tone, 0))
	})
}

// rows lists the supporting figures, without the one the headline shows.
func rows(c *ui.Context, p *palette, m *Model) {
	v := m.View
	names := [4]string{i18n.Get("电池端净功率"), i18n.Get("CPU 包功率"), i18n.Get("平台功率"), v.WholeCaption}
	tips := [4]string{"", "", "", v.WholeCaptionTooltip}
	hidden := meter.WholeSystemRow
	if m.Mode == meter.Battery {
		hidden = meter.BatteryRow
	}
	group(c, p).Padding(0, groupPadding, 0, groupPadding).Children(func() {
		first := true
		for i, row := range v.Rows {
			if i == hidden {
				continue
			}
			if !first {
				hairline(c, p)
			}
			first = false
			ui.Row(c).Height(rowHeight - 1).AlignItems(ui.Center).Gap(12).Children(func() {
				label(c, names[i], bodySize, p.ink).Shrink(1).Ellipsis("…").Tooltip(tips[i])
				ui.Spacer(c)
				figureText(c, p, row.Text, bodySize, captionSize, mediumFont, p.tone(row.Tone, v.Accent)).Tooltip(row.Tooltip)
			})
		}
	})
}

// notice is COUI's tips card: why a reading or a setting is missing, and
// the restart that would help.
func notice(c *ui.Context, p *palette, m *Model, text string) {
	h := float32(noticeHeight)
	if m.offersElevation() {
		h += noticeActionHeight
	}
	group(c, p).Height(h).Padding(10, groupPadding, 10, 14).Justify(ui.Center).Gap(4).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Icon(c, icon(iconTriangleAlert, 1.75)).Size(14, 14).TextColor(p.warn).Margin(2, 0, 0, 0)
			ui.Text(c, text).Font(textFont).FontSize(captionSize).LineHeight(1.5).TextColor(p.ink2).MaxLines(2).
				Ellipsis("…").Grow(1).Shrink(1).Tooltip(text)
		})
		if m.offersElevation() {
			link := ui.ButtonBase(c).Height(20).Margin(0, 0, 0, 22).AlignSelf(ui.Start).TextColor(p.accent).FocusRing(false)
			if link.Hovered() {
				link.TextColor(withAlpha(p.accent, 0.75))
			}
			link.Children(func() {
				ui.Text(c, i18n.Get("以管理员身份重新启动")).Font(mediumFont).FontSize(captionSize).SingleLine()
			})
			if link.Clicked() && m.Actions.Elevate != nil {
				m.Actions.Elevate()
			}
		}
	})
}

func footer(c *ui.Context, p *palette, m *Model) {
	ui.Row(c).Height(footerHeight).Padding(0, 12, 2, 22).AlignItems(ui.Center).Gap(6).Children(func() {
		if m.View.Updated != "" {
			label(c, i18n.Get("更新于"), captionSize, p.ink3)
			label(c, m.View.Updated, captionSize, p.ink3).FontFeatures(tabular)
		}
		ui.Spacer(c)
		gear := iconButton(c, p, iconSettings, i18n.Get("设置"), m.SettingsOpen, false)
		if gear.Clicked() {
			m.SettingsOpen = !m.SettingsOpen
		}
		settings(c, p, m, gear)
	})
}
