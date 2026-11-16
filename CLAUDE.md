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
- Gate: `bash scripts/check.sh` (Linux checks; `wt land` runs it). Before
  landing, run `pwsh -NoProfile -File .\scripts\test.ps1` and the UI acceptance
  in `docs/testing/ui-localization.md` from a Windows-local checkout; neither
  runs in WSL (.NET Framework rejects WSL UNC paths). A pushed task branch that
  ran manual CI lands the same way; see `docs/testing/manual-ci.md`.
- Dependency updates are verified by an agent on the Windows host and then
  landed; they go to the maintainer only when evidence shows the update breaks
  the program.
- `dist/` is generated; binaries belong in GitHub Releases.
