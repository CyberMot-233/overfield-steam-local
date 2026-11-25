# GitHub 发布流程

检查源码后，使用官方 GitHub CLI 完成认证、建仓和首次上传。默认创建私有仓库；构建与安装不会自动发布。

1. 检查 README、patches、launcher/new、server/new、companion 和文件清单。
2. 安装官方 GitHub CLI，确认 `gh --version`。
3. 在明确决定发布后运行下面的脚本，仓库名称自行填写。默认私有，可通过参数改为公开。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\Publish-GitHub.ps1 -RepositoryName "<账号>/<仓库名>" -Visibility private -Reviewed
```

脚本执行官方 `gh auth status`；需要时以 `gh auth login --web --git-protocol https` 完成交互认证，再执行 `gh auth setup-git`。本地 Git 提交使用已有作者配置，未配置时应先设置自己的名字与提交邮箱。创建与首次上传通过 `gh repo create --source --remote origin --push` 完成。未使用直接HTTP/API调用。

脚本在提交前检查被跟踪文件，拒绝已知游戏资源、存档、运行时密钥、工具缓存与日志。此检查不能替代人工审查；README中说明的第三方来源与许可证保持可见。
