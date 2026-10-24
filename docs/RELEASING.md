# 自动构建与发布

GitHub Actions 工作流为 [Windows Release](../.github/workflows/release-windows.yml)。

## 发布新版本

先将需要发布的改动提交并推送到 GitHub，再创建不重复的正式版本标签：

```powershell
git tag v0.1.1
git push origin v0.1.1
```

只接受 `vMAJOR.MINOR.PATCH`，例如 `v0.1.1`、`v1.0.0`。标签指向的提交必须包含该工作流。不要移动已经发布的标签；需要修正时发布新版本。

版本号直接从标签读取，构建时自动写入临时工作区的 Wails 配置，不需要先手动修改 `productVersion`。这个修改不会推送回仓库。

工作流在 GitHub 托管的全新 Windows runner 上运行，完成以下步骤后才发布：

1. 检查受版本管理的文件，发现个人游戏清单、配置、密钥文件或数据目录则失败；验证空游戏库模板并执行发布保护测试。
2. 安装 Node.js、Go 与固定版本 Wails；构建前端、运行 Go 测试、生成 Windows x64 应用及安装程序。
3. 从独立暂存目录的显式文件白名单生成便携 ZIP。ZIP 与安装包使用同一份程序、文档、许可及空游戏库。
4. 核对 ZIP 文件、实际安装包内嵌数据及 SHA-256；发布任务下载后再次验证，禁止夹带其他文件。
5. 创建草稿 Release，上传完整附件后再公开。已有正式 Release 不会被覆盖；同一提交遗留的工作流草稿可在重跑后继续完成。

## 发布附件

- `GameNest-v版本-windows-amd64.zip`：便携版。
- `GameNest-v版本-windows-amd64-setup.exe`：中文安装程序。
- `SHA256SUMS.txt`：上述附件和清单的 SHA-256。
- `release-manifest.json`：版本、源提交及实际打包文件摘要，不包含本机路径或密钥值。

发布内容不包含本机 `data/`、游戏图片缓存、收藏记录、个人偏好或 SteamGridDB 密钥。唯一的数据文件是内置空 `data/games.json`，内容也会严格验证。

## 手动验证构建

在 GitHub **Actions → Windows Release → Run workflow** 中选择分支运行。此入口只构建与校验，产物保存在该次运行的 Artifacts 中，保留七天；不会创建标签或发布 Release。

手动构建使用 `desktop/wails.json` 中的版本。标签发布使用标签中的版本。源码与打包文件验证失败、依赖下载失败或构建测试失败时都不会进入发布任务。

## 权限与限制

- 不需要配置个人 PAT、SteamGridDB Key 或其他仓库 Secret。
- 构建任务只有 `contents: read`，Git 凭据不保留在检出目录中。
- 只有发布任务拥有 `contents: write`，内置 `GITHUB_TOKEN` 仅提供给发布步骤。上传范围是四个明确指定的附件。
- Actions 依赖使用固定的提交 SHA。NSIS 下载有固定 SHA-256 校验，WebView2 引导程序有微软签名校验。
- 当前未设置代码签名；自动化不执行实际安装或卸载测试，也不使用开发者本机数据。

实现与配置依据：[GitHub 工作流语法](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)、[GITHUB_TOKEN 权限](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)、[GitHub CLI Release](https://cli.github.com/manual/gh_release)。
