//go:build windows

package winapp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMain lets a test run this binary as a second process: an instance of
// the installation named by the arguments (see probeInstance).
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "--instance-probe" {
		os.Exit(probeInstance(os.Args[2], os.Args[3]))
	}
	os.Exit(m.Run())
}

// probeInstance takes the lock of identity. The first instance writes
// first.txt in dir, waits up to a minute for a restore request and exits 0
// when one came (4 otherwise). A later instance asks the first one for its
// window, writes second.txt and exits 3 when the request was taken (5
// otherwise). Both record whether they run elevated.
func probeInstance(identity, dir string) int {
	inst, ok, err := Acquire(identity, false)
	if err != nil {
		fmt.Println("acquire:", err)
		return 1
	}
	if !ok {
		restored := RestoreExisting(identity)
		_ = os.WriteFile(filepath.Join(dir, "second.txt"), fmt.Appendf(nil, "elevated=%v restored=%v", IsElevated(), restored), 0o644)
		if restored {
			return 3
		}
		return 5
	}
	defer inst.Close()
	requests := make(chan struct{}, 1)
	if err := inst.Listen(func() { requests <- struct{}{} }); err != nil {
		fmt.Println("listen:", err)
		return 1
	}
	_ = os.WriteFile(filepath.Join(dir, "first.txt"), fmt.Appendf(nil, "elevated=%v", IsElevated()), 0o644)
	select {
	case <-requests:
		_ = os.WriteFile(filepath.Join(dir, "restored.txt"), []byte("restored"), 0o644)
		return 0
	case <-time.After(time.Minute):
		return 4
	}
}

func uniqueIdentity(t *testing.T) string {
	t.Helper()
	var b [8]byte
	_, _ = rand.Read(b[:])
	return filepath.Join(t.TempDir(), "pm-instance-"+hex.EncodeToString(b[:]), ExecutableName)
}

func TestTokenFacts(t *testing.T) {
	sid, err := UserSID()
	if err != nil || len(sid) < 8 || sid[:4] != "S-1-" {
		t.Fatalf("user SID: %q %v", sid, err)
	}
	exe, _ := os.Executable()
	if !SamePath(Identity(), exe) {
		t.Errorf("a process outside a protected copy is its own installation: %s %s", Identity(), exe)
	}
	t.Logf("elevated=%v sid=%s", IsElevated(), sid)
}

func TestInstanceLock(t *testing.T) {
	identity := uniqueIdentity(t)
	first, ok, err := Acquire(identity, false)
	if err != nil || !ok {
		t.Fatalf("the first instance takes the lock: %v", err)
	}
	if _, ok, err := Acquire(identity, false); ok || err != nil {
		t.Errorf("a second instance of the same installation does not: %v %v", ok, err)
	}
	other, ok, err := Acquire(uniqueIdentity(t), false)
	if err != nil || !ok {
		t.Errorf("another installation has a lock of its own: %v", err)
	} else {
		other.Close()
	}
	if RestoreExisting(identity) {
		t.Error("nothing takes a restore request before the instance listens")
	}
	requests := make(chan struct{}, 2)
	if err := first.Listen(func() { requests <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	if !RestoreExisting(identity) {
		t.Fatal("the listening instance takes the restore request")
	}
	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		t.Fatal("the restore callback ran")
	}
	first.Close()
	again, ok, err := Acquire(identity, false)
	if err != nil || !ok {
		t.Fatalf("a closed instance releases the lock: %v", err)
	}
	again.Close()
	if RestoreExisting(identity) {
		t.Error("a closed instance stops listening")
	}
}

// The elevated copy started by the elevation request waits for the process
// that started it to let go.
func TestHandoffWaitsForTheLock(t *testing.T) {
	identity := uniqueIdentity(t)
	parent, ok, err := Acquire(identity, false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	type result struct {
		inst *Instance
		ok   bool
		err  error
		at   time.Time
	}
	child := make(chan result, 1)
	go func() {
		inst, ok, err := Acquire(identity, true)
		child <- result{inst, ok, err, time.Now()}
	}()
	time.Sleep(500 * time.Millisecond)
	released := time.Now()
	parent.Close()
	r := <-child
	if r.err != nil || !r.ok || r.at.Before(released) {
		t.Fatalf("the handoff waits for the parent and then takes the lock: %+v", r)
	}
	r.inst.Close()
}

func TestWithLock(t *testing.T) {
	name := "PowerMeter.Test." + filepath.Base(filepath.Dir(uniqueIdentity(t)))
	held, err := WithLock(name, time.Second, func() error {
		inner := make(chan bool, 1)
		go func() {
			held, _ := WithLock(name, 100*time.Millisecond, func() error { return nil })
			inner <- held
		}()
		if <-inner {
			t.Error("a second holder waits and gives up")
		}
		return nil
	})
	if err != nil || !held {
		t.Errorf("the first holder runs: %v %v", held, err)
	}
}
