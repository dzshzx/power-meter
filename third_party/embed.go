// Package thirdparty carries the third-party files the executable embeds:
// the PawnIO IntelMSR module, NOTICE.md and the licence texts listed below.
package thirdparty

import (
	"embed"
	"errors"
	"os"
	"strings"
)

// IntelMSR is the unmodified upstream PawnIO module. A file of the same name
// beside the executable takes precedence over it (see NOTICE.md).
//
//go:embed IntelMSR.bin
var IntelMSR []byte

// IntelMSRFileName is the module's name both here and beside the executable.
const IntelMSRFileName = "IntelMSR.bin"

//go:embed NOTICE.md LICENSE.Go.txt LICENSE.purego.txt LICENSE.mygo.txt LICENSE.go-ole.txt
//go:embed LICENSE.go-text-typesetting.txt LICENSE.Lucide.txt LICENSE.LGPL-2.1.txt
var files embed.FS

// component is one third-party part of the executable and the licence it is
// used under. NOTICE.md describes each one: a Go component (the standard
// library or a module) in its table, any other under a heading of its name.
type component struct {
	// name is the module path, or what NOTICE.md calls the component.
	name string
	// version is the module version or Go release; empty for a component
	// NOTICE.md versions in its own section.
	version string
	// module marks a Go module linked into the executable.
	module bool
	// spdx is the licence as an SPDX expression.
	spdx string
	// file is the licence text in this directory.
	file string
}

// components are the export's sections after NOTICE.md, in its order. The
// tests hold them to the modules the executable links.
var components = []component{
	{name: "Go standard library and runtime", version: "go1.27.4", spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "github.com/ebitengine/purego", version: "v0.11.1", module: true, spdx: "Apache-2.0", file: "LICENSE.purego.txt"},
	{name: "github.com/egoist/mygo", version: "v0.10.0", module: true, spdx: "MIT", file: "LICENSE.mygo.txt"},
	{name: "github.com/go-ole/go-ole", version: "v1.3.0", module: true, spdx: "MIT", file: "LICENSE.go-ole.txt"},
	{name: "github.com/go-text/typesetting", version: "v0.3.5", module: true, spdx: "Unlicense OR BSD-3-Clause",
		file: "LICENSE.go-text-typesetting.txt"},
	{name: "golang.org/x/image", version: "v0.46.0", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "golang.org/x/net", version: "v0.59.0", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "golang.org/x/sys", version: "v0.48.0", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "golang.org/x/text", version: "v0.43.0", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "Lucide icons", spdx: "ISC", file: "LICENSE.Lucide.txt"},
	{name: "IntelMSR.bin", spdx: "LGPL-2.1-or-later", file: "LICENSE.LGPL-2.1.txt"},
}

// heading is the line that introduces the component's licence text.
func (c component) heading() string {
	h := c.name
	if c.version != "" {
		h += " " + c.version
	}
	return h + " - " + c.spdx
}

// Notice is NOTICE.md as embedded.
func Notice() string { return text("NOTICE.md") }

// Notices is the complete third-party notice: NOTICE.md followed by every
// component's heading and licence text. The embedded files keep their
// upstream text; the checkout gives them LF line endings.
func Notices() string {
	var b strings.Builder
	b.WriteString(Notice())
	for _, c := range components {
		b.WriteString("\n\n---\n" + c.heading() + "\n---\n" + text(c.file))
	}
	return b.String()
}

// WriteNotices writes Notices to path as UTF-8, the --third-party-notices
// export.
func WriteNotices(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("a notice output path is required")
	}
	return os.WriteFile(path, []byte(Notices()), 0o644)
}

func text(name string) string {
	data, err := files.ReadFile(name)
	if err != nil {
		// Every name is a literal listed in the embed patterns above.
		panic(err)
	}
	return string(data)
}
