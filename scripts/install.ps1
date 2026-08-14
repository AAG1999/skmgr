# Installs the latest skmgr release for Windows.
# Usage:
#   irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex
#
# Optional overrides (mainly for CI / advanced installs):
#   SKMGR_ZIP_PATH     — local zip path; skips the GitHub download
#   SKMGR_INSTALL_DIR  — install directory (default: %LOCALAPPDATA%\skmgr)
#   SKMGR_SKIP_PATH    — if set, do not modify the user PATH

$ErrorActionPreference = "Stop"

# Windows PowerShell 5.1 defaults can omit TLS 1.2; GitHub requires it.
if ($PSVersionTable.PSVersion.Major -lt 6) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
}

$Repo = "AAG1999/skmgr"

Write-Host "Installing skmgr..."

# 32-bit PowerShell on 64-bit Windows reports PROCESSOR_ARCHITECTURE=x86;
# the real host arch is in PROCESSOR_ARCHITEW6432.
$ProcessArch = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) {
    $ProcessArch = $env:PROCESSOR_ARCHITEW6432
}

switch ($ProcessArch) {
    "AMD64" { $Arch = "x86_64" }
    "ARM64" { $Arch = "arm64" }
    default {
        throw "Unsupported architecture: $ProcessArch (install a 64-bit build from https://github.com/$Repo/releases)"
    }
}

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("skmgr-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
    if ($env:SKMGR_ZIP_PATH) {
        if (-not (Test-Path -LiteralPath $env:SKMGR_ZIP_PATH)) {
            throw "SKMGR_ZIP_PATH not found: $($env:SKMGR_ZIP_PATH)"
        }
        $ZipPath = $env:SKMGR_ZIP_PATH
        Write-Host "Using local zip $ZipPath..."
    }
    else {
        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{
            "Accept"     = "application/vnd.github+json"
            "User-Agent" = "skmgr-install"
        }
        $Tag = $Release.tag_name
        if (-not $Tag) {
            throw "Failed to fetch latest release."
        }

        $Filename = "skmgr_Windows_${Arch}.zip"
        $DownloadUrl = "https://github.com/$Repo/releases/download/$Tag/$Filename"
        $ZipPath = Join-Path $TempDir $Filename
        Write-Host "Downloading $DownloadUrl..."
        # -UseBasicParsing avoids a dependency on Internet Explorer on Windows PowerShell 5.1.
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing
    }

    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

    $Binary = Get-ChildItem -Path $TempDir -Filter "skmgr.exe" -Recurse -File | Select-Object -First 1
    if (-not $Binary) {
        throw "skmgr.exe not found in archive"
    }

    if ($env:SKMGR_INSTALL_DIR) {
        $InstallDir = $env:SKMGR_INSTALL_DIR
    }
    else {
        $InstallDir = Join-Path $env:LOCALAPPDATA "skmgr"
    }

    # Refuse to install into the temp extract dir — finally{} deletes it.
    $TempFull = [System.IO.Path]::GetFullPath($TempDir)
    $InstallFull = [System.IO.Path]::GetFullPath($InstallDir)
    if ($InstallFull.StartsWith($TempFull, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "SKMGR_INSTALL_DIR cannot be inside the temporary extract directory"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    $Dest = Join-Path $InstallDir "skmgr.exe"
    try {
        Copy-Item -Path $Binary.FullName -Destination $Dest -Force
    }
    catch {
        throw "Failed to write $Dest. If skmgr is running, close it and retry. $($_.Exception.Message)"
    }

    if (-not $env:SKMGR_SKIP_PATH) {
        $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
        $PathParts = @()
        if ($UserPath) {
            $PathParts = $UserPath -split ";" | Where-Object { $_ -ne "" }
        }
        $alreadyOnPath = $PathParts | Where-Object {
            $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\')
        }
        if (-not $alreadyOnPath) {
            $NewPath = ($PathParts + $InstallDir) -join ";"
            [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
            $env:Path = "$InstallDir;$env:Path"
            Write-Host "Added $InstallDir to your user PATH (new shells will pick this up)."
        }
    }

    Write-Host "skmgr installed successfully to $Dest"
    & $Dest --version
}
finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
