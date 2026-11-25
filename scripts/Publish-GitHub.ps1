[CmdletBinding()]
param([Parameter(Mandatory)][string]$RepositoryName,[ValidateSet('private','public')][string]$Visibility='private',[switch]$Reviewed)
$ErrorActionPreference='Stop'
if (!$Reviewed) { throw 'Review the source package first. Run this script only after explicitly deciding to publish, with -Reviewed.' }
$repo=Split-Path -Parent $PSScriptRoot
if (!(Get-Command gh -ErrorAction SilentlyContinue)) { throw 'Install the official GitHub CLI (gh) first.' }
function Check-Exit { if ($LASTEXITCODE -ne 0) { throw "Command failed ($LASTEXITCODE)" } }
& gh auth status
if ($LASTEXITCODE -ne 0) { & gh auth login --web --git-protocol https; Check-Exit }
& gh auth setup-git; Check-Exit
if (!(Test-Path -LiteralPath (Join-Path $repo '.git'))) { & git -C $repo init -b codex/steam-local-fix; Check-Exit }
& git -C $repo add --all; Check-Exit
# Prevent known private artifacts even if a future edit changes .gitignore.
$files=@(& git -C $repo ls-files)
$blocked=@($files | Where-Object { $_ -match '(?i)(^|/)(profiles|runtime|Resource|build|tools|存档备份|配置备份|Steam客户端配置)/|\.(exe|dll|db|pcap|pem|key|log|jsonl)$|admin-key|panel-token|launcher.settings' })
if ($blocked.Count) { throw ('Private artifacts detected: '+($blocked -join ', ')) }
& git -C $repo diff --cached --quiet
if ($LASTEXITCODE -eq 1) { & git -C $repo commit -m 'Add isolated Steam local startup repair and companion tools'; Check-Exit }
elseif ($LASTEXITCODE -ne 0) { Check-Exit }
& gh repo create $RepositoryName --source $repo --remote origin ("--"+$Visibility) --push
Check-Exit
