// Package fonts embeds the MiSans faces the window and the tray icon draw
// with, and registers them with mygo, each face under a family of its own:
// MiSans declares its weights irregularly, so a face is chosen by family,
// not by weight.
//
// The faces are not in the repository (the MiSans licence forbids
// distributing them on their own); go generate ./internal/fonts, or
// go run ./tools/fonts, puts them in misans/ from Xiaomi's official
// package, checked against pinned SHA-256 sums and cut down to the
// characters the app shows. The package compiles without them, so the
// gate and code scanning need no download: such a build draws with the
// system's fonts, and its --self-test fails, so a release cannot ship
// without the faces.
package fonts

//go:generate go run ../../tools/fonts

import (
	"embed"
	"errors"
	"io/fs"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The families, each one face of MiSans.
const (
	Regular  = "MiSans"
	Medium   = "MiSans Medium"
	Semibold = "MiSans Semibold"
)

// misans/ holds a README in the repository, so the directory always embeds;
// the faces are there once fetched.
//
//go:embed misans
var files embed.FS

var faces = []struct{ file, family string }{
	{"misans/MiSans-Regular.ttf", Regular},
	{"misans/MiSans-Medium.ttf", Medium},
	{"misans/MiSans-Semibold.ttf", Semibold},
}

var (
	once sync.Once
	err  error
)

// Register registers the faces this build embeds, once; text then finds
// them by family. A build without them registers none and returns nil: the
// window falls back to the system's fonts. An error means an embedded face
// could not be read or registered.
func Register() error {
	once.Do(func() {
		for _, f := range faces {
			data, readErr := files.ReadFile(f.file)
			if errors.Is(readErr, fs.ErrNotExist) {
				continue
			}
			if readErr != nil {
				err = readErr
				return
			}
			if err = ui.RegisterFont(data, f.family); err != nil {
				return
			}
		}
	})
	return err
}

// Embedded reports whether this build carries all three faces: a release
// must, and its --self-test checks it.
func Embedded() bool {
	for _, f := range faces {
		if _, err := fs.Stat(files, f.file); err != nil {
			return false
		}
	}
	return true
}
