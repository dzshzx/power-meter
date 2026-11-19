package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// placeholder is mygo.json's version in the repository: a local build is
// 0.0.0, and only the release build carries a real version.
var placeholder = []byte(`"version": "0.0.0"`)

// runSetVersion writes the release version into mygo.json, for mygo build
// to stamp on the executable and the installer. The change is not
// committed.
func runSetVersion(args []string) error {
	fs := flag.NewFlagSet("set-version", flag.ContinueOnError)
	version := fs.String("version", "", "release version X.Y.Z")
	if err := parse(fs, args, "version"); err != nil {
		return err
	}
	return setVersion(filepath.Join(repoRoot(), "mygo.json"), *version)
}

func setVersion(path, version string) error {
	if !regexp.MustCompile(versionFormat).MatchString(version) {
		return fmt.Errorf("version %q is not X.Y.Z", version)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if n := bytes.Count(data, placeholder); n != 1 {
		return fmt.Errorf("%s must carry %s exactly once, found %d", path, placeholder, n)
	}
	data = bytes.Replace(data, placeholder, fmt.Appendf(nil, `"version": %q`, version), 1)
	return os.WriteFile(path, data, 0o644)
}
