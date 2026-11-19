# Power Meter agent notes

Power terminology and measurement boundaries are defined in `GLOSSARY.md`;
build and packaging commands are in `README.md`, release commands in
`docs/release.md`, and PawnIO obligations in `third_party/NOTICE.md`. Which
Windows interfaces and sensors can or cannot supply whole-system input power,
as measured on the target notebook, is in
`docs/research/system-input-power.md`; read it before adding or changing a
power source. Manual laptop checks are in `docs/testing/host-acceptance.md`,
and the logon-startup security invariants in
`docs/testing/protected-autostart.md`.

The app is Go with mygo native UI (`go tool mygo`, version pinned in
`go.mod`); it ships for windows/amd64 only. Power Meter 1.5 and earlier was
a C# program; the installer refuses to run while it is installed, and nothing
of it is migrated (settings key, task name and install folder are new).

- Keep the manifest `asInvoker`: GUI startup requests elevation once, with
  cancellation falling back to ordinary mode. CLI commands never request UAC.
  PawnIO/Psys requires a user-installed official driver and elevated launch;
  the app must not silently install it.
- Display rules (`≈`/N/A, tones, statistics, tray text) live in
  `meter.Session`, which returns a `meter.View` that `internal/gui` only
  draws; snapshots come only from `power.Compose`. Startup decisions (exit
  codes, steps, switch state) are pure functions in
  `internal/autostart/policy.go`; the Windows files gather facts and carry
  out the plans. Cover new rules through these interfaces with Go tests;
  `--self-test` is the release EXE smoke.
- The window draws everything itself in a transparent frameless window:
  continuous corners (`internal/gui/shape.go`), MiSans, palette and metrics
  in `internal/gui/theme.go`. Check a UI change with the previews
  (`POWERMETER_PREVIEW_DIR=<dir> go test -run TestPreviews ./internal/gui`)
  and show them before committing; the maintainer judges the look.
- MiSans is not in the repository (its licence forbids separate
  distribution): `go run ./tools/fonts` fetches the pinned faces into
  `internal/fonts/misans/`. The program compiles and tests without them, so
  the gate and CodeQL depend only on the commit; such a build draws with
  system fonts and fails `--self-test`. Fetch them before rendering
  previews; only Windows validation and Release fetch them in CI. The app
  must keep saying it uses MiSans (settings popover and notices).
- Portable EXE and per-user installer are equal release options. Preserve the
  replaceable `IntelMSR.bin` sidecar, LGPL notice/licence and exact corresponding
  source bundle; release assets include both formats, and every asset has a
  `.sha256` file.
- Gate: `bash scripts/check.sh` (Linux: format, vet and staticcheck for linux
  and windows, Go tests, cognitive complexity; `wt land` runs it). Windows-only
  tests, the smoke run and the installer test run on Windows: the manually
  dispatched `Windows validation` workflow, or by hand as in
  `docs/testing/manual-ci.md`.
- Dependency updates are verified by an agent on the Windows host and then
  landed; they go to the maintainer only when evidence shows the update breaks
  the program.
- `dist/` is generated; binaries belong in GitHub Releases.
