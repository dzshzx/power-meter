[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
    [Parameter(Mandatory)][string]$PortablePath,
    [Parameter(Mandatory)][string]$InstallerPath
)

# Checks that release artifacts carry version X.Y.Z: the manifest embedded in
# the portable EXE reads X.Y.Z.0 and keeps asInvoker, and the installer's file
# version reads X.Y.Z.0. The Release workflow runs it on the built packages and
# verify-release.ps1 on the downloaded ones.

$ErrorActionPreference = 'Stop'

$fileVersion = "$Version.0"
$portable = (Resolve-Path -LiteralPath $PortablePath).Path
$installer = (Resolve-Path -LiteralPath $InstallerPath).Path

if (-not ('ReleaseManifest' -as [type])) {
    Add-Type @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;
public static class ReleaseManifest {
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    static extern IntPtr LoadLibraryEx(string path, IntPtr file, uint flags);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    static extern IntPtr FindResource(IntPtr module, IntPtr name, IntPtr type);
    [DllImport("kernel32.dll", SetLastError=true)] static extern IntPtr LoadResource(IntPtr module, IntPtr resource);
    [DllImport("kernel32.dll", SetLastError=true)] static extern uint SizeofResource(IntPtr module, IntPtr resource);
    [DllImport("kernel32.dll")] static extern IntPtr LockResource(IntPtr resource);
    [DllImport("kernel32.dll")] static extern bool FreeLibrary(IntPtr module);
    public static string Read(string path) {
        IntPtr module = LoadLibraryEx(path, IntPtr.Zero, 2);
        if (module == IntPtr.Zero) throw new Win32Exception();
        try {
            IntPtr resource = FindResource(module, new IntPtr(1), new IntPtr(24));
            if (resource == IntPtr.Zero) throw new Win32Exception();
            uint size = SizeofResource(module, resource);
            IntPtr bytes = LockResource(LoadResource(module, resource));
            if (size == 0 || bytes == IntPtr.Zero) throw new Win32Exception();
            byte[] data = new byte[size];
            Marshal.Copy(bytes, data, 0, data.Length);
            return Encoding.UTF8.GetString(data).TrimEnd('\0');
        } finally { FreeLibrary(module); }
    }
}
'@
}

[xml]$manifest = [ReleaseManifest]::Read($portable)
$manifestVersion = $manifest.assembly.assemblyIdentity.version
if ($manifestVersion -ne $fileVersion) {
    throw "Portable manifest version is $manifestVersion; expected $fileVersion."
}
$executionLevel = $manifest.assembly.trustInfo.security.requestedPrivileges.requestedExecutionLevel.level
if ($executionLevel -ne 'asInvoker') {
    throw "Portable executable changed elevation policy: $executionLevel"
}
# Inno Setup pads the version string with trailing whitespace.
$installerVersion = (Get-Item -LiteralPath $installer).VersionInfo.FileVersion.Trim()
if ($installerVersion -ne $fileVersion) {
    throw "Installer file version is $installerVersion; expected $fileVersion."
}

[pscustomobject]@{
    Manifest = $manifestVersion
    InstallerVersion = $installerVersion
    Elevation = $executionLevel
}
