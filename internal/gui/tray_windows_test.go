//go:build windows

package gui

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"testing"
	"unsafe"

	"github.com/dzshzx/power-meter/internal/power"
	"github.com/egoist/mygo/ui"
	"golang.org/x/sys/windows"
)

// TestTrayIconKeepsMemory redraws the tray as each new reading does. A
// renderer made per image registers the app's fonts in it again, natively,
// and never releases them: about 23 MB per image with MiSans, so an
// elevated tray, whose Psys figure changes every second, grew by 10 to
// 20 MB a second. Arial stands in for MiSans, which this build may not
// embed, at about 1 MB per image.
func TestTrayIconKeepsMemory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ui.RegisterFont(data, "TrayIconMemoryTest"); err != nil {
		t.Fatal(err)
	}
	TrayIcon("10", power.AccentCharging)
	before := privateBytes(t)
	const images = 40
	for i := range images {
		TrayIcon(strconv.Itoa(11+i), power.AccentDischarging)
	}
	if grown := float64(privateBytes(t)-before) / (1 << 20); grown > 10 {
		t.Fatalf("%d tray images grew private memory by %.0f MB (%.2f MB each)", images, grown, grown/images)
	}
}

var getProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// privateBytes is the process's private commit, after the Go heap has
// returned what it can, so that only native memory that is kept counts.
func privateBytes(t *testing.T) int64 {
	debug.FreeOSMemory()
	var c struct {
		cb, pageFaults                                       uint32
		peakWorkingSet, workingSet                           uintptr
		quotaPeakPaged, quotaPaged, quotaPeakNonPaged, quota uintptr
		pagefile, peakPagefile, private                      uintptr
	}
	c.cb = uint32(unsafe.Sizeof(c))
	if r, _, err := getProcessMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&c)), uintptr(c.cb)); r == 0 {
		t.Fatal(err)
	}
	return int64(c.private)
}
