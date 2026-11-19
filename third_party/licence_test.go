package thirdparty

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The licence check: the components table must name every module the
// Windows executable links, at the version it links, with the module's own
// licence text and a licence this project accepts. Upgrading a dependency
// without updating the notice fails here.

// allowedLicences may appear in any component's SPDX expression.
var allowedLicences = map[string]bool{
	"MIT": true, "BSD-2-Clause": true, "BSD-3-Clause": true, "Apache-2.0": true, "ISC": true, "OFL-1.1": true,
}

// namedComponents may use a licence outside allowedLicences, each only its
// own: the PawnIO module, which is replaceable and ships with its
// corresponding source, and the MiSans fonts, which the app may embed but
// not distribute on their own.
var namedComponents = map[string]string{"IntelMSR.bin": "LGPL-2.1-or-later", "MiSans": "LicenseRef-MiSans"}

// goCommand runs the go command from the module root.
func goCommand(t *testing.T, env []string, args ...string) string {
	t.Helper()
	tool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go command is not on PATH")
	}
	cmd := exec.Command(tool, args...)
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, exit.Stderr)
		}
		t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

type linkedModule struct{ version, dir string }

// linkedModules lists the modules of every package the module's commands
// and libraries build from for windows/amd64, without tests and without the
// tools that go.mod's tool directive brings.
func linkedModules(t *testing.T) map[string]linkedModule {
	out := goCommand(t, []string{"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0"},
		"list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}", "./...")
	modules := map[string]linkedModule{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) == 3 {
			modules[fields[0]] = linkedModule{fields[1], fields[2]}
		}
	}
	if len(modules) == 0 {
		t.Fatal("go list found no modules")
	}
	return modules
}

func componentNamed(name string) (component, bool) {
	for _, c := range components {
		if c.name == name {
			return c, true
		}
	}
	return component{}, false
}

// sameText compares licence texts regardless of line endings, which a
// checkout may convert.
func sameText(t *testing.T, path, embedded string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", path, err)
		return false
	}
	normalize := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	return normalize(string(data)) == normalize(text(embedded))
}

func TestLinkedModulesHaveNotices(t *testing.T) {
	linked := linkedModules(t)
	for path, m := range linked {
		c, ok := componentNamed(path)
		switch {
		case !ok || !c.module:
			t.Errorf("%s %s is linked but has no section in the notice", path, m.version)
		case c.version != m.version:
			t.Errorf("%s is linked at %s but the notice says %s", path, m.version, c.version)
		case !sameText(t, filepath.Join(m.dir, "LICENSE"), c.file):
			t.Errorf("%s's LICENSE differs from %s", path, c.file)
		}
	}
	for _, c := range components {
		if _, ok := linked[c.name]; c.module && !ok {
			t.Errorf("%s has a section but is no longer linked", c.name)
		}
	}
}

func TestGoToolchainNotice(t *testing.T) {
	env := strings.Fields(goCommand(t, nil, "env", "GOVERSION", "GOROOT"))
	if len(env) != 2 {
		t.Fatalf("go env: %q", env)
	}
	c, ok := componentNamed("Go standard library and runtime")
	if !ok || c.version != env[0] {
		t.Errorf("the notice names the Go release %s that builds the executable: %+v", env[0], c)
	}
	if ok && !sameText(t, filepath.Join(env[1], "LICENSE"), c.file) {
		t.Errorf("the Go licence differs from %s", c.file)
	}
}

func TestLicencesAllowed(t *testing.T) {
	for _, c := range components {
		if want, ok := namedComponents[c.name]; ok {
			if c.spdx != want {
				t.Errorf("%s is %s, want %s", c.name, c.spdx, want)
			}
			continue
		}
		if !allowed(c.spdx) {
			t.Errorf("%s is %s, which is not an accepted licence", c.name, c.spdx)
		}
	}
	if allowed("GPL-3.0-only") || allowed("MIT AND GPL-3.0-only") || !allowed("Unlicense OR BSD-3-Clause") {
		t.Error("an expression is accepted when one alternative is, and every part of a conjunction is")
	}
}

// allowed accepts an SPDX expression of OR and AND without parentheses when
// one alternative has every licence in allowedLicences.
func allowed(expression string) bool {
	for _, alternative := range strings.Split(expression, " OR ") {
		ok := true
		for _, id := range strings.Split(alternative, " AND ") {
			ok = ok && allowedLicences[strings.TrimSpace(id)]
		}
		if ok {
			return true
		}
	}
	return false
}

func TestNoticeListsComponents(t *testing.T) {
	notice := Notice()
	for _, c := range components {
		var want string
		switch {
		case c.version == "":
			want = "\n## " + c.name + "\n"
		case c.module:
			want = "| `" + c.name + "` | `" + c.version + "` | " + c.spdx + " |"
		default:
			want = "| " + c.name + " | `" + c.version + "` | " + c.spdx + " |"
		}
		if !strings.Contains(notice, want) {
			t.Errorf("NOTICE.md lacks %q", want)
		}
	}
}
