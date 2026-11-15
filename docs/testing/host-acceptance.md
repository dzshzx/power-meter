# Physical laptop acceptance

The automated Windows-local suite exercises the portable executable, package
assets, and an isolated per-user installer lifecycle. A laptop with a real
battery and an optional user-installed PawnIO driver needs separate observation.
Record these checks against the exact candidate commit and built executable;
pending checks do not count as passed. Use `dist/test-report.json` for the
automated pass/fail/not-run results.

## Checks

- **Identity**: record `git rev-parse HEAD`, Windows build (`winver`),
  PowerShell version, .NET Framework 4.x Release registry value, Inno Setup
  version, and SHA-256 of the tested EXE/setup. Match the report's commit and
  tool versions.
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
3. Change modes through both the selector and tray menu. Check the labels, number,
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
