package thirdparty

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntelMSRMatchesNotice(t *testing.T) {
	sum := sha256.Sum256(IntelMSR)
	if len(IntelMSR) == 0 || !strings.Contains(Notice(), "`"+hex.EncodeToString(sum[:])+"`") {
		t.Error("the embedded IntelMSR.bin is the file whose SHA-256 NOTICE.md records")
	}
}

func TestNotices(t *testing.T) {
	notices := Notices()
	if !strings.HasPrefix(notices, "# Third-party components\n") {
		t.Error("the export starts with NOTICE.md")
	}
	for _, c := range components {
		if !strings.Contains(notices, "\n\n---\n"+c.heading()+"\n---\n"+text(c.file)) {
			t.Errorf("%s carries %s after its heading", c.name, c.file)
		}
	}
	// The release checks look for these two.
	for _, want := range []string{"GNU LESSER GENERAL PUBLIC LICENSE", "PawnIO.Modules-0.2.10-source.zip"} {
		if !strings.Contains(notices, want) {
			t.Errorf("the export lacks %q", want)
		}
	}
	if strings.ContainsAny(notices, "\r\uFEFF") {
		t.Error("the export has LF line endings and no byte order mark")
	}
}

func TestWriteNotices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notice.md")
	if err := WriteNotices(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != Notices() {
		t.Error("the export is the notice as UTF-8")
	}
	if WriteNotices(" ") == nil {
		t.Error("a blank path is rejected")
	}
}
