// Package thirdparty carries the third-party files the executable embeds:
// the PawnIO IntelMSR module and the notice and licence texts that
// NOTICE.md in this directory describes.
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

//go:embed NOTICE.md LICENSE.*.txt
var files embed.FS

// newline separates the exported sections, as Windows text files do. The
// embedded texts keep their own bytes.
const newline = "\r\n"

// sections follow NOTICE.md in the export: a heading line, then the licence
// text it introduces.
var sections = []struct{ heading, file string }{
	{"---" + newline + "GNU Lesser General Public License 2.1" + newline + "---", "LICENSE.LGPL-2.1.txt"},
	{"AntdUI - Apache License 2.0", "LICENSE.Apache-2.0.txt"},
	{"Lucide icons", "LICENSE.Lucide.txt"},
	{"SVG.NET (included in AntdUI)", "LICENSE.Ms-PL.txt"},
	{"TaskScheduler - MIT License", "LICENSE.TaskScheduler.txt"},
	{"System.CommandLine, System.Memory, System.Buffers, System.Numerics.Vectors," +
		" System.Runtime.CompilerServices.Unsafe - MIT License", "LICENSE.dotnet-MIT.txt"},
}

// Notice is NOTICE.md as embedded.
func Notice() string { return text("NOTICE.md") }

// Notices is the complete third-party notice: NOTICE.md followed by every
// licence text it refers to.
func Notices() string {
	var b strings.Builder
	b.WriteString(Notice())
	for _, s := range sections {
		b.WriteString(newline + newline + s.heading + newline + text(s.file))
	}
	return b.String()
}

// WriteNotices writes Notices to path as UTF-8 with a byte order mark, the
// --third-party-notices export.
func WriteNotices(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("a notice output path is required")
	}
	return os.WriteFile(path, []byte("\uFEFF"+Notices()), 0o644)
}

func text(name string) string {
	data, err := files.ReadFile(name)
	if err != nil {
		// Every name is a literal matched by the embed pattern above.
		panic(err)
	}
	return strings.TrimPrefix(string(data), "\uFEFF")
}
