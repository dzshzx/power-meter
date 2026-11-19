package fonts

import (
	"os"
	"testing"
)

// Register succeeds with the faces and without them (a checkout that has not
// fetched them), and keeps its first result.
func TestRegister(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	if err := Register(); err != nil {
		t.Fatal("registering twice keeps the first result:", err)
	}
}

// Embedded says whether all three faces were in misans/ when the package was
// compiled.
func TestEmbedded(t *testing.T) {
	all := true
	for _, f := range faces {
		if _, err := os.Stat(f.file); err != nil {
			all = false
		}
	}
	if Embedded() != all {
		t.Fatalf("Embedded() = %v, but the faces on disk say %v", Embedded(), all)
	}
}
