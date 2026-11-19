package meter

import (
	"testing"
	"time"

	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/power"
)

var sessionTime = time.Date(2026, 9, 27, 10, 20, 30, 0, time.FixedZone("", 8*3600))

func TestSession(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		cpu := power.FromValue(power.CPUPackage, power.Measured, 14.11, "fixture", time.Second)
		platform := power.FromValue(power.Platform, power.Measured, 27, "fixture", time.Second)
		noPlatform := power.Unsupported(power.Platform, "需要管理员权限")
		charging := battery(true, true, true, 39.08)

		session := NewSession(WholeSystem)
		chargingShowsTheEstimatedTotal(t, session, charging, cpu, platform)
		modeChangeRestartsStatistics(t, session, charging, cpu, platform)
		missingPlatformIsExplainedOnce(t, charging, cpu, noPlatform)
		supplementationAndSensorFailure(t, cpu)
		beforeTheFirstReading(t)
	})
}

func chargingShowsTheEstimatedTotal(t *testing.T, session *Session, charging *power.BatteryReading, cpu, platform *power.Sample) {
	v := session.Observe(power.Compose(sessionTime, 1, charging, cpu, platform))
	check(t, "charging headline, row and tray show the estimated total, not battery or extra CPU",
		v.Headline.Text == "≈ 66.08 W" && v.Headline.Tone == ToneNumericMuted &&
			v.Rows[WholeSystemRow].Text == "≈ 66.08 W" && v.Rows[WholeSystemRow].Tone == ToneMuted &&
			v.Rows[CPUPackageRow].Text == "14.11 W" && v.Rows[CPUPackageRow].Tone == ToneInk &&
			v.Title.Tone == ToneMuted && v.Supply.Tone == ToneAccent &&
			v.TrayGlyph == "66" && v.Accent == power.AccentCharging &&
			v.TrayTooltip == i18n.Get("估算整机输入功率")+": ≈ 66.08 W" &&
			v.Supply.Text == i18n.Get("已接电源 · 充电中") && v.Updated == "10:20:30")
}

func modeChangeRestartsStatistics(t *testing.T, session *Session, charging *power.BatteryReading, cpu, platform *power.Sample) {
	v := session.SetMode(Battery)
	check(t, "battery mode changes headline and tray together and restarts statistics",
		session.Mode() == Battery && v.Headline.Text == "39.08 W" && v.Headline.Tone == ToneAccent &&
			v.TrayGlyph == "39" && v.TrayTooltip == i18n.Get("电池端净功率")+": 39.08 W" &&
			v.AverageValue == "-- W" && v.AverageTone == ToneUnavailable &&
			v.AverageCaption == i18n.Format("均值 · {0}/10分", i18n.Format("{0}秒", "0")))
	v = session.Observe(power.Compose(sessionTime, 2, charging, cpu, platform))
	check(t, "mode change seeds statistics with the current reading",
		v.AverageValue == "39.08 W" && v.AverageTone == ToneInk && v.PeakTone == ToneInk &&
			v.AverageCaption == i18n.Format("均值 · {0}/10分", i18n.Format("{0}秒", "1")))
	session.Render()
	v = session.Render()
	check(t, "re-rendering does not record another reading",
		v.AverageCaption == i18n.Format("均值 · {0}/10分", i18n.Format("{0}秒", "1")))
}

func missingPlatformIsExplainedOnce(t *testing.T, charging *power.BatteryReading, cpu, noPlatform *power.Sample) {
	session := NewSession(WholeSystem)
	v := session.Observe(power.Compose(sessionTime, 1, charging, cpu, noPlatform))
	check(t, "missing platform clears the total and tray and explains it once",
		v.Headline.Text == "N/A" && v.Headline.Tone == ToneUnavailable &&
			v.Rows[WholeSystemRow].Text == "N/A" && v.Rows[WholeSystemRow].Tone == ToneUnavailable &&
			v.Rows[PlatformRow].Tone == ToneUnavailable && v.TrayGlyph == "--" && len(v.Diagnostics) == 1 &&
			v.Diagnostics[0] == i18n.Get("平台功率")+": "+i18n.Diagnostic("需要管理员权限"))
	unplugged := battery(false, false, true, -17.5)
	v = session.Observe(power.Compose(sessionTime, 2, unplugged, cpu, noPlatform))
	check(t, "unplugged whole display works without platform or elevation",
		v.Headline.Text == "17.50 W" && v.TrayGlyph == "18" &&
			v.TrayTooltip == i18n.Get("系统负载功率")+": 17.50 W" &&
			v.WholeCaption == i18n.Get("系统负载功率") && len(v.Diagnostics) == 1)
}

func supplementationAndSensorFailure(t *testing.T, cpu *power.Sample) {
	supplemented := battery(true, false, true, -5)
	session := NewSession(WholeSystem)
	platform24 := power.FromValue(power.Platform, power.Measured, 24, "fixture", time.Second)
	session.Observe(power.Compose(sessionTime, 0, supplemented, cpu, platform24))
	v := session.Observe(power.Compose(sessionTime, 1, supplemented, cpu, platform24))
	check(t, "battery supplementation reduces the estimate and statistics stay estimated",
		v.Headline.Text == "≈ 19.00 W" && v.AverageValue == "≈ 19.00 W" && v.PeakValue == "≈ 19.00 W" &&
			v.Supply.Text == i18n.Get("已接电源 · 电池补充") && v.Accent == power.AccentDischarging)

	v = session.Fail("sensor failed")
	check(t, "sensor failure clears the readout",
		v.Headline.Text == "N/A" && v.Accent == power.AccentError &&
			v.Supply.Text == i18n.Get("传感器异常") && v.TrayGlyph == "--" &&
			v.TrayTooltip == i18n.Get("整机功率")+": N/A" &&
			len(v.Diagnostics) == 1 && v.Diagnostics[0] == "sensor failed")
	check(t, "sensor failure clears the statistics and never shows a stale figure",
		v.AverageValue == "-- W" && v.PeakValue == "-- W" && v.Updated == "10:20:30" &&
			v.AverageTone == ToneUnavailable && v.PeakTone == ToneUnavailable &&
			v.Headline.Tone == ToneUnavailable && v.Rows[PlatformRow].Text == "N/A" &&
			v.Rows[PlatformRow].Tone == ToneUnavailable && v.BatteryLevel == -1)
	v = session.Render()
	check(t, "sensor failure survives re-rendering until the next reading", v.Supply.Text == i18n.Get("传感器异常"))
	v = session.Observe(power.Compose(sessionTime, 10, supplemented, cpu, platform24))
	check(t, "readings after a failure start new statistics", v.Headline.Text == "≈ 19.00 W" && v.AverageValue == "-- W")
}

func beforeTheFirstReading(t *testing.T) {
	v := NewSession(Battery).Render()
	check(t, "before the first reading the tray keeps the application icon",
		v.TrayGlyph == "" && v.Headline.Text == "--.-- W" && len(v.Diagnostics) == 0 &&
			v.Supply.Tone == ToneMuted && v.AverageTone == ToneUnavailable && v.Rows[PlatformRow].Tone == ToneInk &&
			v.Updated == "--:--:--" && v.Title.Text == i18n.Get("电池端净功率"))
}

func TestDisplay(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		platform := platformOf(24)
		for _, b := range []*power.BatteryReading{battery(true, true, true, 13), battery(true, false, true, -5), battery(false, false, true, -26)} {
			snap := snapshotAt(b, platform, 0)
			whole := NewSession(WholeSystem).Observe(snap)
			terminal := NewSession(Battery).Observe(snap)
			check(t, "selected boundary is shared without substitution: "+b.SupplyState.String(),
				whole.Headline.Text == whole.Rows[WholeSystemRow].Text &&
					whole.Title.Text == i18n.Get(snap.WholeSystem.Boundary.Label()) &&
					terminal.Headline.Text == terminal.Rows[BatteryRow].Text &&
					terminal.Title.Text == i18n.Get(power.BatteryTerminal.Label()))
		}
		charging := battery(true, true, true, 13)
		missing := snapshotAt(charging, power.Unsupported(power.Platform, "missing"), 0)
		check(t, "whole mode does not fall back to available battery",
			NewSession(WholeSystem).Observe(missing).Headline.Text == "N/A" &&
				NewSession(Battery).Observe(missing).Headline.Text == "13.00 W")
		idle := NewSession(Battery).Observe(snapshotAt(battery(true, false, true, 0), platform, 0))
		check(t, "a measured idle headline is ink, not the grey idle accent",
			idle.Accent == power.AccentIdle && idle.Headline.Tone == ToneInk && idle.Supply.Tone == ToneAccent &&
				idle.BatteryTone == ToneMuted)
	})
	check(t, "preferences default safely for missing or malformed storage",
		ParseMode("") == WholeSystem && ParseMode("invalid") == WholeSystem && ParseMode("Battery") == Battery &&
			ParseMode(Battery.String()) == Battery && ParseMode(WholeSystem.String()) == WholeSystem)
}

func TestReadingDetails(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		b := battery(false, false, true, -17.5)
		b.VoltageAvailable, b.VoltageVolts = true, 12.3456
		b.CurrentAvailable, b.CurrentAmps = true, 1.4178
		b.Percentage = 64
		v := NewSession(WholeSystem).Observe(snapshotAt(b, nil, 0))
		check(t, "voltage, current and percentage",
			v.Voltage == "12.35 V" && v.Current == "≈ 1.42 A" && v.Percentage == "64%" && v.BatteryLevel == 64 &&
				v.VoltageCaption == i18n.Get("电池端电压") && v.CurrentCaption == i18n.Get("电流") &&
				v.SupplyIcon == IconBattery)
		v = NewSession(WholeSystem).Observe(power.Compose(sessionTime, 0, nil, nil, nil))
		check(t, "a missing battery reading shows placeholders",
			v.Voltage == "--.-- V" && v.Current == "≈ --.-- A" && v.Percentage == "--%" &&
				v.Supply.Text == i18n.Get("电池不可用") && v.SupplyIcon == IconAlert &&
				v.Rows[CPUPackageRow].Text == "N/A" && v.Rows[CPUPackageRow].Tooltip == i18n.Get("无数据"))
		v = NewSession(WholeSystem).Fail("x")
		check(t, "failure placeholders", v.Voltage == "-- V" && v.Current == "≈ -- A" && v.SupplyIcon == IconAlert)
		check(t, "tray glyph rounds halves away and caps at 99+",
			trayGlyph(99.49) == "99" && trayGlyph(99.5) == "99+" && trayGlyph(-0.5) == "1" && trayGlyph(0.49) == "0")
	})
}

func TestHistory(t *testing.T) {
	var h History
	supply := power.BatteryDischarging
	gapsAndStateChangesClearTheBaseline(t, &h, supply)
	peakAndMeanKeepTheirOwnWindows(t, &h, supply)
	weightedMeanClipsAndSkipsGaps(t, &h, supply)
}

func gapsAndStateChangesClearTheBaseline(t *testing.T, h *History, supply power.SupplyState) {
	h.Add(0, Battery, supply, value(-10))
	h.Add(1, Battery, supply, value(-20))
	h.Add(4, Battery, supply, value(-20))
	average, ok, coverage := h.Average()
	check(t, "irregular observations use elapsed weighting and partial window",
		ok && near(average, -17.5) && near(coverage, 4) && near(h.Duration(PeakWindow), 4))
	peak, _ := h.Peak()
	check(t, "peak retains sign at greatest magnitude", near(peak, -20))
	h.Add(5, Battery, supply, gap())
	_, peakOK := h.Peak()
	_, averageOK, _ := h.Average()
	check(t, "missing sample leaves a gap and immediately unavailable statistics",
		!peakOK && !averageOK && !h.points[3].ok)
	h.Add(6, Battery, supply, value(-30))
	average, _, coverage = h.Average()
	check(t, "missing intervals are excluded without zero fill", near(coverage, 4) && near(average, -17.5))
	h.Add(20, Battery, supply, value(-30))
	_, averageOK, _ = h.Average()
	check(t, "resume gap clears baseline", h.Len() == 1 && !averageOK)
	h.Add(21, WholeSystem, supply, value(30))
	check(t, "mode change clears history", h.Len() == 1)
	h.Add(22, WholeSystem, power.ExternalPowerIdle, value(30))
	check(t, "supply state change clears history", h.Len() == 1)
}

func peakAndMeanKeepTheirOwnWindows(t *testing.T, h *History, supply power.SupplyState) {
	h.Clear()
	for second := 0; second <= 700; second++ {
		w := -10.0
		if second == 200 {
			w = -100
		}
		h.Add(float64(second), Battery, supply, value(w))
	}
	average, _, coverage := h.Average()
	peak, _ := h.Peak()
	check(t, "10 minute mean still holds a reading the 5 minute peak has dropped",
		near(average, (-10*599-100)/600.0) && near(coverage, 600) && near(peak, -10) && near(h.Duration(PeakWindow), 300))
	h.Add(701, Battery, supply, value(-100))
	for second := 702; second <= 1001; second++ {
		h.Add(float64(second), Battery, supply, value(-10))
	}
	peak, _ = h.Peak()
	check(t, "a peak stays for exactly 5 minutes", near(peak, -100))
	h.Add(1002, Battery, supply, value(-10))
	peak, _ = h.Peak()
	check(t, "old peak expires at the real 5 minute boundary", near(peak, -10))
}

func weightedMeanClipsAndSkipsGaps(t *testing.T, h *History, supply power.SupplyState) {
	h.Clear()
	for second := 0; second <= 600; second += 4 {
		w := 10.0
		if second == 0 {
			w = 40
		}
		h.Add(float64(second), Battery, supply, value(w))
	}
	h.Add(602, Battery, supply, value(10))
	average, _, coverage := h.Average()
	check(t, "weighted mean clips the first interval at the exact 10 minute cutoff", near(average, 10.1) && near(coverage, 600))
	h.Clear()
	for second := 0; second <= 700; second++ {
		s := value(20)
		if second >= 300 && second < 400 {
			s = gap()
		}
		h.Add(float64(second), Battery, supply, s)
	}
	average, _, coverage = h.Average()
	peak, _ := h.Peak()
	check(t, "an N/A spell inside 10 minutes is excluded, never averaged as zero",
		near(average, 20) && near(coverage, 600-101) && near(peak, 20))
}

func TestCaptions(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		s := NewSession(Battery)
		var v *View
		for second := 0; second <= 700; second++ {
			v = s.Observe(power.Compose(sessionTime, float64(second), battery(false, false, true, -10), nil, nil))
		}
		check(t, "full windows use the plain captions",
			v.AverageCaption == i18n.Get("10 分钟均值") && v.PeakCaption == i18n.Get("5 分钟峰值") &&
				v.AverageTooltip == i18n.Format("10 分钟均值 · 有效 {0}", i18n.Format("{0} 分 {1} 秒", 10, 0)) &&
				v.PeakTooltip == i18n.Format("5 分钟峰值 · 最近 {0}", i18n.Format("{0} 分 {1} 秒", 5, 0)))
		s = NewSession(Battery)
		for second := 0; second <= 125; second++ {
			v = s.Observe(power.Compose(sessionTime, float64(second), battery(false, false, true, -10), nil, nil))
		}
		check(t, "partial windows count minutes after the first",
			v.AverageCaption == i18n.Format("均值 · {0}/10分", i18n.Format("{0}分", "2")) &&
				v.PeakCaption == i18n.Format("峰值 · {0}/5分", i18n.Format("{0}分", "2")))
	})
}
