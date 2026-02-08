$ErrorActionPreference = 'Stop'

$packageName = 'ugudu'
$version = '0.1.0'

$url64 = "https://github.com/arcslash/ugudu/releases/download/v$version/ugudu_${version}_windows_amd64.zip"
$checksum64 = 'REPLACE_WITH_ACTUAL_SHA256'

$toolsDir = "$(Split-Path -Parent $MyInvocation.MyCommand.Definition)"

$packageArgs = @{
  packageName   = $packageName
  unzipLocation = $toolsDir
  url64bit      = $url64
  checksum64    = $checksum64
  checksumType64= 'sha256'
}

Install-ChocolateyZipPackage @packageArgs

# Add to PATH
$binPath = Join-Path $toolsDir 'ugudu.exe'
Install-ChocolateyPath -PathToInstall $toolsDir -PathType 'Machine'

Write-Host "Ugudu has been installed!"
Write-Host ""
Write-Host "Quick Start:"
Write-Host "  1. Configure your API key: ugudu config init"
Write-Host "  2. Start the daemon: ugudu daemon"
Write-Host "  3. Create a team: ugudu spec new my-team && ugudu team create alpha --spec my-team"
Write-Host "  4. Talk to your team: ugudu ask alpha 'Hello team!'"
