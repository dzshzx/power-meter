[CmdletBinding()]
param(
    [string]$Executable = (Join-Path $PSScriptRoot '..\dist\PowerMeter.exe'),
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '..\dist\ui-preview'),
    # Optional: also render the review set (states, languages, scales and the
    # settings popover over the window) into this directory.
    [string]$ShowcaseDirectory = ''
)

$ErrorActionPreference = 'Stop'
Import-Module Pester -MinimumVersion 6.2.0 -MaximumVersion 6.999 -ErrorAction Stop
if ($PSVersionTable.PSEdition -ne 'Desktop') {
    & "$env:WINDIR\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -File $PSCommandPath `
        -Executable $Executable -OutputDirectory $OutputDirectory -ShowcaseDirectory $ShowcaseDirectory
    if ($LASTEXITCODE -ne 0) { throw "UI verification failed: $LASTEXITCODE" }
    return
}
Add-Type -AssemblyName System.Windows.Forms, System.Drawing
# Fixture readings and pixel probes: test-ui-fixture.cs beside this script.
Add-Type -ReferencedAssemblies System.Windows.Forms, System.Drawing -Path (Join-Path $PSScriptRoot 'test-ui-fixture.cs')
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
                # A screen smaller than the scaled layout clamps the window and
                # scrolls its content; probe the body inside the visible client area.
                $bodyY = [int][Math]::Min(300 * $scale, ($titleHeight + $form.ClientSize.Height) / 2)
                Should -ActualValue ([FooterTextProbe]::HitTest($form, [int](220 * $scale), $bodyY) -eq 1) -BeTrue -Because 'The body must not drag the window'
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
    # A reading bound while the window sits in the tray must be complete in the
    # first frame after restoring, dotted leaders included.
    $null = Set-Scenario 'idle'
    Invoke-Internal $form 'MinimizeToTray'
    [Windows.Forms.Application]::DoEvents()
    $rebind = { Invoke-Internal $form 'ShowView' @((Invoke-Internal (Get-Field $form 'session') 'Render')) }
    & $rebind
    Invoke-Internal $form 'RestoreFromTray'
    [Windows.Forms.Application]::DoEvents()
    # Independent of any capture: every supporting row already has its leader.
    $headlineRow = Get-Property $form 'HeadlineRow'
    $leaders = Get-Field $form 'leaders'
    for ($index = 0; $index -lt $leaders.Length; $index++) {
        if ($index -eq $headlineRow) { continue }
        $leader = $leaders[$index]
        $from = Get-Field $leader 'From'
        $to = Get-Field $leader 'To'
        Should -ActualValue ($to -gt $from) -BeTrue -Because "Row $index has no dotted leader after restoring from the tray ($from..$to)"
    }
    # The product's own capture (PrintWindow with its DrawToBitmap fallback).
    $restored = Invoke-Internal $form 'CaptureWindow' @($null)
    & $rebind
    $visible = Invoke-Internal $form 'CaptureWindow' @($null)
    $differing = [FooterTextProbe]::DifferingPixels($restored, $visible)
    $restored.Dispose(); $visible.Dispose()
    Should -ActualValue ($differing -eq 0) -BeTrue -Because "The first frame after restoring from the tray differs from a visible binding in $differing pixels"
    Write-Host 'PASS restore from tray: first frame matches a visible binding'
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
