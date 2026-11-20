//go:build windows

package sensors

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// TestBatteryQueryKeepsHandles reads the battery as the tray does once a
// second, beside a single-threaded apartment as the app's main thread
// keeps. Each read used to make the multithreaded apartment and tear it
// down again, leaving an event open every time. WMI opens some handles of
// its own over the first reads, more on some machines than others, and
// keeps them: they are counted out before the reads that are measured.
func TestBatteryQueryKeepsHandles(t *testing.T) {
	release := holdSTA(t)
	defer release()
	// A machine without a battery, as CI's, may fail the query; the
	// apartment is made and left all the same.
	if _, err := queryBatteryStatus(); err != nil {
		t.Log("battery query:", err)
	}
	for range 30 {
		_, _ = queryBatteryStatus()
	}
	before := handleCount(t)
	const reads = 60
	for range reads {
		_, _ = queryBatteryStatus()
	}
	if grown := handleCount(t) - before; grown > reads/4 {
		t.Fatalf("%d battery reads left %d handles open", reads, grown)
	}
}

var getProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

func handleCount(t *testing.T) int {
	var n uint32
	if r, _, err := getProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n))); r == 0 {
		t.Fatal(err)
	}
	return int(n)
}

// holdSTA keeps a single-threaded apartment on a thread of its own until
// release, as the app's main thread does.
func holdSTA(t *testing.T) (release func()) {
	ready, done := make(chan error), make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
			ready <- err
			return
		}
		defer ole.CoUninitialize()
		ready <- nil
		<-done
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	return func() { close(done) }
}
