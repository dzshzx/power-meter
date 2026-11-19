//go:build !windows

package autostart

// Logon startup exists only on Windows. Elsewhere the commands fail and the
// switch reads as unknown, so the program still builds for development.

const unsupported = "only supported on Windows"

// RunSync fails outside Windows.
func RunSync() int { return 1 }

// RunRemove fails outside Windows.
func RunRemove() int { return 1 }

// SyncOnStart does nothing outside Windows.
func SyncOnStart() {}

// ReadStatus is unknown outside Windows.
func ReadStatus() Status { return readFailed(unsupported) }

// Change fails outside Windows.
func Change(bool, func(title, question string) bool) string { return changeFailed + unsupported }
