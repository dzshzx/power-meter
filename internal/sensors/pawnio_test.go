package sensors

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dzshzx/power-meter/internal/power"
)

func TestOpenFailure(t *testing.T) {
	cases := []struct {
		hr       int32
		elevated bool
		want     string
	}{
		{hresultAccessDenied, false, power.ReasonElevationRequired},
		{hresultAccessDenied, true, "pawnio_open 失败: 0x80070005"},
		{-2147467259, false, "pawnio_open 失败: 0x80004005"},
	}
	for _, c := range cases {
		if got := describeOpenFailure(c.hr, c.elevated); got != c.want {
			t.Errorf("describeOpenFailure(%d, %v) = %q, want %q", c.hr, c.elevated, got, c.want)
		}
	}
	if hresult(0x1F) != "0x0000001F" {
		t.Error("an HRESULT has eight upper-case hex digits")
	}
}

func TestEnergyUnit(t *testing.T) {
	// MSR_RAPL_POWER_UNIT 0x000A0E03: energy status unit 14, 61 µJ.
	if !near(energyUnitJoules(0x000A0E03), 1.0/16384) || energyUnitJoules(0) != 1 ||
		!near(energyUnitJoules(0xFFFFFFFFFFFFFFFF), 1.0/(1<<31)) {
		t.Error("energy unit is 0.5 to the power of bits 12:8")
	}
}

func TestModuleOverride(t *testing.T) {
	dir := t.TempDir()
	embedded := []byte("embedded")
	if m, err := loadModule(dir, "IntelMSR.bin", embedded); err != nil || string(m) != "embedded" {
		t.Errorf("without a file beside the executable the embedded module loads: %q %v", m, err)
	}
	if m, err := loadModule(dir, "IntelMSR.bin", nil); err != nil || m != nil {
		t.Errorf("no module at all: %q %v", m, err)
	}
	if err := os.Mkdir(filepath.Join(dir, "IntelMSR.bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if m, _ := loadModule(dir, "IntelMSR.bin", embedded); string(m) != "embedded" {
		t.Error("a directory of that name does not replace the module")
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "IntelMSR.bin"), []byte("user build"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m, err := loadModule(other, "IntelMSR.bin", embedded); err != nil || string(m) != "user build" {
		t.Errorf("a file beside the executable replaces the embedded module: %q %v", m, err)
	}
}

// fakeMSR advances the package and platform counters by fixed steps on
// every read.
type fakeMSR struct {
	pkg, psys, pkgStep, psysStep uint64
	err                          error
	closed                       bool
}

func (f *fakeMSR) readMSR(msr uint32) (uint64, error) {
	if f.err != nil {
		return 0, f.err
	}
	switch msr {
	case msrPkgEnergyStatus:
		f.pkg += f.pkgStep
		return f.pkg, nil
	case msrPlatformEnergyStatus:
		f.psys += f.psysStep
		return f.psys, nil
	}
	return 0, errors.New("unexpected MSR")
}

func (f *fakeMSR) close() { f.closed = true }

// fakeClock is a clock that only moves when told.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time        { return c.t }
func (c *fakeClock) sleep(d time.Duration) { c.t = c.t.Add(d) }
func newClock() *fakeClock {
	return &fakeClock{time.Date(2026, 11, 19, 20, 0, 0, 0, time.FixedZone("", 8*3600))}
}
func pawnSensor(m msrReader, c *fakeClock) *pawnIOSensor {
	return &pawnIOSensor{msr: m, unitJoules: 1.0 / 16384, now: c.now, start: c.now()}
}

func TestPawnIORead(t *testing.T) {
	clock := newClock()
	msr := &fakeMSR{pkgStep: 12 * 16384, psysStep: 30 * 16384}
	s := pawnSensor(msr, clock)
	if p, _, ok := s.read(); ok || p.UnavailableReason != "正在建立基准" {
		t.Errorf("baseline: %+v", p)
	}
	clock.sleep(time.Second)
	p, pkg, ok := s.read()
	if !ok || !near(p.Watts, 30) || !near(pkg, 12) || p.Window != time.Second {
		t.Errorf("platform and package over one window: %+v %v", p, pkg)
	}
	msr.err = errors.New("MSR 0x611 读取失败: 0x80004005")
	if p, _, ok = s.read(); ok || p.UnavailableReason != "MSR 读取失败: MSR 0x611 读取失败: 0x80004005" {
		t.Errorf("read failure: %+v", p)
	}
	s.close()
	if !msr.closed {
		t.Error("close releases the handle")
	}
	if p, _, ok = s.read(); ok || p.Available {
		t.Error("a closed sensor reads nothing")
	}
	s.close()

	if p, _, ok = (&pawnIOSensor{reason: power.ReasonElevationRequired}).read(); ok || p.UnavailableReason != power.ReasonElevationRequired {
		t.Errorf("an unavailable sensor reports its reason: %+v", p)
	}
}
