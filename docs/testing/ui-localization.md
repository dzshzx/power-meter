# Compact UI, localization and upgrade compatibility

Power Meter is displayed as 功率计 in Simplified Chinese. The executable and
release assets use `PowerMeter`; the repository is `dzshzx/power-meter`.
The settings key, installer AppId and logon-task ownership identity retain
their pre-rename values for compatibility.

The window uses one dark instrument theme (it does not follow the system
light/dark setting). The normal client area is 388 × 568 logical pixels; a
diagnostic expands it to 660, and clearing it removes both its space and the
restart action. The system caption is replaced by a drawn title bar holding
the application tile (Lucide zap), the display-mode segments, pin, minimize
and close. The window keeps the native caption, system-menu and minimize
styles while `WM_NCCALCSIZE` removes the frame, so dragging uses the system
move loop, Alt+Space opens the system menu, the taskbar and minimize animation
behave normally, and DWM supplies the shadow and rounded corners. Without a
maximize style a caption double-click does nothing and maximize commands are
ignored. Windows 11 is the only supported target; Windows 10 is not tested.

Below the boundary caption and supply state (with its Lucide glyph) the
reading uses Bahnschrift Light SemiCondensed at 84 px; other figures use
Bahnschrift named instances and fall back to Segoe UI; interface text uses
Microsoft YaHei UI. Display figures use the typographic minus (U+2212); CLI
reports and logs keep ASCII. Battery voltage and current follow, then the
30-minute trend, the 10-minute mean, 5-minute peak and a ten-segment battery
level, and the three other boundaries joined to their values by dotted
leaders. Estimates keep `≈` and a quieter colour.

The trend records both display boundaries from the first reading, one point
per second for 30 minutes, in memory only; switching mode keeps both. Until
30 minutes are recorded the axis counts minutes from the first reading
(0 … 30 分) and states the recorded time; the curve fills from the left and is
never stretched. Afterwards the axis reads −30 分 … 现在. Each pixel column
keeps the min, max and mean of its readings: the mean is the line, the
min–max range a faint band, and three translucent wider strokes approximate a
glow. The peak marker is the reading with the largest magnitude in the window,
sign kept — the same rule as the 5-minute peak, so a discharge peak is the
most negative reading. N/A readings, failed captures and timing gaps break
the line instead of drawing zero. The current value has a cursor ring.

Language defaults to the Windows UI language (Chinese locales use Simplified
Chinese, other locales use English). The window and tray language menus offer
system default, Chinese and English. Explicit choices persist in the existing
HKCU settings key. Cached hardware diagnostics are translated at presentation
time, leaving OS exception details and user paths intact.

## Verification

Run from a Windows-local checkout:

```powershell
pwsh -NoProfile -File scripts/test.ps1
```

Local Windows verification and packaging run on NERV, from a Windows-local
checkout using PowerShell 7. GitHub Windows CI is an additional check.
On 2026-09-23, verification included:

- Power derivation, signed battery supplementation, measured/estimated display,
  per-monitor DPI transitions, CLI routing and embedded third-party notices.
- Chinese → English → Chinese switching at 96, 144, 168, 192, 288 and back to
  96 DPI. Each pass covers idle, charging and battery discharge with a full
  30-minute trend (including a peak and an N/A spell that must stay a gap),
  external-power supplementation, unavailable platform power, startup, a
  two-minute warm-up and a four-minute partial trend. Actual WinForms
  controls are checked for overlapping bounds and clipped content (labels,
  readouts with their affixes and captions, the status glyph). Diagnostic
  expansion and collapse are exercised repeatedly at each scale.
- Full statistics use concise captions; partial windows average and peak the
  readings so far and show their coverage («均值 · 2分/10分», «Avg · 2m/10m»:
  whole seconds under a minute, then whole minutes). The fixtures keep the
  charging spike at −10 min and the discharge spike at −12 min outside the
  5-minute peak window. Each mode exposes exactly three supporting rows.
- The drawn title bar: the window keeps caption, system-menu and minimize
  styles and no maximize style; the title area hit-tests as caption and the
  body as client; without scroll bars the client area equals the window; pin,
  minimize and close fill the title height, render their glyphs and carry
  localized names; minimize minimizes and a maximize command is ignored.
- The unselected mode segment gains a quiet hover fill and text emphasis.
  Bitmap comparisons verify the feedback appears, disappears on mouse leave,
  and never changes the selected mode by itself.
- Actual pin clicks toggle the window's TopMost state in both directions; its
  glyph must render in both states. The settings popover opens in each language
  and scale, and its content is checked for clipping, overlap and double scaling.
  A DPI transition disposes the popover before replacing the fonts.
- The unavailable-platform notice shows the boundary in bold, at least the
  lines it needs up to two, the full text as tooltip, and the restart action
  in an ordinary session.
- Real English install, Chinese upgrade, localized uninstall names, and
  upgrade replacement of the old executable with the compatibility launcher.
  An isolated legacy logon task is recognized by the new executable and, since
  2026-09-26, migrated to the protected copy; a different portable copy cannot
  claim it. See `protected-autostart.md`.
- The actual legacy launcher starts the new GUI hidden with `--autostart`,
  retaining the inherited token. A manual new-name launch restores that same
  process. Cleanup forwarding preserves the child exit code.
- Real uninstall cancellation, cleanup failure and successful removal. The
  tests use unique installer/task identities and temporary directories.
- All 11 Python version-plan tests.

The installer regression also retains the actual Start Menu names inside an
isolated folder: English install, legacy-name migration during Chinese upgrade,
Chinese-to-English upgrade, and removal on uninstall. A locked executable makes
a real Chinese upgrade fail; the regression checks that the previous executables
and Start Menu entries survive, the legacy entry still targets a working
executable, and no Chinese entry appears. A manually seeded obsolete-name
shortcut targeting a different copy survives cleanup. That shortcut is created
after a complete uninstall, so no older uninstall log owns its name.

Run this installer lifecycle on Windows hosts with legacy code pages and UTF-8.
The Chinese shortcut name exercises target lookup outside an English process's
ANSI code page; a UTF-8 host can hide failures in that lookup. On 2026-10-10,
separate COM probes used process-specific `activeCodePage` manifests to select
ACP 1252 and ACP 65001 without changing the machine locale. The Unicode Shell
reader preserved Chinese and Devanagari shortcut names, parent directories and
executable targets in all 12 reads across 32-bit and 64-bit processes.

`scripts/test-ui.ps1` also runs separately and saves PNGs plus layout metadata
under `dist/ui-preview/`; `-ShowcaseDirectory <dir>` adds the review set,
including the settings popover composed over the window from the controls'
own renderings (nothing is read from the screen). These use deterministic fixture measurements, not
live laptop readings. The 175% Chinese and English idle previews were visually
reviewed, along with unavailable-source and small-icon renders.

The initial installer acceptance used real Inno Setup 6.7.3. The installer
regression fixture uses a short unique AppId: Inno Setup shortens
long IDs in uninstall registry keys, so a test must not infer an unshortened
key from an oversized ID.

Text alignment is checked from rendered glyphs: the three statistic values must
agree within one physical pixel. Readouts align the smaller `≈` and unit to the
number by GDI ascent (GDI+ family metrics differ for Bahnschrift's named
instances) and the headline is checked for a common rendered baseline. The
settings popover uses AntdUI's popover and selector in AntdUI dark mode; the
switch and buttons are drawn by the application.

The fixture host adopts the application's Per-Monitor V2 thread context before
creating controls. Screenshots use the visible DWM frame, excluding invisible
resize margins that PrintWindow otherwise leaves black. Windows 11 captures
apply DWM's 8-pixel (scaled) corner rounding, since PrintWindow returns the
square bitmap.

## Visual references and toolkit choice

The official screenshots of [Twinkle Tray](https://github.com/xanderfrangos/twinkle-tray)
and [EarTrumpet](https://github.com/File-New-Project/EarTrumpet) informed the
compact utility layout: prominent live values, coherent groups, quiet surfaces,
and secondary operations collected into settings. The Windows
[type ramp](https://learn.microsoft.com/en-us/windows/apps/design/signature-experiences/typography)
informed regular labels, semibold numeric values and a smaller watt unit.

Further reference review covered [Stats](https://github.com/exelban/stats),
[BatteryBoi](https://github.com/thebarbican19/BatteryBoi), and
[EnergyStarX](https://github.com/JasonWei512/EnergyStarX). Stats' battery panel
separates one primary figure from supporting measurements; BatteryBoi focuses a
small surface on one state and value; EnergyStarX keeps Windows-native controls
and a clear central focus. The resulting meter centers the watts reading,
reduces repeated data and uses one quiet supporting group. No normalization ring
is used for watts because the app has no measured maximum input capacity.
The headline uses Bahnschrift (Segoe UI where absent). The watt unit is aligned
by font ascent rather than line-box height, and estimates retain a neutral
muted color and the approximation mark.

[AntdUI](https://github.com/AntdUI/AntdUI) and
[Krypton Toolkit](https://github.com/Krypton-Suite/Standard-Toolkit) were checked
for existing .NET Framework support. AntdUI supplies the specific buttons,
popover, panel and settings controls without replacing the WinForms lifecycle.
Its pinned 2.4.17 net46 package has no additional NuGet runtime dependencies.
The assembly is embedded and resolved before the GUI entry point is JIT-compiled;
the existing isolated-EXE CLI/preview tests verify that no DLL sidecar is needed.
The restore script checks the archive hash and re-extracts the assembly on each
build. The executable is approximately 3.3 MiB with the UI toolkit and notices.

Lucide stroke geometry is wrapped in a group because AntdUI applies a root fill
when tinting SVGs. Both states of the pin explicitly carry the same glyph.
Settings bounds are already scaled by the application, so automatic content
scaling inside the popover is disabled to avoid applying DPI twice.

## Upgrade compatibility

After the new files and the Start Menu entry selected by the installer language
are installed, setup removes obsolete `Battery Charge Meter`, `Power Meter` and
`功率计` shortcuts whose target is this installation's current or legacy
executable. If the replacement entry is absent or does not target the installed
`PowerMeter.exe`, setup retains the old entries. In particular, an upgrade that
fails while copying the executable keeps the previous functioning entry.

Target inspection uses the Windows Shell's Unicode
[ShellLinkObject interface](https://learn.microsoft.com/en-us/windows/win32/shell/folderitem-getlink).
This keeps localized shortcut filenames readable when they contain characters
outside the process's ANSI code page. The installer lifecycle's target
assertions use that Unicode interface too.

This ownership check applies to obsolete-name cleanup during installation.
That cleanup leaves shortcuts to other copies and unreadable shortcuts in place.
The normal `[Icons]` entry can overwrite an existing shortcut with the selected
language's name. Uninstall can also remove a repointed shortcut whose name was
recorded by an earlier real installation: Inno Setup
[appends upgrades to the existing uninstall log](https://jrsoftware.org/ishelp/topic_appendnotes.htm),
including those earlier shortcut records.
Changing the running application's language only updates its window and tray;
Start Menu names follow the most recent installation language.

A fresh installation contains `PowerMeter.exe`. When an old
`BatteryChargeMeter.exe` exists in the same installation directory, setup
replaces it with the small forwarding launcher. The launcher accepts GUI,
ordinary GUI, logon startup and startup cleanup entry points; diagnostic CLI
commands use `PowerMeter.exe`. Setup refreshes protected startup copies and
migrates tasks that target user-writable legacy launchers. When synchronization
needs elevation, setup requests UAC; declining keeps a stale protected copy
running or removes an unprotected task. Uninstall removes the matching logon
task and requests elevation to delete its protected copy before deleting the
installed executables.

Portable files that are moved or renamed need startup enabled again from the
new location, with the existing replacement confirmation. Keeping the old
launcher is an installation compatibility path, not a portable alias rule.
