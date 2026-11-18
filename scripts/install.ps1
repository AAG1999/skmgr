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
        # Persist the install dir on the user PATH. Read and write the registry
        # value raw: [Environment]::SetEnvironmentVariable expands entries like
        # %USERPROFILE%\bin and saves them that way, and it can truncate a long
        # PATH. Writing the registry alone does not notify other processes, so
        # a dummy user variable is set and removed to broadcast the change.
        $regKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Environment", $true)
        if (-not $regKey) {
            $regKey = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey("Environment")
        }
        try {
            $rawUserPath = [string]$regKey.GetValue("Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
            $userEntries = @($rawUserPath -split ";" | Where-Object { $_ -ne "" })
            $persisted = $false
            foreach ($entry in $userEntries) {
                $expanded = [Environment]::ExpandEnvironmentVariables($entry).TrimEnd("\")
                if ($expanded -ieq $InstallFull.TrimEnd("\")) {
                    $persisted = $true
                    break
                }
            }
            if (-not $persisted) {
                $regKey.SetValue("Path", (($userEntries + $InstallFull) -join ";"), [Microsoft.Win32.RegistryValueKind]::ExpandString)
                [Environment]::SetEnvironmentVariable("SKMGR_INSTALL_PATH_CHANGED", "1", "User")
                [Environment]::SetEnvironmentVariable("SKMGR_INSTALL_PATH_CHANGED", $null, "User")
                Write-Host "Added $InstallFull to your user PATH."
            }
        }
        finally {
            $regKey.Close()
        }

        # The running window keeps the PATH it started with. Refresh it even
        # when the directory was already saved, so `skmgr` works here without
        # a restart. Other already-open terminals still need to be reopened.
        $visibleNow = $false
        foreach ($entry in ($env:Path -split ";")) {
            if ($entry.TrimEnd("\") -ieq $InstallFull.TrimEnd("\")) {
                $visibleNow = $true
                break
            }
        }
        if (-not $visibleNow) {
            $env:Path = "$InstallFull;$env:Path"
            Write-Host "This window can run skmgr now. Reopen any other terminal so it picks up the new PATH."
        }
    }

    Write-Host "skmgr installed successfully to $Dest"
    & $Dest --version
}
finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
