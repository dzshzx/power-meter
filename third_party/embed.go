// Package thirdparty carries the third-party files the executable embeds:
// the PawnIO IntelMSR module, NOTICE.md and the licence texts listed below.
package thirdparty

import (
	"embed"
	"errors"
	"os"
	"runtime"
	"runtime/debug"
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
	// module marks a Go module linked into the executable.
	module bool
	// toolchain marks the Go standard library and runtime.
	toolchain bool
	// spdx is the licence as an SPDX expression.
	spdx string
	// file is the licence text in this directory.
	file string
}

// components are the export's sections after NOTICE.md, in its order. The
// tests hold them to the modules the executable links.
var components = []component{
	{name: "Go standard library and runtime", toolchain: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "github.com/ebitengine/purego", module: true, spdx: "Apache-2.0", file: "LICENSE.purego.txt"},
	{name: "github.com/egoist/mygo", module: true, spdx: "MIT", file: "LICENSE.mygo.txt"},
	{name: "github.com/go-ole/go-ole", module: true, spdx: "MIT", file: "LICENSE.go-ole.txt"},
	{name: "github.com/go-text/typesetting", module: true, spdx: "Unlicense OR BSD-3-Clause",
		file: "LICENSE.go-text-typesetting.txt"},
	{name: "golang.org/x/image", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "golang.org/x/net", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "golang.org/x/sys", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "golang.org/x/text", module: true, spdx: "BSD-3-Clause", file: "LICENSE.Go.txt"},
	{name: "Lucide icons", spdx: "ISC", file: "LICENSE.Lucide.txt"},
	{name: "IntelMSR.bin", spdx: "LGPL-2.1-or-later", file: "LICENSE.LGPL-2.1.txt"},
}

// buildInfo is the running executable's build information; tests replace it.
var buildInfo = debug.ReadBuildInfo

// version is the Go release or module version this executable was built
// with, read from its build information so that a dependency or toolchain
// upgrade needs no notice edit; "" for a component NOTICE.md versions in its
// own section, or a module the build information does not list.
func (c component) version() string {
	switch {
	case c.toolchain:
		return runtime.Version()
	case !c.module:
		return ""
	}
	info, ok := buildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path != c.name {
			continue
		}
		if dep.Replace != nil {
			return dep.Replace.Version
		}
		return dep.Version
	}
	return ""
}

// heading is the line that introduces the component's licence text.
func (c component) heading() string {
	h := c.name
	if v := c.version(); v != "" {
		h += " " + v
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
