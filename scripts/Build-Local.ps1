[CmdletBinding()]
param([string]$WorkRoot = '', [string]$GoExe = 'go')
$ErrorActionPreference='Stop'
$repo=Split-Path -Parent $PSScriptRoot
if (!$WorkRoot) { $WorkRoot=Join-Path $repo 'build' }
$WorkRoot=[IO.Path]::GetFullPath($WorkRoot)
$spec=Get-Content -LiteralPath (Join-Path $repo 'upstream.json') -Raw | ConvertFrom-Json
function Run-Checked([string]$Tool,[string[]]$Arguments) {
    & $Tool @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Tool failed ($LASTEXITCODE)" }
}
function Copy-Overlay([string]$Source,[string]$Destination) {
    foreach($f in Get-ChildItem -LiteralPath $Source -File -Recurse) {
        $relative=$f.FullName.Substring($Source.Length).TrimStart('\')
        $target=Join-Path $Destination $relative
        [IO.Directory]::CreateDirectory((Split-Path -Parent $target)) | Out-Null
        Copy-Item -LiteralPath $f.FullName -Destination $target -Force
    }
}
if (!(Get-Command gh -ErrorAction SilentlyContinue)) { throw 'Install the official GitHub CLI (gh) first.' }
[IO.Directory]::CreateDirectory($WorkRoot) | Out-Null
$launcher=Join-Path $WorkRoot 'launcher-source'
$server=Join-Path $WorkRoot 'lolo-source'
foreach($pair in @(@($spec.launcher,$launcher,'launcher.patch','launcher'),@($spec.server,$server,'lolo.patch','server'))) {
    $source=$pair[0];$dir=$pair[1]
    if (Test-Path -LiteralPath $dir) { throw "Use a new WorkRoot; source directory already exists: $dir" }
    Run-Checked 'gh' @('repo','clone',$source.repository,$dir,'--','--no-checkout')
    Run-Checked 'git' @('-C',$dir,'checkout','--detach',$source.commit)
    $patch=Join-Path $repo ('patches\'+$pair[2])
    Run-Checked 'git' @('-C',$dir,'apply','--check',$patch)
    Run-Checked 'git' @('-C',$dir,'apply',$patch)
    Copy-Overlay (Join-Path $repo ($pair[3]+'\new')) $dir
}
$bundle=Join-Path $WorkRoot 'output'
$companion=Join-Path $bundle '单机修复与管理'
$portable=Join-Path $bundle 'OverFieldLauncher-portable'
[IO.Directory]::CreateDirectory($companion) | Out-Null
[IO.Directory]::CreateDirectory($portable) | Out-Null
Push-Location $server
try {
    Run-Checked $GoExe @('build','-trimpath','-o',(Join-Path $companion 'Lolo-local.exe'),'.')
    Run-Checked $GoExe @('build','-trimpath','-o',(Join-Path $companion 'localgm.exe'),'./cmd/localgm')
} finally { Pop-Location }
Copy-Overlay (Join-Path $repo 'companion') $companion
foreach($file in @($spec.launcher.modifiedFiles)+@('steam_private_agent.js','steam_private_runner.py','steam_runtime_agent.js','steam_live_tools.js','steam_local_compat.js')) {
    Copy-Item -LiteralPath (Join-Path $launcher $file) -Destination (Join-Path $portable $file) -Force
}
$csc=Join-Path $env:WINDIR 'Microsoft.NET\Framework64\v4.0.30319\csc.exe'
Run-Checked $csc @('/nologo','/target:winexe','/optimize+','/reference:System.Windows.Forms.dll',('/out:'+(Join-Path $companion '开放空间单机助手.exe')),(Join-Path $companion 'ManagerLauncher.cs'))
Write-Host "Build complete: $bundle"
Write-Host 'Install into an existing portable launcher with Install-Local.ps1. Game resources and runtime dependencies are not redistributed here.'
