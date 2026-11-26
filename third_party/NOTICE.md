# Third-party components

The Windows executable is a Go program. It statically links the Go standard
library and the Go modules below, draws Lucide icon geometry and embeds the
PawnIO `IntelMSR.bin` module. The complete licence text of each listed component
follows this notice in the `--third-party-notices` export, the standalone
release notice and the installed notice. The `third_party` package tests fail
when a module linked into the executable is missing here, its version differs,
its licence text changed or its licence is not one this project accepts.

## Go standard library and modules

| Component | Version | Licence | Upstream |
|---|---|---|---|
| Go standard library and runtime | `go1.27.4` | BSD-3-Clause | https://go.dev/ |
| `github.com/ebitengine/purego` | `v0.11.3` | Apache-2.0 | https://github.com/ebitengine/purego |
| `github.com/egoist/mygo` | `v0.10.0` | MIT | https://github.com/egoist/mygo |
| `github.com/go-ole/go-ole` | `v1.3.0` | MIT | https://github.com/go-ole/go-ole |
| `github.com/go-text/typesetting` | `v0.3.5` | Unlicense OR BSD-3-Clause | https://github.com/go-text/typesetting |
| `golang.org/x/image` | `v0.48.0` | BSD-3-Clause | https://go.googlesource.com/image |
| `golang.org/x/net` | `v0.63.0` | BSD-3-Clause | https://go.googlesource.com/net |
| `golang.org/x/sys` | `v0.50.0` | BSD-3-Clause | https://go.googlesource.com/sys |
| `golang.org/x/text` | `v0.44.0` | BSD-3-Clause | https://go.googlesource.com/text |

The modules are used unmodified; the Go toolchain fetches them against the
checksums in `go.sum`. `go-text/typesetting` is used under BSD-3-Clause, one
of the two licences it offers. mygo embeds only its own shaders, scripts and
messages; text is drawn with DirectWrite. The licence texts are kept here as
`LICENSE.Go.txt` (the Go
standard library and every `golang.org/x` module), `LICENSE.purego.txt`,
`LICENSE.mygo.txt`, `LICENSE.go-ole.txt` and `LICENSE.go-text-typesetting.txt`.

## Lucide icons

The application icon draws the unmodified `battery-medium` paths from
https://github.com/lucide-icons/lucide/blob/main/icons/battery-medium.svg in
white on a rounded green tile. The interface also uses Lucide `zap`, `pin`,
`minus`, `x`, `plug`, `plug-zap`, `battery-medium`, `triangle-alert`, `shield`,
`settings`, `chevron-down` and `circle-check` paths from lucide-static 1.59.0,
stroked in the interface colours; compact SVG arc flags are written out in full
without changing the geometry. The complete Lucide/Feather license text is
retained in `LICENSE.Lucide.txt` and included in the same notice export.

## IntelMSR.bin

A PawnIO module that exposes a whitelisted set of Intel MSRs, including
`MSR_PLATFORM_ENERGY_STATUS` (`0x64D`). This program uses it to read platform
power; see the "平台功率" section of the README.

| | |
|---|---|
| Upstream | https://github.com/namazso/PawnIO.Modules |
| Release | `0.2.10`, published 2026-07-27 |
| Asset | `release_0_2_10.zip` |
| File | `IntelMSR.bin` |
| SHA-256 | `d6ed85d65ab17a22f813ef98207d6d537155ee2ded5976a21cb48413c9b92e5f` |
| Licence | LGPL-2.1-or-later |
| Source | `IntelMSR.p` in the upstream repository at the tag above |
| Source commit | `c683032770575d7705d1149f9d7fa7fd381766fc` |
| Source bundle | `PawnIO.Modules-0.2.10-source.zip` |
| Source bundle SHA-256 | `afb96a3d6f562350d3cd0b0af1ca3dc5c3d53ff6dc4d28b15d23f015b2a4030d` |

The file is redistributed unmodified and is embedded into the executable with
`go:embed` so that the portable application remains self-contained.
Placing a file named `IntelMSR.bin` beside the executable overrides the embedded
copy, which is how the LGPL requirement to allow replacing the library is
satisfied.

Rebuilding it from source needs the PawnIO compiler toolchain from the upstream
project. The corresponding upstream tag archive is vendored beside the binary;
it includes `IntelMSR.p`, the Pawn headers, the compiler source/binary packages,
the upstream build workflow, and the complete LGPL-2.1 text. The compiled
artefact is vendored here only so that a normal build does not need that
toolchain or network access.

Every GitHub Release publishes the source bundle and a standalone third-party
notice beside the portable executable and installer. The installer also places
that notice and the LGPL-2.1 text beside the installed application. The same
notice and licence are embedded in the EXE and can be extracted with
`--third-party-notices <path>`.

## PawnIO driver and PawnIOLib

Not redistributed. The driver is installed separately by the user from
https://pawnio.eu/ and `PawnIOLib.dll` is loaded from that installation.

PawnIO itself is GPL-2.0 with an exception for independent modules that
communicate with it solely through its device IO control interface, which is
what `PawnIOLib` does on this program's behalf. `PawnIOLib` is
LGPL-2.1-or-later and is dynamically loaded, never statically linked.
