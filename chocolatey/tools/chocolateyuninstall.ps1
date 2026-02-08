$ErrorActionPreference = 'Stop'

$packageName = 'ugudu'
$toolsDir = "$(Split-Path -Parent $MyInvocation.MyCommand.Definition)"

# Remove from PATH
Uninstall-ChocolateyPath -PathToUninstall $toolsDir -PathType 'Machine'

# Remove binaries
Remove-Item -Path (Join-Path $toolsDir 'ugudu.exe') -Force -ErrorAction SilentlyContinue

Write-Host "Ugudu has been uninstalled."
