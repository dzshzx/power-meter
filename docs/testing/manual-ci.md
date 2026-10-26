# Manual Windows CI

Windows build and install acceptance is available through the manually
dispatched `Windows validation` workflow (`.github/workflows/ci.yml`) and the
Release workflow (`.github/workflows/release.yml`). Both run
`scripts/test.ps1 -RequireInstaller`, so a missing Inno Setup compiler fails
the run.

## Task-branch flow

Manual CI runs on a pushed task branch, so finalize its commits before
pushing. Once the run passes, integrate the branch without rewriting the
pushed commits:

1. In the task worktree, merge the latest origin master into the branch
   (`git fetch`, then `git merge origin/master`).
2. From the same task worktree, run `land`. It pushes the worktree HEAD to
   origin master; the local master advances only by land's post-push sync.
   If land exits 3 because origin moved, repeat both steps.
