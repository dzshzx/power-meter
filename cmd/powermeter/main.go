// Command powermeter is Power Meter: a notebook power monitor in a small
// window and the notification area, with command-line diagnostics.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/dzshzx/power-meter/internal/app"
	"github.com/dzshzx/power-meter/internal/autostart"
	"github.com/dzshzx/power-meter/internal/cli"
	"github.com/dzshzx/power-meter/internal/gui"
	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/dzshzx/power-meter/internal/selftest"
	"github.com/dzshzx/power-meter/internal/sensors"
	"github.com/dzshzx/power-meter/internal/settings"
	"github.com/dzshzx/power-meter/internal/winapp"
	thirdparty "github.com/dzshzx/power-meter/third_party"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches one parsed route and returns the exit code: 2 for an
// invalid command line, as before.
func run(args []string) int {
	route := cli.Parse(args)
	if !route.Valid {
		return 2
	}
	i18n.SetSystemCulture(systemCulture())
	i18n.Load(settings.Language())
	switch route.Command {
	case cli.GUI:
		return runGUI(route)
	case cli.RemoveAutostart:
		return autostart.RunRemove()
	case cli.SyncAutostart:
		return autostart.RunSync()
	}
	if err := runCommand(route); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runGUI starts the window, or brings back the one already running for
// this installation. A plain start under an ordinary token asks for
// elevation once; the elevated copy waits for this process's lock.
func runGUI(route cli.Route) int {
	identity := winapp.Identity()
	instance, ok, err := winapp.Acquire(identity, route.Handoff)
	if err != nil || !ok {
		if err == nil && !route.StartHidden {
			winapp.RestoreExisting(identity)
		}
		return 0
	}
	defer instance.Close()
	elevation := cli.Decide(route, winapp.IsElevated(), winapp.TryElevate)
	if elevation.Started {
		return 0
	}
	elevated := winapp.IsElevated()
	notice := elevation.Message
	if elevated {
		autostart.SyncOnStart()
		notice = ""
	}
	return app.Run(app.Options{
		StartHidden:     route.StartHidden,
		Elevated:        elevated,
		ElevationNotice: notice,
		Elevate:         winapp.TryElevate,
		Listen:          instance.Listen,
	})
}

// selfTestFailed is the error of a failing self-test; its report is written.
type selfTestFailed struct{}

func (selfTestFailed) Error() string { return "self-test failed" }

// runCommand runs the preview, diagnostic and notice commands.
func runCommand(route cli.Route) error {
	switch route.Command {
	case cli.SelfTest:
		report, passed := selftest.Run(append(selftest.Core(), gui.SelfTest()...))
		if err := os.WriteFile(route.Path, []byte(report), 0o644); err != nil {
			return err
		}
		if !passed {
			return selfTestFailed{}
		}
	case cli.ThirdPartyNotices:
		return thirdparty.WriteNotices(route.Path)
	case cli.PowerProbe:
		return os.WriteFile(route.Path, []byte(sensors.Probe(route.Seconds)), 0o644)
	case cli.Screenshot:
		return screenshot(route.Path, systemDPI())
	case cli.DPIPreview:
		return screenshot(route.Path, route.DPIs[len(route.DPIs)-1])
	case cli.TrayPreview:
		accent := power.AccentCharging
		if route.TrayDischarging {
			accent = power.AccentDischarging
		}
		return os.WriteFile(route.Path, gui.TrayIcon(route.TrayGlyph, accent), 0o644)
	}
	return nil
}

// screenshot draws the window with one live reading at dpi, and its size
// beside it in path.txt.
func screenshot(path string, dpi int) error {
	sources := sensors.New()
	defer sources.Close()
	session := meter.NewSession(settings.Mode())
	m := &gui.Model{
		View:     session.Observe(sources.Capture()),
		Mode:     session.Mode(),
		Pinned:   true,
		Elevated: winapp.IsElevated(),
		Language: i18n.Preference(),
	}
	if err := os.WriteFile(path, gui.Render(m, float32(dpi)/96, darkAppearance()), 0o644); err != nil {
		return err
	}
	meta := "Dpi=" + strconv.Itoa(dpi) + "; Content=" + strconv.Itoa(gui.Width) + "x" + strconv.Itoa(gui.Height(m))
	return os.WriteFile(path+".txt", []byte(meta), 0o644)
}
