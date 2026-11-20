//go:build windows

package winapp

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dwmapi                        = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmEnableBlurBehindWindow = dwmapi.NewProc("DwmEnableBlurBehindWindow")
	procDwmSetWindowAttribute     = dwmapi.NewProc("DwmSetWindowAttribute")
	gdi32                         = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRectRgn             = gdi32.NewProc("CreateRectRgn")
	procDeleteObject              = gdi32.NewProc("DeleteObject")
)

// BlendPixels has DWM blend the window over what is behind it by the
// alpha of each pixel GDI puts in its redirection bitmap, as a transparent
// window shows, without DirectComposition or Direct3D. Blur behind an
// empty region does that since Windows 8, where it blurs nothing; a frame
// extended over the window would not, as Windows 11 paints the frame's
// colour under it. The corners Windows 11 rounds are left square: the
// window draws its own.
func BlendPixels(hwnd uintptr) error {
	region, _, _ := procCreateRectRgn.Call(0, 0, ^uintptr(0), ^uintptr(0)) // (0, 0, -1, -1): empty
	if region != 0 {
		defer procDeleteObject.Call(region)
	}
	const enable, blurRegion = 0x1, 0x2
	bb := struct {
		flags      uint32
		enable     int32
		region     uintptr
		transition int32
	}{flags: enable | blurRegion, enable: 1, region: region}
	if r, _, _ := procDwmEnableBlurBehindWindow.Call(hwnd, uintptr(unsafe.Pointer(&bb))); r != 0 {
		return windows.Errno(r)
	}
	const cornerPreference, doNotRound = 33, 1
	v := uint32(doNotRound)
	procDwmSetWindowAttribute.Call(hwnd, cornerPreference, uintptr(unsafe.Pointer(&v)), unsafe.Sizeof(v))
	return nil
}
