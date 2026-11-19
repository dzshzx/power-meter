// Package app runs the meter: one capture a second off the main thread,
// the session on it, the window and the tray. The window only draws the
// model; this package carries out what it asks for.
package app

import (
	"fmt"
	"time"

	"github.com/dzshzx/power-meter/internal/autostart"
	"github.com/dzshzx/power-meter/internal/cli"
	"github.com/dzshzx/power-meter/internal/gui"
	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/dzshzx/power-meter/internal/sensors"
	"github.com/dzshzx/power-meter/internal/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Options are what the command line and the elevation decision gave.
type Options struct {
	StartHidden bool
	Elevated    bool
	// ElevationNotice says why the window runs without elevation.
	ElevationNotice string
	// Elevate starts an elevated copy; Started quits this one.
	Elevate func() cli.ElevationResult
	// Listen registers the callback a second launch triggers, on a
	// goroutine of its own, to bring the window back.
	Listen func(onRestore func()) error
}

type app struct {
	opts    Options
	session *meter.Session
	sources *sensors.Sources
	model   gui.Model
	win     *mygo.Window
	tray    *mygo.Tray
	menu    *mygo.Menu
	done    chan struct{}
	fitted  fit
	glyph   string
	accent  power.Accent
	// startupBusy: a startup change is running; lastChange is its notice,
	// and lastDone says the change took effect.
	startupBusy bool
	lastChange  string
	lastDone    bool
}

// fit is what the window was last fitted to: the content height and the
// work area and scale of its display.
type fit struct {
	content int
	area    mygo.Rectangle
	scale   float64
}

// Run shows the meter until the user exits, and returns the exit code.
func Run(o Options) int {
	a := &app{
		opts:    o,
		session: meter.NewSession(settings.Mode()),
		sources: sensors.New(),
		done:    make(chan struct{}),
		glyph:   "\x00",
	}
	defer a.sources.Close()
	a.model = gui.Model{
		Mode:            a.session.Mode(),
		Pinned:          true,
		Elevated:        o.Elevated,
		Language:        i18n.Preference(),
		ElevationNotice: o.ElevationNotice,
	}
	a.model.View = a.session.Render()
	a.model.Actions = gui.Actions{
		SetMode:      a.setMode,
		SetPinned:    a.setPinned,
		Minimize:     a.hide,
		Close:        mygo.App.Quit,
		SetAutostart: a.setStartup,
		SetLanguage:  a.setLanguage,
		Elevate:      a.elevate,
	}
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.stop() })
	mygo.App.WhenReady(a.start)
	if err := mygo.App.Run(); err != nil {
		return 1
	}
	a.stop()
	return 0
}

func (a *app) stop() {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}

func (a *app) start() {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             i18n.AppName(),
		Content:           ui.View(func(c *ui.Context) { gui.Build(c, &a.model) }),
		Width:             gui.Width,
		Height:            gui.Height(&a.model),
		UseContentSize:    true,
		Hidden:            a.opts.StartHidden,
		Frameless:         true,
		Transparent:       true,
		DisableShadow:     true,
		DisableResize:     true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		AlwaysOnTop:       true,
	})
	_ = a.win.SetIcon(gui.AppIcon(256))
	a.resize()
	a.win.OnMinimize(a.hide)
	a.win.OnFocus(a.refreshStartup)
	a.buildTray()
	if a.opts.Listen != nil {
		_ = a.opts.Listen(func() { mygo.RunOnMain(a.show) })
	}
	a.refreshStartup()
	go a.capture()
}

// capture reads the sources once a second until the app quits. Reading
// WMI takes tens of milliseconds, so it stays off the main thread.
func (a *app) capture() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		snapshot, failure := a.read()
		a.win.Update(func() {
			if failure != "" {
				a.present(a.session.Fail(failure))
			} else {
				a.present(a.session.Observe(snapshot))
			}
		})
		select {
		case <-a.done:
			return
		case <-tick.C:
		}
	}
}

// read captures one snapshot; a panic in a source becomes the failure the
// session shows, as an exception did.
func (a *app) read() (s *power.Snapshot, failure string) {
	defer func() {
		if r := recover(); r != nil {
			s, failure = nil, fmt.Sprint(r)
		}
	}()
	return a.sources.Capture(), ""
}

// present shows a view: the window's model, its height and the tray. Main
// thread only.
func (a *app) present(v *meter.View) {
	a.model.View = v
	a.resize()
	a.updateTray()
}

// resize fits the window to its content, within the work area of its
// display; the content scrolls where the screen is shorter. It runs with
// every view but acts only when the content height, the work area or the
// display's scale changed, so a new scaling refits the window within a
// second while a window the user moved stays where it is.
func (a *app) resize() {
	if a.win == nil {
		return
	}
	x, y := a.win.Position()
	d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x, Y: y})
	f := fit{content: gui.Height(&a.model), area: d.WorkArea, scale: d.ScaleFactor}
	if f == a.fitted {
		return
	}
	a.fitted = f
	a.win.SetContentSize(gui.Width, fitHeight(f.content, f.area.Height))
	a.setInputRegion()
	_, wh := a.win.Size()
	if top := fitTop(y, wh, d.WorkArea.Y, d.WorkArea.Height); top != y {
		a.win.SetPosition(x, top)
	}
}

// fitTop is where a window wh tall puts its top edge to stay inside a work
// area from top, height tall: moved up when it runs past the bottom, down
// when it starts above the top. A taller window keeps its title bar.
func fitTop(y, wh, top, height int) int {
	return max(top, min(y, top+height-wh))
}

// fitHeight is the window's content height for content on a work area
// workHeight tall: all of it, or the work area less a small margin.
func fitHeight(content, workHeight int) int {
	if limit := workHeight - 16; limit > 0 && content > limit {
		return limit
	}
	return content
}

// setInputRegion lets the mouse through the transparent room around the
// panel, where its shadow is, to the windows behind.
func (a *app) setInputRegion() {
	x, y, w, h := gui.Panel(&a.model)
	a.win.SetInputRegion([]mygo.Rectangle{{X: x, Y: y, Width: w, Height: h}})
}

func (a *app) setMode(m meter.Mode) {
	if m != a.session.Mode() {
		_ = settings.SaveMode(m)
	}
	a.model.Mode = m
	a.present(a.session.SetMode(m))
	a.checkMenu()
}

func (a *app) setPinned(on bool) {
	a.model.Pinned = on
	a.win.SetAlwaysOnTop(on)
}

// hide puts the window in the tray: no taskbar button until it is shown.
func (a *app) hide() {
	a.model.SettingsOpen = false
	a.win.Hide()
}

func (a *app) show() {
	a.win.Show()
	if a.win.IsMinimized() {
		a.win.Restore()
	}
	a.win.Focus()
}

func (a *app) setLanguage(lang string) {
	if err := i18n.Select(lang); err != nil {
		return
	}
	_ = settings.SaveLanguage(lang)
	a.model.Language = lang
	a.win.SetTitle(i18n.AppName())
	a.refreshStartup()
	a.present(a.session.Render())
	a.translateMenu()
}

func (a *app) elevate() {
	if a.opts.Elevate == nil {
		return
	}
	r := a.opts.Elevate()
	if r.Started {
		mygo.App.Quit()
		return
	}
	a.model.ElevationNotice = r.Message
	a.present(a.model.View)
}

// refreshStartup reads the startup switch off the main thread: the
// scheduler and the file system can block.
func (a *app) refreshStartup() {
	go func() {
		status := autostart.ReadStatus()
		a.win.Update(func() { a.applyStartup(status) })
	}()
}

// setStartup asks to turn logon startup on or off. The switch waits while
// the change runs; replacing another installation's startup asks first.
func (a *app) setStartup(on bool) {
	if a.startupBusy {
		return
	}
	a.startupBusy = true
	a.model.Autostart.Enabled = false
	go func() {
		notice, done := autostart.Change(on, a.confirm)
		status := autostart.ReadStatus()
		a.win.Update(func() {
			a.startupBusy = false
			a.lastChange, a.lastDone = notice, done
			a.applyStartup(status)
		})
	}()
}

// confirm asks a yes/no question in a dialog over the window. It runs on
// the goroutine of the change.
func (a *app) confirm(title, question string) bool {
	r, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:        a.win,
		Type:          mygo.MessageQuestion,
		Title:         title,
		Message:       question,
		Buttons:       []string{i18n.Get("是"), i18n.Get("否")},
		DefaultButton: 1,
		CancelButton:  1,
	})
	return err == nil && r.Button == 0
}

// applyStartup shows the switch and the notice: a standing problem first,
// else the outcome of the last change.
func (a *app) applyStartup(s autostart.Status) {
	a.model.Autostart = gui.Autostart{
		Checked: s.On,
		Enabled: !a.startupBusy,
		Tooltip: s.Tooltip,
	}
	a.model.StartupNotice, a.model.StartupConfirmed = s.Notice, false
	if a.model.StartupNotice == "" {
		a.model.StartupNotice, a.model.StartupConfirmed = a.lastChange, a.lastDone
	}
	if a.menu != nil {
		item := a.menu.ItemByID("autostart")
		item.SetChecked(a.model.Autostart.Checked)
		item.SetEnabled(a.model.Autostart.Enabled)
	}
	a.present(a.model.View)
}
