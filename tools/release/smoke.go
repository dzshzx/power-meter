package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// runSmoke runs the built program's command-line routes, none of which may
// open a window or ask for UAC.
func runSmoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	exe := fs.String("exe", "", "PowerMeter.exe to test")
	work := fs.String("work", "", "directory for the outputs")
	if err := parse(fs, args, "exe", "work"); err != nil {
		return err
	}
	return smoke(*exe, *work)
}

func smoke(exe, work string) error {
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	checks := []struct {
		name  string
		check func(exe, work string) error
	}{
		{"invalid arguments exit 2", smokeInvalid},
		{"third-party notices", smokeNotices},
		{"power probe", smokeProbe},
		{"window screenshot", smokeScreenshot},
		{"window screenshot at 175%", smokeDPI},
		{"tray preview", smokeTray},
		{"self-test", smokeSelfTest},
	}
	for _, c := range checks {
		if err := c.check(exe, work); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		fmt.Println("PASS", c.name)
	}
	return nil
}

func smokeInvalid(exe, _ string) error {
	for _, args := range [][]string{{"--wat"}, {"--self-test"}, {"--help"}, {"--power-probe", "x", "0"}, {"--no-elevate", "extra"}} {
		code, _ := runCode(exe, args...)
		if code != 2 {
			return fmt.Errorf("%v exited %d", args, code)
		}
	}
	return nil
}

func smokeNotices(exe, work string) error {
	path := filepath.Join(work, "notices.txt")
	if err := run(exe, "--third-party-notices", path); err != nil {
		return err
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, want := range []string{"GNU LESSER GENERAL PUBLIC LICENSE", "IntelMSR.bin", "Lucide", "MiSans字体知识产权许可协议", "github.com/egoist/mygo"} {
		if !bytes.Contains(text, []byte(want)) {
			return fmt.Errorf("notices lack %q", want)
		}
	}
	return nil
}

func smokeProbe(exe, work string) error {
	path := filepath.Join(work, "probe.txt")
	if err := run(exe, "--power-probe", path, "1"); err != nil {
		return err
	}
	report, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, field := range []string{"Elevated token", "EMI", "PawnIO"} {
		if !bytes.Contains(report, []byte(field)) {
			return fmt.Errorf("the report omits %q", field)
		}
	}
	return nil
}

func smokeScreenshot(exe, work string) error {
	path := filepath.Join(work, "screenshot.png")
	if err := run(exe, "--screenshot", path); err != nil {
		return err
	}
	return checkImage(path, 0)
}

func smokeDPI(exe, work string) error {
	path := filepath.Join(work, "dpi.png")
	if err := run(exe, "--screenshot", path, "168"); err != nil {
		return err
	}
	meta, err := os.ReadFile(path + ".txt")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(meta), "Dpi=168;") {
		return fmt.Errorf("the screenshot is not at the DPI asked for: %s", meta)
	}
	return checkImage(path, 0)
}

func smokeTray(exe, work string) error {
	path := filepath.Join(work, "tray.png")
	if err := run(exe, "--tray-preview", path, "42", "discharging"); err != nil {
		return err
	}
	return checkImage(path, 64)
}

func smokeSelfTest(exe, work string) error {
	path := filepath.Join(work, "self-test.txt")
	err := run(exe, "--self-test", path)
	report, _ := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, report)
	}
	if bytes.Contains(report, []byte("FAIL")) || !bytes.Contains(report, []byte("PASS")) {
		return errors.New("unexpected report:\n" + string(report))
	}
	return nil
}

// checkImage decodes a PNG; a non-zero width must match.
func checkImage(path string, width int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return err
	}
	if b := img.Bounds(); b.Dx() == 0 || b.Dy() == 0 || width > 0 && b.Dx() != width {
		return fmt.Errorf("%s is %dx%d", filepath.Base(path), b.Dx(), b.Dy())
	}
	return nil
}
