// Package cli is the one parse of the command line and the startup
// elevation decision. A valid route carries every argument its command needs,
// already converted, so the dispatcher never re-reads the raw arguments.
package cli

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
)

// Commands. The GUI is the default; the others are spelled like options.
const (
	GUI               = "gui"
	RemoveAutostart   = "--remove-autostart"
	SyncAutostart     = "--sync-autostart"
	SelfTest          = "--self-test"
	ThirdPartyNotices = "--third-party-notices"
	Screenshot        = "--screenshot"
	PowerProbe        = "--power-probe"
	TrayPreview       = "--tray-preview"
)

// GUI markers: each starts the window and must stand alone.
const (
	NoElevate          = "--no-elevate"
	ElevationAttempted = "--elevation-attempted"
	Autostart          = "--autostart"
)

// Route is the parsed command line.
type Route struct {
	Command string
	Valid   bool
	// SuppressElevation: a GUI marker was given, so the window does not ask
	// for elevation itself.
	SuppressElevation bool
	// StartHidden: the logon task's launch, straight into the tray.
	StartHidden bool
	// Handoff: this process was started by the elevation request.
	Handoff bool
	// Path is the output file of the CLI commands that write one.
	Path    string
	Seconds int
	// DPI is the screenshot's DPI; 0 means the primary display's.
	DPI             int
	TrayGlyph       string
	TrayDischarging bool
}

// argument parses the positional arguments of one command; ok false makes
// the route invalid.
type argument func(r *Route, values []string) bool

var commands = map[string]argument{
	RemoveAutostart:   none,
	SyncAutostart:     none,
	SelfTest:          output,
	ThirdPartyNotices: output,
	Screenshot:        screenshot,
	PowerProbe:        probe,
	TrayPreview:       trayPreview,
}

// Parse reads the command line without the program name. Command names are
// case-insensitive; there is no help, version, response file or directive,
// and anything unknown is invalid.
func Parse(args []string) Route {
	r := Route{Command: GUI, Seconds: 5}
	if len(args) == 0 {
		r.Valid = true
		return r
	}
	first := strings.ToLower(args[0])
	switch first {
	case NoElevate, ElevationAttempted, Autostart:
		r.Valid = len(args) == 1
		r.SuppressElevation = true
		r.StartHidden = first == Autostart
		r.Handoff = first == ElevationAttempted
		return r
	}
	parse, ok := commands[first]
	if !ok {
		return r
	}
	r.Command = first
	r.Valid = parse(&r, args[1:])
	return r
}

func none(_ *Route, values []string) bool { return len(values) == 0 }

func output(r *Route, values []string) bool {
	return len(values) == 1 && setPath(r, values[0])
}

func setPath(r *Route, path string) bool {
	r.Path = path
	return strings.TrimSpace(path) != ""
}

func probe(r *Route, values []string) bool { return pathAndInteger(r, values, 1, 3600, &r.Seconds) }

func screenshot(r *Route, values []string) bool { return pathAndInteger(r, values, 96, 768, &r.DPI) }

// pathAndInteger parses an output path and an optional integer in [lo, hi]
// into n.
func pathAndInteger(r *Route, values []string, lo, hi int, n *int) bool {
	if len(values) < 1 || len(values) > 2 || !setPath(r, values[0]) {
		return false
	}
	if len(values) == 1 {
		return true
	}
	v, ok := integer(values[1], lo, hi)
	*n = v
	return ok
}

func trayPreview(r *Route, values []string) bool {
	if len(values) != 3 || !setPath(r, values[0]) || strings.TrimSpace(values[1]) == "" {
		return false
	}
	r.TrayGlyph = values[1]
	switch strings.ToLower(values[2]) {
	case "charging":
	case "discharging":
		r.TrayDischarging = true
	default:
		return false
	}
	return true
}

func integer(text string, lo, hi int) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	return n, err == nil && n >= lo && n <= hi
}

// ElevationResult says whether an elevated copy took over, and otherwise
// why this process runs without elevation.
type ElevationResult struct {
	Started bool
	Message string
}

// Elevation notices: Chinese source strings, translated where shown.
const (
	MessageAdmin     = "管理员模式"
	MessageOrdinary  = "普通模式：平台功率可能需要管理员权限。"
	MessageNotStart  = "管理员启动未成功，继续普通模式。"
	MessageCancelled = "已取消管理员授权，继续普通模式。"
	MessageFailed    = "管理员启动失败，继续普通模式："
)

// Decide asks for elevation once, for a plain GUI start under an ordinary
// token. Every other route, and an elevated process, runs as it is; launch is
// the only OS call, so tests never display UAC.
func Decide(r Route, elevated bool, launch func() ElevationResult) ElevationResult {
	if !r.Valid || r.Command != GUI || elevated || r.SuppressElevation {
		if elevated {
			return ElevationResult{Message: MessageAdmin}
		}
		return ElevationResult{Message: MessageOrdinary}
	}
	return launch()
}

// errCancelled is ERROR_CANCELLED: the user declined the UAC prompt.
const errCancelled = syscall.Errno(1223)

// Launch runs the elevation request and turns its outcome into a result;
// cancelling or failing keeps the ordinary window with the reason.
func Launch(start func() (bool, error)) ElevationResult {
	started, err := start()
	switch {
	case errors.Is(err, errCancelled):
		return ElevationResult{Message: MessageCancelled}
	case err != nil:
		return ElevationResult{Message: MessageFailed + err.Error()}
	case !started:
		return ElevationResult{Message: MessageNotStart}
	}
	return ElevationResult{Started: true}
}
