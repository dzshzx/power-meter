package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// The faces are cut down to the characters the app can show: those of its
// own text, read from the strings of its Go code and from the catalogs,
// and the ASCII, Latin-1 and punctuation that text from Windows and the
// hardware mostly holds. A character beyond them, as in a Windows error
// message, falls back to a font of the system's. The cut keeps every
// layout feature, the hinting and every name record, the copyright and the
// licence among them; HarfBuzz's hb-subset makes it.

// charactersFile, in internal/fonts, holds the text the faces were cut
// for; it is not embedded with them.
const charactersFile = "characters.txt"

// kept are the ranges of characters kept whatever the code says.
var kept = [][2]rune{
	{0x20, 0x7e},     // ASCII
	{0xa0, 0xff},     // Latin-1
	{0x2010, 0x205e}, // general punctuation
	{0x3000, 0x303f}, // CJK symbols and punctuation
	{0xff01, 0xff5e}, // fullwidth forms
}

// characters is the text the faces are cut for: each character of kept,
// of the string and rune literals of the Go code under root's internal and
// cmd but its tests, and of the catalogs in root's locales, once, in order.
func characters(root string) (string, error) {
	set := map[rune]bool{}
	for _, r := range kept {
		for c := r[0]; c <= r[1]; c++ {
			set[c] = true
		}
	}
	add := func(s string) {
		for _, c := range s {
			if c >= 0x20 && c != 0x7f && c != 0xfffd {
				set[c] = true
			}
		}
	}
	for _, dir := range []string{"internal", "cmd"} {
		if err := literals(filepath.Join(root, dir), add); err != nil {
			return "", err
		}
	}
	if err := catalogs(filepath.Join(root, "locales"), add); err != nil {
		return "", err
	}
	runes := make([]rune, 0, len(set))
	for c := range set {
		runes = append(runes, c)
	}
	slices.Sort(runes)
	return string(runes), nil
}

// literals gives add the value of each string and rune literal of the Go
// files under dir but the tests.
func literals(dir string, add func(string)) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		var s scanner.Scanner
		s.Init(fset.AddFile(path, -1, len(src)), src, nil, 0)
		for {
			_, tok, lit := s.Scan()
			if tok == token.EOF {
				return nil
			}
			if tok == token.STRING || tok == token.CHAR {
				if v, err := strconv.Unquote(lit); err == nil {
					add(v)
				}
			}
		}
	})
}

// catalogs gives add every key and text of the JSON catalogs in dir.
func catalogs(dir string, add func(string)) error {
	names, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		walkJSON(v, add)
	}
	return nil
}

func walkJSON(v any, add func(string)) {
	switch v := v.(type) {
	case string:
		add(v)
	case map[string]any:
		for k, x := range v {
			add(k)
			walkJSON(x, add)
		}
	case []any:
		for _, x := range v {
			walkJSON(x, add)
		}
	}
}

// subset cuts the face in the file font down to the characters of the file
// text.
func subset(font, text string) ([]byte, error) {
	out, err := os.CreateTemp("", "misans-*.ttf")
	if err != nil {
		return nil, err
	}
	out.Close()
	defer os.Remove(out.Name())
	cmd := exec.Command("hb-subset", "--font-file="+font, "--text-file="+text, "--output-file="+out.Name(),
		"--layout-features=*", "--layout-scripts=*", "--name-IDs=*", "--name-languages=*", "--notdef-outline")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("hb-subset is not installed: it comes with HarfBuzz (libharfbuzz-bin on Ubuntu)")
		}
		return nil, fmt.Errorf("hb-subset: %w", err)
	}
	return os.ReadFile(out.Name())
}
