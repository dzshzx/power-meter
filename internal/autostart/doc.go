// Package autostart is the logon startup: a Task Scheduler task for the
// signed-in user that runs a protected copy of the app elevated, straight
// into the tray. The rules are in docs/testing/protected-autostart.md.
//
// Every decision (exit codes, steps, switch state and notices) is a pure
// function of what was read, in policy.go, and the task shape this program
// registers and recognises is in task.go; both are tested on every
// platform. The Windows files gather the facts (the task through the
// scheduler's COM interface, the copy below Program Files, the token) and
// carry out the plans.
//
// # Commands
//
// RunSync is --sync-autostart and RunRemove is --remove-autostart; both
// never ask for UAC and return the exit code:
//
//	--sync-autostart    0  current, or applied (elevated)
//	                    3  this installation's copy is outdated (ordinary token)
//	                    4  a task in this program's shape for this installation runs a
//	                       user-writable file (ordinary token); this program never
//	                       registers one, the elevated run moves it to the copy
//	                    5  this installation's copy has no task any more (ordinary token)
//	                    1  failure
//	--remove-autostart  0  removed, or there was nothing to remove
//	                    3  task deleted; deleting the copy needs an elevated rerun
//	                    6  removed; a same-name task in a foreign shape was left as it is
//	                    1  failure
//
// An elevated GUI launch calls SyncOnStart before it shows the window.
//
// # The window's switch
//
// The settings panel and the tray menu read the switch with ReadStatus on
// open and on every window activation:
//
//   - Status.Switch: SwitchOn checked, SwitchOff unchecked, SwitchUnknown
//     indeterminate (the tray item then says to see the main window).
//   - Status.Tooltip: the switch's tooltip, already in the current language.
//     It names the other installation when enabling would replace its
//     startup.
//   - Status.Notice: a standing problem (a foreign same-name task, a task
//     that needs repair, a read failure) for the diagnostics area, as a
//     Chinese source string shown through i18n.Diagnostic. It takes
//     precedence over the notice of the last change.
//
// Toggling calls Change(enable, confirm) and shows the notice it returns
// the same way (nothing when it returns ""), then reads the switch again.
// Enabling needs an elevated process: under an ordinary token Change returns
// a notice that points at the "restart as administrator" button. When
// enabling would replace another installation's startup (its task, or the
// protected copy it recorded), Change calls confirm with a title and a
// yes/no question in the current language and goes on only on true; the
// window shows them as a dialog. Change and ReadStatus block on the
// scheduler and the file system; confirm runs on the goroutine that called
// Change. Disabling never asks: it removes this installation's task, and
// its copy too when elevated (otherwise the next elevated launch removes
// it).
package autostart
