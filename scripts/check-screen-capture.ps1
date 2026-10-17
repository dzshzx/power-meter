# Rejects screen-coordinate capture APIs under scripts/ and src/. A screen-region
# capture can include unrelated windows on the interactive desktop; UI captures
# must use the window's own rendering instead.
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
# Split so this script does not match itself.
$forbidden = 'CopyFrom' + 'Screen'
$hits = @(
    foreach ($directory in @('scripts', 'src')) {
        Get-ChildItem -LiteralPath (Join-Path $repoRoot $directory) -Recurse -File |
            Where-Object { $_.FullName -notmatch '[\\/](bin|obj|__pycache__)[\\/]' } |
            Select-String -SimpleMatch -Pattern $forbidden
    }
)
if ($hits.Count -gt 0) {
    $locations = $hits | ForEach-Object {
        '{0}:{1}' -f [IO.Path]::GetRelativePath($repoRoot, $_.Path), $_.LineNumber
    }
    throw ("Screen-coordinate capture ($forbidden) is not allowed; capture the window's own " +
        "rendering with PrintWindow or Control.DrawToBitmap instead. Found at:`n  " +
        ($locations -join "`n  "))
}
