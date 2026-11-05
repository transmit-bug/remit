# mygo 平台能力盘点（工单 #4）

调研日期：2026-11-06。信息源：仓库内 `.agents/skills/mygo-*/SKILL.md`、上游 `github.com/egoist/mygo`（main 分支源码与 docs/、examples/，clone 于 /tmp/mygo-src）、官方 showcase https://mygo.egoist.dev/showcase。除标注「推断」外均为事实，逐条给来源。

上游仓库许可证：**MIT**（/tmp/mygo-src/LICENSE），examples/ 随仓库同 MIT，可自由拷贝改造。~/.agents/skills/mygo-* 与仓库内 skills 内容一致（未逐字 diff，推断等同）。

## 1. 分发

- `mygo build` 产物：macOS `.app` + `.dmg`（可签名+notarize）；Windows exe + NSIS 安装器；Linux 二进制 + `.desktop` + tar.gz + `install.sh` + `.deb` + Arch `PKGBUILD`。来源：docs/distribution.md（上游仓库）。
- 跨平台交叉编译：`-platform darwin/universal,windows/amd64,...`，**无 cgo 依赖**（默认 `CGO_ENABLED=0`），任意机器可编全平台。来源：docs/distribution.md「Platforms」。
- 自动更新：**内置且完整**。`mygo keygen` 签名密钥对，`mygo.Updater`（Check/Install/Relaunch），发布源支持 GitHub Releases（`-upload` 一键上传 draft release）、任意 HTTPS url、S3/R2；支持 delta 更新（bsdiff）、prerelease 频道、Sparkle 式更新窗口（plugins/updater）。来源：docs/updates.md、mygo-desktop SKILL「Install a released update」。
- CI：官方 GitHub Actions workflow 文档 docs/github-actions.md。来源：同仓库 docs/。

结论：分发与更新是 mygo 的强项，对 Remit（开源、GitHub 发布、三平台）零缺口。

## 2. desktop 层服务范围（mygo-desktop SKILL + docs/native.md、app.md）

事实清单（来源：仓库内 `.agents/skills/mygo-desktop/SKILL.md`，逐行对应）：

- 对话框：`mygo.Dialog.Open/Save/Message`（阻塞式，macOS sheet）。
- 通知：`mygo.NewNotification`（macOS 需打包应用；首次需权限）。
- 剪贴板：`Clipboard.WriteText/ReadText/WriteImage`；自定义格式走 `transfer.Data`（docs/data-transfer.md）。
- 托盘：`mygo.NewTray`（Linux 需 AppIndicator 库）。
- 菜单：`mygo.NewMenu`，模板/roles/accelerators/上下文菜单（docs/menus.md）。
- 文件关联与深链：`App.OnOpenFile` / `App.OnOpenURL`，schemes 与文件类型在 mygo 配置声明。✅ Remit 若需 `remit://` 深链可用。
- 单实例：`RequestSingleInstanceLock` + `OnSecondInstance`。
- 全局快捷键：`GlobalShortcut.Register`（Wayland 需 portal，GNOME 48+/KDE 6）——快速捕获灵感卡片可用。
- 深色模式/强调色：`mygo.Theme`；系统偏好 `c.Preferences()` 默认跟随。
- 文件监听 `WatchFiles`（50ms debounce）、密钥 `Secrets`（≤2560B，系统 keyring）、电源 `Power`、网络 `Network`/`netproxy`。
- 生命周期：`WhenReady`、`OnWindowAllClosed`（macOS 常驻模式）、`Quit/Exit/Relaunch`、`SetOpenAtLogin`。
- 数据目录：`App.Path(PathUserData/PathCache/PathLogs/PathResources)`。

结论：desktop 层覆盖 Remit 全部预期需求（通知复习提醒、托盘、深链、全局快捷键、数据目录），无缺口。

## 3. 组件库对「块编辑器 + 复习界面」的覆盖与缺口

来源：仓库内 `.agents/skills/mygo-components/SKILL.md`「Need → widget」全表 + 上游 docs/ui/（87 个目录页）与 examples/。

### 覆盖（现成 widget，勿手写）

| Remit 需求 | mygo 提供 | 来源 |
|---|---|---|
| Markdown 只读渲染 | `ui/markdown.View`，含跨 block 选择复制、代码块高亮、流式追加（增量只重析未闭合 block） | components SKILL Text 表；docs/ui/markdown.md |
| 富文本文档/编辑 | `richtext.NewDocument` + `richtext.ParseMarkdown/ParseHTML/ParseRTF` + `richtext.Editor`（格式、链接、标题、列表、图片、附件、输入法、无障碍、大文档章节） | components SKILL；docs/ui/rich-text-editor.md |
| 代码级编辑器 | `ui/editor`：`editor.View`+`Buffer`，caret、折叠、补全、诊断、diff（`DiffView`/`DiffState`） | components SKILL；docs/ui/code-editor.md、diff-viewer.md |
| 文本输入 | `TextInput/TextArea/SearchField/EditableText/TokenField/FindBar`；大文档 `TextBuffer`（indexed chunks，example 演示 10 万行可编辑） | components SKILL；examples/text-buffer |
| 列表/滚动 | `ui.List`（任意高度、Key、FollowEnd、Reorder、DragSelect）、百万行级；`Scroll/ScrollBoth`；`Table`、`Tree`、`GridView` | components SKILL Collections 表；docs/ui/performance.md（百万行可见帧 14.5µs） |
| 复习界面骨架 | `Split`（编辑/预览双栏）、`Sidebar`+`SidebarItem`+`Badge`（牌组列表）、`Router`+`Breadcrumbs`、`Tabs`、`Collapsible/Accordion`、`Form`+`Field`、`Rating`（难度/信心评分直配 FSRS 评分）、`Slider/StepSlider`、`chart`（复习曲线/遗忘曲线）、`Calendar`（复习日历）、`Toast`、`AlertDialog`、`DialogBase/Modal` | components SKILL 各表 |
| 快捷键 | `OnShortcut`/`OnClick`（动作级）、`tt.Compose/Command("copy")`（测试级）、菜单 accelerators | mygo-usage SKILL；components SKILL |
| 主题/密度/无障碍/Rtl | `c.SetTheme`、`Preferences()`、`.Label()/.Role()`、`c.Announce` | components SKILL Platform 表 |

### 缺口（需自建或拷贝改造）

- **真·块编辑器（Notion/Logseq 式 slash 命令、块拖拽嵌套、块级折叠大纲）**：无现成 widget。最接近的积木是 `richtext.Editor`（富文本+Markdown 快捷键+附件）+ `ui.List`（Reorder/DragSelect）+ `ui.Tree`/`Outline`；推断需在其上自建块树模型与 slash 菜单（`MenuButton`/`Popover` 可承载）。
- **间隔重复复习组件（翻面卡片、FSRS 评分按钮条）**：无；但 `Rating`/`Button`/`ToggleGroup` + `chart` 覆盖 90%，属薄封装而非缺口级工作量（推断）。

### 拷贝改造候选（上游 MIT，直接抄 examples/）

| 示例 | 与 Remit 的关系 | 来源 |
|---|---|---|
| `examples/notes`（283 行，native UI） | **最佳整应用参考**：侧栏列表 + 每条富文本编辑（ui/richtext）+ 格式工具栏 + Markdown 快捷键 + 导出 + JSON 持久化 | 上游仓库 examples/notes/main.go 首注释 |
| `examples/markdown-chat`（161 行） | Markdown 流式渲染（`markdown.Stream`→`View`）+ 列表 FollowEnd；卡片正文的增量渲染范式 | examples/markdown-chat/main.go 首注释 |
| `examples/code-editor`（222 行） | caret/补全/诊断/gutter/diff 全家桶，块内代码块编辑可借 | examples/code-editor/main.go 首注释 |
| `examples/text-buffer`（36 行） | 10 万行 TextBuffer 大文档编辑 | examples/text-buffer/main.go |
| `examples/todo` | 完整小应用：类型化服务/事件/持久化/多窗口同步（web 前端形态） | examples/todo/main.go 首注释 |
| `examples/timeline`、`gallery` | 时间线（复习历史时间轴）、组件总览 | 上游 examples/ 目录 |

## 4. 嵌入本地 HTTP / MCP server

- mygo 应用就是普通 Go 进程（`CGO_ENABLED=0`），`net/http` 起本地 listener 无任何框架限制（事实：docs 全篇以普通 Go 程序为前提；推断：无阻碍，惯用法即 goroutine + `http.Server`，注意按 mygo-usage SKILL 用 `Window.Update` 回 UI 线程）。
- WebView 侧已有 `http.Handler` 桥：`Protocol.Handle/HandleFunc/FileServer` 接受 net/http 类型，自定义 scheme 在 Windows 上映射到 `http://*.localhost`（docs/architecture.md:45,63,82）。即「本地 HTTP 服务」是 mygo 自身内部机制的一部分。
- `mygo dev -web` 本身就在 localhost:5174 起服务器（docs/ui/browsers.md）。
- **MCP：mygo 无内置 MCP 支持**（全仓库 grep "mcp" 仅命中 sqlite.c 无关行，事实）。但 Go MCP SDK 生态可直接引入；showcase 中 **Godwit**（godwit.email，native UI）明确以「MCP servers extend it」为卖点，**Lumilo** 是管理本地 LLM server 的菜单栏应用，**Kero** 内置 CLI 供 agent 驱动——证明 mygo 应用承载本地 server + agent 协议是已验证路径。来源：showcase 页。

结论：内嵌本地 HTTP server 完全可行且有一等公民支持（Protocol）；MCP 需引第三方 Go SDK，无框架障碍。

## 5. 渲染与性能

来源：docs/ui/rendering.md、docs/ui/performance.md、mygo-drawing SKILL。

- GPU 自绘管线：macOS Metal、Windows Direct3D 11（无驱动回退 WARP）、Linux OpenGL（GtkGLArea）、浏览器 WebGL 2；圆角/边框/渐变/阴影由距离场 shader 计算，任意缩放清晰；文字走 glyph atlas 只上传变化部分。
- **按需重绘**：只有变化才画；空闲窗口零绘制，2 秒后释放内存帧。即时模式视图函数 + 保留式引擎（mygo-usage SKILL 开篇）。
- 基准（Apple M5、CPU 渲染、GOMAXPROCS=1，2026-10 实测，docs/ui/performance.md）：1000 静态标签 397µs/帧 0 分配；**百万行 List 可见行 14.5µs/帧**；**50KB Markdown 整页显示 1.3ms/帧**、流式 16B/帧 0.8ms；1000 标签常驻堆 ~2.5MB。
- 大文本量编辑手感风险：低。证据链：`editor.Buffer`（caret/折叠）+ `TextBuffer` indexed chunks（10 万行示例）+ `TextInputClient`/`ShapeText` 保留式几何（mygo-drawing SKILL tier 3、docs/ui/text-input-client.md）。推断：Remit 卡片体量（KB 级 Markdown）远低于基准的 50KB 全页渲染，帧预算充足。
- 工具：`MYGO_FRAME_STATS=1` 逐帧 build/layout/paint/present 拆分；`MYGO_GPU=0` 强制 CPU 对比。

## 6. showcase 整应用级参考（地图维护者补充项）

来源：https://mygo.egoist.dev/showcase（2026-11-06 读取）。

| 应用 | 相关性 | License |
|---|---|---|
| **留白 Liubai**（liubai.echovic.com） | **整应用级参考**：开源 Markdown 编辑器/阅读器（macOS 正式 + Windows beta），打字即排版、.md 纯文件、11 主题。注意是 **web 前端形态**（非 native UI） | MIT（GitHub echovic/liubai，gh api 确认） |
| Godwit | native UI 邮件客户端，MCP servers 扩展——MCP 集成参考 | 未查（闭源/未标） |
| Kero / GoRex | agent 终端，会话常驻 + CLI 驱动 | GoRex、2048 仓库**无 LICENSE 文件**（gh api 确认），不可直接拷码，仅作参考 |
| MixMusic | native UI 跨平台音乐播放器 | **GPL-3.0**，拷码会传染，只可看不可抄 |
| Pulse / MyTwig / 留白 / Clipnest 等 | 桌面生产力 app 形态参考 | — |

许可注意：mygo 仓库本体及 examples/ = MIT ✅；showcase 第三方应用 license 各异，已逐一标注，凡无 LICENSE 或 GPL 的只作行为参考。

## 结论

mygo 可承载 Remit 桌面实现。能力边界：分发/更新/desktop 服务/渲染性能均为强项且有官方文档背书；唯二自建点是**块编辑器交互层**（在 richtext.Editor + List/Tree 之上组装，examples/notes 为最佳起点拷贝改造候选）和 **MCP 接入**（引 Go MCP SDK）。与地图决策（GUI 主体 mygo、开源、本地优先）无冲突事实。
