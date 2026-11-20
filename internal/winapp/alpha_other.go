//go:build !windows

package winapp

// BlendPixels does nothing off Windows.
func BlendPixels(uintptr) error { return nil }
