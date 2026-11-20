package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The text is the code's strings and runes and the catalogs', without the
// tests', with the ranges always kept.
func TestCharacters(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/gui/a.go", "package gui\n\n// 注释不算\nvar a, b, c = \"功率\", `计·`, '≈'\n")
	write("cmd/app/main.go", "package main\n\nfunc main() { println(\"\\u7535\") }\n")
	write("internal/gui/a_test.go", "package gui\n\nvar d = \"测\"\n")
	write("locales/zh.json", `{"键": "值", "list": ["池"]}`)
	text, err := characters(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range "功率计·≈电键值池Az0，" {
		if !strings.ContainsRune(text, c) {
			t.Errorf("%q is left out", c)
		}
	}
	for _, c := range "注释不算测\n" {
		if strings.ContainsRune(text, c) {
			t.Errorf("%q is kept", c)
		}
	}
}

// The faces this checkout embeds were cut for the text of its code: a
// string added since shows in a fallback font until go run ./tools/fonts.
func TestFacesCutForTheCode(t *testing.T) {
	root := filepath.Join("..", "..")
	cut, err := os.ReadFile(filepath.Join(root, "internal", "fonts", charactersFile))
	if os.IsNotExist(err) {
		t.Skip("the faces have not been fetched")
	}
	if err != nil {
		t.Fatal(err)
	}
	text, err := characters(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(cut) != text {
		t.Fatalf("the faces were cut for other text (missing %q, extra %q): run go run ./tools/fonts",
			apart(text, string(cut)), apart(string(cut), text))
	}
}

// apart is the characters of a that b lacks.
func apart(a, b string) string {
	var out []rune
	for _, c := range a {
		if !strings.ContainsRune(b, c) {
			out = append(out, c)
		}
	}
	return string(out)
}
