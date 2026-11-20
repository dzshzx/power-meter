package gui

import (
	"runtime"
	"runtime/metrics"
	"testing"
)

// TestWindowHeap draws the window as the app shows it, at 175%, and checks
// what the frame keeps in the Go heap. The window's own shapes must take no
// mask along a side (shape.go): the glyph atlas, the process's, would grow
// to 2048×2048 for a side's edge drawn as a path and to 4096×4096 for the
// panel as whole paths, and keep the 4 or 16 MB in the tray too.
func TestWindowHeap(t *testing.T) {
	before := liveHeap()
	r := previewRecorder("settings")
	tt := r.tester(false)
	tt.SetScale(1.75)
	tt.Frame()
	grown := float64(liveHeap()-before) / (1 << 20)
	t.Logf("the window keeps %.1f MB", grown)
	if grown > 4 {
		t.Fatalf("drawing the window keeps %.1f MB of the Go heap", grown)
	}
	runtime.KeepAlive(tt)
}

func liveHeap() uint64 {
	runtime.GC()
	s := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}
