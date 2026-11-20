package gui

import (
	"runtime"
	"runtime/metrics"
	"testing"
)

// TestWindowHeap draws the window as the app shows it, at 175%, and checks
// what the frame keeps in the Go heap. The window's own shapes must not
// take masks of its size: one path the size of the panel grows the glyph
// atlas to 4096×4096, 16 MB held while the window shows.
func TestWindowHeap(t *testing.T) {
	before := liveHeap()
	r := previewRecorder("settings")
	tt := r.tester(false)
	tt.SetScale(1.75)
	tt.Frame()
	grown := float64(liveHeap()-before) / (1 << 20)
	t.Logf("the window keeps %.1f MB", grown)
	if grown > 8 {
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
