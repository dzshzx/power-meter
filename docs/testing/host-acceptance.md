# Physical laptop acceptance

The automated Windows suite (`go test` on Windows, `tools/release smoke` and
`tools/release install-test`, run by the `Windows validation` workflow)
exercises the portable executable, package assets, and an isolated per-user
installer lifecycle. A laptop with a real battery and an optional
user-installed PawnIO driver needs separate observation. Record these checks
against the exact candidate commit and built executable; pending checks do not
count as passed.

## Checks

- **Identity**: record `git rev-parse HEAD`, Windows build (`winver`), and
  SHA-256 of the tested EXE/setup. Match the workflow run's commit.
- **Battery operation**: on battery power, run
  `PowerMeter.exe --power-probe report.txt 10` and observe the GUI. The battery
  terminal value is negative, System Load Power is its positive magnitude, and
  the display does not label it as measured adapter input. Save a redacted
  probe report and screenshot.
- **External power**: connect AC and repeat the probe. Charging shows positive
  battery terminal power; supplementation, if observed, shows negative battery
  terminal power. Estimated System Input Power uses platform plus signed battery
  power and is marked estimated. Do not infer unobserved supplementation from
  another state.
- **Ordinary launch**: launch without elevation or cancel the GUI's UAC prompt.
  Battery and CPU measurements remain available when supported; platform/Psys
  explains unavailability. CLI diagnostics must not show UAC.
- **Driver and admin launch**: only if the user has installed the official
  PawnIO driver, launch the GUI as administrator and run `--power-probe`.
  Record driver and hardware availability; Psys may still be unavailable on
  unsupported wiring. Do not install a driver as part of this check.
- **Protected logon startup**: follow `protected-autostart.md`: enable startup,
  replace the user-folder EXE with the ordinary token, run the logon task,
  upgrade and uninstall. The task only runs the admin-only copy, whose hash
  tracks the installed version.
- **Installed application**: install the exact setup under the current user,
  check the installed EXE, Start Menu entry, notices and LGPL text, then
  uninstall. Record the installed EXE SHA-256 and cleanup outcome.

## Interactive acceptance

Use a separately copied development EXE; keep the installed release intact.

1. Open without arguments under an ordinary token. Cancel UAC. Confirm a usable
   ordinary window remains, the reason is visible, and battery/CPU readings are
   retained. On external power, unavailable platform data means `N/A` / `--` for
   the whole-system display.
2. Select the administrator restart action. Cancel once, then accept. Verify
   cancellation preserves the window, successful handoff leaves one development
   window/tray, and the new process has an elevated token. Driver absence must
   produce an explicit reason without another elevation loop.
3. Change modes through the tabs in the reading card and the tray menu. Check the labels, number,
   menu checkmark, statistics reset and saved mode after relaunch. When using another
   administrator's credentials, preferences belong to that Windows account.
4. Check 100%, 175% and 300% scaling and a return to 100%. Controls remain reachable
   on a constrained screen; error messages remain readable/scrollable.
5. Check charging, full/idle, unplugging, resuming from sleep and unavailable source
   recovery. A mode or supply-state transition must not retain mixed statistics.
6. If comparing a physical meter, identify its measurement boundary and record
   values over the same interval as the software. Report the average difference
   and response to load/charge changes; do not infer accuracy from nonsynchronous
   screenshots or convert smoothing into an accuracy claim.
7. Enable login startup from the development copy using a disposable test
   account. Verify the registered executable path, user, interactive logon and
   elevated run level. Start the task explicitly: the app must enter the tray
   without another UAC dialog, and another scheduled start must not create a
   duplicate instance. Battery operation must not prevent or stop the task.
8. Disable login startup and read back that its task is gone. A moved portable
   copy must not silently adopt another copy's registration; uninstall cleanup
   must preserve registrations targeting other executable paths. Keep actual
   reboot/logon acceptance distinct from a manual scheduled-task launch.

## Confirmed on the target notebook

- Cancelling the first UAC prompt keeps the ordinary window usable; the
  administrator restart action succeeds after approval, with matching
  whole-system window and tray readings.
- Account-name normalization and the task ACL that lets the ordinary token
  delete the task were set from live scheduler results, not only from tests.
- Protected logon startup: see `protected-autostart.md`.

Power Meter 1.6.0, the first Go release, replaced 1.5.5 on the same notebook:

- `tools/release verify -version 1.6.0` passed on the notebook, and the EXE
  installed from the public setup has the portable asset's SHA-256.
- The 1.5.5 uninstaller removed its entry, folder, task and protected copy;
  only its settings key stays, which 1.6.0 does not read.
- Paired `--power-probe` runs of 1.5.5 and 1.6.0 on external power, under both
  tokens, give the same report structure and readings within sampling noise.
- The maintainer cancelled the first UAC prompt, then cancelled and accepted
  the administrator restart: one elevated instance remained, with platform
  power. On battery the window says System Load Power without `≈` and the tray
  is amber; on external power it shows the estimate with `≈` and a green
  tray. Statistics start again after the supply change.
- Scaling 100%, 300% and back to 175%: no overlapping or clipped text. At
  300% 1.6.0 kept the window (585 DIPs) taller than the work area (552
  DIPs), so its footer overlapped the taskbar. Later builds refit the window
  to the work area whenever the scaling or work area changes, also at
  startup, and scroll the rest; checked on the notebook switching 175% to
  300%, starting at 300% and returning to 175%.
- Readings continue after resuming from modern standby. In the first minutes
  after one resume the trend broke several times (sample gaps over 5 s);
  the cause is not known yet.
