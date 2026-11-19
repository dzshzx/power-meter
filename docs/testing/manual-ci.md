# Manual Windows CI

The Windows part of the checks runs on Windows only: Windows-only Go tests,
the live autostart tests, the smoke run of the built executable and the
installer test. The manually dispatched `Windows validation` workflow
(`.github/workflows/ci.yml`) and the Release workflow
(`.github/workflows/release.yml`) both run them on a hosted Windows runner,
which is elevated and thrown away afterwards:

```sh
go run ./tools/fonts
go test -count=1 ./...
POWERMETER_LIVE=1 go test -count=1 ./internal/autostart
go tool mygo build -platform windows/amd64
go run ./tools/release smoke -exe dist/windows-amd64/PowerMeter.exe -work <dir>
go run ./tools/release install-test -version 0.0.0 -setup dist/windows-amd64/power-meter-setup-0.0.0-amd64.exe
```

## Local run

The executable and the installer cross-compile from Linux (`mygo build`
needs `makensis` for the installer there, or `-skip-installer`); copy them to
the Windows host and run the smoke test there. The live autostart tests
register tasks and write below Program Files, so run them only on a
disposable machine or the runner. `install-test` refuses to run where Power
Meter, or Power Meter 1.5 or earlier, is installed.

An SSH session on the Windows host runs in session 0 without an interactive
desktop, so the window cannot be shown there. Run GUI checks through a
one-off interactive scheduled task (`fleet-admin win-elevated run
--run-level Limited` in fleet-state), or rely on the dispatched workflow: its
runner has an interactive session.

## Task-branch flow

Manual CI runs on a pushed task branch, so finalize its commits before
pushing. Once the run passes, run `wt land` from the task worktree. For a
branch that has been pushed it merges the latest origin master into the branch
instead of rebasing, so the pushed commits stay as they are; it then runs the
repository gate `bash scripts/check.sh` and fast-forwards origin master to the
result. The Windows part is the manual run that just passed. If origin master
moves while the gate runs, `wt land` starts over by itself.
