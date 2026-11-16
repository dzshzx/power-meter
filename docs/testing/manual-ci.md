# Manual Windows CI

Windows build and install acceptance is available through the manually
dispatched `Windows validation` workflow (`.github/workflows/ci.yml`) and the
Release workflow (`.github/workflows/release.yml`). Both run
`scripts/test.ps1 -RequireInstaller`, so a missing Inno Setup compiler fails
the run.

## Local run over SSH

An SSH session on the Windows host runs in session 0 without an interactive
desktop. The installer lifecycle test clicks the real Inno uninstall dialog,
so under SSH it times out at that step. Run `scripts/test.ps1` there through
a one-off interactive scheduled task, or rely on the dispatched workflow: its
runner has an interactive session.

## Task-branch flow

Manual CI runs on a pushed task branch, so finalize its commits before
pushing. Once the run passes, run `wt land` from the task worktree. For a
branch that has been pushed it merges the latest origin master into the branch
instead of rebasing, so the pushed commits stay as they are; it then runs the
repository gate `bash scripts/check.sh` and fast-forwards origin master to the
result. The Windows part is the manual run that just passed. If origin master
moves while the gate runs, `wt land` starts over by itself.
