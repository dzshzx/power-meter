package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	exeName       = "PowerMeter.exe"
	noticesAsset  = "PowerMeter-THIRD-PARTY-NOTICES.txt"
	sourceBundle  = "PawnIO.Modules-0.2.10-source.zip"
	commandWait   = 30 * time.Second
	versionFormat = `^\d+\.\d+\.\d+$`
)

func portableAsset(version string) string  { return "PowerMeter-v" + version + "-windows.exe" }
func installerAsset(version string) string { return "PowerMeter-v" + version + "-windows-setup.exe" }

// releaseAssets are the published files: each asset and its .sha256.
func releaseAssets(version string) []string {
	var names []string
	for _, a := range []string{portableAsset(version), installerAsset(version), noticesAsset, sourceBundle} {
		names = append(names, a, a+".sha256")
	}
	slices.Sort(names)
	return names
}

// runPackage turns a mygo build into the release assets: the portable
// executable, the installer, the notices it exports and the PawnIO source
// bundle, each with a checksum file, in an otherwise empty directory.
func runPackage(args []string) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	version := fs.String("version", "", "release version X.Y.Z")
	build := fs.String("build", "", "mygo build output for windows/amd64")
	out := fs.String("out", "", "empty directory for the assets")
	if err := parse(fs, args, "version", "build", "out"); err != nil {
		return err
	}
	if !regexp.MustCompile(versionFormat).MatchString(*version) {
		return fmt.Errorf("version %q is not X.Y.Z", *version)
	}
	if err := emptyDir(*out); err != nil {
		return err
	}
	exe := filepath.Join(*build, exeName)
	setup := filepath.Join(*build, "power-meter-setup-"+*version+"-amd64.exe")
	if err := checkFileVersion(exe, *version); err != nil {
		return err
	}
	steps := []struct{ from, to string }{
		{exe, portableAsset(*version)},
		{setup, installerAsset(*version)},
		{filepath.Join(repoRoot(), "third_party", sourceBundle), sourceBundle},
	}
	for _, s := range steps {
		if err := copyFile(s.from, filepath.Join(*out, s.to)); err != nil {
			return err
		}
	}
	if err := run(exe, "--third-party-notices", filepath.Join(*out, noticesAsset)); err != nil {
		return fmt.Errorf("export notices: %w", err)
	}
	for _, name := range []string{portableAsset(*version), installerAsset(*version), noticesAsset, sourceBundle} {
		if err := writeChecksum(filepath.Join(*out, name)); err != nil {
			return err
		}
	}
	return checkAssets(*out, *version)
}

// checkAssets checks that dir holds exactly the release assets, that every
// checksum file matches, and that the source bundle is the pinned one.
func checkAssets(dir, version string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	if want := releaseAssets(version); !slices.Equal(names, want) {
		return fmt.Errorf("assets %v, want %v", names, want)
	}
	for _, name := range names {
		if strings.HasSuffix(name, ".sha256") {
			continue
		}
		if err := checkChecksum(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	pinned, err := pinnedSourceHash()
	if err != nil {
		return err
	}
	if got, err := fileHash(filepath.Join(dir, sourceBundle)); err != nil || got != pinned {
		return fmt.Errorf("source bundle hash %s, want the pinned %s (%v)", got, pinned, err)
	}
	return nil
}

// pinnedSourceHash reads the source bundle's SHA-256 from NOTICE.md.
func pinnedSourceHash() (string, error) {
	notice, err := os.ReadFile(filepath.Join(repoRoot(), "third_party", "NOTICE.md"))
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile("Source bundle SHA-256 \\| `([0-9a-f]{64})`").FindSubmatch(notice)
	if m == nil {
		return "", errors.New("NOTICE.md has no source bundle SHA-256")
	}
	return string(m[1]), nil
}

// writeChecksum writes "<sha256>  <name>" beside the file, as sha256sum
// prints it.
func writeChecksum(path string) error {
	sum, err := fileHash(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".sha256", []byte(sum+"  "+filepath.Base(path)+"\r\n"), 0o644)
}

func checkChecksum(path string) error {
	entry, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return err
	}
	m := regexp.MustCompile(`^([0-9a-f]{64})  (.+)$`).FindStringSubmatch(strings.TrimSpace(string(entry)))
	if m == nil || m[2] != filepath.Base(path) {
		return fmt.Errorf("invalid checksum entry for %s: %q", filepath.Base(path), entry)
	}
	if got, err := fileHash(path); err != nil || got != m[1] {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}
	return nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o644)
}

func emptyDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s must be empty: it receives only release assets", dir)
	}
	return nil
}

// repoRoot is the checkout: the tool runs with go run from it.
func repoRoot() string {
	dir, _ := os.Getwd()
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

// run runs the program and fails on a non-zero exit or after commandWait.
func run(name string, args ...string) error {
	_, err := runCode(name, args...)
	return err
}

// runCode runs the program and returns its exit code; err reports a start
// failure, a timeout or a non-zero exit.
func runCode(name string, args ...string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if ctx.Err() != nil {
		return -1, fmt.Errorf("%s %v did not exit within %s", filepath.Base(name), args, commandWait)
	}
	code := cmd.ProcessState.ExitCode()
	if err != nil {
		return code, fmt.Errorf("%s %v: %w %s", filepath.Base(name), args, err, strings.TrimSpace(out.String()))
	}
	return code, nil
}
