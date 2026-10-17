[CmdletBinding()]
param(
    [string]$Executable = (Join-Path $PSScriptRoot '..\dist\PowerMeter.exe'),
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '..\dist\ui-preview'),
    # Optional: also render the review set (states, languages, scales and the
    # settings popover over the window) into this directory.
    [string]$ShowcaseDirectory = ''
)

$ErrorActionPreference = 'Stop'
Import-Module Pester -MinimumVersion 5.9.1 -MaximumVersion 5.999 -ErrorAction Stop
if ($PSVersionTable.PSEdition -ne 'Desktop') {
    & "$env:WINDIR\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -File $PSCommandPath `
        -Executable $Executable -OutputDirectory $OutputDirectory -ShowcaseDirectory $ShowcaseDirectory
    if ($LASTEXITCODE -ne 0) { throw "UI verification failed: $LASTEXITCODE" }
    return
}
Add-Type -AssemblyName System.Windows.Forms, System.Drawing
Add-Type -ReferencedAssemblies System.Windows.Forms, System.Drawing @'
using System;
using System.Drawing;
using System.Reflection;
using System.Windows.Forms;
// Deterministic fixture readings. Every reading goes through the production
// PowerSnapshot.Compose and MeterSession; only the final one is bound to the
// window, so a thirty-minute trend costs one render.
public static class UiFixture {
    const BindingFlags All = BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic;
    static double Noise(int tick, int salt) {
        unchecked {
            uint x = (uint)(tick * 374761393 + salt * 668265263);
            x = (x ^ (x >> 13)) * 1274126177u;
            return ((x ^ (x >> 16)) & 0xffff) / 65535.0 - 0.5;
        }
    }
    static object Sample(Assembly assembly, string boundary, double watts) {
        Type sample = assembly.GetType("BatteryChargeMeter.PowerSample");
        object kind = Enum.Parse(assembly.GetType("BatteryChargeMeter.MeasurementKind"), "Measured");
        return sample.GetMethod("FromValue", All).Invoke(null, new object[] {
            Enum.Parse(assembly.GetType("BatteryChargeMeter.PowerBoundary"), boundary), kind, watts, "UI acceptance fixture", TimeSpan.FromSeconds(1) });
    }
    static object Missing(Assembly assembly, string boundary, string reason) {
        Type sample = assembly.GetType("BatteryChargeMeter.PowerSample");
        return sample.GetMethod("Unsupported", All).Invoke(null, new object[] {
            Enum.Parse(assembly.GetType("BatteryChargeMeter.PowerBoundary"), boundary), reason });
    }
    // Platform power: a slow drift, a busy spell peaking near tick 1200 and an
    // N/A spell (ticks 520-639) in the full charging trend.
    static double PlatformWatts(string scenario, int tick, int last) {
        if (tick == last) return 19.39;
        if (scenario == "charging" || scenario == "early" || scenario == "warming")
            return 18.6 + 1.6 * Math.Sin(tick / 97.0) + 0.9 * Math.Sin(tick / 23.0 + 1) + 0.9 * Noise(tick, 1)
                + 13.5 * Math.Exp(-Math.Pow((tick - 1200) / 38.0, 2)) + (last - tick < 60 ? 0.6 * Math.Sin(tick * 0.43) : 0);
        return 19.39 + Math.Sin(tick * 0.43) * 2.4;
    }
    static double BatteryWatts(string scenario, int tick, int last) {
        switch (scenario) {
            case "charging": case "early": case "warming": return tick == last ? 39.08 : 40.2 - 1.2 * tick / 1800.0 + 0.25 * Noise(tick, 2);
            case "discharging":
                if (tick == last) return -17.50;
                return -(16.4 + 1.1 * Math.Sin(tick / 61.0) + 0.6 * Math.Sin(tick / 13.0) + 0.5 * Noise(tick, 3)
                    + 9.0 * Math.Exp(-Math.Pow((tick - 1080) / 30.0, 2)));
            case "supplemented": return -5.0;
            default: return 0.0;
        }
    }
    public static void Feed(Form form, Assembly assembly, string scenario, int last) {
        object session = form.GetType().GetField("session", All).GetValue(form);
        MethodInfo observe = session.GetType().GetMethod("Observe", All);
        MethodInfo compose = assembly.GetType("BatteryChargeMeter.PowerSnapshot").GetMethod("Compose", All);
        Type readingType = assembly.GetType("BatteryChargeMeter.BatteryReading");
        Type stateType = assembly.GetType("BatteryChargeMeter.BatterySupplyState");
        string state = scenario == "charging" || scenario == "early" || scenario == "warming" ? "ExternalPowerCharging"
            : scenario == "discharging" ? "BatteryDischarging" : scenario == "supplemented" ? "ExternalPowerSupplemented" : "ExternalPowerIdle";
        DateTimeOffset end = DateTimeOffset.Parse("2026-09-23T14:38:15+08:00");
        object cpu = Sample(assembly, "CpuPackage", 10.1);
        for (int tick = 0; tick <= last; tick++) {
            object battery = Activator.CreateInstance(readingType, true);
            double watts = BatteryWatts(scenario, tick, last);
            readingType.GetField("SupplyState").SetValue(battery, Enum.Parse(stateType, state));
            readingType.GetField("PowerWatts").SetValue(battery, watts);
            // The discharge trend has an N/A spell where the firmware rate is missing.
            readingType.GetField("RateAvailable").SetValue(battery, !(scenario == "discharging" && last >= 1800 && tick >= 300 && tick < 400));
            readingType.GetField("VoltageAvailable").SetValue(battery, true);
            readingType.GetField("CurrentAvailable").SetValue(battery, true);
            readingType.GetField("VoltageVolts").SetValue(battery, 17.51);
            readingType.GetField("CurrentAmps").SetValue(battery, watts / 17.51);
            readingType.GetField("Percentage").SetValue(battery, scenario == "discharging" ? 76 : 98);
            object platform = scenario == "unavailable" || (scenario == "charging" && last >= 1800 && tick >= 520 && tick < 640)
                ? Missing(assembly, "Platform", "需要以管理员身份运行才能读取平台功率")
                : Sample(assembly, "Platform", PlatformWatts(scenario, tick, last));
            object snapshot = compose.Invoke(null, new object[] { end.AddSeconds(tick - last), (double)tick, battery, cpu, platform });
            if (tick < last) observe.Invoke(session, new object[] { snapshot });
            else form.GetType().GetMethod("ShowReading", All).Invoke(form, new object[] { snapshot });
        }
    }
}
public static class FooterTextProbe {
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern int GetWindowLong(IntPtr window, int index);
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern IntPtr SendMessage(IntPtr window, int message, IntPtr wParam, IntPtr lParam);
    public static int HitTest(Form form, int clientX, int clientY) {
        Point screen = form.PointToScreen(new Point(clientX, clientY));
        return SendMessage(form.Handle, 0x84, IntPtr.Zero, new IntPtr((screen.Y << 16) | (screen.X & 0xffff))).ToInt32();
    }
    public static void Maximize(Form form) { SendMessage(form.Handle, 0x112, new IntPtr(0xF030), IntPtr.Zero); }
    public static void AssertReadoutBaseline(Control control) {
        string text = control.Text;
        Color color = control.ForeColor, back = control.BackColor;
        try {
            control.Text = "88.88 W";
            control.ForeColor = Color.Black;
            control.BackColor = Color.White;
            using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
                control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
                int[] bottoms = new int[bitmap.Width];
                for (int x = 0; x < bitmap.Width; x++) {
                    bottoms[x] = -1;
                    for (int y = 0; y < bitmap.Height; y++) {
                        Color pixel = bitmap.GetPixel(x, y);
                        // The unit keeps its quieter grey, so any clearly non-white pixel is ink.
                        if (pixel.R + pixel.G + pixel.B < 600) bottoms[x] = y;
                    }
                }
                int right = bitmap.Width - 1;
                while (right >= 0 && bottoms[right] < 0) right--;
                int unitLeft = right;
                while (unitLeft >= 0 && bottoms[unitLeft] >= 0) unitLeft--;
                int unitBottom = -1, numberBottom = -1;
                for (int x = unitLeft + 1; x <= right; x++) unitBottom = Math.Max(unitBottom, bottoms[x]);
                for (int x = 0; x < unitLeft; x++) numberBottom = Math.Max(numberBottom, bottoms[x]);
                int tolerance = Math.Max(2, (int)Math.Ceiling(1.5 * control.Font.SizeInPoints / 44));
                if (numberBottom < 0 || unitBottom < 0 || Math.Abs(numberBottom - unitBottom) > tolerance)
                    throw new InvalidOperationException("Readout/unit baseline mismatch: " + numberBottom + " / " + unitBottom);
            }
        } finally { control.Text = text; control.ForeColor = color; control.BackColor = back; }
    }
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
    public static void AssertHover(Control control) {
        var flags = System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.NonPublic;
        var leave = control.GetType().GetMethod("OnMouseLeave", flags);
        var move = control.GetType().GetMethod("OnMouseMove", flags);
        var selected = control.GetType().GetProperty("SelectedIndex", flags);
        object selection = selected.GetValue(control, null);
        leave.Invoke(control, new object[] { EventArgs.Empty });
        using (Bitmap before = new Bitmap(control.Width, control.Height))
        using (Bitmap hover = new Bitmap(control.Width, control.Height))
        using (Bitmap after = new Bitmap(control.Width, control.Height)) {
            Rectangle bounds = new Rectangle(Point.Empty, before.Size);
            control.DrawToBitmap(before, bounds);
            move.Invoke(control, new object[] { new MouseEventArgs(MouseButtons.None, 0, control.Width * 3 / 4, control.Height / 2, 0) });
            control.DrawToBitmap(hover, bounds);
            leave.Invoke(control, new object[] { EventArgs.Empty });
            control.DrawToBitmap(after, bounds);
            int changes = 0;
            for (int y = 0; y < control.Height; y++)
                for (int x = 0; x < control.Width; x++) {
                    if (before.GetPixel(x, y) != hover.GetPixel(x, y)) changes++;
                    if (before.GetPixel(x, y) != after.GetPixel(x, y))
                        throw new InvalidOperationException("Segment hover did not clear on mouse leave.");
                }
            if (changes == 0 || !Object.Equals(selection, selected.GetValue(control, null)))
                throw new InvalidOperationException("Segment hover must render feedback without changing the selection.");
        }
    }
    public static void AssertIcon(Control control) {
        int ink = 0;
        using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
            control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
            for (int y = control.Height / 4; y < control.Height * 3 / 4; y++)
                for (int x = control.Width / 4; x < control.Width * 3 / 4; x++) {
                    Color pixel = bitmap.GetPixel(x, y);
                    if (pixel.R + pixel.G + pixel.B > 240) ink++;
                }
        }
        if (ink < 4) throw new InvalidOperationException(control.AccessibleName + " icon did not render.");
    }
    public static void Click(Control control) { ((IButtonControl)control).PerformClick(); }
    public static void Capture(Control control, string path) {
        using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
            control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
            bitmap.Save(path, System.Drawing.Imaging.ImageFormat.Png);
        }
    }
    public static void AssertLayout(Control parent) {
        foreach (Control control in parent.Controls) {
            if (!control.Visible) continue;
            Type type = control.GetType();
            while (type != null && type.Name != "AlignedLabel") type = type.BaseType;
            if (type != null) {
                // Every AlignedLabel-derived control (plain labels, the status
                // label and the readouts) reports its rendered content size,
                // including glyphs, captions and smaller affixes.
                Size required = (Size)type.GetMethod("MeasureContent",
                    System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.NonPublic).Invoke(control, null);
                if (required.Width > control.Width || required.Height > control.Height)
                    throw new InvalidOperationException("Clipped label: " + control.Text + "; required=" + required + "; actual=" + control.Size);
            }
            foreach (Control other in parent.Controls)
                if (other.Visible && other != control && control.Bounds.IntersectsWith(other.Bounds))
                    throw new InvalidOperationException("Overlapping controls: " + control.Text + " " + control.Bounds + " / " + other.Text + " " + other.Bounds);
            if (control.HasChildren) AssertLayout(control);
        }
    }
    public static int InkTop(Control control) {
        string text = control.Text;
        Color back = control.BackColor, fore = control.ForeColor;
        LinkLabel link = control as LinkLabel;
        Color linkColor = link == null ? Color.Empty : link.LinkColor;
        LinkBehavior behavior = link == null ? LinkBehavior.SystemDefault : link.LinkBehavior;
        try {
            control.Text = "Ag09";
            control.BackColor = Color.White;
            control.ForeColor = Color.Black;
            if (link != null) {
                link.LinkColor = Color.Black;
                link.LinkBehavior = LinkBehavior.NeverUnderline;
            }
            using (Bitmap bitmap = new Bitmap(control.Width, control.Height)) {
                control.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
                for (int y = 0; y < bitmap.Height; y++)
                    for (int x = 0; x < bitmap.Width; x++) {
                        Color pixel = bitmap.GetPixel(x, y);
                        if (pixel.R < 96 && pixel.G < 96 && pixel.B < 96) return control.Top + y;
                    }
            }
            throw new InvalidOperationException("Footer text did not render.");
        } finally {
            control.Text = text;
            control.BackColor = back;
            control.ForeColor = fore;
            if (link != null) {
                link.LinkColor = linkColor;
                link.LinkBehavior = behavior;
            }
        }
    }
}
'@
$originalDpiContext = [FooterTextProbe]::SetThreadDpiAwarenessContext([IntPtr](-4))
if ($originalDpiContext -eq [IntPtr]::Zero) { throw 'Unable to match the application Per-Monitor V2 DPI context.' }
[Windows.Forms.Application]::EnableVisualStyles()
[Windows.Forms.Application]::SetUnhandledExceptionMode([Windows.Forms.UnhandledExceptionMode]::ThrowException)
$Executable = (Resolve-Path -LiteralPath $Executable).Path
$assembly = [Reflection.Assembly]::LoadFile($Executable)
$assembly.GetType('BatteryChargeMeter.EmbeddedLibraries').GetMethod('Initialize', [Reflection.BindingFlags]'Static,NonPublic').Invoke($null, @())
$flags = [Reflection.BindingFlags]'Instance,Public,NonPublic'
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null

function New-Internal([string]$Name) {
    [Activator]::CreateInstance($assembly.GetType("BatteryChargeMeter.$Name"), $true)
}
function Get-Field($Target, [string]$Name) {
    $Target.GetType().GetField($Name, $flags).GetValue($Target)
}
function Set-Field($Target, [string]$Name, $Value) {
    $Target.GetType().GetField($Name, $flags).SetValue($Target, $Value)
}
function Get-Property($Target, [string]$Name) {
    $Target.GetType().GetProperty($Name, $flags).GetValue($Target, $null)
}
function Invoke-Internal($Target, [string]$Name, [object[]]$Arguments = @()) {
    $plain = New-Object object[] $Arguments.Count
    for ($index = 0; $index -lt $Arguments.Count; $index++) {
        $plain[$index] = $Arguments[$index].PSObject.BaseObject
    }
    $Target.GetType().GetMethod($Name, $flags).Invoke($Target, $plain)
}
function Assert-Layout($Parent) {
    [FooterTextProbe]::AssertLayout($Parent)
}

$form = [Activator]::CreateInstance($assembly.GetType('BatteryChargeMeter.MainForm'), @($false))
$scenarioTicks = @{ idle = 600; charging = 1800; discharging = 1800; supplemented = 60; unavailable = 60; starting = 0; warming = 120; early = 240 }
function Set-Scenario([string]$Scenario) {
    $mode = if ($Scenario -eq 'discharging') { 'Battery' } else { 'WholeSystem' }
    # A fresh session per scenario starts empty statistics and trend without
    # persisting the display mode.
    Set-Field $form 'session' ([Activator]::CreateInstance($sessionType, $flags, $null, [object[]]@([Enum]::Parse($modeType, $mode)), $null))
    $segments = Get-Field $form 'modeSegments'
    $segments.GetType().GetProperty('SelectedIndex', $flags).SetValue($segments, [int]($mode -eq 'Battery'), $null)
    Invoke-Internal $form 'LayoutSourceRows'
    [UiFixture]::Feed($form, $assembly, $Scenario, $scenarioTicks[$Scenario])
    [Windows.Forms.Application]::DoEvents()
    return $mode
}
try {
    $form.Icon = [Drawing.Icon]::ExtractAssociatedIcon($Executable)
    $form.Show()
    [Windows.Forms.Application]::DoEvents()
    (Get-Field $form 'timer').Stop()
    Set-Field $form 'elevationMessage' $null
    Set-Field $form 'autostartMessage' $null
    Set-Field $form 'autostartStateMessage' $null
    $modeType = $assembly.GetType('BatteryChargeMeter.DisplayMode')
    $sessionType = $assembly.GetType('BatteryChargeMeter.MeterSession')
    $stringsType = $assembly.GetType('BatteryChargeMeter.Strings')
    $style = [FooterTextProbe]::GetWindowLong($form.Handle, -16)
    Should -ActualValue (($style -band 0x00C00000) -eq 0x00C00000 -and ($style -band 0x00080000) -ne 0 -and ($style -band 0x00020000) -ne 0) -BeTrue -Because `
        'The drawn title bar must keep the native caption, system menu and minimize styles (move loop, Alt+Space, taskbar)'
    Should -ActualValue (($style -band 0x00010000) -eq 0 -and -not $form.MaximizeBox) -BeTrue -Because 'The window must not be maximizable'
    foreach ($language in @('zh-CN', 'en', 'zh-CN')) {
    $stringsType.GetMethod('Select', [Reflection.BindingFlags]'Static,NonPublic').Invoke($null, @($language, $false))
    Invoke-Internal $form 'ApplyLanguage'
    Should -ActualValue ($form.Text -eq $(if ($language -eq 'en') { 'Power Meter' } else { '功率计' })) -BeTrue -Because 'Incorrect localized application name'
    foreach ($dpi in @(96, 144, 168, 192, 288, 96)) {
        Invoke-Internal $form 'SendDpiTransition' @([int]$dpi)
        $scale = $dpi / 96.0
        foreach ($scenario in @('idle', 'charging', 'discharging', 'supplemented', 'unavailable', 'starting', 'warming', 'early')) {
            $mode = Set-Scenario $scenario
            $diagnostics = Get-Field $form 'errorLabel'
            $expectedHeight = if ($scenario -eq 'unavailable') { 660 } else { 568 }
            Should -ActualValue ($form.AutoScrollMinSize.Height -eq [Math]::Round($expectedHeight * $scale)) -BeTrue -Because `
                'Diagnostic expansion/collapse did not resize the content'
            Should -ActualValue ($diagnostics.Visible -eq ($scenario -eq 'unavailable')) -BeTrue -Because 'Unexpected diagnostic visibility'
            Should -ActualValue ($diagnostics -isnot [Windows.Forms.TextBoxBase]) -BeTrue -Because 'The diagnostic notice must wrap without a native scrollbar'
            $elevate = Get-Field $form 'elevateButton'
            if ($diagnostics.Visible) {
                Should -ActualValue ((Get-Field $form 'sourceTip').GetToolTip($diagnostics) -eq $diagnostics.Text) -BeTrue -Because 'The complete diagnostic text must stay reachable'
                $lines = $diagnostics.GetType().GetProperty('VisibleLineCount', $flags).GetValue($diagnostics, $null)
                $needed = [Math]::Min(2, $diagnostics.GetType().GetProperty('LinesNeeded', $flags).GetValue($diagnostics, $null))
                Should -ActualValue ($lines -ge 1 -and $lines -ge $needed) -BeTrue -Because "The notice must show $needed wrapped line(s) at $dpi DPI (got $lines)"
                $elevated = $assembly.GetType('BatteryChargeMeter.Startup').GetMethod('IsElevated', [Reflection.BindingFlags]'Static,NonPublic,Public').Invoke($null, @())
                Should -ActualValue ($elevate.Visible -eq (-not $elevated)) -BeTrue -Because 'The elevation-gated platform reading must offer the restart action'
            } else {
                Should -ActualValue (-not $elevate.Visible) -BeTrue -Because 'The restart action belongs to the notice'
            }
            if ($scenario -ne 'starting') {
                $chip = Get-Field $form 'stateLabel'
                Should -ActualValue ($null -ne $chip.GetType().GetProperty('IconBody', $flags).GetValue($chip, $null)) -BeTrue -Because 'The supply state should carry its glyph'
            }
            Assert-Layout $form
            $sourceValues = Get-Field $form 'sourceValues'
            Should -ActualValue (@($sourceValues | Where-Object Visible).Count -eq 3) -BeTrue -Because 'Only the three supporting metrics should repeat below the headline'
            $headlineIndex = if ($mode -eq 'Battery') { 0 } else { 3 }
            Should -ActualValue (-not $sourceValues[$headlineIndex].Visible) -BeTrue -Because 'Headline metric was duplicated in the detail group'
            $chart = Get-Field $form 'trendChart'
            $trend = Get-Property $chart 'View'
            $series = $trend.Series
            if ($scenario -in @('charging', 'discharging')) {
                $buckets = Invoke-Internal $series 'Buckets' @([int](Get-Property $chart 'Columns'))
                Should -ActualValue ((Get-Property $series 'Full') -and $null -eq $trend.Recorded -and $null -ne $trend.PeakLabel) -BeTrue -Because 'A full 30-minute trend has a peak and no fill notice'
                Should -ActualValue (@($buckets | Where-Object { -not $_.HasData }).Count -gt 0) -BeTrue -Because 'The N/A spell must remain a gap in the trend'
            }
            if ($scenario -eq 'early') {
                Should -ActualValue (-not (Get-Property $series 'Full') -and $trend.Recorded -eq $stringsType.GetMethod('Format', [Reflection.BindingFlags]'Static,NonPublic').Invoke($null, @('已记录 {0} 分钟', [object[]]@('4')))) -BeTrue -Because 'An unfilled trend states its recorded duration'
            }
            if ($scenario -eq 'unavailable') {
                Should -ActualValue (-not (Get-Property $series 'HasData') -and $trend.Empty.Length -gt 0) -BeTrue -Because 'An unavailable boundary shows the empty trend notice'
            }
            if ($scenario -eq 'idle') {
                [FooterTextProbe]::AssertReadoutBaseline((Get-Field $form 'powerLabel'))
                $expectedAverage = if ($language -eq 'en') { '10-min average' } else { '10 分钟均值' }
                $expectedPeak = if ($language -eq 'en') { '5-min peak' } else { '5 分钟峰值' }
                Should -ActualValue ((Get-Field $form 'statisticsCaption').Text -eq $expectedAverage) -BeTrue -Because 'Full average window should have a concise caption'
                Should -ActualValue ((Get-Field $form 'historyCaption').Text -eq $expectedPeak) -BeTrue -Because 'Full peak window should have a concise caption'
                [FooterTextProbe]::AssertHover((Get-Field $form 'modeSegments'))
                $inkTops = @(@((Get-Field $form 'statisticsLabel'), (Get-Field $form 'historyLabel'), (Get-Field $form 'percentageLabel')) | ForEach-Object { [FooterTextProbe]::InkTop($_) })
                $range = $inkTops | Measure-Object -Minimum -Maximum
                Should -ActualValue ($inkTops.Count -eq 3 -and $range.Maximum - $range.Minimum -le 1) -BeTrue -Because `
                    "Statistic baselines differ at $dpi DPI ($language): $($inkTops -join ', ')"
                # Drawn title bar: caption area drags, the body does not, and
                # the client area covers the whole window when no scrollbar is shown.
                $titleHeight = [Math]::Round(44 * $scale)
                Should -ActualValue ([FooterTextProbe]::HitTest($form, [int](220 * $scale), [int]($titleHeight / 2)) -eq 2) -BeTrue -Because 'The title bar must act as the caption'
                Should -ActualValue ([FooterTextProbe]::HitTest($form, [int](220 * $scale), [int](300 * $scale)) -eq 1) -BeTrue -Because 'The body must not drag the window'
                if (-not $form.VerticalScroll.Visible -and -not $form.HorizontalScroll.Visible) {
                    Should -ActualValue ($form.Size -eq $form.ClientSize) -BeTrue -Because 'The window must have no native non-client frame'
                }
                foreach ($name in @('pinButton', 'minimizeButton', 'closeButton')) {
                    $button = Get-Field $form $name
                    Should -ActualValue ($button.Height -eq $titleHeight -and $button.Top -le 0) -BeTrue -Because "$name must fill the title bar height"
                    [FooterTextProbe]::AssertIcon($button)
                }
                $expectedClose = if ($language -eq 'en') { 'Close' } else { '关闭' }
                Should -ActualValue ((Get-Field $form 'closeButton').AccessibleName -eq $expectedClose) -BeTrue -Because 'Caption buttons must be localized'
                [FooterTextProbe]::Click((Get-Field $form 'minimizeButton'))
                [Windows.Forms.Application]::DoEvents()
                Should -ActualValue ($form.WindowState -eq 'Minimized') -BeTrue -Because 'Minimize did not minimize the window'
                $form.WindowState = 'Normal'
                [Windows.Forms.Application]::DoEvents()
                [FooterTextProbe]::Maximize($form)
                [Windows.Forms.Application]::DoEvents()
                Should -ActualValue ($form.WindowState -eq 'Normal') -BeTrue -Because 'A maximize command must be ignored'
                Invoke-Internal $form 'SendDpiTransition' @([int]$dpi)
            }
            if ($scenario -eq 'warming') {
                $partialAverage = if ($language -eq 'en') { '2m/10m' } else { '2分/10分' }
                $partialPeak = if ($language -eq 'en') { '2m/5m' } else { '2分/5分' }
                Should -ActualValue ((Get-Field $form 'statisticsCaption').Text.Contains($partialAverage)) -BeTrue -Because 'Partial average coverage must remain visible'
                Should -ActualValue ((Get-Field $form 'historyCaption').Text.Contains($partialPeak)) -BeTrue -Because 'Partial peak duration must remain visible'
                Should -ActualValue (-not (Get-Field $form 'statisticsLabel').Text.StartsWith('--')) -BeTrue -Because 'A partial window averages the readings so far'
            }
            if ($language -eq 'en') {
                Should -ActualValue ($diagnostics.Text -notmatch '[一-鿿]') -BeTrue -Because 'English diagnostics contain untranslated application text'
            }
            $path = Join-Path $OutputDirectory "$language-$scenario-$dpi.png"
            # Each simulated DPI change nudges the window; parts left off-screen
            # are not composed by DWM and would capture black.
            $form.Location = New-Object Drawing.Point(([Windows.Forms.Screen]::PrimaryScreen.WorkingArea.Left + 8), ([Windows.Forms.Screen]::PrimaryScreen.WorkingArea.Top + 8))
            [Windows.Forms.Application]::DoEvents()
            Invoke-Internal $form 'CapturePreview' @($path, "Fixture=$scenario; Dpi=$dpi")
            if ($scenario -eq 'idle') {
                $pin = Get-Field $form 'pinButton'
                [FooterTextProbe]::Click($pin)
                Should -ActualValue (-not $form.TopMost) -BeTrue -Because 'Pin action did not clear TopMost'
                [FooterTextProbe]::AssertIcon($pin)
                [FooterTextProbe]::Click($pin)
                Should -ActualValue $form.TopMost -BeTrue -Because 'Pin action did not restore TopMost'
                [FooterTextProbe]::AssertIcon($pin)
                [FooterTextProbe]::Click((Get-Field $form 'settingsButton'))
                [Windows.Forms.Application]::DoEvents()
                $settings = Get-Field $form 'settingsContent'
                Should -ActualValue ($null -ne $settings -and -not $settings.IsDisposed) -BeTrue -Because 'Settings did not open'
                Should -ActualValue ($settings.Width -eq [Math]::Round(296 * $scale)) -BeTrue -Because 'Settings content was scaled twice'
                Assert-Layout $settings
                [FooterTextProbe]::Capture($settings, (Join-Path $OutputDirectory "$language-settings-$dpi.png"))
                Invoke-Internal $form 'SendDpiTransition' @([int]$dpi)
                Should -ActualValue ($null -eq (Get-Field $form 'settingsContent')) -BeTrue -Because 'DPI transition did not close settings before rescaling fonts'
            }
        }
        Write-Host "PASS $language at $dpi DPI: eight reading scenarios, trend, title bar, text fit, hover, geometry, diagnostics and captures"
    }
    }
    if ($ShowcaseDirectory) {
        New-Item -ItemType Directory -Force -Path $ShowcaseDirectory | Out-Null
        $shots = @(
            @('zh-CN', 144, 'charging', 'zh-CN-charging-144'), @('zh-CN', 144, 'discharging', 'zh-CN-discharging-144'),
            @('zh-CN', 144, 'unavailable', 'zh-CN-unavailable-144'), @('zh-CN', 144, 'early', 'zh-CN-charging-4min-144'),
            @('en', 144, 'charging', 'en-US-charging-144'), @('zh-CN', 96, 'charging', 'zh-CN-charging-96'),
            @('zh-CN', 192, 'charging', 'zh-CN-charging-192'), @('zh-CN', 144, 'warming', 'zh-CN-warming-144'), @('zh-CN', 144, 'settings', 'zh-CN-settings-144'))
        foreach ($shot in $shots) {
            $stringsType.GetMethod('Select', [Reflection.BindingFlags]'Static,NonPublic').Invoke($null, @($shot[0], $false))
            Invoke-Internal $form 'ApplyLanguage'
            Invoke-Internal $form 'SendDpiTransition' @([int]$shot[1])
            $scenario = if ($shot[2] -eq 'settings') { 'charging' } else { $shot[2] }
            $null = Set-Scenario $scenario
            $form.Location = New-Object Drawing.Point(([Windows.Forms.Screen]::PrimaryScreen.WorkingArea.Left + 8), ([Windows.Forms.Screen]::PrimaryScreen.WorkingArea.Top + 8))
            [Windows.Forms.Application]::DoEvents()
            $path = Join-Path $ShowcaseDirectory "$($shot[3]).png"
            if ($shot[2] -eq 'settings') {
                [FooterTextProbe]::Click((Get-Field $form 'settingsButton'))
                for ($i = 0; $i -lt 10; $i++) { [Windows.Forms.Application]::DoEvents(); Start-Sleep -Milliseconds 20 }
                Invoke-Internal $form 'CaptureWithSettings' @($path)
                Invoke-Internal $form 'CloseSettings'
            } else {
                Invoke-Internal $form 'CapturePreview' @($path, "Showcase=$($shot[3])")
            }
        }
        Write-Host "Showcase captures written to $ShowcaseDirectory"
    }
}
finally {
    $form.Close()
    $form.Dispose()
    [FooterTextProbe]::SetThreadDpiAwarenessContext($originalDpiContext) | Out-Null
}
