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
	// StartupConfirmed: StartupNotice confirms a startup change that took
	// effect, rather than reporting a problem.
	StartupConfirmed bool
	Actions          Actions
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

// confirmsOnly: the notice card holds nothing but the confirmation of a
// startup change, so it shows no warning.
func (m *Model) confirmsOnly() bool {
	return m.StartupConfirmed && m.ElevationNotice == "" && len(m.View.Diagnostics) == 0
}

// offersElevation: restarting as administrator would help, and this window
// is not elevated.
func (m *Model) offersElevation() bool {
	return !m.Elevated && (m.View.ElevationHelps || m.ElevationNotice != "")
}

// Fixed section heights: the window is sized from them and from which
// lines fit side by side (heroLayout), not measured after layout.
const (
	noticeHeight       = 58 // two lines of text
	noticeActionHeight = 24
	rowsCount          = 3
	stateHeight        = 20
	stateGap           = 16 // between the supply state and the battery's figures
	statsHeight        = 20
	statsGap           = 8 // between the trend's time labels and the statistics
)

// heroLayout says which pairs of lines in the hero group fit side by side
// at the group's width; English text is longer.
type heroLayout struct {
	// stackBattery puts the battery's figures under the supply state.
	stackBattery bool
	// stackStats puts each statistic's figure under its caption.
	stackStats bool
}

// heroWidth is the width inside the hero group.
const heroWidth = panelWidth - 2*inset - 2*groupPadding

func layoutFor(v *meter.View) heroLayout {
	ensureFonts()
	supplyW := 14 + 6 + textWidth(v.Supply.Text, textFont, captionSize)
	batteryW := textWidth(batteryLine(v), textFont, captionSize)
	half := float32(heroWidth-16) / 2
	stat := func(caption, value string) float32 {
		f := splitFigure(value)
		return textWidth(caption, textFont, captionSize) + 8 + textWidth(f.prefix, textFont, captionSize) +
			textWidth(f.number, mediumFont, 15) + textWidth(f.unit, textFont, captionSize) + 8
	}
	return heroLayout{
		stackBattery: supplyW+stateGap+batteryW > heroWidth,
		stackStats:   stat(v.AverageCaption, v.AverageValue) > half || stat(v.PeakCaption, v.PeakValue) > half,
	}
}

// textWidth is the width of s on one line.
func textWidth(s, family string, size float32) float32 {
	var w float32
	for _, g := range ui.Shape(s, ui.Font{Family: family, Size: size, Features: tabular}) {
		w = max(w, g.X+g.Advance)
	}
	return w
}

func batteryLine(v *meter.View) string {
	return strings.Join([]string{v.Voltage, v.Current, v.Percentage}, " · ")
}

// heroHeight is the hero group's height for v.
func heroHeight(v *meter.View) int {
	l := layoutFor(v)
	const headline = 51 // headSize at a line height of 1.15
	h := 16 + 24 + 8 + headline + 4 + stateHeight + 14 + chartHeight + 10 + statsGap + statsHeight + 16
	if l.stackBattery {
		h += 18
	}
	if l.stackStats {
		h += 18
	}
	return h
}

// panelHeight is the panel's height for m: the notice group adds to it.
func panelHeight(m *Model) int {
	h := titleBar + heroHeight(m.View) + inset + rowsCount*rowHeight + footerHeight
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
// view. The window is transparent: the panel draws its own continuous
// corners, outline and shadow.
func Build(c *ui.Context, m *Model) {
	ensureFonts()
	p := paletteFor(c.Theme().Dark)
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted, t.Accent, t.Font = ui.Transparent, p.ink, p.ink2, p.accent, textFont
	t.Inverse, t.InverseText = p.popup, p.ink
	c.SetTheme(&t)
	ui.Column(c).Fill().Background(ui.Transparent).Padding(shadowTop, shadowSide, shadowBottom, shadowSide).Children(func() {
		ui.Column(c).Grow(1).Draw(panelShape(p)).Font(textFont).TextColor(p.ink).Children(func() {
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

// panelShape draws the panel: its shadow, its surface and a hairline
// outline, all with continuous corners.
func panelShape(p *palette) func(*ui.Painter, ui.Rect) {
	return func(pt *ui.Painter, r ui.Rect) {
		pt.Shadow(r, panelRadius, 0, 8, 22, 0, p.shadow)
		fillSquircle(pt, r, panelRadius, p.page)
		strokeSquircle(pt, r, panelRadius, 1/pt.Scale(), p.border)
	}
}

func titleBarView(c *ui.Context, p *palette, m *Model) {
	ui.Row(c).Height(titleBar).Padding(0, 14, 0, 22).AlignItems(ui.Center).Gap(4).DragWindow().Children(func() {
		ui.Text(c, i18n.AppName()).Font(figureFont).FontSize(titleSize).TextColor(p.ink).SingleLine().
			Margin(2*titleDrop, 0, 0, 0)
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

// tabs choose the headline's boundary: ColorOS page tabs, the chosen one
// in ink and the other quieter, in the same face.
func tabs(c *ui.Context, p *palette, m *Model) {
	selected := int(m.Mode)
	parts := ui.SegmentedBase(c.Key("mode"), &selected, 2)
	parts.Track.Gap(16).AlignItems(ui.Center).Label(i18n.Get("功率显示口径")).Children(func() {
		for i, key := range []string{"整机功率", "电池端"} {
			tab := parts.Segment(i).FocusRing(false)
			color := p.ink3
			switch {
			case i == selected:
				color = p.ink
			case tab.Hovered():
				color = p.ink2
			}
			tab.Children(func() {
				ui.Text(c, i18n.Get(key)).Font(mediumFont).FontSize(tabSize).TextColor(color).SingleLine()
			})
		}
	})
	if next := meter.Mode(selected); next != m.Mode && m.Actions.SetMode != nil {
		m.Actions.SetMode(next)
	}
}

// hero is the first group: the boundary tabs and what the headline
// measures, the headline, the supply state beside the battery's figures,
// the trend and its statistics.
func hero(c *ui.Context, p *palette, m *Model) {
	v := m.View
	l := layoutFor(v)
	group(c, p).Height(float32(heroHeight(v))).Padding(16, groupPadding, 16, groupPadding).Children(func() {
		ui.Row(c).Height(24).AlignItems(ui.Center).Gap(12).Children(func() {
			tabs(c, p, m)
			ui.Spacer(c)
			label(c, v.Title.Text, captionSize, p.ink2).Shrink(1).Ellipsis("…").Tooltip(v.Title.Tooltip)
		})
		ui.Column(c).Margin(8, 0, 0, 0).Children(func() {
			figureText(c, p, v.Headline.Text, headSize, 18, figureFont, p.tone(v.Headline.Tone, v.Accent)).
				Tooltip(v.Headline.Tooltip)
		})
		battery := func() ui.Element {
			return label(c, batteryLine(v), captionSize, p.ink2).FontFeatures(tabular).
				Tooltip(v.VoltageCaption + " · " + v.CurrentCaption + " · " + i18n.Get("电池电量"))
		}
		ui.Row(c).Height(stateHeight).Margin(4, 0, 0, 0).AlignItems(ui.Center).Gap(stateGap / 2).Children(func() {
			supply(c, p, v)
			ui.Spacer(c)
			if !l.stackBattery {
				battery()
			}
		})
		if l.stackBattery {
			battery().Height(18).Margin(0, 0, 0, 20)
		}
		view := v.Trend
		ui.Column(c).Height(chartHeight).Margin(14, 0, 0, 0).Tooltip(view.Tooltip).
			Draw(chart{p: p, view: &view}.draw)
		ui.Spacer(c)
		ui.Row(c).Margin(statsGap, 0, 0, 0).Gap(16).Children(func() {
			statistic(c, p, v.AverageCaption, v.AverageValue, v.AverageTone, v.AverageTooltip, l.stackStats)
			statistic(c, p, v.PeakCaption, v.PeakValue, v.PeakTone, v.PeakTooltip, l.stackStats)
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
	ui.Row(c).AlignItems(ui.Center).Gap(6).Shrink(1).Tooltip(v.Supply.Tooltip).Children(func() {
		if body := supplyIconBody(v.SupplyIcon); body != "" {
			ui.Icon(c, icon(body, 1.75)).Size(14, 14).TextColor(color)
		}
		label(c, v.Supply.Text, captionSize, color).Shrink(1).Ellipsis("…")
	})
}

// statistic is a caption and its figure, half the group's width: on one
// baseline, or the figure under the caption where they do not fit.
func statistic(c *ui.Context, p *palette, caption, value string, tone meter.Tone, tooltip string, stacked bool) {
	var box ui.Element
	if stacked {
		box = ui.Column(c).AlignItems(ui.Start).Gap(2)
	} else {
		box = ui.Row(c).AlignItems(ui.Baseline).Gap(8)
	}
	box.Grow(1).Basis(0).Tooltip(tooltip).Children(func() {
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
	glyph, tone := iconTriangleAlert, p.warn
	if m.confirmsOnly() {
		glyph, tone = iconCircleCheck, p.ink2
	}
	group(c, p).Height(h).Padding(10, groupPadding, 10, 16).Justify(ui.Center).Gap(4).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Icon(c, icon(glyph, 1.75)).Size(14, 14).TextColor(tone).Margin(2, 0, 0, 0)
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
	ui.Row(c).Height(footerHeight).Padding(0, 14, 0, 22).AlignItems(ui.Center).Gap(6).Children(func() {
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
