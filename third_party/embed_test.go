package thirdparty

import (
	"bytes"
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
	if !strings.HasPrefix(notices, "# Third-party components\r\n") {
		t.Error("the export starts with NOTICE.md")
	}
	for _, c := range components {
		if !strings.Contains(notices, "\r\n\r\n---\r\n"+c.heading()+"\r\n---\r\n"+crlf(text(c.file))) {
			t.Errorf("%s carries %s after its heading", c.name, c.file)
		}
	}
	// The release checks look for these two.
	for _, want := range []string{"GNU LESSER GENERAL PUBLIC LICENSE", "PawnIO.Modules-0.2.10-source.zip"} {
		if !strings.Contains(notices, want) {
			t.Errorf("the export lacks %q", want)
		}
	}
	if strings.Count(notices, "\n") != strings.Count(notices, "\r\n") || strings.Contains(notices, "\r\r") {
		t.Error("every line of the export ends in CRLF")
	}
	if strings.Contains(notices, byteOrderMark) {
		t.Error("embedded byte order marks are dropped")
	}
}

func TestCRLF(t *testing.T) {
	if got := crlf("a\nb\r\nc\n"); got != "a\r\nb\r\nc\r\n" {
		t.Errorf("crlf = %q", got)
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
	if string(data) != byteOrderMark+Notices() || !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF, '#'}) {
		t.Error("the export is the notice as UTF-8 with a byte order mark")
	}
	if WriteNotices(" ") == nil {
		t.Error("a blank path is rejected")
	}
}
