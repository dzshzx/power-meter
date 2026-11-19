package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mygo.json")
	write := func(s string) {
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("{\n  \"name\": \"PowerMeter\",\n  \"version\": \"0.0.0\",\n  \"license\": \"MIT\"\n}\n")
	if err := setVersion(path, "1.6.0"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if want := "{\n  \"name\": \"PowerMeter\",\n  \"version\": \"1.6.0\",\n  \"license\": \"MIT\"\n}\n"; string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The placeholder is gone now: a second run fails instead of stamping twice.
	if err := setVersion(path, "1.6.1"); err == nil || !strings.Contains(err.Error(), "found 0") {
		t.Fatalf("second run: %v", err)
	}
	write(`{"version": "0.0.0"}`)
	for _, bad := range []string{"1.6", "v1.6.0", "1.6.0-rc1", ""} {
		if err := setVersion(path, bad); err == nil {
			t.Errorf("setVersion(%q) accepted", bad)
		}
	}
}

// The repository's own mygo.json carries the placeholder the release
// workflow replaces.
func TestRepositoryPlaceholder(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(), "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), string(placeholder)); n != 1 {
		t.Fatalf("mygo.json carries %s %d times, want once", placeholder, n)
	}
}
