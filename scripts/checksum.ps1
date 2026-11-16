# SHA-256 checksum files for Release assets. Dot-source this file:
# scripts/package-release.ps1 writes <asset>.sha256 with Write-Checksum;
# scripts/test.ps1 and scripts/verify-release.ps1 check it with Test-Checksum.
# The format is one sha256sum line: "<lowercase hex>  <asset file name>".

# Writes <asset>.sha256 beside the asset and returns its path.
function Write-Checksum([string]$AssetPath) {
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $AssetPath).Hash.ToLowerInvariant()
    $checksum = "$AssetPath.sha256"
    "$hash  $(Split-Path -Leaf $AssetPath)" | Set-Content -LiteralPath $checksum -Encoding ascii
    $checksum
}

# Checks that <asset>.sha256 names this asset and matches its content, and
# returns the lowercase hash. Throws on a missing, malformed or wrong entry.
function Test-Checksum([string]$AssetPath) {
    $name = Split-Path -Leaf $AssetPath
    $checksum = "$AssetPath.sha256"
    if (-not (Test-Path -LiteralPath $checksum)) { throw "Missing checksum file for $name" }
    $entry = (Get-Content -LiteralPath $checksum -Raw).Trim()
    if ($entry -notmatch '^([0-9a-f]{64})  (.+)$' -or $Matches[2] -ne $name) {
        throw "Invalid checksum entry for ${name}: $entry"
    }
    $expected = $Matches[1]
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $AssetPath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Checksum mismatch for $name" }
    $actual
}
