[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
    [string]$Directory,
    [string]$Repository = 'dzshzx/power-meter'
)

# Downloads the public assets of release vX.Y.Z and checks them against this
# checkout: checksums, manifest version and elevation, installer version,
# self-test, embedded notices and the corresponding PawnIO source bundle.
# Run from the interactive Windows desktop; --screenshot needs a window station.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
if (-not $Directory) { $Directory = Join-Path ([IO.Path]::GetTempPath()) "PowerMeter-release-v$Version" }
New-Item -ItemType Directory -Force -Path $Directory | Out-Null
$Directory = (Resolve-Path -LiteralPath $Directory).Path
$fileVersion = "$Version.0"

$sourceBundles = @(Get-ChildItem -LiteralPath (Join-Path $repoRoot 'third_party') -Filter 'PawnIO.Modules-*-source.zip')
if ($sourceBundles.Count -ne 1) { throw 'Expected exactly one PawnIO source bundle in third_party.' }
$sourceBundle = $sourceBundles[0]

$portableName = "PowerMeter-v$Version-windows.exe"
$setupName = "PowerMeter-v$Version-windows-setup.exe"
$noticeName = 'PowerMeter-THIRD-PARTY-NOTICES.txt'
$assets = @($portableName, "$portableName.sha256", $setupName, "$setupName.sha256", $noticeName, $sourceBundle.Name)
foreach ($name in $assets) {
    $uri = "https://github.com/$Repository/releases/download/v$Version/$name"
    Invoke-WebRequest -Uri $uri -OutFile (Join-Path $Directory $name) -UseBasicParsing
}

$hashes = [ordered]@{}
foreach ($name in @($portableName, $setupName)) {
    $checksum = (Get-Content -LiteralPath (Join-Path $Directory "$name.sha256") -Raw).Trim()
    if ($checksum -notmatch '^([a-fA-F0-9]{64})  (.+)$' -or $Matches[2] -ne $name) { throw "Invalid checksum entry for $name" }
    $expected = $Matches[1].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath (Join-Path $Directory $name) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Checksum mismatch for $name" }
    $hashes[$name] = $actual
}

$portable = Join-Path $Directory $portableName
$versions = & (Join-Path $PSScriptRoot 'assert-release-version.ps1') `
    -Version $Version -PortablePath $portable -InstallerPath (Join-Path $Directory $setupName)
$installerVersion = $versions.InstallerVersion
$publishedSource = Join-Path $Directory $sourceBundle.Name
if ((Get-FileHash $publishedSource -Algorithm SHA256).Hash -ne (Get-FileHash $sourceBundle.FullName -Algorithm SHA256).Hash) { throw 'Corresponding source bundle mismatch.' }

function Invoke-ReleaseCli([string[]]$Arguments) {
    $process = Start-Process -FilePath $portable -ArgumentList $Arguments -PassThru
    if (-not $process.WaitForExit(15000)) { $process.Kill(); $process.WaitForExit(); throw 'Published CLI timed out.' }
    if ($process.ExitCode -ne 0) { throw "Published CLI exited with $($process.ExitCode)." }
}
$selfTest = Join-Path $Directory 'published-self-test.txt'
Invoke-ReleaseCli @('--self-test', ('"{0}"' -f $selfTest))
if ((Get-Content $selfTest -Raw) -notmatch 'Power self test passed\.') { throw 'Published self-test did not pass.' }
$exported = Join-Path $Directory 'published-notices.txt'
Invoke-ReleaseCli @('--third-party-notices', ('"{0}"' -f $exported))
$releasedNotice = Join-Path $Directory $noticeName
if ((Get-FileHash $exported).Hash -ne (Get-FileHash $releasedNotice).Hash) { throw 'Published notices differ from executable resources.' }
$text = Get-Content $releasedNotice -Raw
foreach ($term in @('GNU LESSER GENERAL PUBLIC LICENSE', 'Apache License', 'Microsoft Public License', 'Lucide', 'AntdUI')) {
    if (-not $text.Contains($term)) { throw "Missing license or attribution: $term" }
}
Invoke-ReleaseCli @('--screenshot', ('"{0}"' -f (Join-Path $Directory 'published-window.png')))

$result = [ordered]@{
    host = $env:COMPUTERNAME
    version = $Version
    manifest = $fileVersion
    installerVersion = $installerVersion
    sha256 = $hashes
    elevation = 'asInvoker'
    selfTest = 'passed'
    notices = 'matched'
    sourceBundle = 'matched'
    screenshot = 'published-window.png'
    verifiedAt = [DateTimeOffset]::UtcNow.ToString('o')
}
$result | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $Directory 'verification.json') -Encoding utf8
$result | ConvertTo-Json -Depth 5
