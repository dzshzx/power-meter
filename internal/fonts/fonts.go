// Package fonts embeds the MiSans faces the window and the tray icon draw
// with, and registers them with mygo, each face under a family of its own:
// MiSans declares its weights irregularly, so a face is chosen by family,
// not by weight.
//
// The faces are not in the repository (the MiSans licence forbids
// distributing them on their own); go generate ./internal/fonts, or
// go run ./tools/fonts, puts them in misans/ from Xiaomi's official
// package, checked against pinned SHA-256 sums.
package fonts

//go:generate go run ../../tools/fonts

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The families, each one face of MiSans.
const (
	Regular  = "MiSans"
	Medium   = "MiSans Medium"
	Semibold = "MiSans Semibold"
)

//go:embed misans/*.ttf.gz
var files embed.FS

var faces = []struct{ file, family string }{
	{"misans/MiSans-Regular.ttf.gz", Regular},
	{"misans/MiSans-Medium.ttf.gz", Medium},
	{"misans/MiSans-Semibold.ttf.gz", Semibold},
}

var (
	once sync.Once
	err  error
)

// Register registers the faces once; text then finds them by family. On
// failure the window falls back to the system's fonts.
func Register() error {
	once.Do(func() {
		for _, f := range faces {
			var data []byte
			if data, err = read(f.file); err != nil {
				return
			}
			if err = ui.RegisterFont(data, f.family); err != nil {
				return
			}
		}
	})
	return err
}

func read(name string) ([]byte, error) {
	packed, err := files.ReadFile(name)
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(z)
}
