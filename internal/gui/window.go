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
	noticeHeight       = 60
	noticeActionHeight = 28
	rowsCount          = 3
)

// Height is the window's content height for m: the notice card adds to it.
func Height(m *Model) int {
	h := titleBar + segmentHeight + margin + heroHeight + margin + trendHeight + margin +
		rowsCount*rowHeight + footerHeight
	if m.notice() != "" {
		h += margin + noticeHeight
		if m.offersElevation() {
			h += noticeActionHeight
		}
	}
	return h
}

// Width is the window's content width.
const Width = windowWidth

// Build builds the window for m; the app gives it to mygo as the window's
// view.
func Build(c *ui.Context, m *Model) {
	p := paletteFor(c.Theme().Dark)
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted, t.Accent, t.Font = p.background, p.label1, p.label2, p.primary, textFont
	t.Inverse, t.InverseText = p.popup, p.label1
	c.SetTheme(&t)
	ui.Column(c).Fill().Background(p.background).Font(textFont).Children(func() {
		titleBarView(c, p, m)
		ui.Scroll(c.Key("page")).Grow(1).Children(func() {
			ui.Column(c).Padding(0, margin, 0, margin).Gap(margin).Children(func() {
				segments(c, p, m)
				hero(c, p, m.View)
				trend(c, p, m.View)
				rows(c, p, m)
				if text := m.notice(); text != "" {
					notice(c, p, m, text)
				}
			})
		})
		footer(c, p, m)
	})
}

func titleBarView(c *ui.Context, p *palette, m *Model) {
	ui.Row(c).Height(titleBar).Padding(0, 8, 0, 14).AlignItems(ui.Center).Gap(4).DragWindow().Children(func() {
		appTileView(c, 20)
		ui.Text(c, i18n.AppName()).FontSize(titleSize).FontWeight(500).TextColor(p.label1).SingleLine().Margin(0, 0, 0, 6)
		ui.Spacer(c)
		pin := iconButton(c, p, iconPin, i18n.Get("置顶"), 32, false).Selected(m.Pinned)
		if m.Pinned {
			pin.TextColor(p.primary)
		}
		if pin.Clicked() && m.Actions.SetPinned != nil {
			m.Actions.SetPinned(!m.Pinned)
		}
		if iconButton(c, p, iconMinus, i18n.Get("最小化"), 32, false).Clicked() && m.Actions.Minimize != nil {
			m.Actions.Minimize()
		}
		if iconButton(c, p, iconX, i18n.Get("关闭"), 32, true).Clicked() && m.Actions.Close != nil {
			m.Actions.Close()
		}
	})
}

// appTileView is the app's mark, as its icon.
func appTileView(c *ui.Context, size float32) ui.Element {
	return ui.Column(c).Size(size, size).Draw(drawAppMark)
}

// segments is COUI's segmented control choosing the headline's boundary.
func segments(c *ui.Context, p *palette, m *Model) {
	selected := int(m.Mode)
	parts := ui.SegmentedBase(c.Key("mode"), &selected, 2)
	parts.Track.Height(segmentHeight).Padding(3).Radius(segmentHeight / 2).Background(p.segment).Label(i18n.Get("功率显示口径")).
		Children(func() {
			for i, key := range []string{"整机功率", "电池端"} {
				seg := parts.Segment(i).Grow(1).Radius((segmentHeight - 6) / 2).Justify(ui.Center).AlignItems(ui.Center).FocusRing(false)
				switch {
				case i == selected:
					seg.Background(p.indicator).Shadow(0, 1, 3, 0, p.shadow)
				case seg.Hovered():
					seg.Background(p.hover)
				}
				seg.Children(func() {
					ui.Text(c, i18n.Get(key)).FontSize(bodySize).FontWeight(500).TextColor(p.label1).SingleLine()
				})
			}
		})
	if next := meter.Mode(selected); next != m.Mode && m.Actions.SetMode != nil {
		m.Actions.SetMode(next)
	}
}

func hero(c *ui.Context, p *palette, v *meter.View) {
	accent := p.accent(v.Accent)
	card(c, p).Height(heroHeight).Padding(cardPadding).Children(func() {
		ui.Text(c, v.Title.Text).FontSize(captionSize + 1).TextColor(p.label2).SingleLine().Tooltip(v.Title.Tooltip)
		figureText(c, p, v.Headline.Text, heroSize, p.tone(v.Headline.Tone, v.Accent), 600).Tooltip(v.Headline.Tooltip)
		ui.Row(c).Height(22).Children(func() {
			if v.Supply.Text != "" {
				stateChip(c, p, v, accent)
			}
		})
		ui.Spacer(c)
		ui.Row(c).Children(func() {
			metric(c, p, v.VoltageCaption, v.Voltage, nil)
			metric(c, p, v.CurrentCaption, v.Current, nil)
			metric(c, p, i18n.Get("电池电量"), v.Percentage, func() {
				batteryGlyph(c, p, v)
			})
		})
	})
}

// stateChip is the supply state: its glyph and text on a tint of its hue.
func stateChip(c *ui.Context, p *palette, v *meter.View, accent ui.Color) {
	chip := ui.Row(c).AlignItems(ui.Center).Gap(4).Padding(2, 10, 2, 8).Radius(11).Height(22).
		Background(withAlpha(accent, 0.12)).TextColor(accent).Tooltip(v.Supply.Tooltip)
	if v.Accent == power.AccentIdle {
		chip.Background(p.segment).TextColor(p.label2)
	}
	chip.Children(func() {
		if body := supplyIconBody(v.SupplyIcon); body != "" {
			ui.Icon(c, icon(body, 2.2)).Size(14, 14)
		}
		ui.Text(c, v.Supply.Text).FontSize(captionSize).FontWeight(500).SingleLine()
	})
}

func metric(c *ui.Context, p *palette, caption, value string, after func()) {
	ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
		ui.Text(c, caption).FontSize(captionSize).TextColor(p.label2).SingleLine()
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			figureText(c, p, value, 16, p.label1, 400)
			if after != nil {
				after()
			}
		})
	})
}

// batteryGlyph is a small battery filled to the charge level in the state's
// hue.
func batteryGlyph(c *ui.Context, p *palette, v *meter.View) {
	fill := p.tone(v.BatteryTone, v.Accent)
	if fill == p.label1 {
		fill = p.green
	}
	level := v.BatteryLevel
	ui.Column(c).Size(24, 12).Draw(func(pt *ui.Painter, r ui.Rect) {
		body := ui.Rect{X: r.X, Y: r.Y, W: r.W - 2.5, H: r.H}
		pt.Stroke(body, p.label3, 3, 1.2)
		pt.Fill(ui.Rect{X: r.X + r.W - 1.8, Y: r.Y + r.H*0.32, W: 1.8, H: r.H * 0.36}, p.label3, 0.9)
		if level >= 0 {
			inner := ui.Rect{X: body.X + 2, Y: body.Y + 2, W: (body.W - 4) * float32(min(level, 100)) / 100, H: body.H - 4}
			if inner.W > 0 {
				pt.Fill(inner, fill, 1.5)
			}
		}
	})
}

func trend(c *ui.Context, p *palette, v *meter.View) {
	view := v.Trend
	ch := chart{p: p, view: &view, accent: p.chartAccent(v.Accent)}
	card(c, p).Height(trendHeight).Children(func() {
		ui.Column(c).Height(124).Margin(12, 14, 0, 14).Tooltip(view.Tooltip).Draw(ch.draw)
		ui.Column(c).Margin(8, cardPadding, 0, cardPadding).Children(func() { hairline(c, p) })
		ui.Row(c).Grow(1).Padding(0, cardPadding, 0, cardPadding).AlignItems(ui.Center).Children(func() {
			statistic(c, p, v.AverageCaption, v.AverageValue, v.AverageTone, v.AverageTooltip, v)
			ui.Column(c).Width(1).Height(28).Background(p.divider).Margin(0, 12, 0, 12)
			statistic(c, p, v.PeakCaption, v.PeakValue, v.PeakTone, v.PeakTooltip, v)
		})
	})
}

func statistic(c *ui.Context, p *palette, caption, value string, tone meter.Tone, tooltip string, v *meter.View) {
	ui.Column(c).Grow(1).Basis(0).Gap(2).Tooltip(tooltip).Children(func() {
		ui.Text(c, caption).FontSize(captionSize).TextColor(p.label2).SingleLine()
		figureText(c, p, value, statSize, p.tone(tone, v.Accent), 500)
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
	card(c, p).Children(func() {
		first := true
		for i, row := range v.Rows {
			if i == hidden {
				continue
			}
			if !first {
				ui.Column(c).Padding(0, cardPadding, 0, cardPadding).Children(func() { hairline(c, p) })
			}
			first = false
			ui.Row(c).Height(rowHeight-1).Padding(0, cardPadding, 0, cardPadding).AlignItems(ui.Center).Children(func() {
				ui.Text(c, names[i]).FontSize(bodySize).TextColor(p.label1).SingleLine().Shrink(1).Tooltip(tips[i])
				ui.Spacer(c)
				figureText(c, p, row.Text, 15, p.tone(row.Tone, v.Accent), 400).Tooltip(row.Tooltip)
			})
		}
	})
}

// notice is COUI's tips card: the reasons a reading or a setting is
// missing, and the restart that would help.
func notice(c *ui.Context, p *palette, m *Model, text string) {
	h := float32(noticeHeight)
	if m.offersElevation() {
		h += noticeActionHeight
	}
	card(c, p).Height(h).Padding(12, cardPadding, 12, 14).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Icon(c, icon(iconTriangleAlert, 2.2)).Size(16, 16).TextColor(p.red).Margin(1, 0, 0, 0)
			ui.Text(c, text).FontSize(captionSize + 1).LineHeight(1.3).TextColor(p.label1).MaxLines(2).Ellipsis("…").
				Grow(1).Shrink(1).Tooltip(text)
		})
		ui.Spacer(c)
		if m.offersElevation() {
			link := ui.ButtonBase(c).Height(noticeActionHeight-4).Padding(0, 10, 0, 10).Radius(12).AlignSelf(ui.End).
				TextColor(p.primary).FocusRing(false)
			if link.Hovered() {
				link.Background(withAlpha(p.primary, 0.1))
			}
			link.Children(func() {
				ui.Icon(c, icon(iconShield, 2.2)).Size(14, 14).Margin(0, 6, 0, 0)
				ui.Text(c, i18n.Get("以管理员身份重新启动")).FontSize(captionSize + 1).FontWeight(500).SingleLine()
			})
			if link.Clicked() && m.Actions.Elevate != nil {
				m.Actions.Elevate()
			}
		}
	})
}

func footer(c *ui.Context, p *palette, m *Model) {
	ui.Row(c).Height(footerHeight).Padding(0, 8, 0, margin+4).AlignItems(ui.Center).Gap(6).Children(func() {
		if m.View.Updated != "" {
			ui.Text(c, i18n.Get("更新于")).FontSize(captionSize).TextColor(p.label3).SingleLine()
			ui.Text(c, m.View.Updated).FontSize(captionSize).TextColor(p.label2).FontFeatures(tabular).SingleLine()
		}
		ui.Spacer(c)
		gear := iconButton(c, p, iconSettings, i18n.Get("设置"), 32, false)
		if gear.Clicked() {
			m.SettingsOpen = !m.SettingsOpen
		}
		settings(c, p, m, gear)
	})
}
