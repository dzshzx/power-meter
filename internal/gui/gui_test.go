package gui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/dzshzx/power-meter/internal/fonts"
	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
	"github.com/dzshzx/power-meter/internal/power"
	"github.com/egoist/mygo/ui"
)

// recorder is a model whose actions record what the window asked for.
type recorder struct {
	model     Model
	modes     []meter.Mode
	pinned    []bool
	minimized int
	closed    int
	autostart []bool
	languages []string
	elevated  int
}

func newRecorder(scenario string) *recorder {
	v, mode := fixture(scenario)
	r := &recorder{}
	r.model = Model{View: v, Mode: mode, Pinned: true, Language: i18n.System, Autostart: Autostart{Enabled: true}}
	r.model.Actions = Actions{
		SetMode:      func(m meter.Mode) { r.modes = append(r.modes, m) },
		SetPinned:    func(on bool) { r.pinned = append(r.pinned, on) },
		Minimize:     func() { r.minimized++ },
		Close:        func() { r.closed++ },
		SetAutostart: func(on bool) { r.autostart = append(r.autostart, on) },
		SetLanguage:  func(l string) { r.languages = append(r.languages, l) },
		Elevate:      func() { r.elevated++ },
	}
	return r
}

func (r *recorder) view(c *ui.Context) { Build(c, &r.model) }

func (r *recorder) tester(dark bool) *ui.Tester {
	return ui.NewTesterWith(r.view, Width, Height(&r.model), ui.TesterOptions{Dark: dark})
}

func eachLanguage(t *testing.T, fn func(t *testing.T)) {
	for _, lang := range []string{i18n.Chinese, i18n.English} {
		t.Run(lang, func(t *testing.T) {
			if err := i18n.Select(lang); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = i18n.Select(i18n.Chinese) }()
			fn(t)
		})
	}
}

var cjk = regexp.MustCompile(`\p{Han}`)

func TestScenariosFitTheWindow(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		for _, scenario := range scenarios {
			r := newRecorder(scenario)
			tt := r.tester(false)
			m := &r.model
			wantNotice := scenario == "unavailable"
			if got := m.notice() != ""; got != wantNotice {
				t.Errorf("%s: notice shown %v", scenario, got)
			}
			restart := i18n.Get("以管理员身份重新启动")
			if tt.HasText(restart) != wantNotice {
				t.Errorf("%s: the restart action belongs to the elevation-gated notice", scenario)
			}
			// The headline's boundary shows once, in the hero card; the rows
			// repeat only the three supporting figures.
			for _, name := range []string{i18n.Get("电池端净功率"), i18n.Get("CPU 包功率"), i18n.Get("平台功率"), m.View.WholeCaption} {
				if n := count(tt.Texts(), name); n != 1 {
					t.Errorf("%s: %q shows %d times: %q", scenario, name, n, tt.Texts())
				}
			}
			if i18n.IsChinese() == false {
				for _, text := range tt.Texts() {
					if cjk.MatchString(text) {
						t.Errorf("%s: untranslated text %q", scenario, text)
					}
				}
			}
			assertFits(t, scenario, tt)
		}
	})
}

// assertFits checks that every text lies inside the window, above the
// footer unless it is in it, and overlaps no other text: the fixed section
// heights hold their content.
func assertFits(t *testing.T, scenario string, tt *ui.Tester) {
	t.Helper()
	type box struct {
		text string
		r    ui.Rect
	}
	var boxes []box
	for _, text := range tt.Texts() {
		r, ok := tt.Find(text)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		if r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > Width+0.5 {
			t.Errorf("%s: %q is clipped or outside the window: %+v", scenario, text, r)
		}
		boxes = append(boxes, box{text, r})
	}
	for i, a := range boxes {
		for _, b := range boxes[i+1:] {
			if a.text != b.text && overlaps(a.r, b.r) && !contains(a.r, b.r) && !contains(b.r, a.r) {
				t.Errorf("%s: %q %+v overlaps %q %+v", scenario, a.text, a.r, b.text, b.r)
			}
		}
	}
}

// contains: a labelled container, as the segmented track, holds its texts.
func contains(a, b ui.Rect) bool {
	return a.X <= b.X && a.Y <= b.Y && a.X+a.W >= b.X+b.W && a.Y+a.H >= b.Y+b.H
}

func overlaps(a, b ui.Rect) bool {
	return a.X < b.X+b.W-0.5 && b.X < a.X+a.W-0.5 && a.Y < b.Y+b.H-0.5 && b.Y < a.Y+a.H-0.5
}

func count(texts []string, s string) int {
	n := 0
	for _, text := range texts {
		if text == s {
			n++
		}
	}
	return n
}

func TestFullWindowCaptions(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		tt := newRecorder("idle").tester(false)
		for _, key := range []string{"10 分钟均值", "5 分钟峰值"} {
			if !tt.HasText(i18n.Get(key)) {
				t.Errorf("a full window has the concise caption %q: %q", i18n.Get(key), tt.Texts())
			}
		}
		warming := newRecorder("warming")
		tt = warming.tester(false)
		avg, peak := warming.model.View.AverageCaption, warming.model.View.PeakCaption
		if !strings.Contains(avg, i18n.Format("{0}分", "2")) && !strings.Contains(avg, "2m") ||
			!tt.HasText(avg) || !tt.HasText(peak) || strings.HasPrefix(warming.model.View.AverageValue, "--") {
			t.Errorf("a partial window shows its coverage and averages what it has: %q %q %q", avg, peak, warming.model.View.AverageValue)
		}
	})
}

func TestActions(t *testing.T) {
	eachLanguage(t, func(t *testing.T) {
		r := newRecorder("unavailable")
		tt := r.tester(false)
		for _, label := range []string{"置顶", "最小化", "关闭", "电池端", "以管理员身份重新启动"} {
			if err := tt.Click(i18n.Get(label)); err != nil {
				t.Fatalf("click %s: %v", label, err)
			}
		}
		if len(r.pinned) != 1 || r.pinned[0] || r.minimized != 1 || r.closed != 1 ||
			len(r.modes) != 1 || r.modes[0] != meter.Battery || r.elevated != 1 {
			t.Errorf("caption buttons, the mode and the restart reach the app: %+v", r)
		}
		if err := tt.Click(i18n.Get("设置")); err != nil {
			t.Fatal(err)
		}
		if !r.model.SettingsOpen || !tt.HasText(i18n.Get("开机自启")) || !tt.HasText(i18n.Get("普通模式")) {
			t.Fatalf("settings open with the startup switch and the permission: %q", tt.Texts())
		}
		if err := tt.Click(i18n.Get("开机自启")); err != nil {
			t.Fatal(err)
		}
		if len(r.autostart) != 1 || !r.autostart[0] {
			t.Errorf("the switch asks to enable startup: %v", r.autostart)
		}
		if err := tt.Click(i18n.Get("语言") + " / Language"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("English"); err != nil {
			t.Fatalf("choose English: %v (%q)", err, tt.Texts())
		}
		if len(r.languages) != 1 || r.languages[0] != i18n.English {
			t.Errorf("the language choice reaches the app: %v", r.languages)
		}
	})
}

func TestElevatedWindowOffersNoRestart(t *testing.T) {
	r := newRecorder("unavailable")
	r.model.Elevated = true
	tt := r.tester(false)
	if tt.HasText(i18n.Get("以管理员身份重新启动")) || r.model.offersElevation() {
		t.Error("an elevated window offers no restart")
	}
	if err := tt.Click(i18n.Get("设置")); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(i18n.Get("管理员模式")) || tt.HasText(i18n.Get("以管理员身份重新启动")) {
		t.Errorf("elevated settings show the mode without the restart: %q", tt.Texts())
	}
}

// startupEnabled is the notice after the switch turned startup on.
const startupEnabled = "已启用开机自启：登录后以管理员权限运行受保护副本，在托盘显示。"

func TestNoticeWarnsUnlessItOnlyConfirms(t *testing.T) {
	if !previewRecorder("startup").model.confirmsOnly() {
		t.Error("a confirmed startup change alone shows no warning")
	}
	failed := newRecorder("charging")
	failed.model.StartupNotice = "自启设置未完成：拒绝访问。"
	withReason := previewRecorder("startup")
	withReason.model.ElevationNotice = "已取消管理员权限请求。"
	withProblem := newRecorder("unavailable")
	withProblem.model.StartupNotice, withProblem.model.StartupConfirmed = startupEnabled, true
	for name, r := range map[string]*recorder{"failed change": failed, "elevation reason": withReason, "diagnostics": withProblem} {
		if r.model.confirmsOnly() {
			t.Errorf("%s: the card keeps its warning", name)
		}
	}
}

func TestDisabledAutostartIgnoresClicks(t *testing.T) {
	r := newRecorder("idle")
	r.model.Autostart = Autostart{Enabled: false, Tooltip: "why"}
	r.model.SettingsOpen = true
	tt := r.tester(false)
	_ = tt.Click(i18n.Get("开机自启"))
	if len(r.autostart) != 0 {
		t.Error("a disabled switch does not ask")
	}
}

func TestSplitFigure(t *testing.T) {
	cases := map[string]figure{
		"≈ 19.39 W": {"≈", "19.39", "W"},
		"−17.50 W":  {"", "−17.50", "W"},
		"98%":       {"", "98", "%"},
		"--%":       {"", "--", "%"},
		"N/A":       {"", "N/A", ""},
		"--":        {"", "--", ""},
		"≈ 2.23 A":  {"≈", "2.23", "A"},
	}
	for text, want := range cases {
		if got := splitFigure(text); got != want {
			t.Errorf("splitFigure(%q) = %+v, want %+v", text, got, want)
		}
	}
}

func TestTrayIcons(t *testing.T) {
	for _, glyph := range []string{"8", "42", "99+", "--"} {
		for _, accent := range []power.Accent{power.AccentCharging, power.AccentDischarging, power.AccentIdle} {
			img, err := png.Decode(strings.NewReader(string(TrayIcon(glyph, accent))))
			if err != nil || img.Bounds().Dx() != TrayIconSize {
				t.Fatalf("tray icon %q: %v", glyph, err)
			}
			tile := img.At(TrayIconSize/2, 6)
			if _, _, _, a := tile.RGBA(); a == 0 {
				t.Errorf("tray icon %q has no tile", glyph)
			}
			if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
				t.Errorf("tray icon %q has an opaque corner", glyph)
			}
		}
	}
}

// TestTrayIconRedrawsEachReading: each tray image is the one a renderer of
// its own draws, also after other readings were drawn before it.
func TestTrayIconRedrawsEachReading(t *testing.T) {
	for _, c := range []struct {
		glyph  string
		accent power.Accent
	}{{"11", power.AccentCharging}, {"99+", power.AccentDischarging}, {"11", power.AccentCharging}, {"--", power.AccentIdle}} {
		fresh := trayImage{glyph: c.glyph, tile: trayTile(c.accent)}
		want := encode(ui.Render(fresh.view, TrayIconSize, TrayIconSize, 1))
		if got := TrayIcon(c.glyph, c.accent); !bytes.Equal(got, want) {
			t.Errorf("tray icon %q, accent %d differs from a fresh drawing", c.glyph, c.accent)
		}
	}
}

// previewRecorder is the recorder for a preview: a fixture scenario, or
// the charging window with the settings popover open ("settings") or a
// confirmed startup change ("startup").
func previewRecorder(scenario string) *recorder {
	switch scenario {
	case "settings":
		r := newRecorder("charging")
		r.model.SettingsOpen = true
		return r
	case "startup":
		r := newRecorder("charging")
		r.model.Elevated = true
		r.model.StartupNotice, r.model.StartupConfirmed = startupEnabled, true
		return r
	}
	return newRecorder(scenario)
}

// TestPreviews writes every scenario in both languages and appearances at
// two scales to $POWERMETER_PREVIEW_DIR, for review beside the previous
// release's captures. The faces are embedded, so a run on Linux draws the
// same MiSans as Windows; without them the previews would not show the
// shipped look, so the test refuses to run.
func TestPreviews(t *testing.T) {
	dir := os.Getenv("POWERMETER_PREVIEW_DIR")
	if dir == "" {
		t.Skip("POWERMETER_PREVIEW_DIR is not set")
	}
	if !fonts.Embedded() {
		t.Fatal("MiSans is missing: run go run ./tools/fonts first")
	}
	eachLanguage(t, func(t *testing.T) {
		lang := "zh-CN"
		if !i18n.IsChinese() {
			lang = "en"
		}
		for _, scenario := range append(scenarios, "settings", "startup") {
			for _, dark := range []bool{false, true} {
				for _, scale := range []float32{1, 1.75} {
					r := previewRecorder(scenario)
					tt := r.tester(dark)
					tt.SetScale(scale)
					tt.Frame()
					appearance := "light"
					if dark {
						appearance = "dark"
					}
					path := filepath.Join(dir, lang+"-"+scenario+"-"+appearance+"-"+strconv.Itoa(int(scale*100))+".png")
					writePNG(t, path, tt.Image())
				}
			}
		}
	})
	for _, accent := range []power.Accent{power.AccentCharging, power.AccentDischarging, power.AccentIdle} {
		if err := os.WriteFile(filepath.Join(dir, "tray-"+strconv.Itoa(int(accent))+".png"), TrayIcon("42", accent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "app-256.png"), AppIcon(256), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestSelfTestCases(t *testing.T) {
	for _, c := range SelfTest() {
		if !c.Pass() {
			t.Errorf("self-test case failed: %s", c.Name)
		}
	}
}
