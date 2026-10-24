# 游栖 · GameNest 桌面工程

项目介绍、环境要求、构建方式和主界面截图见 [根目录 README](../README.md)，操作与迁移说明见 [使用指南](../docs/USAGE.md)。

首次从 `data.example/games.json` 初始化本机 `data/games.json`，然后使用 `wails dev` 开发。发布构建运行 `./build-release.ps1`，输出为 `../游栖/游栖.exe`，首次使用空游戏库，重复构建保留已有数据。

本机 `data/`、构建产物及验收临时文件不纳入版本管理。直接运行 Go 测试前先执行 `npm --prefix frontend ci` 和 `npm --prefix frontend run build`，生成 `go:embed` 所需的前端产物。
