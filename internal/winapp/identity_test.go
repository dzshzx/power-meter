package winapp

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

const testSID = "S-1-5-21-111-222-333-1001"

func TestFullPath(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{`C:\Program Files\Power Meter\PowerMeter.exe`, `C:\Program Files\Power Meter\PowerMeter.exe`, true},
		{`C:/Test & 测试//Meter 1.2.1.exe`, `C:\Test & 测试\Meter 1.2.1.exe`, true},
		{`c:\a\.\b\..\c.exe`, `c:\a\c.exe`, true},
		{`C:\..\..\x`, `C:\x`, true},
		{`C:\`, `C:\`, true},
		{`\\server\share\dir\..\f.exe`, `\\server\share\f.exe`, true},
		{`\\server\share`, `\\server\share\`, true},
		{`\\?\C:\long\path.exe`, `C:\long\path.exe`, true},
		{`\\?\UNC\server\share\f.exe`, `\\server\share\f.exe`, true},
		{`relative\path.exe`, "", false},
		{`C:relative.exe`, "", false},
		{`\rooted\no-drive.exe`, "", false},
		{`\\server`, "", false},
		{`\\.\pipe\x`, "", false},
		{``, "", false},
		{`"C:\quoted.exe"`, "", false},
	}
	for _, c := range cases {
		got, ok := FullPath(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("FullPath(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestSamePath(t *testing.T) {
	if !SamePath(`C:\Program Files\Power Meter\autostart\S-1\POWERMETER.EXE`, `c:/program files/power meter/autostart/s-1/PowerMeter.exe`) {
		t.Error("paths compare case-insensitively across separators")
	}
	if !SamePath(`C:\a\b\..\c.exe`, `C:\a\c.exe`) {
		t.Error("dot segments resolve before comparing")
	}
	if SamePath(`C:\a\c.exe`, `D:\a\c.exe`) || SamePath(`a\c.exe`, `a\c.exe`) || SamePath("", "") {
		t.Error("different drives and relative paths never match")
	}
}

func TestSplitPath(t *testing.T) {
	for _, c := range []struct{ in, dir, name string }{
		{`C:\a\b.exe`, `C:\a`, "b.exe"},
		{`C:\b.exe`, `C:\`, "b.exe"},
		{`\\server\share\b.exe`, `\\server\share`, "b.exe"},
	} {
		if dir, name := SplitPath(c.in); dir != c.dir || name != c.name {
			t.Errorf("SplitPath(%q) = %q, %q", c.in, dir, name)
		}
	}
}

// files fakes the file system for identityFor.
func files(m map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if v, ok := m[p]; ok {
			return []byte(v), nil
		}
		return nil, fs.ErrNotExist
	}
}

func TestProtectedCopyIdentity(t *testing.T) {
	none := files(nil)
	copyDir := `C:\Program Files\Power Meter\autostart\` + testSID
	cases := []struct {
		name, image, want string
		read              func(string) ([]byte, error)
	}{
		{"an ordinary installation is itself", `C:\Elsewhere\PowerMeter.exe`, `C:\Elsewhere\PowerMeter.exe`, none},
		{"another name in a SID folder is itself", `C:\Missing\` + testSID + `\Other.exe`, `C:\Missing\` + testSID + `\Other.exe`, none},
		{"a copy without source.txt is itself", `C:\Missing\` + testSID + `\PowerMeter.exe`, `C:\Missing\` + testSID + `\PowerMeter.exe`, none},
		{"a protected copy is its recorded source", copyDir + `\PowerMeter.exe`, `C:\Users\Me\Power Meter\PowerMeter.exe`,
			files(map[string]string{copyDir + `\source.txt`: "\uFEFF  C:/Users/Me/Power Meter/./PowerMeter.exe\r\n"})},
		{"names compare case-insensitively", strings.ToLower(copyDir) + `\powermeter.EXE`, `D:\Portable\PowerMeter.exe`,
			files(map[string]string{strings.ToLower(copyDir) + `\source.txt`: `D:\Portable\PowerMeter.exe`})},
		{"a relative source is ignored", copyDir + `\PowerMeter.exe`, copyDir + `\PowerMeter.exe`,
			files(map[string]string{copyDir + `\source.txt`: `Portable\PowerMeter.exe`})},
		{"an empty source is ignored", copyDir + `\PowerMeter.exe`, copyDir + `\PowerMeter.exe`,
			files(map[string]string{copyDir + `\source.txt`: "  "})},
		{"another user's copy is itself", `C:\Program Files\Power Meter\autostart\S-1-5-21-9\PowerMeter.exe`,
			`C:\Program Files\Power Meter\autostart\S-1-5-21-9\PowerMeter.exe`,
			files(map[string]string{`C:\Program Files\Power Meter\autostart\S-1-5-21-9\source.txt`: `C:\x.exe`})},
		{"an unreadable source is ignored", copyDir + `\PowerMeter.exe`, copyDir + `\PowerMeter.exe`,
			func(string) ([]byte, error) { return nil, errors.New("denied") }},
	}
	for _, c := range cases {
		if got := identityFor(c.image, testSID, c.read); got != c.want {
			t.Errorf("%s: identityFor(%q) = %q, want %q", c.name, c.image, got, c.want)
		}
	}
}

func TestInstanceKey(t *testing.T) {
	a := instanceKey(testSID, `C:\Users\Me\Power Meter\PowerMeter.exe`)
	if a != instanceKey(testSID, `c:/users/me/power meter/./POWERMETER.exe`) {
		t.Error("one installation has one key, whatever the spelling of its path")
	}
	if a == instanceKey(testSID, `D:\Portable\PowerMeter.exe`) || a == instanceKey("S-1-5-21-9", `C:\Users\Me\Power Meter\PowerMeter.exe`) {
		t.Error("other installations and other users have their own keys")
	}
	if !strings.HasPrefix(a, testSID+".") || len(a) != len(testSID)+1+64 || strings.ToUpper(a) != a {
		t.Errorf("key is the SID and an upper-case SHA-256: %q", a)
	}
}
