# Ugudu Installer for Windows
# Usage: irm https://raw.githubusercontent.com/arcslash/ugudu/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$REPO = "arcslash/ugudu"
$BINARY_NAME = "ugudu"
$INSTALL_DIR = "$env:LOCALAPPDATA\Programs\Ugudu"

function Write-Banner {
    Write-Host @"

  _   _                 _
 | | | | __ _ _   _  __| |_   _
 | | | |/ _`` | | | |/ _`` | | | |
 | |_| | (_| | |_| | (_| | |_| |
  \___/ \__, |\__,_|\__,_|\__,_|
        |___/

"@ -ForegroundColor Blue
    Write-Host "AI Team Orchestration" -ForegroundColor Blue
    Write-Host ""
}

function Get-LatestVersion {
    Write-Host "Fetching latest version..." -ForegroundColor Cyan

    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO/releases/latest"
    $version = $release.tag_name -replace '^v', ''

    Write-Host "Latest version: v$version" -ForegroundColor Green
    return $version
}

function Install-Ugudu {
    param($Version)

    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    $downloadUrl = "https://github.com/$REPO/releases/download/v$Version/${BINARY_NAME}_${Version}_windows_${arch}.zip"

    Write-Host "Downloading from: $downloadUrl" -ForegroundColor Cyan

    # Create temp directory
    $tempDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString()))

    try {
        $zipPath = Join-Path $tempDir "ugudu.zip"

        # Download
        Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

        # Extract
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

        # Find binary
        $binaryPath = Get-ChildItem -Path $tempDir -Recurse -Filter "$BINARY_NAME.exe" | Select-Object -First 1

        if (-not $binaryPath) {
            throw "Binary not found in archive"
        }

        # Create install directory
        if (-not (Test-Path $INSTALL_DIR)) {
            New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
        }

        # Copy binary
        Copy-Item -Path $binaryPath.FullName -Destination (Join-Path $INSTALL_DIR "$BINARY_NAME.exe") -Force

        Write-Host "Installed to: $INSTALL_DIR\$BINARY_NAME.exe" -ForegroundColor Green
    }
    finally {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Add-ToPath {
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

    if ($currentPath -notlike "*$INSTALL_DIR*") {
        Write-Host "Adding $INSTALL_DIR to PATH..." -ForegroundColor Cyan

        $newPath = "$currentPath;$INSTALL_DIR"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")

        # Update current session
        $env:Path = "$env:Path;$INSTALL_DIR"

        Write-Host "Added to PATH. Restart your terminal for changes to take effect." -ForegroundColor Yellow
    }
}

function Show-Success {
    Write-Host ""
    Write-Host "Installation complete!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Quick start:" -ForegroundColor Cyan
    Write-Host "  1. Open a new terminal"
    Write-Host "  2. Start the daemon: ugudu daemon"
    Write-Host "  3. Open web UI: http://localhost:8080"
    Write-Host ""
    Write-Host "Or run: $INSTALL_DIR\$BINARY_NAME.exe --help" -ForegroundColor Gray
}

# Main
Write-Banner

$version = Get-LatestVersion
Install-Ugudu -Version $version
Add-ToPath
Show-Success
