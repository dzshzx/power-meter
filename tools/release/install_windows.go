package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	uninstallRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall\`
	uninstallKey  = uninstallRoot + "io.github.dzshzx.powermeter"
	// oldUninstallKey is the Inno Setup entry of Power Meter 1.5 and earlier.
	oldUninstallKey = uninstallRoot + "{FDDC9FC9-109E-4B41-AE4A-BA30420295D0}_is1"
	shortcutName    = "Power Meter.lnk"
	// installedNotices is the notices file the installer writes beside the
	// program.
	installedNotices = "THIRD-PARTY-NOTICES.txt"
)

// runInstallTest installs, upgrades and uninstalls the installer of a mygo
// build silently for the current user, after checking that it refuses to
// install over Power Meter 1.5 or earlier. It refuses to run where Power
// Meter is installed.
func runInstallTest(args []string) error {
	fs := flag.NewFlagSet("install-test", flag.ContinueOnError)
	version := fs.String("version", "", "the installer's version X.Y.Z")
	build := fs.String("build", "", "mygo build output for windows/amd64")
	if err := parse(fs, args, "version", "build"); err != nil {
		return err
	}
	setup := builtInstaller(*build, *version)
	if installLocation() != "" || keyExists(oldUninstallKey) {
		return errors.New("found an installed Power Meter; uninstall it before the test")
	}
	steps := []struct {
		name string
		run  func() error
	}{
		{"refuses to install over 1.5", func() error { return blockedByOldVersion(setup) }},
		{"installs", func() error { return install(setup, *version) }},
		{"installs over itself", func() error { return install(setup, *version) }},
		{"uninstalls", uninstall},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		fmt.Println("PASS", s.name)
	}
	return nil
}

func blockedByOldVersion(setup string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, oldUninstallKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = registry.DeleteKey(registry.CURRENT_USER, oldUninstallKey) }()
	err = k.SetStringValue("UninstallString", `"C:\nonexistent\unins000.exe"`)
	k.Close()
	if err != nil {
		return err
	}
	code, _ := runCode(setup, "/S")
	if code != 3 {
		return fmt.Errorf("the installer exited %d, want 3", code)
	}
	if installLocation() != "" {
		return errors.New("the installer installed anyway")
	}
	return nil
}

func install(setup, version string) error {
	if err := run(setup, "/S"); err != nil {
		return err
	}
	dir := installLocation()
	if dir == "" {
		return errors.New("no uninstall entry")
	}
	if want := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Power Meter"); !samePath(dir, want) {
		return fmt.Errorf("installed in %s, want %s", dir, want)
	}
	exe := filepath.Join(dir, exeName)
	if err := checkFileVersion(exe, version); err != nil {
		return err
	}
	if v := stringValue(uninstallKey, "DisplayVersion"); v != version {
		return fmt.Errorf("DisplayVersion %q, want %q", v, version)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`, shortcutName)); err != nil {
		return fmt.Errorf("no Start menu shortcut: %w", err)
	}
	notices, err := os.ReadFile(filepath.Join(dir, installedNotices))
	if err != nil {
		return fmt.Errorf("no notices beside the program: %w", err)
	}
	if !strings.Contains(string(notices), "GNU LESSER GENERAL PUBLIC LICENSE") {
		return errors.New("the installed notices lack the LGPL-2.1 text")
	}
	return smokeSelfTest(exe, os.TempDir())
}

// uninstall runs the uninstaller silently. It goes on from a copy of itself,
// so the test waits for the folder and the entry to go.
func uninstall() error {
	dir := installLocation()
	if err := run(filepath.Join(dir, "Uninstall.exe"), "/S"); err != nil {
		return err
	}
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		_, statErr := os.Stat(dir)
		if os.IsNotExist(statErr) && installLocation() == "" {
			if _, err := os.Stat(filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`, shortcutName)); err == nil {
				return errors.New("the Start menu shortcut is left")
			}
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("%s or its uninstall entry is still there", dir)
}

func installLocation() string { return stringValue(uninstallKey, "InstallLocation") }

func stringValue(key, name string) string {
	k, err := registry.OpenKey(registry.CURRENT_USER, key, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return v
}

func keyExists(key string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, key, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
