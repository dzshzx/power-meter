# Window, localization and installer

## Window

The window is one panel in a transparent, frameless window: the app draws
the panel's continuous corners, hairline outline and shadow itself, and only
the panel takes the mouse (the shadow margin lets clicks through). It follows
the system's light or dark appearance. Windows 11 is the only supported
target; Windows 10 is not tested.

Geometry is in DIPs, which mygo scales to each monitor's DPI. The panel is
360 wide with a corner radius of 32; the cards sit 12 inside it with a
radius of 20, so the corners are concentric. Every corner is continuous
(`internal/gui/shape.go`: Figma's corner smoothing at 0.6), not a circular
arc. The title bar and the footer are each 48 high. The title is MiSans
Semibold 16, set 1.5 below the bar's centre: centred by its box, the Chinese
title looks higher than the thin icon buttons beside it.

From the top:

- Title bar: the app's name, then pin, minimize and close (close hovers red).
  Dragging the bar moves the window.
- Reading card: the display-mode tabs (整机功率 / 电池端, one face, the chosen
  one in full ink) with the reading's exact boundary on the right; the
  headline, with a smaller `≈` and unit; the supply state (Lucide glyph and
  words) beside the battery's voltage, current and level; the 30-minute
  trend; the 10-minute mean and 5-minute peak. Where a pair of lines does not
  fit side by side (English is longer), the app measures them in MiSans and
  stacks them, and the window grows to fit; nothing is cut off.
- Rows card: the three boundaries the headline does not show.
- Notice card, when needed: why a reading or setting is missing, and the
  restart as administrator that would help.
- Footer: the update time and the settings button. Its popover holds logon
  startup, language, the current privilege with the administrator restart,
  and the note that the interface uses MiSans (the font licence asks for it).

Colours: neutral greys, one blue for data and actions, amber only to warn and
red only for a sensor error. Estimates keep `≈` and a step lighter ink. The
tray tile keeps solid state colours (green charging, amber discharging, grey
otherwise) under white figures.

Text uses MiSans Regular, Medium and Semibold, embedded in the executable
(`internal/fonts`), with Microsoft YaHei UI as the fallback; changing figures
use tabular digits. Display figures use the typographic minus (U+2212); CLI
reports and logs keep ASCII.

The trend records both display boundaries from the first reading, one point
per second for 30 minutes, in memory only; switching mode keeps both. Until
30 minutes are recorded the axis counts minutes from the first reading
(0 … 30 分) and states the recorded time; the curve fills from the left and is
never stretched. Afterwards the axis reads −30 分 … 现在. The line is the mean
of each pixel column over a soft area, with the live point at its end. N/A
readings, failed captures and timing gaps break the line instead of drawing
zero.

Language defaults to the Windows UI language (Chinese locales use Simplified
Chinese, other locales use English). The settings popover and the tray's
「语言 / Language」 menu offer system default, Chinese and English; an explicit
choice persists in `HKCU\Software\PowerMeter`. Cached hardware diagnostics
are translated at presentation time, leaving OS exception details and user
paths intact.

## Verification

```sh
go run ./tools/fonts
POWERMETER_PREVIEW_DIR=<dir> go test -run TestPreviews ./internal/gui
```

renders every fixture state (starting, warming, early, charging, idle,
discharging, supplemented, unavailable, and the settings popover) in Chinese
and English, light and dark, at 100% and 175%, plus the tray tiles. They use
deterministic fixture readings, not live laptop readings, and run on Linux
too, once `go run ./tools/fonts` has fetched the fonts. Visual review covers the 175%
previews in both languages and both appearances. On Windows, `PowerMeter.exe --screenshot` and
`--tray-preview` render through the real app; `tools/release smoke` runs
them.

## Installer

The installer is the NSIS installer `mygo build` makes, with the steps in
`installer/installer.nsh`. It installs for the current user into
`%LOCALAPPDATA%\Programs\Power Meter`, without administrator rights, adds the
Start menu entry `Power Meter`, and writes `THIRD-PARTY-NOTICES.txt` beside
the program. It stops (exit code 3) while Power Meter 1.5 or earlier is
installed and asks for that to be uninstalled first; nothing of 1.5 is
migrated.

After the files are in place, setup runs `--sync-autostart`: an outdated
protected copy is refreshed with one UAC prompt. The uninstaller first runs `--remove-autostart` and requests
elevation to delete this installation's protected copy; a failed cleanup
stops the uninstall and keeps the program. `tools/release install-test`
checks the refusal over 1.5, install, reinstall and uninstall silently.

Portable files that are moved or renamed need startup enabled again from the
new location, with the existing replacement confirmation.
