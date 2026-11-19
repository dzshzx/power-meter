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
	if !strings.HasPrefix(notices, "# Third-party components") {
		t.Error("the export starts with NOTICE.md")
	}
	lgpl := "\r\n\r\n---\r\nGNU Lesser General Public License 2.1\r\n---\r\n"
	if !strings.Contains(notices, lgpl+"                  GNU LESSER GENERAL PUBLIC LICENSE") {
		t.Error("the LGPL-2.1 text follows its heading")
	}
	for _, s := range sections {
		if !strings.Contains(notices, s.heading+"\r\n"+text(s.file)) {
			t.Errorf("section %q carries %s", s.heading, s.file)
		}
	}
	if strings.Contains(notices, "\uFEFF") {
		t.Error("embedded byte order marks are dropped")
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
	if string(data) != "\uFEFF"+Notices() {
		t.Error("the export is the notice as UTF-8 with a byte order mark")
	}
	if WriteNotices(" ") == nil {
		t.Error("a blank path is rejected")
	}
}
