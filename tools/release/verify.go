package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const releaseURL = "https://github.com/dzshzx/power-meter/releases/download/"

// runVerify downloads a published release and checks it as a user gets it:
// the asset set and checksums, the executable's version and that it asks
// for no elevation, the command-line smoke, and that the notices asset is
// what the executable exports. It writes verification.json and a screenshot
// of the window beside the downloads.
func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	version := fs.String("version", "", "published version X.Y.Z")
	out := fs.String("out", "", "empty directory for the downloads")
	if err := parse(fs, args, "version", "out"); err != nil {
		return err
	}
	if err := emptyDir(*out); err != nil {
		return err
	}
	assets := filepath.Join(*out, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return err
	}
	for _, name := range releaseAssets(*version) {
		if err := download(releaseURL+"v"+*version+"/"+name, filepath.Join(assets, name)); err != nil {
			return err
		}
	}
	if err := checkAssets(assets, *version); err != nil {
		return err
	}
	exe := filepath.Join(*out, exeName)
	if err := copyFile(filepath.Join(assets, portableAsset(*version)), exe); err != nil {
		return err
	}
	checks := []struct {
		name  string
		check func() error
	}{
		{"file version", func() error { return checkFileVersion(exe, *version) }},
		{"no elevation in the manifest", func() error { return checkAsInvoker(exe) }},
		{"command-line smoke", func() error { return smoke(exe, filepath.Join(*out, "smoke")) }},
		{"published notices match the export", func() error {
			return sameFile(filepath.Join(assets, noticesAsset), filepath.Join(*out, "smoke", "notices.txt"))
		}},
	}
	report := map[string]any{"version": *version, "verifiedAt": time.Now().UTC().Format(time.RFC3339)}
	for _, c := range checks {
		if err := c.check(); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		report[c.name] = "pass"
	}
	if err := copyFile(filepath.Join(*out, "smoke", "screenshot.png"), filepath.Join(*out, "window.png")); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	return os.WriteFile(filepath.Join(*out, "verification.json"), data, 0o644)
}

func download(url, path string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// checkAsInvoker checks that the embedded manifest requests no elevation:
// the program runs with the token it is started with and asks for UAC once
// on its own.
func checkAsInvoker(exe string) error {
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if !bytes.Contains(data, []byte("<assembly xmlns=\"urn:schemas-microsoft-com:asm.v1\"")) {
		return errors.New("no application manifest")
	}
	for _, level := range []string{"requireAdministrator", "highestAvailable"} {
		if bytes.Contains(data, []byte(level)) {
			return fmt.Errorf("the manifest requests %s", level)
		}
	}
	return nil
}

func sameFile(a, b string) error {
	x, err := os.ReadFile(a)
	if err != nil {
		return err
	}
	y, err := os.ReadFile(b)
	if err != nil {
		return err
	}
	if !bytes.Equal(x, y) {
		return fmt.Errorf("%s and %s differ", filepath.Base(a), strings.TrimSuffix(filepath.Base(b), ""))
	}
	return nil
}
