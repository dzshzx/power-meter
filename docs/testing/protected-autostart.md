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
- A task that still runs a user-writable file (older versions, including the
  `BatteryChargeMeter.exe` compatibility launcher) is never reported as working
  startup. Setup migrates it with one UAC prompt or deletes it when elevation
  is declined; an elevated launch of the new version also migrates it.
- A confirmation to replace another installation's startup covers the task
  definition and the copy's recorded owner.
- The task ACL allows the ordinary token to read, run and delete it,
  but not rewrite its action.
- A same-name task in a shape this program never registers (edited by the
  user or another program) is never modified or adopted. Removal treats it as
  absent: this installation's copy is still removed, `--remove-autostart`
  returns 6 instead of failing, uninstall completes with a manual-deletion
  notice, and the in-app switch reads as off with the same hint.

## Testing

`tests/PowerMeter.Tests` covers the `AutostartPolicy` decision table and task
inspection; `scripts/test-installer-uninstall.ps1` (inside `scripts/test.ps1`,
elevated token required) covers real install, upgrade, ACL and uninstall.

`scripts/test-autostart.ps1 -CheckOrdinaryClient` is manual and needs an
elevated interactive session. From an elevated but non-interactive SSH session,
run it through a one-off scheduled task whose principal is the signed-in
user's SID with `-LogonType Interactive -RunLevel Highest`, since the account
name does not map there, and unregister the task afterwards.

The UAC prompt raised by an unelevated setup or uninstaller cannot be answered
by automation, so that branch (and its declined fallback) is covered only by
code review.

## Host results

Observed on a real installation, with the real AppId, task name and protected
root:

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

Sign-out and sign-in were not performed; the scheduler's on-demand run uses
the same principal and action as the logon trigger.
