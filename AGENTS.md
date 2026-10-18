# Power Meter agent notes

Windows WinForms monitor targeting .NET Framework 4.8.1. Power terminology and
measurement boundaries are defined in `GLOSSARY.md`; packaging and release
commands are in `README.md`, PawnIO obligations in `third_party/NOTICE.md`, and
runtime and toolchain decisions (target framework, SDK, Inno Setup) in `docs/adr/`.

- Battery terminal power is signed: charging positive, discharge negative.
  On external power, estimated System Input Power is Platform Power plus this
  signed value, including battery supplementation. On battery, System Load
  Power is the magnitude of discharge. Conversion losses are not measured.
- Keep the manifest `asInvoker`: GUI startup requests elevation once, with
  cancellation falling back to ordinary mode. CLI commands never request UAC.
  PawnIO/Psys requires a user-installed official driver and elevated launch;
  the app must not silently install it.
- Display rules (`≈`/N/A, tones, statistics, tray text) live in `MeterSession`,
  which returns a `MeterView` that `MainForm` only binds; snapshots come only
  from `PowerSnapshot.Compose`. Startup decisions (exit codes, steps, switch
  state) live in the pure `AutostartPolicy`; `AutostartManager` gathers facts
  and executes. Cover new rules through these interfaces in the xUnit
  project `tests/PowerMeter.Tests`; `--self-test` is the release EXE smoke.
- Portable EXE and per-user installer are equal release options. Preserve the
  replaceable `IntelMSR.bin` sidecar, LGPL notice/licence and exact corresponding
  source bundle; release assets include both formats and SHA-256 files.
- Verification entry: `pwsh -NoProfile -File .\scripts\test.ps1` from a
  Windows-local checkout (.NET Framework rejects WSL UNC paths). With ISCC it
  tests a real per-user install/uninstall; otherwise only packaging inputs.
  CI and Release pass `-RequireInstaller`, so missing ISCC fails. The JSON
  report at `dist/test-report.json` marks unrun real-installer acceptance.
  Local host and UI acceptance requirements are in `docs/testing/ui-localization.md`.
- Daily changes complete the relevant local checks before `land --no-recut`
  synchronizes master. Renovate PRs require local native validation before manual merging.
  Windows build/install acceptance remains available through manual CI and
  the Release workflow. UI layout or `scripts/test-ui.ps1` changes also pass
  manual Windows validation before landing, because the runner's smaller screen
  clamps the window at high DPI (until #31 lands). Manual CI runs on a pushed
  task branch, so finalize its commits before pushing; afterwards integrate it
  by merging master into it and landing with `git merge --ff-only` from the
  main checkout. An authorized annotated `vX.Y.Z` tag must match manifest
  `X.Y.Z.0`; master synchronization itself does not publish. Published tags
  are immutable; fixes use a new patch version.
- `dist/` is generated; binaries belong in GitHub Releases.
- `scripts/format.sh` 覆盖 ruff format、prettier、shfmt；C# 与 PowerShell 不在范围内。
