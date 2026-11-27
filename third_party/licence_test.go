package thirdparty

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The licence check: the components table must name every module the
// Windows executable links, with the module's own licence text and a licence
// this project accepts. Versions come from the executable's build
// information, so a version upgrade needs no notice edit; adding or
// dropping a dependency without updating the notice fails here.

// allowedLicences may appear in any component's SPDX expression.
var allowedLicences = map[string]bool{
	"MIT": true, "BSD-2-Clause": true, "BSD-3-Clause": true, "Apache-2.0": true, "ISC": true, "OFL-1.1": true,
}

// namedComponents may use a licence outside allowedLicences, each only its
// own: the PawnIO module, which is replaceable and ships with its
// corresponding source.
var namedComponents = map[string]string{"IntelMSR.bin": "LGPL-2.1-or-later"}

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

// linkedModules maps the path of each module to its directory, for the modules of every package the module's commands
// and libraries build from for windows/amd64, without tests and without the
// tools that go.mod's tool directive brings.
func linkedModules(t *testing.T) map[string]string {
	out := goCommand(t, []string{"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0"},
		"list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Dir}}{{end}}{{end}}", "./...")
	modules := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if path, dir, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok {
			modules[path] = dir
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
	for path, dir := range linked {
		c, ok := componentNamed(path)
		switch {
		case !ok || !c.module:
			t.Errorf("%s is linked but has no section in the notice", path)
		case !sameText(t, filepath.Join(dir, "LICENSE"), c.file):
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
	goroot := strings.TrimSpace(goCommand(t, nil, "env", "GOROOT"))
	c, ok := componentNamed("Go standard library and runtime")
	if !ok || !c.toolchain {
		t.Fatalf("the notice lists the Go standard library and runtime: %+v", c)
	}
	if !sameText(t, filepath.Join(goroot, "LICENSE"), c.file) {
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
		case c.module:
			want = "| `" + c.name + "` | " + c.spdx + " |"
		case c.toolchain:
			want = "| " + c.name + " | " + c.spdx + " |"
		default:
			want = "\n## " + c.name + "\n"
		}
		if !strings.Contains(notice, want) {
			t.Errorf("NOTICE.md lacks %q", want)
		}
	}
}
