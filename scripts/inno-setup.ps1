# Inno Setup 7 compiler for the per-user installer. Dot-source this file:
# scripts/test.ps1 and scripts/package-release.ps1 locate the compiler with
# Find-InnoSetupCompiler; CI, Release and local hosts run Install-InnoSetup
# (needs an elevated token) to install the pinned release.

# x64 edition from the official GitHub release, verified against the SHA-256
# digest published with that release.
$innoSetupVersion = '7.1.0'
$innoSetupInstallerSha256 = '0362a383ed217d4c4239b5933866dd96d3eb2102737da92f80f6057a4b40df2f'

function Get-InnoSetupCompilerVersion([string]$CompilerPath) {
    # ISCC.exe's Windows version resource can be 0.0.0.0. Query its own
    # preprocessor instead, so a Chocolatey shim cannot supply the version.
    $probeDir = Join-Path ([IO.Path]::GetTempPath()) ('iscc-version-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $probeDir | Out-Null
    try {
        $probePath = Join-Path $probeDir 'compiler-version.iss'
        @'
#pragma message "ISCC_VERSION_PACKED=" + Str(Ver)
[Setup]
AppName=CompilerVersionProbe
AppVersion=1
DefaultDirName={autopf}\CompilerVersionProbe
'@ | Set-Content -LiteralPath $probePath -Encoding utf8
        $output = (& $CompilerPath '/O-' $probePath 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or $output -notmatch 'ISCC_VERSION_PACKED=(?<version>\d+)') {
            throw "Inno Setup compiler version probe failed ($CompilerPath): $output"
        }
        $packed = [long]$Matches.version
    }
    finally {
        Remove-Item -LiteralPath $probeDir -Recurse -Force
    }
    [version]('{0}.{1}.{2}.{3}' -f `
        (($packed -shr 24) -band 255),
        (($packed -shr 16) -band 255),
        (($packed -shr 8) -band 255),
        ($packed -band 255))
}

# Returns the path and version of the first Inno Setup 7 ISCC.exe, or $null.
# Compilers of another major version (such as a preinstalled Inno Setup 6 on
# PATH) are reported and skipped.
function Find-InnoSetupCompiler {
    $candidates = @(
        (Join-Path $env:ProgramFiles 'Inno Setup 7\ISCC.exe'),
        (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 7\ISCC.exe'),
        (Get-Command 'ISCC.exe' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -First 1)
    ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -Unique
    foreach ($candidate in $candidates) {
        $version = Get-InnoSetupCompilerVersion $candidate
        if ($version.Major -eq 7) {
            return [pscustomobject]@{ Path = $candidate; Version = $version.ToString() }
        }
        Write-Warning "Skipping Inno Setup $version at ${candidate}: version 7 is required."
    }
    $null
}

function Install-InnoSetup {
    $expectedVersion = "$innoSetupVersion.0"
    $installed = Find-InnoSetupCompiler
    if ($installed -and $installed.Version -eq $expectedVersion) {
        Write-Host "Inno Setup $($installed.Version) is already installed: $($installed.Path)"
        return $installed
    }

    $name = "innosetup-$innoSetupVersion-x64.exe"
    $uri = 'https://github.com/jrsoftware/issrc/releases/download/is-{0}/{1}' -f $innoSetupVersion.Replace('.', '_'), $name
    $downloadDir = Join-Path ([IO.Path]::GetTempPath()) ('inno-setup-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $downloadDir | Out-Null
    try {
        $installerPath = Join-Path $downloadDir $name
        Invoke-WebRequest -UseBasicParsing -Uri $uri -OutFile $installerPath
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $installerPath).Hash.ToLowerInvariant()
        if ($hash -ne $innoSetupInstallerSha256) {
            throw "$name has SHA-256 $hash; expected $innoSetupInstallerSha256."
        }
        $process = Start-Process -FilePath $installerPath -Wait -PassThru -ArgumentList @(
            '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-', '/ALLUSERS'
        )
        if ($process.ExitCode -ne 0) {
            throw "Inno Setup $innoSetupVersion setup exited with code $($process.ExitCode)."
        }
    }
    finally {
        Remove-Item -LiteralPath $downloadDir -Recurse -Force
    }

    $installed = Find-InnoSetupCompiler
    if (-not $installed -or $installed.Version -ne $expectedVersion) {
        throw "Inno Setup $innoSetupVersion is not available after installation."
    }
    Write-Host "Installed Inno Setup $($installed.Version): $($installed.Path)"
    $installed
}
