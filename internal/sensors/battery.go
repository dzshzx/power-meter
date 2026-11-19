package sensors

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dzshzx/power-meter/internal/power"
)

// unknownRate is how WMI BatteryStatus reports a rate or voltage the
// firmware does not know.
const unknownRate = math.MaxUint32

// Battery reasons: Chinese source strings, translated where shown.
const (
	reasonFlagsConflict      = "电池供电标志互相矛盾"
	reasonNoActiveBattery    = "本机没有活动电池"
	reasonStatusUnavailable  = "电池状态不可用"
	reasonPackStatus         = "至少一块电池状态不可用"
	reasonPacksDisagree      = "多块电池报告的供电状态不一致"
	reasonPackRate           = "至少一块电池速率不可用"
	reasonAggregateDirection = "聚合后的电池功率方向与供电状态不一致"
	reasonChargeRate         = "电池充电速率不可用"
	reasonDischargeRate      = "电池放电速率不可用"
	reasonOfflineDirection   = "离线状态下电池方向不明"
	reasonRateUnavailable    = "电池速率不可用"
)

// wmiBattery is one active BatteryStatus instance as WMI reported it.
// Rates are milliwatts and the voltage millivolts; unknownRate means unknown.
type wmiBattery struct {
	PowerOnline, Charging, Discharging bool
	ChargeRate, DischargeRate, Voltage uint32
}

// batteryReading combines the active packs and assigns the operating
// system's aggregate charge percentage (-1 when unknown).
func batteryReading(packs []wmiBattery, percentage int) *power.BatteryReading {
	readings := make([]*power.BatteryReading, len(packs))
	for i, p := range packs {
		readings[i] = packReading(p)
	}
	combined := combineReadings(readings)
	assignSystemPercentage(combined, percentage)
	return combined
}

// packReading resolves one pack's flags, rate and voltage.
func packReading(b wmiBattery) *power.BatteryReading {
	r := power.NewBatteryReading()
	r.SupplyState = power.SupplyFromFlags(b.PowerOnline, b.Charging, b.Discharging)
	r.ActiveBatteryCount = 1
	if !r.StatusAvailable() {
		r.StatusUnavailableReason = reasonFlagsConflict
	}
	r.RawReadings = append(r.RawReadings, fmt.Sprintf(
		"ChargeRate_mW=%d; DischargeRate_mW=%d; Voltage_mV=%d; PowerOnline=%s; Charging=%s; Discharging=%s (4294967295=unknown)",
		b.ChargeRate, b.DischargeRate, b.Voltage, boolText(b.PowerOnline), boolText(b.Charging), boolText(b.Discharging)))
	resolveRate(r, b.ChargeRate, b.DischargeRate)
	resolveElectricalDetails(r, b.Voltage)
	return r
}

// boolText spells a flag as the reports always have: True or False.
func boolText(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// combineReadings merges the active packs into one reading. Packs are summed
// only when they agree on external power; a single voltage or current would
// not describe several packs, so the aggregate has neither.
func combineReadings(readings []*power.BatteryReading) *power.BatteryReading {
	switch {
	case len(readings) == 0:
		r := power.NewBatteryReading()
		r.StatusUnavailableReason = reasonNoActiveBattery
		r.RateUnavailableReason = reasonStatusUnavailable
		return r
	case len(readings) == 1 && readings[0] == nil:
		r := power.NewBatteryReading()
		r.StatusUnavailableReason = reasonStatusUnavailable
		r.RateUnavailableReason = reasonStatusUnavailable
		return r
	case len(readings) == 1:
		readings[0].ActiveBatteryCount = 1
		return readings[0]
	}

	combined := power.NewBatteryReading()
	combined.ActiveBatteryCount = len(readings)
	combined.RateAvailable = true
	var agg aggregate
	for _, r := range readings {
		agg.add(combined, r)
	}
	agg.resolve(combined)
	combined.VoltageAvailable = false
	combined.CurrentAvailable = false
	return combined
}

// aggregate collects what the packs say about supply and direction.
type aggregate struct {
	onlineKnown, online                    bool
	charging, discharging, idle, conflicts bool
}

func (a *aggregate) add(combined, r *power.BatteryReading) {
	if r != nil {
		combined.RawReadings = append(combined.RawReadings, r.RawReadings...)
	}
	if r == nil || !r.StatusAvailable() {
		a.conflict(combined, reasonPackStatus)
		return
	}
	if !a.onlineKnown {
		a.onlineKnown, a.online = true, r.PowerOnline()
	} else if r.PowerOnline() != a.online {
		a.conflict(combined, reasonPacksDisagree)
	}
	a.charging = a.charging || r.Charging()
	a.discharging = a.discharging || r.Discharging()
	a.idle = a.idle || (!r.Charging() && !r.Discharging())
	if !r.RateAvailable {
		combined.RateAvailable = false
		if combined.RateUnavailableReason == "" {
			combined.RateUnavailableReason = r.RateUnavailableReason
			if combined.RateUnavailableReason == "" {
				combined.RateUnavailableReason = reasonPackRate
			}
		}
	}
	combined.PowerWatts += r.PowerWatts
}

func (a *aggregate) conflict(combined *power.BatteryReading, reason string) {
	a.conflicts = true
	combined.RateAvailable = false
	combined.StatusUnavailableReason = reason
}

// resolve sets the combined supply state: from the summed power when every
// pack has a rate, otherwise from the directions the packs share.
func (a *aggregate) resolve(combined *power.BatteryReading) {
	switch {
	case a.conflicts || !a.onlineKnown:
		combined.SupplyState = power.SupplyInconsistent
	case combined.RateAvailable:
		combined.SupplyState = power.SupplyFromFlags(a.online, combined.PowerWatts > 0, combined.PowerWatts < 0)
		if !combined.StatusAvailable() {
			combined.RateAvailable = false
			combined.StatusUnavailableReason = reasonAggregateDirection
		}
	default:
		combined.SupplyState = a.sharedDirection()
	}
}

func (a *aggregate) sharedDirection() power.SupplyState {
	directions := 0
	for _, d := range []bool{a.charging, a.discharging, a.idle} {
		if d {
			directions++
		}
	}
	switch {
	case directions == 1:
		return power.SupplyFromFlags(a.online, a.charging, a.discharging)
	case a.online:
		return power.ExternalPowerDirectionUnknown
	default:
		return power.BatteryDirectionUnknown
	}
}

// resolveRate sets the signed terminal power from the rate of the direction
// in effect. A rate for the other direction is no measurement of this one.
func resolveRate(r *power.BatteryReading, chargeRate, dischargeRate uint32) {
	r.RateAvailable, r.PowerWatts, r.RateUnavailableReason = false, 0, ""
	switch r.SupplyState {
	case power.ExternalPowerCharging:
		if chargeRate == unknownRate {
			r.RateUnavailableReason = reasonChargeRate
			return
		}
		r.RateAvailable, r.PowerWatts = true, float64(chargeRate)/1000
	case power.ExternalPowerSupplemented, power.BatteryDischarging:
		if dischargeRate == unknownRate {
			r.RateUnavailableReason = reasonDischargeRate
			return
		}
		r.RateAvailable, r.PowerWatts = true, -(float64(dischargeRate) / 1000)
	case power.ExternalPowerIdle:
		// Neither direction on external power is a real zero.
		r.RateAvailable = true
	case power.BatteryDirectionUnknown:
		// Firmware has not said what supplies the running machine; a zero
		// here would fabricate a whole-system reading.
		r.RateUnavailableReason = reasonOfflineDirection
	case power.SupplyInconsistent:
		r.RateUnavailableReason = reasonFlagsConflict
	default:
		r.RateUnavailableReason = reasonRateUnavailable
	}
}

// resolveElectricalDetails sets the terminal voltage and, when the power is
// known, the estimated current.
func resolveElectricalDetails(r *power.BatteryReading, voltageMillivolts uint32) {
	r.VoltageAvailable, r.CurrentAvailable, r.VoltageVolts, r.CurrentAmps = false, false, 0, 0
	if voltageMillivolts == unknownRate || voltageMillivolts == 0 {
		return
	}
	r.VoltageAvailable, r.VoltageVolts = true, float64(voltageMillivolts)/1000
	if r.RateAvailable {
		r.CurrentAvailable, r.CurrentAmps = true, math.Abs(r.PowerWatts)/r.VoltageVolts
	}
}

// assignSystemPercentage keeps the operating system's percentage only for a
// reading whose status is known.
func assignSystemPercentage(r *power.BatteryReading, percentage int) {
	if r == nil {
		return
	}
	r.Percentage = -1
	if r.StatusAvailable() && percentage >= 0 && percentage <= 100 {
		r.Percentage = percentage
	}
}

// wmiBool reads a WMI flag; anything missing or unreadable is false.
func wmiBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "true")
	}
	n, ok := wmiInteger(v)
	return ok && n != 0
}

// wmiUint32 reads a WMI uint32; anything missing or out of range is
// unknownRate. Scripting WMI hands CIM uint32 over as a signed 32-bit
// VARIANT, so a negative int32 is the same bits read as unsigned.
func wmiUint32(v any) uint32 {
	switch x := v.(type) {
	case int32:
		return uint32(x)
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(x), 10, 32)
		if err != nil {
			return unknownRate
		}
		return uint32(n)
	}
	n, ok := wmiInteger(v)
	if !ok || n < 0 || n > math.MaxUint32 {
		return unknownRate
	}
	return uint32(n)
}

func wmiInteger(v any) (int64, bool) {
	switch x := v.(type) {
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	case uint:
		if uint64(x) > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	}
	return 0, false
}
