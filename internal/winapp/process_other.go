//go:build !windows

package winapp

import (
	"errors"
	"os"
	"time"

	"github.com/dzshzx/power-meter/internal/cli"
)

// The app ships for Windows only. Elsewhere the process is never elevated
// and every instance runs on its own, so the program still builds and runs
// for development.

var errUnsupported = errors.New("only supported on Windows")

// IsElevated is always false outside Windows.
func IsElevated() bool { return false }

// UserSID has no meaning outside Windows.
func UserSID() (string, error) { return "", errUnsupported }

// Identity is the executable itself outside Windows.
func Identity() string {
	exe, _ := os.Executable()
	return exe
}

// TryElevate never starts anything outside Windows.
func TryElevate() cli.ElevationResult {
	return cli.Launch(func() (bool, error) { return false, errUnsupported })
}

// Instance is a lock that never conflicts outside Windows.
type Instance struct{}

// Acquire always succeeds outside Windows.
func Acquire(string, bool) (*Instance, bool, error) { return &Instance{}, true, nil }

// Listen never calls back outside Windows.
func (*Instance) Listen(func()) error { return nil }

// Close does nothing outside Windows.
func (*Instance) Close() {}

// RestoreExisting finds no instance outside Windows.
func RestoreExisting(string) bool { return false }

// WithLock runs fn without a lock outside Windows.
func WithLock(_ string, _ time.Duration, fn func() error) (bool, error) { return true, fn() }
