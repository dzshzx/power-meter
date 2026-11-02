# Power Meter agent notes

Windows WinForms monitor targeting .NET Framework 4.8.1. Power terminology and
measurement boundaries are defined in `GLOSSARY.md`; build and packaging
commands are in `README.md`, release commands in `docs/release.md`, PawnIO
obligations in `third_party/NOTICE.md`, and runtime and toolchain decisions
(target framework, SDK, Inno Setup) in `docs/adr/`.

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
  Windows-local checkout (.NET Framework rejects WSL UNC paths). Local host and
  UI acceptance requirements are in `docs/testing/ui-localization.md`.
- `scripts/test.ps1` and the UI acceptance above cannot run in WSL; run them
  manually before landing.
- Dependency updates are verified by an agent on the Windows host and then
  landed; they go to the maintainer only when evidence shows the update breaks
  the program (fleet-state ADR 0053).
- `dist/` is generated; binaries belong in GitHub Releases.
