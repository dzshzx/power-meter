# Manual Windows CI

Windows build and install acceptance is available through the manually
dispatched `Windows validation` workflow (`.github/workflows/ci.yml`) and the
Release workflow (`.github/workflows/release.yml`). Both run
`scripts/test.ps1 -RequireInstaller`, so a missing Inno Setup compiler fails
the run instead of skipping real-installer acceptance.

## Task-branch flow

Manual CI runs on a pushed task branch, so finalize its commits before
pushing. Once the run passes, integrate the branch without rewriting the
pushed commits:

1. In the task worktree, merge master into the branch (`git merge master`).
2. From the main checkout, fast-forward master with
   `git merge --ff-only <task-branch>`.
3. Synchronize master with `land --no-recut`.
