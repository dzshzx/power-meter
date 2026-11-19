# Protected logon startup

The logon task runs with the highest available token. It must never execute a
file that the signed-in user's ordinary token can replace, because a replaced
file would then start elevated at the next sign-in without a UAC prompt.

## Invariants

- The task action is always
  `C:\Program Files\Power Meter\autostart\<SID>\PowerMeter.exe`. That folder,
  `autostart` and `Power Meter` carry an explicit, non-inherited DACL (SYSTEM
  and Administrators full control, Users read and execute) and are owned by
  Administrators, as are the copied files.
- Only an elevated process writes the copy, and it copies only its own image
  plus an `IntelMSR.bin` beside it. A process running from a protected copy
  never copies from its recorded source (`source.txt`).
- A task in this program's shape that runs a user-writable file is never
  reported as working startup. This program never registers one; setup moves
  it to the protected copy with one UAC prompt or deletes it when elevation is
  declined, and an elevated launch also moves it.
- A confirmation to replace another installation's startup covers the task
  definition and the copy's recorded owner.
- The task ACL allows the ordinary token to read, run and delete it,
  but not rewrite its action.
- A same-name task in a shape this program never registers (edited by the
  user or another program) is never modified or adopted. Removal treats it as
  absent: this installation's copy is still removed, `--remove-autostart`
  returns 6 instead of failing, uninstall completes with a manual-deletion
  notice, and the in-app switch reads as off with the same hint.
- Nothing refreshes the copy but an elevated process. The app has no
  self-update; one would have to go through an elevated run as well.

## Testing

`internal/autostart/policy_test.go` and `task_test.go` cover the decision
table and the task shape on every platform. With `POWERMETER_LIVE=1` and an
elevated token, `go test ./internal/autostart` on Windows registers, reads,
replaces and removes real tasks and protected copies under isolated names,
and checks the ordinary token's attempts on them through a helper process
(`probe_windows_test.go`); the `Windows validation` workflow runs it.
`tools/release install-test` covers real install, reinstall and uninstall.

The UAC prompt raised by an unelevated setup or uninstaller cannot be answered
by automation, so that branch (and its declined fallback) is checked in the
physical acceptance (`host-acceptance.md`) and by code review.

## Host results

Observed with Power Meter 1.5 (the C# program, whose task and installer
followed the same rules under its old names) on a real installation, with the
real AppId, task name and protected root:

- Upgrading over an older version whose task ran the user-writable
  `%LOCALAPPDATA%\Programs\Battery Charge Meter\BatteryChargeMeter.exe` moves
  the task action and working folder to the protected copy; its SHA-256
  matches the installed EXE.
- Under the ordinary token, replacing the user-folder EXEs succeeds, while
  opening the copy for write, creating a file beside it, renaming or deleting
  it, renaming its folder, adding an ACE and rewriting the task action are all
  denied. The on-demand task run then starts the original protected build,
  elevated.
- With a newer build placed in the install folder by the ordinary token,
  `--sync-autostart` returns 3 and leaves the copy unchanged. Upgrading while
  the logon instance runs refreshes the copy, keeps the task definition
  byte-identical, and restarts the instance from the refreshed copy, elevated.
- Ordinary `--remove-autostart` on a migrated task returns 3 (task deleted,
  copy kept); an elevated `--sync-autostart` then removes the unused copy and
  folders. Uninstall removes task, copy and `Program Files\Power Meter`.

Sign-out and sign-in were not performed on 1.5; the scheduler's on-demand run
uses the same principal and action as the logon trigger.

Observed with Power Meter 1.6.0 (Go) after a fresh install on the same
notebook:

- Enabling startup from the elevated window registers
  `PowerMeter.Logon.<SID>` for the user's interactive logon at the highest
  run level. The task runs the protected copy with `--autostart`, does not
  stop on battery, ignores new instances and has no time limit. The copy
  matches the installed EXE; only SYSTEM and Administrators can write to its
  folder.
- An on-demand run while the window is open starts no second process. After
  the instance is stopped, a run starts the protected copy elevated in the
  tray without a window or UAC. Starting the installed EXE under the ordinary
  token then brings that window back without UAC.
- Sign-out and sign-in: the task started the protected copy elevated in the
  tray right after logon, with no UAC.
