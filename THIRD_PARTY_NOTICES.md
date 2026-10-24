# 第三方声明与致谢

游栖自有代码采用根目录 MIT 协议；第三方组件、字体及平台素材保留各自版权与许可。感谢以下项目及其贡献者。

## 开源组件

| 项目 | 用途 | 许可原文 |
| --- | --- | --- |
| [Wails](https://github.com/wailsapp/wails) | Go 桌面框架、WebView2 集成及生成的 JS 运行时 | [MIT](licenses/go/github.com_wailsapp_wails_v2/LICENSE) |
| [Vue](https://github.com/vuejs/core) | 响应式界面 | [MIT](licenses/npm/vue/LICENSE) |
| [Phosphor Icons](https://github.com/phosphor-icons/vue) | 界面图标 | [MIT](licenses/npm/@phosphor-icons_vue/LICENSE) |
| [Vite](https://github.com/vitejs/vite) | 前端开发与打包 | [许可](licenses/npm/vite/LICENSE.md) |
| [TypeScript](https://github.com/microsoft/TypeScript) | 类型检查 | [Apache-2.0](licenses/npm/typescript/LICENSE.txt) |
| [go-ole](https://github.com/go-ole/go-ole) | Windows COM 集成 | [MIT](licenses/go/github.com_go-ole_go-ole/LICENSE) |
| [Go](https://go.dev/) | 后端语言及标准库 | [BSD 风格许可](licenses/go/stdlib/LICENSE) |
| [Nunito](https://github.com/googlefonts/nunito) | 工程附带的模板字体 | [SIL OFL 1.1](desktop/frontend/src/assets/fonts/OFL.txt) |

锁定版本见 `desktop/go.mod`、`desktop/go.sum` 及 `desktop/frontend/package-lock.json`。Go 模块及前端直接依赖和 Vue 运行时组件的许可副本、版本清单见 [licenses/INDEX.md](licenses/INDEX.md)。Wails 生成的运行时和构建模板保留原作者声明；Nunito 的许可与字体一同保留。

## 资料、图片及检索来源

- [Steam / Valve](https://store.steampowered.com/)：商店名称、AppID、封面和 Library Hero 等图片。
- [VNDB](https://vndb.org/) 与 [API 文档](https://api.vndb.org/kana)：作品资料、别名、平台链接及图片查询。
- [DLsite](https://www.dlsite.com/)：作品名称、RJ 编号及商品图片。
- [SteamGridDB](https://www.steamgriddb.com/) 与 [API 文档](https://www.steamgriddb.com/api/v2)：可选社区封面、背景，需用户自己的 API Key。
- [DuckDuckGo](https://duckduckgo.com/)、[Yahoo! JAPAN](https://search.yahoo.co.jp/)、[Bing](https://www.bing.com/)：DLsite 官方商品链接的备用检索入口。

上述平台名称、商标、游戏信息、封面及背景归各自权利人所有。MIT 协议不授予第三方游戏素材的再分发权；使用在线服务须遵守对应服务规则。游栖与上述平台、游戏开发商或发行商无隶属或背书关系。

## 项目视觉素材与截图

- 应用图标 `desktop/build/appicon.png`、`desktop/build/windows/icon.ico` 和 `desktop/frontend/src/assets/app-icon.png` 为本项目使用内置 ImageGen 生成并调整的素材。
- `desktop/frontend/src/assets/wuxia-hero.png` 为项目早期界面生成的武侠示意插图，不应标注为官方游戏原画。应用优先显示匹配到的正式背景。
- 前端模板中的 `logo-universal.png` 随 Wails 模板保留，适用 Wails 的 MIT 声明。
- `docs/screenshots/main-steam.jpg` 为实际应用的 Steam 来源筛选界面截图，截图中的游戏封面、背景和商标仅用于说明应用功能，版权归各自权利人所有。
- 私人游戏清单、原始下载图片、API 密钥、设计参考图及构建产物不纳入本仓库。
