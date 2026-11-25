[CmdletBinding()]
param([Parameter(Mandatory)][string]$BundleRoot,[Parameter(Mandatory)][string]$PortableRoot)
$ErrorActionPreference='Stop'
$PortableRoot=(Resolve-Path -LiteralPath $PortableRoot).Path
$BundleRoot=(Resolve-Path -LiteralPath $BundleRoot).Path
if ((Split-Path -Leaf $PortableRoot) -ne 'OverFieldLauncher-portable') { throw 'Select the existing OverFieldLauncher-portable directory.' }
if (!(Test-Path -LiteralPath (Join-Path $PortableRoot 'profile_manager.py'))) { throw 'Existing launcher files not found.' }
$busy=@(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($PortableRoot+'\',[StringComparison]::OrdinalIgnoreCase) -and $_.Name -in @('launcher.exe','Lolo.exe','python.exe','pythonw.exe') })
if ($busy.Count) { throw 'Save and exit the local game and companion before installing.' }
$backup=Join-Path $PortableRoot ('repair-backups\'+(Get-Date -Format 'yyyyMMdd-HHmmss'))
$targetCompanion=Join-Path (Split-Path -Parent $PortableRoot) '单机修复与管理'
foreach($pair in @(@((Join-Path $BundleRoot 'OverFieldLauncher-portable'),$PortableRoot,'launcher'),@((Join-Path $BundleRoot '单机修复与管理'),$targetCompanion,'companion'))) {
    foreach($f in Get-ChildItem -LiteralPath $pair[0] -File) {
        $target=Join-Path $pair[1] $f.Name
        if (Test-Path -LiteralPath $target) {
            $save=Join-Path $backup ($pair[2]+'\'+$f.Name)
            [IO.Directory]::CreateDirectory((Split-Path -Parent $save)) | Out-Null
            Copy-Item -LiteralPath $target -Destination $save
        }
        [IO.Directory]::CreateDirectory($pair[1]) | Out-Null
        Copy-Item -LiteralPath $f.FullName -Destination $target -Force
    }
}
foreach($rel in @('server-kit\Lolo.exe','profiles\solo\server\Lolo.exe')) {
    $target=Join-Path $PortableRoot $rel
    if (Test-Path -LiteralPath $target) {
        $save=Join-Path $backup $rel
        [IO.Directory]::CreateDirectory((Split-Path -Parent $save)) | Out-Null
        Copy-Item -LiteralPath $target -Destination $save
        Copy-Item -LiteralPath (Join-Path $targetCompanion 'Lolo-local.exe') -Destination $target -Force
    }
}
Write-Host "Installed. Original files backed up: $backup"
