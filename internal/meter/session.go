package meter

import (
	"math"
	"strconv"
	"time"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/power"
)

// Tone is the colour intent of one piece of text; the window maps it to a
// colour, and ToneAccent takes the view's supply-state accent.
type Tone int

const (
	ToneInk Tone = iota
	// ToneMuted is quieter text: captions and estimated supporting figures.
	ToneMuted
	// ToneNumericMuted is an estimated headline: lighter than ink, legible
	// at size.
	ToneNumericMuted
	ToneAccent
	// ToneUnavailable is a placeholder with no value: N/A and "--".
	ToneUnavailable
)

// SupplyIcon is the pictogram of the supply state.
type SupplyIcon int

const (
	IconNone SupplyIcon = iota
	IconExternalPower
	IconCharging
	IconBattery
	IconAlert
)

// Readout is one piece of text with its tone and tooltip.
type Readout struct {
	Text    string
	Tone    Tone
	Tooltip string
}

// TrendView is the thirty-minute trend of the selected boundary, already
// labelled; the chart only picks pixel columns and draws.
type TrendView struct {
	Series *TrendSeries
	// AxisLabels are for the left edge, one third, two thirds and the right
	// edge.
	AxisLabels [4]string
	// Recorded is the recorded duration while the window fills; empty once
	// full.
	Recorded string
	// PeakLabel is the largest-magnitude reading, empty without data.
	PeakLabel string
	Empty     string
	Tooltip   string
}

// Rows of the supporting figures, in fixed order.
const (
	BatteryRow = iota
	CPUPackageRow
	PlatformRow
	WholeSystemRow
)

// View is everything the window and tray show at one moment, already
// localized. It carries no colours or controls.
type View struct {
	Accent         power.Accent
	Title          Readout
	Headline       Readout
	Supply         Readout
	SupplyIcon     SupplyIcon
	VoltageCaption string
	Voltage        string
	CurrentCaption string
	Current        string
	Percentage     string
	// BatteryLevel is the charge level, or -1 when unknown.
	BatteryLevel   int
	BatteryTone    Tone
	AverageCaption string
	AverageValue   string
	AverageTone    Tone
	AverageTooltip string
	PeakCaption    string
	PeakValue      string
	PeakTone       Tone
	PeakTooltip    string
	// Rows: battery terminal, CPU package, platform, whole system.
	Rows                [4]Readout
	WholeCaption        string
	WholeCaptionTooltip string
	// Diagnostics are measurement reasons only; elevation and startup
	// notices are the window's.
	Diagnostics []string
	// ElevationHelps reports that restarting as administrator would make a
	// missing reading available.
	ElevationHelps bool
	Trend          TrendView
	// TrayGlyph is empty until the first reading: the tray keeps the app
	// icon.
	TrayGlyph   string
	TrayTooltip string
	Updated     string
}

const wholeSystemFormula = "外电：平台功率 + 带符号的电池端净功率，未含转换损耗。\n电池供电：电池端放电功率。CPU 包已包含在平台功率内。"

// Session is the meter's display state: the selected mode, the latest
// snapshot, the statistics history and any sensor failure. Every operation
// returns the complete view, so the headline, rows, statistics and tray
// always come from the same reading.
type Session struct {
	history       History
	trend         Trend
	mode          Mode
	latest        *power.Snapshot
	failed        bool
	failure       string
	timestamp     time.Time
	hasTimestamp  bool
	wholeBoundary power.Boundary
}

// NewSession starts a session showing mode.
func NewSession(mode Mode) *Session {
	return &Session{mode: mode, wholeBoundary: power.EstimatedSystemInput}
}

// Mode is the selected mode.
func (s *Session) Mode() Mode { return s.mode }

// Observe records one reading and appends it to the statistics.
func (s *Session) Observe(snapshot *power.Snapshot) *View {
	s.latest = snapshot
	s.failed, s.failure = false, ""
	s.timestamp, s.hasTimestamp = snapshot.Timestamp, true
	if snapshot.WholeSystem != nil {
		s.wholeBoundary = snapshot.WholeSystem.Boundary
	}
	s.trend.Add(snapshot.ElapsedSeconds, snapshot.WholeSystem, snapshot.BatteryTerminal)
	s.appendLatest()
	return s.Render()
}

// Fail drops the latest reading and the statistics: nothing is known until
// the next capture. The trend keeps its history and breaks the line here.
func (s *Session) Fail(reason string) *View {
	s.latest = nil
	s.failed, s.failure = true, reason
	s.history.Clear()
	s.trend.Break()
	return s.Render()
}

// SetMode starts new statistics for another mode, seeded with the current
// reading. Both trend boundaries are always recorded, so the trend stays.
func (s *Session) SetMode(next Mode) *View {
	if s.mode != next {
		s.mode = next
		s.history.Clear()
	}
	if s.latest != nil && s.history.Len() == 0 {
		s.appendLatest()
	}
	return s.Render()
}

// Render rebuilds the current view without recording anything.
func (s *Session) Render() *View {
	switch {
	case s.latest != nil:
		return s.readingView(s.latest)
	case s.failed:
		return s.failureView()
	default:
		return s.initialView()
	}
}

func (s *Session) appendLatest() {
	reading := s.latest.Battery
	if reading == nil {
		reading = power.NewBatteryReading()
	}
	s.history.Add(s.latest.ElapsedSeconds, s.mode, reading.SupplyState, s.selected(s.latest))
}

func (s *Session) selected(snapshot *power.Snapshot) *power.Sample {
	if s.mode == Battery {
		return snapshot.BatteryTerminal
	}
	return snapshot.WholeSystem
}

func (s *Session) selectedBoundary(selected *power.Sample) power.Boundary {
	if selected == nil {
		return s.wholeBoundary
	}
	return selected.Boundary
}

func (s *Session) readingView(snapshot *power.Snapshot) *View {
	reading := snapshot.Battery
	if reading == nil {
		reading = power.NewBatteryReading()
	}
	profile := reading.Profile()
	selected := s.selected(snapshot)
	headline := watts(selected)
	v := &View{Accent: profile.Accent}
	v.Supply = Readout{Text: i18n.Get(supplyCaptionKey(reading.SupplyState)), Tone: ToneAccent}
	v.SupplyIcon = supplyIconFor(reading.SupplyState)
	v.Headline = Readout{Text: headline, Tone: headlineTone(selected, profile.Accent)}
	if selected != nil {
		if selected.Available {
			v.Headline.Tooltip = i18n.Diagnostic(selected.Source)
		} else {
			v.Headline.Tooltip = i18n.Diagnostic(selected.UnavailableReason)
		}
	}
	voltage, current := "--.-- V", "≈ --.-- A"
	if reading.VoltageAvailable {
		voltage = Number(reading.VoltageVolts, 2) + " V"
	}
	if reading.CurrentAvailable {
		current = "≈ " + Number(reading.CurrentAmps, 2) + " A"
	}
	electrical(v, voltage, current)
	v.Percentage = "--%"
	if reading.Percentage >= 0 {
		v.Percentage = strconv.Itoa(reading.Percentage) + "%"
	}
	v.BatteryLevel = reading.Percentage
	v.BatteryTone = ToneAccent
	if profile.Accent == power.AccentIdle {
		v.BatteryTone = ToneMuted
	}
	v.Rows = [4]Readout{row(snapshot.BatteryTerminal), row(snapshot.CPUPackage), row(snapshot.Platform), row(snapshot.WholeSystem)}
	boundary := s.selectedBoundary(selected)
	v.Title = Readout{Text: i18n.Get(boundary.Label()), Tone: ToneMuted, Tooltip: s.titleTooltip(v.Rows)}
	s.wholeSystemCaption(v)
	v.Diagnostics = diagnostics(snapshot, selected)
	v.ElevationHelps = elevationHelps(snapshot)
	v.TrayGlyph = "--"
	if w, ok := selected.Value(); ok {
		v.TrayGlyph = trayGlyph(w)
	}
	v.TrayTooltip = i18n.Get(boundary.Label()) + ": " + headline
	v.Updated = s.updatedText()
	s.statistics(v, selected != nil && selected.Kind == power.Estimated)
	estimated := s.mode == WholeSystem && s.wholeBoundary == power.EstimatedSystemInput
	if selected != nil {
		estimated = selected.Kind == power.Estimated
	}
	s.trendView(v, estimated)
	return v
}

func (s *Session) failureView() *View {
	reason := i18n.Diagnostic(s.failure)
	v := &View{Accent: power.AccentError}
	v.Supply = Readout{Text: i18n.Get("传感器异常"), Tone: ToneAccent}
	v.SupplyIcon = IconAlert
	v.Headline = Readout{Text: "N/A", Tone: ToneUnavailable, Tooltip: reason}
	electrical(v, "-- V", "≈ -- A")
	v.Percentage = "--%"
	v.BatteryLevel = -1
	v.BatteryTone = ToneAccent
	for i := range v.Rows {
		v.Rows[i] = Readout{Text: "N/A", Tone: ToneUnavailable, Tooltip: reason}
	}
	v.Title = Readout{Text: i18n.Get(s.idleBoundary().Label()), Tone: ToneMuted, Tooltip: s.titleTooltip(v.Rows)}
	s.wholeSystemCaption(v)
	v.Diagnostics = []string{reason}
	v.TrayGlyph = "--"
	modeKey := "整机功率"
	if s.mode == Battery {
		modeKey = "电池端净功率"
	}
	v.TrayTooltip = i18n.Get(modeKey) + ": N/A"
	v.Updated = s.updatedText()
	s.statistics(v, false)
	s.trendView(v, s.mode == WholeSystem && s.wholeBoundary == power.EstimatedSystemInput)
	return v
}

// initialView is shown only until the first capture completes.
func (s *Session) initialView() *View {
	v := &View{Accent: power.AccentCharging}
	v.Supply = Readout{Text: i18n.Get("正在读取"), Tone: ToneMuted}
	v.SupplyIcon = IconNone
	v.Headline = Readout{Text: "--.-- W", Tone: ToneAccent}
	electrical(v, "--.-- V", "≈ --.-- A")
	v.Percentage = "--%"
	v.BatteryLevel = -1
	v.BatteryTone = ToneAccent
	for i := range v.Rows {
		v.Rows[i] = Readout{Text: "--", Tone: ToneInk}
	}
	v.Title = Readout{Text: i18n.Get(s.idleBoundary().Label()), Tone: ToneMuted, Tooltip: s.titleTooltip(v.Rows)}
	s.wholeSystemCaption(v)
	v.Diagnostics = []string{}
	v.TrayTooltip = i18n.AppName() + ": " + i18n.Get("正在读取")
	v.Updated = s.updatedText()
	s.statistics(v, false)
	s.trendView(v, false)
	return v
}

func (s *Session) idleBoundary() power.Boundary {
	if s.mode == Battery {
		return power.BatteryTerminal
	}
	return s.wholeBoundary
}

// titleTooltip explains the headline the way its supporting row does.
func (s *Session) titleTooltip(rows [4]Readout) string {
	if s.mode == WholeSystem {
		return i18n.Get(wholeSystemFormula)
	}
	return rows[BatteryRow].Tooltip
}

// wholeSystemCaption follows the most recent whole-system reading, because
// that row's boundary changes with the supply state.
func (s *Session) wholeSystemCaption(v *View) {
	v.WholeCaption = i18n.Get(s.wholeBoundary.Label())
	v.WholeCaptionTooltip = i18n.Get(wholeSystemFormula)
}

func (s *Session) updatedText() string {
	if !s.hasTimestamp {
		return "--:--:--"
	}
	return s.timestamp.Format("15:04:05")
}

func diagnostics(snapshot *power.Snapshot, selected *power.Sample) []string {
	lines := []string{}
	for _, sample := range []*power.Sample{snapshot.BatteryTerminal, snapshot.CPUPackage, snapshot.Platform} {
		if sample != nil && !sample.Available {
			lines = append(lines, i18n.Get(sample.Boundary.Label())+": "+i18n.Diagnostic(sample.UnavailableReason))
		}
	}
	// A whole-system figure missing only because platform power is missing
	// has already been explained by the platform line.
	platform := snapshot.Platform
	explained := platform != nil && !platform.Available && selected != nil &&
		selected.UnavailableReason == platform.UnavailableReason
	if selected != nil && !selected.Available && selected.Boundary != power.BatteryTerminal && !explained {
		lines = append(lines, i18n.Get(selected.Boundary.Label())+": "+i18n.Diagnostic(selected.UnavailableReason))
	}
	return lines
}

func (s *Session) statistics(v *View, estimated bool) {
	average, hasAverage, coverage := s.history.Average()
	peak, hasPeak := s.history.Peak()
	prefix := ""
	if estimated {
		prefix = "≈ "
	}
	duration := s.history.Duration(PeakWindow)
	v.AverageTooltip = i18n.Format("10 分钟均值 · 有效 {0}", elapsed(coverage))
	v.PeakTooltip = i18n.Format("5 分钟峰值 · 最近 {0}", elapsed(duration))
	// A partial window shows how much of it is covered; its value uses the
	// readings so far.
	if coverage >= AverageWindow-0.000001 {
		v.AverageCaption = i18n.Get("10 分钟均值")
	} else {
		v.AverageCaption = i18n.Format("均值 · {0}/10分", secondsOrMinutes(coverage, "{0}秒", "{0}分"))
	}
	if duration >= PeakWindow-0.000001 {
		v.PeakCaption = i18n.Get("5 分钟峰值")
	} else {
		v.PeakCaption = i18n.Format("峰值 · {0}/5分", secondsOrMinutes(duration, "{0}秒", "{0}分"))
	}
	v.AverageValue, v.AverageTone = "-- W", ToneUnavailable
	if hasAverage {
		v.AverageValue, v.AverageTone = prefix+Number(average, 2)+" W", ToneInk
	}
	v.PeakValue, v.PeakTone = "-- W", ToneUnavailable
	if hasPeak {
		v.PeakValue, v.PeakTone = prefix+Number(peak, 2)+" W", ToneInk
	}
}

// secondsOrMinutes is whole seconds under a minute, then whole minutes.
func secondsOrMinutes(seconds float64, secondsKey, minutesKey string) string {
	if seconds < 60 {
		return i18n.Format(secondsKey, strconv.Itoa(int(math.Floor(seconds))))
	}
	return i18n.Format(minutesKey, strconv.Itoa(int(math.Floor(seconds/60))))
}

func elapsed(seconds float64) string {
	total := int(math.Floor(seconds))
	return i18n.Format("{0} 分 {1} 秒", total/60, total%60)
}

func electrical(v *View, voltage, current string) {
	v.VoltageCaption = i18n.Get("电池端电压")
	v.Voltage = voltage
	v.CurrentCaption = i18n.Get("电流")
	v.Current = current
}

// elevationHelps: only the elevation-gated platform counter is fixed by
// restarting.
func elevationHelps(snapshot *power.Snapshot) bool {
	p := snapshot.Platform
	return p != nil && !p.Available && p.UnavailableReason == power.ReasonElevationRequired
}

func (s *Session) trendView(v *View, estimated bool) {
	series := s.trend.Series(s.mode)
	t := TrendView{Series: series}
	if series.Full() || series.Count() == 0 {
		t.AxisLabels = [4]string{i18n.Get("−30 分"), Number(-20, 0), Number(-10, 0), i18n.Get("现在")}
	} else {
		t.AxisLabels = [4]string{"0", "10", "20", i18n.Get("30 分")}
	}
	if series.Count() > 0 && !series.Full() {
		t.Recorded = secondsOrMinutes(series.Recorded, "已记录 {0} 秒", "已记录 {0} 分钟")
	}
	if _, w, ok := series.Peak(); ok {
		prefix := ""
		if estimated {
			prefix = "≈ "
		}
		t.PeakLabel = i18n.Format("峰值 {0}", prefix+Number(w, 2)+" W")
	}
	switch {
	case series.Count() == 0 && s.latest == nil && !s.failed:
		t.Empty = i18n.Get("等待第一次读数")
	case s.mode == Battery:
		t.Empty = i18n.Get("电池端功率不可用，暂无走势")
	default:
		t.Empty = i18n.Get("整机功率不可用，暂无走势")
	}
	t.Tooltip = i18n.Get("最近 30 分钟，每秒一个读数；峰值是绝对值最大的读数，与 5 分钟峰值规则相同。")
	v.Trend = t
}

// row shows an unavailable source as N/A with its reason in the tooltip; it
// is never filled in from a neighbouring boundary.
func row(sample *power.Sample) Readout {
	if sample == nil {
		return Readout{Text: "N/A", Tone: ToneUnavailable, Tooltip: i18n.Get("无数据")}
	}
	if !sample.Available {
		return Readout{Text: "N/A", Tone: ToneUnavailable, Tooltip: i18n.Diagnostic(sample.UnavailableReason)}
	}
	tone := ToneInk
	if sample.Kind == power.Estimated {
		tone = ToneMuted
	}
	return Readout{Text: watts(sample), Tone: tone, Tooltip: i18n.Diagnostic(sample.Source)}
}

// headlineTone keeps a measured idle headline ink: with no battery flow the
// grey idle accent would make a valid reading look unavailable.
func headlineTone(selected *power.Sample, accent power.Accent) Tone {
	switch {
	case !selected.IsAvailable():
		return ToneUnavailable
	case selected.Kind == power.Estimated:
		return ToneNumericMuted
	case accent == power.AccentIdle:
		return ToneInk
	default:
		return ToneAccent
	}
}

// watts formats a figure; estimates always carry the ≈ prefix.
func watts(sample *power.Sample) string {
	if !sample.IsAvailable() {
		return "N/A"
	}
	prefix := ""
	if sample.Kind == power.Estimated {
		prefix = "≈ "
	}
	return prefix + Number(sample.Watts, 2) + " W"
}

// trayGlyph is the whole number of watts the tray icon shows.
func trayGlyph(w float64) string {
	w = math.Abs(w)
	if w >= 99.5 {
		return "99+"
	}
	return strconv.Itoa(int(math.Round(w)))
}

func supplyCaptionKey(state power.SupplyState) string {
	switch state {
	case power.ExternalPowerIdle:
		return "已接电源 · 未充电"
	case power.ExternalPowerCharging:
		return "已接电源 · 充电中"
	case power.ExternalPowerSupplemented:
		return "已接电源 · 电池补充"
	case power.ExternalPowerDirectionUnknown:
		return "已接电源 · 状态未知"
	case power.BatteryDischarging:
		return "电池供电 · 放电中"
	case power.BatteryDirectionUnknown:
		return "电池供电 · 速率未知"
	case power.SupplyInconsistent:
		return "电池状态异常"
	default:
		return "电池不可用"
	}
}

func supplyIconFor(state power.SupplyState) SupplyIcon {
	switch state {
	case power.ExternalPowerIdle, power.ExternalPowerSupplemented, power.ExternalPowerDirectionUnknown:
		return IconExternalPower
	case power.ExternalPowerCharging:
		return IconCharging
	case power.BatteryDischarging, power.BatteryDirectionUnknown:
		return IconBattery
	default:
		return IconAlert
	}
}
