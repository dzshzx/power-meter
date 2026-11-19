package power

// SupplyState is the battery driver's answer to "where is power coming from
// right now", resolved from the power-online, charging and discharging flags
// into exactly one of eight states (GLOSSARY.md). The state, not the raw
// flags, decides which formula applies and which caption shows.
type SupplyState int

const (
	SupplyUnavailable SupplyState = iota
	ExternalPowerIdle
	ExternalPowerCharging
	ExternalPowerSupplemented
	ExternalPowerDirectionUnknown
	BatteryDischarging
	BatteryDirectionUnknown
	SupplyInconsistent
)

var supplyNames = [...]string{
	"Unavailable", "ExternalPowerIdle", "ExternalPowerCharging", "ExternalPowerSupplemented",
	"ExternalPowerDirectionUnknown", "BatteryDischarging", "BatteryDirectionUnknown", "Inconsistent",
}

// String is the state's name as reports print it.
func (s SupplyState) String() string {
	if s >= 0 && int(s) < len(supplyNames) {
		return supplyNames[s]
	}
	return supplyNames[0]
}

// Accent is the colour family of a supply state; colours only ever mean the
// supply state.
type Accent int

const (
	AccentError Accent = iota
	AccentIdle
	AccentCharging
	AccentDischarging
)

// SupplyProfile is the complete meaning of one supply state. Consumers read
// it instead of interpreting combinations of WMI flags themselves.
type SupplyProfile struct {
	StatusAvailable bool
	PowerOnline     bool
	Charging        bool
	Discharging     bool
	StateText       string
	TrayMode        string
	Accent          Accent
}

var profiles = map[SupplyState]SupplyProfile{
	SupplyUnavailable:             {false, false, false, false, "BATTERY UNAVAILABLE", "Battery unavailable", AccentError},
	ExternalPowerIdle:             {true, true, false, false, "AC / IDLE", "AC / idle", AccentIdle},
	ExternalPowerCharging:         {true, true, true, false, "CHARGING", "Charging", AccentCharging},
	ExternalPowerSupplemented:     {true, true, false, true, "DISCHARGING", "Discharging", AccentDischarging},
	ExternalPowerDirectionUnknown: {true, true, false, false, "AC / UNKNOWN", "AC / battery direction unknown", AccentError},
	BatteryDischarging:            {true, false, false, true, "DISCHARGING", "Discharging", AccentDischarging},
	BatteryDirectionUnknown:       {true, false, false, false, "ON BATTERY", "On battery", AccentDischarging},
	SupplyInconsistent:            {false, false, false, false, "BATTERY STATE INVALID", "Battery state invalid", AccentError},
}

// SupplyFromFlags resolves the driver's flags into one state. Contradictory
// flags form the Inconsistent state; nothing is guessed.
func SupplyFromFlags(powerOnline, charging, discharging bool) SupplyState {
	switch {
	case charging && discharging, charging && !powerOnline:
		return SupplyInconsistent
	case charging:
		return ExternalPowerCharging
	case discharging && powerOnline:
		return ExternalPowerSupplemented
	case discharging:
		return BatteryDischarging
	case powerOnline:
		return ExternalPowerIdle
	default:
		return BatteryDirectionUnknown
	}
}

// Profile describes the state; an unknown value describes Unavailable.
func (s SupplyState) Profile() SupplyProfile {
	if p, ok := profiles[s]; ok {
		return p
	}
	return profiles[SupplyUnavailable]
}

// BatteryReading is the combined state of the machine's active batteries.
type BatteryReading struct {
	SupplyState             SupplyState
	StatusUnavailableReason string
	ActiveBatteryCount      int
	RateAvailable           bool
	RateUnavailableReason   string
	// PowerWatts is the signed net battery terminal power: positive charges.
	PowerWatts       float64
	VoltageAvailable bool
	CurrentAvailable bool
	VoltageVolts     float64
	CurrentAmps      float64
	// Percentage is the charge level, or -1 when unknown.
	Percentage  int
	RawReadings []string
}

// NewBatteryReading is a reading that knows nothing yet.
func NewBatteryReading() *BatteryReading {
	return &BatteryReading{SupplyState: SupplyUnavailable, Percentage: -1}
}

func (r *BatteryReading) Profile() SupplyProfile { return r.SupplyState.Profile() }
func (r *BatteryReading) StatusAvailable() bool  { return r.Profile().StatusAvailable }
func (r *BatteryReading) PowerOnline() bool      { return r.Profile().PowerOnline }
func (r *BatteryReading) Charging() bool         { return r.Profile().Charging }
func (r *BatteryReading) Discharging() bool      { return r.Profile().Discharging }
