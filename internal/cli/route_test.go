package cli

import (
	"errors"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func TestRoutes(t *testing.T) {
	launches := 0
	launch := func() ElevationResult {
		launches++
		return ElevationResult{Started: true}
	}
	if !Decide(Parse(nil), false, launch).Started || launches != 1 {
		t.Error("ordinary GUI requests elevation once and exits parent on success")
	}
	if r := Decide(Parse(nil), true, launch); r.Started || r.Message != MessageAdmin {
		t.Error("an elevated GUI runs as it is")
	}
	if r := Decide(Parse([]string{NoElevate}), false, launch); r.Message != MessageOrdinary {
		t.Error("an explicit ordinary GUI explains the ordinary mode")
	}
	Decide(Parse([]string{ElevationAttempted}), false, launch)
	commands := [][]string{
		{"--self-test", "x"},
		{"--power-probe", "x", "2"},
		{"--third-party-notices", "x"},
		{"--screenshot", "x"},
		{"--dpi-preview", "x", "168"},
		{"--tray-preview", "x", "42", "charging"},
	}
	for _, args := range commands {
		r := Parse(args)
		if !r.Valid || r.Command == GUI {
			t.Errorf("CLI route valid without GUI: %v", args)
		}
		Decide(r, false, launch)
		missing := Parse(args[:1])
		if missing.Valid || Decide(missing, false, launch).Started {
			t.Errorf("missing CLI arguments fail without UAC: %v", args)
		}
	}
	if launches != 1 {
		t.Error("already elevated, explicit ordinary GUI, child marker and all CLI bypass UAC")
	}
	invalid := [][]string{
		{"--wat"},
		{"--self-test"},
		{"--no-elevate", "extra"},
		{"--power-probe", "x", "0"},
		{"--power-probe", "x", "3601"},
		{"--power-probe", "x", "2", "3"},
		{"--dpi-preview", "x", "bad"},
		{"--dpi-preview", "x"},
		{"--dpi-preview", "x", "95"},
		{"--tray-preview", "x", "42"},
		{"--tray-preview", "x", "42", "bogus"},
		{"--tray-preview", "x", " ", "charging"},
		{"--dpi-preview", "x", "96", "96", "96"},
		{"--self-test", " "},
		{"--self-test", "x", "y"},
		{"--no-elevate", "--self-test", "x"},
		{"--self-test", "x", "--no-elevate"},
		{"--no-elevate", "--autostart"},
		{"--help"},
		{"--version"},
		{"@args.rsp"},
		{"[diagram]", "--self-test", "x"},
		{"--remove-autostart", "extra"},
	}
	for _, args := range invalid {
		r := Parse(args)
		if r.Valid {
			t.Errorf("invalid arguments accepted: %s", strings.Join(args, " "))
		}
		Decide(r, false, launch)
	}
	if launches != 1 {
		t.Error("invalid route cannot launch UAC")
	}
}

func TestParsedArgumentsReachDispatch(t *testing.T) {
	probe := Parse([]string{"--POWER-PROBE", "out.txt", "12"})
	defaultProbe := Parse([]string{"--power-probe", "out.txt"})
	dpi := Parse([]string{"--dpi-preview", "shot.png", "168", "96"})
	tray := Parse([]string{"--tray-preview", "icon.png", "99+", "Discharging"})
	if !probe.Valid || probe.Command != PowerProbe || probe.Path != "out.txt" || probe.Seconds != 12 ||
		defaultProbe.Seconds != 5 || dpi.Path != "shot.png" || !slices.Equal(dpi.DPIs, []int{168, 96}) ||
		tray.Path != "icon.png" || tray.TrayGlyph != "99+" || !tray.TrayDischarging {
		t.Errorf("CLI routes carry parsed arguments: %+v %+v %+v %+v", probe, defaultProbe, dpi, tray)
	}
	if !Parse([]string{"--tray-preview", "i.png", "7", "CHARGING"}).Valid || Parse([]string{"--tray-preview", "i.png", "7", "charging"}).TrayDischarging {
		t.Error("tray state is case-insensitive")
	}
}

func TestLaunchFailuresKeepTheOrdinaryGui(t *testing.T) {
	cancel := Launch(func() (bool, error) { return false, syscall.Errno(1223) })
	failure := Launch(func() (bool, error) { return false, errors.New("failure") })
	notStarted := Launch(func() (bool, error) { return false, nil })
	if cancel.Started || cancel.Message != MessageCancelled || failure.Started ||
		!strings.Contains(failure.Message, "failure") || notStarted.Started || notStarted.Message != MessageNotStart {
		t.Errorf("cancel and launch failure retain ordinary GUI with reason: %+v %+v %+v", cancel, failure, notStarted)
	}
	if !Launch(func() (bool, error) { return true, nil }).Started {
		t.Error("a started elevated copy takes over")
	}
}

func TestAutostartRoutes(t *testing.T) {
	calls := 0
	launch := func() ElevationResult { calls++; return ElevationResult{Started: true} }
	r := Parse([]string{"--autostart"})
	Decide(r, false, launch)
	if !r.Valid || r.Command != GUI || !r.StartHidden || !r.SuppressElevation || calls != 0 {
		t.Error("autostart hides GUI and never requests UAC even with ordinary token")
	}
	cleanup := Parse([]string{"--remove-autostart"})
	Decide(cleanup, false, launch)
	if !cleanup.Valid || cleanup.Command == GUI || calls != 0 {
		t.Error("uninstall cleanup is non-GUI and never elevates")
	}
	if Parse([]string{"--autostart", "extra"}).Valid {
		t.Error("autostart rejects extra arguments")
	}
	sync := Parse([]string{"--sync-autostart"})
	Decide(sync, false, launch)
	if !sync.Valid || sync.Command != SyncAutostart || calls != 0 || Parse([]string{"--sync-autostart", "extra"}).Valid {
		t.Error("installer startup synchronization is non-GUI and never elevates")
	}
	if h := Parse([]string{"--elevation-attempted"}); !h.Handoff || h.StartHidden {
		t.Error("the elevated child is marked as the handoff")
	}
}
