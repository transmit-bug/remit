# Obsidian 笔记与 Anki 制卡的事实边界

> 研究票：transmit-bug/remit #203
> 研究日期：2026-11-08
> 基线：`origin/master`（`bad1b77`）；Remit 模型以 ADR 0002、0003、0008 与
> `internal/core`、`internal/cards`、`internal/core/importmd`、
> `internal/core/importanki` 的当前代码为准。外部事实只取第一方来源：
> Obsidian 官方帮助（`obsidian.md/help`）、Obsidian 官方开发者文档与
> `obsidian.d.ts`、Anki 官方手册（`docs.ankiweb.net`）与 Anki 源码。

## 判定口径

- **核心**：用户真正依赖它来完成“写下来 / 记住”的动作；失去它，笔记或复习
  的核心行为就断了。
- **可选**：只服务部分工作流，可以由宿主视图、插件或以后的服务承担。
- **不采纳**：属于容器、云端、插件信任层或历史包袱；进入 Remit 核心会破坏
  “明文库 / 一条单向依赖 / 插件受限”的模型。
- Remit 状态另用 **已覆盖 / 需补齐 / 不采纳**：已覆盖表示现有模型或代码已有
  对应语义；需补齐表示语义应保留但当前实现/迁移不完整；不采纳表示有意留在
  核心之外。

一句话结论：Obsidian 的核心是“文件夹里的 Markdown + 由文本派生的链接索引”，
Anki 的核心是“note 的字段/模板生成多张 card，card 独立调度”；Remit 的
Note/Block/Card/Card Type/Field/Template/Asset/Review Scope 已经覆盖两者最
重要的骨架。真正要补的不是再引入 deck、vault 配置或 HTML 模板，而是
**迁移时的链接、块 ID、frontmatter、附件与复杂 cloze 的诚实降级**。

## 结论摘要

| 事实 | 产品地位 | Remit 判定 |
|---|---|---|
| Obsidian 笔记是纯文本 Markdown；vault 是一个文件夹 | 核心 | 已覆盖：库是明文目录，笔记是 Markdown 文档 |
| Obsidian 的 backlinks / graph 是索引，不是另一份真相 | 核心（导航） | 已覆盖基本 backlink；graph 不采纳为核心 |
| `[[wikilink]]` 简单形式 | 核心（Obsidian 用户） | 已覆盖：`[[标题]]` 与标题索引 |
| alias、heading、路径、`![[embed]]`、块引用 | 核心/可选混合 | 需补齐：Remit 的链接语法窄得多，且块 ID 形态不同 |
| frontmatter / properties | 可选元数据 | 不采纳为模型；迁移需补齐“保留或显式丢弃” |
| callout、`%%comment%%`、`==highlight==` | 可选表达 | 不采纳为核心语法；文本可原样保存 |
| 表格、代码、数学、图片 | 核心阅读/写作 | 已覆盖阅读与存储；表格编辑、公式覆盖范围需补齐 |
| `.obsidian` 配置、workspace、缓存、全局设置 | 应用状态 | 不采纳：不属于库的真相 |
| 社区插件 API、Sync、Publish | 扩展/云服务 | 不采纳：插件能力模型与云服务都不进核心 |
| Anki 一条 note 生成多张 card | 核心 | 已覆盖概念：一个 Block 可派生多张 Card |
| Anki note type 的字段 + 模板 | 核心 | 已覆盖：Card Type = 字段 + 正/反模板 |
| 一个 note type 含多个 card type/模板 | Anki 核心 | 需补齐或由 derive 插件派生；Remit 类型只有一对模板 |
| 简单 cloze（每空一张卡） | 核心 | 已覆盖：`{{...}}` 每空一张 |
| cloze 同号分组、hint、嵌套、`c1,2,3` | 可选/历史包袱 | 不采纳原生语义；导入必须扁平化并报告 |
| deck（容器 + 每 deck 选项） | 可选容器 | 不采纳为模型；Review Scope 已覆盖“本次复习范围” |
| collection（单文件数据库） | 核心存储 | 已覆盖为明文库；不采纳 SQLite 单文件形态 |
| browser / stats | 核心导航/回顾 | 已覆盖基本能力；批量编辑、深图可选 |
| media（图片/音频/视频） | 核心 | 图片 Asset 已覆盖；音频/视频/PDF 需补齐 |
| add-on（任意 Python 代码） | 可选扩展 | 不采纳该形态；Remit 的能力插件已覆盖 |

## 0. Remit 的基线模型

| 概念 | 定义 | 在本次映射中的作用 |
|---|---|---|
| **Note** | 有序 Block 组成的 Markdown 文档 | 对应 Obsidian 的 note；**不等于** Anki 的 note（后者是字段记录） |
| **Block** | 内联 ID、可寻址的最小引用单位 | 对应 Obsidian 的 block；也是 Remit 制卡的来源单位 |
| **Card** | 引用一个 Block 的独立复习实体；存引用、状态、`type + ordinal + producer`，只有用户编辑过的字段落库 | 对应 Anki 的 card；调度历史的归属者 |
| **Card Type** | 有序字段表 + 正/反两段模板 + 版本，库内明文资产 | 对应 Anki 的 note type（但只有一对模板）；不直接等于 Anki 的“card type” |
| **Field** | 具名标量；默认从 Block/快照现算，用户编辑才存覆盖 | 对应 Anki field；但 Anki 把值存在 note 上 |
| **Template** | 宿主解释的纯数据方言：`{{Field}}`、条件段、`{{cloze:}}`、`{{hint:}}`、`{{type:}}` | 对应 Anki 模板的“字段替换”部分；不采纳 HTML/CSS/脚本 |
| **Asset** | 按内容命名的库内图片资产，随库走 | 对应 Obsidian attachment / Anki media 的图片子集 |
| **Review Scope** | 一次复习会话的取卡范围：全库 / 某笔记 / 某标签；会话内选择、不落库 | 对应 Anki deck / filtered deck 的“本次复习范围”部分 |

依据：`GLOSSARY.md`、`docs/adr/0002-content-model.md`、
`docs/adr/0003-storage-layout.md`、`docs/adr/0008-card-types-templates.md`、
`internal/core/model.go`、`internal/core/types.go`、`internal/core/queue/queue.go`。

其中最容易混淆的一点：**Remit 的 Reference 是 card → block，不是 note → note。**
Obsidian 的 `[[wikilink]]` 对应的是 Remit 的 Link/backlink（笔记间连接），不是
Card 的 Reference。两者都在明文里，但一个是复习来源，一个是导航边。

## 1. Obsidian

### 1.1 Obsidian Flavored Markdown 与 CommonMark 的差异

Obsidian 官方说明它同时支持 **CommonMark、GitHub Flavored Markdown 与 LaTeX**，
并在其上增加自己的扩展。本报告没有在官方帮助/开发者文档中找到一份完整、版本化的
方言语法规范；因此下面的“差异”以官方页面逐项列出的支持面为准。
来源：[Obsidian Flavored Markdown](https://obsidian.md/help/obsidian-flavored-markdown)、
[Basic formatting syntax](https://obsidian.md/help/syntax)、
[Advanced formatting syntax](https://obsidian.md/help/advanced-syntax)、
[Callouts](https://obsidian.md/help/callouts)、
[Properties](https://obsidian.md/help/properties)、
[Tags](https://obsidian.md/help/tags)、
[Attachments](https://obsidian.md/help/attachments)、
[Accepted file formats](https://obsidian.md/help/file-formats)。

| 特性 | CommonMark/GFM 基线 | Obsidian 的事实 | Remit 判定 |
|---|---|---|---|
| 段落、标题、列表、引用、强调、外链、图片、代码块 | CommonMark 基础 | 支持；代码块可用反引号/波浪线，语法高亮用 Prism | **核心 / 已覆盖**：笔记文本原样保存，阅读面走 Markdown |
| 表格 | GFM 扩展，不是 CommonMark 核心 | 支持用竖线分列的表格、对齐、单元格内格式；Live Preview 可增删行列 | **核心阅读 / 已覆盖阅读，需补齐编辑**：现有 richtext 编辑器会把表格降级为 tab 分隔段落（见 `docs/research/richtext-tables-and-images.md`） |
| 数学 | 不是 CommonMark | 用 MathJax + LaTeX：`$...$` 行内、`$$...$$` 块 | **核心 / 已覆盖（子集）**：Remit 保存 TeX 源，`internal/ui/maths` 转写并绘制一部分公式；不是完整 MathJax |
| 代码 | CommonMark 围栏代码 | 支持，且可嵌套外层更多反引号 | **核心 / 已覆盖**：文本与代码块保留 |
| HTML | CommonMark 允许内嵌 HTML | Obsidian **不渲染 HTML 元素内部的 Markdown**，这是有意的性能取舍 | **可选 / 不采纳为核心语义**：Remit 不依赖 HTML 作为笔记结构 |
| Callout | 不是 CommonMark | 引用块扩展：`> [!info]`，可折叠、嵌套、用 CSS/插件自定义类型 | **可选 / 不采纳为核心语法**：可作为展示层以后支持；不存在于 Card/Block 模型 |
| Frontmatter / properties | 不是 CommonMark | 文件顶部 YAML（也接受 JSON，最终存为 YAML）；有 text/list/number/checkbox/date/datetime/tags 类型；默认 `tags`、`aliases`、`cssclasses`；属性里不渲染 Markdown | **可选 / 不采纳为模型，导入需补齐**：Remit 没有 per-note property bag；导入时必须决定保留原文、提取 tags/aliases，还是显式丢弃 |
| 行内 tag | 不是 CommonMark | `#tag`；也可写在 `tags` property；支持嵌套 `/`、大小写不敏感、Unicode；不能有空格 | **核心组织 / 已覆盖基础，需补齐**：Remit 只读行内 `#tag`，大小写敏感，且解析器不接受 `/`；YAML tags 不读 |
| `[[wikilink]]` / `![[embed]]` / 块引用 | 不是 CommonMark | Obsidian 专有；官方明确说块引用离开 Obsidian 不工作 | **核心（Obsidian 用户）/ 需补齐**：见 1.2 |
| `%%comment%%`、`==highlight==`、任务列表、脚注 | Obsidian/GFM 扩展 | 支持；注释只在编辑视图可见 | **可选 / 文本已保存；不采纳为核心语法**：渲染可以做，模型不必知道 |
| 附件 | Markdown 图片/链接是标准形式 | 支持图片、音频、视频、PDF、Canvas；附件默认放 vault 根，位置可配置 | **核心（图片）/ 图片 Asset 已覆盖；音视频/PDF 需补齐** |

关键判断：**“标准 CommonMark”不是 Obsidian 的全部**。Obsidian 用户真正每天依赖的
是 CommonMark/GFM 之上的 `[[...]]`、`![[...]]`、frontmatter、callout、tags 与
MathJax。Remit 可以拒绝把 callout/property 做成核心模型，但不能在迁移时假装
它们不存在。

### 1.2 `[[wikilink]]`、`![[embed]]`、backlinks 与 graph

#### 语义与存储形态

Obsidian 支持两种内部链接：

- Wikilink：`[[Three laws of motion]]` 或 `[[Three laws of motion.md]]`
- Markdown 链接：`[Three laws of motion](Three%20laws%20of%20motion.md)`

两者等价；默认生成 Wikilink，若重视互操作可以关掉 Wikilink、改用 Markdown 链接。
链接可以带文件夹路径（从 vault 根起、用 `/`），可以只写最短唯一路径；设置里有
“最短路径 / 相对路径 / vault 绝对路径”三种生成格式。显示文字可用 `|` 覆盖，
或使用 alias。
来源：[Internal links](https://obsidian.md/help/links)、
[Settings · Files and links](https://obsidian.md/help/settings)、
[Aliases](https://obsidian.md/help/links#change-the-link-display-text)。

块链接写作 `[[Note#^block-id]]`，被链接的段落以 `^block-id` 结尾；列表、引用、
callout、表格这类结构化块要把 ID 单独放一行。块 ID 只能由拉丁字母、数字和短横线
组成；官方明确警告块引用是 Obsidian 专有语法，离开 Obsidian 不会工作。
来源：[Internal links · Link to a block in a note](https://obsidian.md/help/links)。

`![[...]]` 是嵌入：在内部链接前加 `!`，把目标内容就地显示，并在源文件变化时保持
更新。可嵌入整篇笔记、标题、块、图片、音频、PDF、Canvas、列表，甚至搜索结果。
来源：[Embed files](https://obsidian.md/help/embeds)。

Backlinks 是“别的笔记指向当前笔记”的派生视图，分 **Linked mentions**（有内部
链接）与 **Unlinked mentions**（未链接但出现笔记名）。Graph view 的节点是笔记/
附件，线是内部链接；节点大小反映被引用次数。两者都不是另存的一份图数据库：
Obsidian 用本地 **metadata cache**（记录 links、embeds、blocks、frontmatter、
headings、tags 等）来支撑 backlinks、outline、graph，缓存可重建，存在 IndexedDB
中。来源：[Backlinks](https://obsidian.md/help/plugins/backlinks)、
[Graph view](https://obsidian.md/help/plugins/graph)、
[How Obsidian stores data](https://obsidian.md/help/data-storage)、
[MetadataCache](https://docs.obsidian.md/Reference/TypeScript+API/MetadataCache)。

| Obsidian 构造 | 明文里的形态 | 谁派生 | Remit 对应 | 判定 |
|---|---|---|---|---|
| 简单 `[[Note]]` | 文本 | 索引 | `core.Links` + `links` 表 + `Backlinks()` | **核心 / 已覆盖**（按标题、忽略大小写与多余空白） |
| Markdown 内部链接 `[x](Note.md)` | 文本 | 索引 | Remit 的 `core.Links` 只认 `[[...]]`，不索引 Markdown 链接 | **需补齐**（若要导入 Obsidian 默认之外的链接格式） |
| `[[Note\|Alias]]` / `[[Note#Heading]]` / 路径 | 文本 | 索引 | Remit 会把 `Note\|Alias` 或 `Note#Heading` 当成完整标题；路径也不拆 | **需补齐**：否则链接静默失效 |
| `[[Note#^id]]` | 文本 + 块 ID | 索引 | Remit 只认 `[[Note#^block_<ID>]]`，且 ID 必须是 Remit 的 `block_` 形态 | **需补齐**：Obsidian 块 ID 与 Remit 块 ID 不是同一种东西 |
| `![[image.png]]` | 文本 + 附件文件 | 渲染时嵌入 | 图片走 Markdown 图片 + `assets/<sha>.<ext>` Asset；`![[...]]` 没有转写语义，`Links` 只会把它当普通 `[[...]]` 边 | **图片核心 / 需补齐导入与路径重写** |
| `![[Note]]` / `![[Note#^id]]` | 文本 | 渲染时转写 | Remit 没有 note/block transclusion | **不采纳为核心**；导入保留原文并提示 |
| Backlinks | 无独立文件 | 由链接索引派生 | Remit 的 `links` 表与 `Backlinks()` 也是派生索引 | **核心 / 已覆盖基本能力** |
| Graph | 无独立文件 | 由链接索引派生 | Remit 没有 graph 视图；backlinks 已够导航 | **可选 / 不采纳为核心** |
| Tags / properties | 文本 | metadata cache | Remit 只索引行内 tag，不索引 properties | **部分已覆盖 / 需补齐迁移** |

**最重要的结论**：Obsidian 的链接和 graph 不是“另一份数据库”，而是从 Markdown
文本派生的索引；Remit 的 `links` 表遵循同样的原则。差别在**解析覆盖**，不在
存储哲学。把 Obsidian vault 直接复制进 Remit 时，简单标题链接有机会工作；
alias、heading、路径、块引用、embed 则必须重写或明确降级。

### 1.3 vault = 文件夹 + 附件 + 配置：哪些是库，哪些是应用状态

Obsidian 官方的定义很直接：**vault 是文件系统里的一个文件夹**，包含笔记、
附件和 `.obsidian` 配置文件夹；`.obsidian` 里是 vault 专属设置，例如 hotkeys、
themes、community plugins。全局设置则存在操作系统的应用数据目录，不在 vault 里。
来源：[Manage vaults](https://obsidian.md/help/manage-vaults)、
[How Obsidian stores data](https://obsidian.md/help/data-storage)、
[Configuration folder](https://obsidian.md/help/configuration-folder)。

| vault 里的东西 | 它是什么 | 是否属于“笔记真相” | Remit 判定 |
|---|---|---|---|
| `*.md` | 笔记正文 | 是 | **核心 / 已覆盖**：库的 `notes/*.md` |
| 图片、音频、视频、PDF 等附件 | 被笔记引用的文件 | 是（内容真相关） | **图片 Asset 已覆盖；其他类型需补齐** |
| `.obsidian/app.json`、appearance、hotkeys、themes、snippets | 应用/外观配置 | 否 | **不采纳**：属于应用状态 |
| `.obsidian/workspace.json`、`workspaces.json` | 当前窗口布局，打开新文件就变 | 否；官方建议 Git 忽略 | **不采纳** |
| `.obsidian/plugins/` 与插件数据 | 第三方代码及其设置 | 否；只是扩展状态 | **不采纳**：插件配置不进库模型 |
| 全局设置目录 | 跨 vault 的应用设置 | 否 | **不采纳** |
| IndexedDB metadata cache | 可重建的派生索引 | 否 | **不采纳**：Remit 自己的 `.remit/index.sqlite` 也是派生 |
| Sync 的设备名、冲突策略、选择性同步设置 | 设备级状态 | 否；官方明确不跨设备同步 | **不采纳** |
| Publish 的 `publish`、`permalink`、`description`、`cover` 等属性 | 发布服务元数据 | 否 | **不采纳**：属于云端发布层 |

Remit 的对应边界更窄也更清楚：`notes/`、`cards/`、`types/`、`assets/`、
`reviews.jsonl`、`remit.json` 是真相；`.remit/index.sqlite` 可删可重建。
`remit.json` 只放格式版本、FSRS 参数、期望保留率这类**库级**配置，不承担
Obsidian `.obsidian` 那样的 UI/插件/工作区状态。来源：
`docs/adr/0003-storage-layout.md`、`GLOSSARY.md`。

判断：**Obsidian vault 的内容层可以迁移，配置层不应迁移。** 把 `.obsidian` 整个
搬进 Remit 会把应用状态、第三方代码和插件配置混进明文库，违反“库是可读、可带走的
真相”这条原则。

### 1.4 插件 API、社区插件、Sync、Publish 的边界

#### Obsidian 插件 API

Obsidian 插件是 TypeScript/JavaScript 写的 `Plugin` 类，有 `onload`/`onunload`
生命周期；API 暴露 `Vault`（文件/文件夹）、`MetadataCache`（links、blocks、
frontmatter、tags）、事件、Markdown post processor、code block processor、
命令、视图、设置页等。插件能读文件、写文件、注册渲染器，也能拿到宿主的事件。
来源：[Build a plugin](https://docs.obsidian.md/Plugins/Getting+started/Build+a+plugin)、
[Anatomy of a plugin](https://docs.obsidian.md/Plugins/Getting+started/Anatomy+of+a+plugin)、
[Vault](https://docs.obsidian.md/Plugins/Vault)、
[Markdown post processing](https://docs.obsidian.md/Plugins/Editor/Markdown+post+processing)、
[Events](https://docs.obsidian.md/Plugins/Events)、
[obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)。

关键是信任边界：Obsidian 官方明确说，由于技术限制，**无法可靠地把插件限制到
特定权限**；插件继承 Obsidian 的访问级别，可以访问本机文件、连接互联网、安装
其他程序。社区插件是第三方代码，默认 Restricted Mode 关闭插件执行，安装前应
自行审计。来源：[Plugin security](https://obsidian.md/help/plugin-security)、
[Community plugins](https://obsidian.md/help/community-plugins)。

#### Sync 与 Publish

Obsidian Sync 是付费附加服务：本地 vault 连接一个远程 vault，按文件级同步变更；
可以选择同步哪些文件类型和哪些 vault 配置，Sync 设置本身是设备级的、不随同步走。
来源：[Introduction to Obsidian Sync](https://obsidian.md/help/sync)、
[Local and remote vaults](https://obsidian.md/help/sync/vault-types)、
[Sync settings and selective syncing](https://obsidian.md/help/sync/settings)。

Obsidian Publish 是云端托管服务：只把用户选中的笔记发到 Obsidian 服务器，其余
留在本地；它支持基本 graph、搜索、媒体，但对社区插件只有最小支持。
来源：[Introduction to Obsidian Publish](https://obsidian.md/help/publish)、
[Publish limitations](https://obsidian.md/help/publish/limitations)、
[Publish security and privacy](https://obsidian.md/help/publish/security)。

| 层 | 它解决的问题 | 信任/边界 | Remit 判定 |
|---|---|---|---|
| 插件 API | 扩展编辑、渲染、命令、视图、设置 | 插件是任意宿主代码；没有可靠权限沙箱 | **可选 / 不采纳该形态**：Remit 用 manifest + capability + 宿主绘制，内置插件走同一加载器（ADR 0006/0007） |
| 社区插件分发 | 发现、安装、更新第三方扩展 | 第三方代码，需用户信任 | **不采纳为库核心**：插件可以存在，但不是笔记真相 |
| 主题 / CSS snippets | 外观 | 表现层 | **可选 / 不采纳为核心模型**：Remit 用设计 token 与宿主渲染 |
| Sync | 多设备传输、冲突处理、版本历史 | 云服务/设备状态 | **可选 / 不采纳为核心**：未来若做同步，是独立服务，不是库格式 |
| Publish | 把笔记发布成网站 | 云托管、选择性上传 | **可选 / 不采纳为核心** |
| Graph view | 可视化链接 | 由索引派生的视图 | **可选 / 不采纳为模型**；backlinks 已覆盖导航 |
| 核心插件（Backlinks、Templates、Outline 等） | Obsidian 自带功能 | 应用功能 | 按功能逐项判断；backlinks 已覆盖，模板/大纲可选 |

**不应进入 Remit 3 合 1 核心的东西**：任意代码插件 API、`.obsidian` 插件配置、
Sync/Publish 服务、graph 存储、主题/CSS、Publish 专用属性。Remit 自己的插件
边界已经更严格：manifest 声明 hook 与 capability，JS/exec 两种受限运行时，
宿主绘制插件贡献的每一个像素，内置插件没有特权入口。来源：
`docs/adr/0006-extension-substrate.md`、`docs/adr/0007-plugin-system.md`、
`docs/specs/plugins/system.md`。

### 1.5 从 Obsidian vault 迁移到 Remit（以及反向）

把“复制文件夹”当成迁移是不够的。下面每一条都是当前代码可以验证的差异。

| 项目 | Obsidian 形态 | Remit 形态 | 迁移难点与判定 |
|---|---|---|---|
| 标题 | 文件里通常有 H1；文件名也可作标题 | 笔记源第一行 `# 标题`；否则导入回退到文件名 | **需补齐**：frontmatter 在顶部时，Remit 的 `ParseNoteSource` 只认第一行 `# `，会把标题回退成文件名，H1 变成普通 Block（`internal/core/store/note.go`、`internal/core/importmd/import.go`） |
| 简单 wikilink | `[[Note]]` | `[[Note]]`，按归一化标题匹配 | **已覆盖**：标题大小写/空白不敏感（`internal/core/links.go`） |
| alias / heading / 路径链接 | `[[Note\|Alias]]`、`[[Note#Heading]]`、`[[Folder/Note]]` | 只解析 `[[标题]]`，不拆竖线、`#标题` 或路径 | **需补齐**：当前会当成完整标题，链接静默失效 |
| Markdown 内部链接 | `[Note](Note.md)` | `core.Links` 不索引 Markdown 链接 | **需补齐**：若保留 Obsidian 的 Markdown 链接模式，backlinks 不会建立 |
| 块引用 | `[[Note#^37066d]]`，ID 只能拉丁字母/数字/短横线 | `[[Note#^block_<id>]]`，ID 是 Remit 的 `block_` 形态 | **需补齐**：必须为每个被引块铸造 Remit ID 并重写链接；不能复用 Obsidian ID |
| 图片嵌入 | `![[image.png]]` 或 `![](path)`，附件可在 vault 根/自定义目录 | `![](assets/<sha>.<ext>)`，内容寻址 | **需补齐**：当前导入只收 `assets/` 子树，不读 `.obsidian` 的附件位置，也不把引用重写到哈希路径（`internal/core/importmd/assets.go`） |
| 笔记/标题/块转写 | `![[Note]]`、`![[Note#^id]]` | 无 transclusion | **不采纳为核心**：保留原文并警告；不要假装已渲染 |
| 音频/视频/PDF | 可嵌入并播放/查看 | Asset 的既定语义是图片；没有播放/PDF 视图 | **需补齐**（若要完整迁移）；或显式降级为链接文字 |
| frontmatter / properties | YAML/JSON 顶部元数据 | 无 property 模型；YAML 会变成 Block 文本 | **不采纳为模型，但导入需补齐**：至少保留原文、提取 `tags`/`aliases`，并在报告中说明丢弃了什么 |
| 行内 tags | `#tag`，支持 `/` 嵌套、大小写不敏感 | 行内 `#tag`，不支持 `/`，大小写敏感；YAML tags 不读 | **需补齐**：`#inbox/to-read` 在 Remit 会只解析成 `inbox`；`#Tag` 与 `#tag` 是两个 tag（`internal/core/tags.go`） |
| 附件目录 | 默认 vault 根，也可配置到任意子目录 | 只认库内 `assets/` | **需补齐**：导入前要移动/重写，或扩展 importer 读取附件配置 |
| 隐藏目录/回收站 | `.obsidian`、`.trash` 等可能含 `.md` | `markdownFiles` 递归收所有 `.md`，没有忽略规则 | **需补齐**：直接选 vault 根可能把插件 README、回收站笔记也导入；导入器应先应用忽略规则 |
| `.obsidian` / workspace / plugins | 应用状态 | 不导入 | **不采纳**：见 1.3 |
| callout / math / table / code | Markdown 方言 | 文本可保存；math 子集渲染；表格编辑降级 | **存储已覆盖；渲染/编辑按现有能力补齐** |
| Canvas / Bases | Obsidian 专有文件格式 | 无对应模型 | **不采纳为笔记核心**；可作为附件或未来独立导入 |

反向（Remit → Obsidian）的难点：

- Remit 的块 ID 是 `block_<hex>`，含下划线；Obsidian 官方只允许拉丁字母、数字、
  短横线。直接把 `^block_...` 导出到 Obsidian，块引用很可能不被识别。要导出就
  必须生成 Obsidian 合法 ID，并维护一份映射。
- Remit 的 `::`、`:::`、`{{...}}`、`::: type` 在 Obsidian 里只是普通文本，卡片
  类型、ordinal、调度历史都不会随 Markdown 走。导出只能是“笔记 + 图片”，不是
  “可复习卡”。
- Remit 的 `#tag` 没有嵌套语义；导出到 Obsidian 不会自动变成层级 tag。
- `assets/<sha>.<ext>` 是相对路径；若 Obsidian vault 的附件设置不同，需要重写。

**判定**：Obsidian → Remit 应定位为“Markdown 文本 + 图片的迁移，带明确损失
报告”，而不是“vault 原样打开”。这与 Anki importer 的定位一致：文本和评分历史
可以过，容器/模板/媒体字节要逐项说明。

## 2. Anki

### 2.1 Note 与 Card 的一对多；note type、字段、模板与生成规则

Anki 的基本概念链：

- **Note**：一组相关信息的集合，每项信息是一个 **field**。例如法语词条可以有
  `French`、`English`、`Page` 三个字段。
- **Note type**：一类 note 的字段表 + 一个或多个 **card type**。
- **Card type**：一套 front/back 模板，决定哪些字段出现在正面/背面；一个 note
  type 可以有多套，因此一条 note 可以生成多张 card。
- **Card**：真正进入复习队列、拥有独立调度状态的实体。notes 和 note types 属于
  整个 collection，不属于某个 deck；同一条 note 的不同 card 甚至可以放进不同
  deck。

来源：[Getting Started · Notes & Fields / Card Types / Note Types](https://docs.ankiweb.net/manual/getting-started)、
[Adding/Editing](https://docs.ankiweb.net/manual/editing)、
[Card Templates](https://docs.ankiweb.net/manual/templates/intro)。

存储层也直接体现一对多：`notes` 表有 `mid`（note type）和 `flds`（字段串），
`cards` 表有 `nid`、`did`（deck）、`ord`（模板/变体序号）；`revlog` 按 `cid`
记录每次评分。索引 `ix_cards_nid` 明确服务“按 note 找 card”。
来源：[schema11.sql](https://github.com/ankitects/anki/blob/main/rslib/src/storage/schema11.sql)、
[Card](https://github.com/ankitects/anki/blob/main/rslib/src/card/mod.rs)、
[Note](https://github.com/ankitects/anki/blob/main/rslib/src/notes/mod.rs)。

生成规则（普通 note type）：

1. 对每个 card type（模板）渲染 front；
2. 只有 front 非空才生成该 card；
3. 每张 card 的 `ord` 是模板序号（0-based）；
4. 编辑 note 时，新出现的非空 front 会补建 card；已经变空的 card 不会立即删除，
   要用 Tools → Empty Cards 清理；
5. 新 note 如果一套模板都生成不了，Anki 仍会建一张 `ord=0` 的空白卡，方便以后
   补全。

来源：[Card Generation](https://docs.ankiweb.net/manual/templates/generation)、
[cardgen.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/cardgen.rs)。

生成只看 front：如果 front 非空、back 为空，card 仍会生成，得到一个空白背面；
要避免空白背面，需要把必填字段放进 front 的条件段。
来源：[Card Generation · Blank Back Sides](https://docs.ankiweb.net/manual/templates/generation#blank-back-sides)。

模板本身是 HTML + CSS：front/back 是 HTML，Styling 是 CSS；支持字段替换、
条件段、`FrontSide`、`Tags`/`Type`/`Deck`/`Card` 等特殊字段、`hint:`、`text:`、
`type:`、`tts:`、`furigana`/`kana`/`kanji`、`cloze:` 等过滤器。模板还能设置
Deck Override 和 Browser Appearance。
来源：[Field Replacements](https://docs.ankiweb.net/manual/templates/fields)、
[Styling & HTML](https://docs.ankiweb.net/manual/templates/styling)。

Remit 的对应关系必须精确到“层级”：

| Anki | Remit | 差异与判定 |
|---|---|---|
| Note（字段记录 + note type） | **不是** Remit Note；其“卡源”部分对应 Block + Card Type 字段 | **不能一对一硬套**。Anki note 不是文档，没有 Block 顺序，也不共享 Remit Note 的标题/段落语义 |
| Card | Card | **已覆盖核心语义**：独立调度实体、历史归 card；但 Anki card 指向 note+template ord，Remit card 指向一个 Block |
| Note type（字段 + N 个 card type） | Card Type（字段 + **一对** front/back 模板） | **已覆盖单 card type 情形；多 card type 需补齐或由 derive 插件拆成多张 card** |
| Card type（一套 front/back 模板） | Card Type 的正/反模板 | **部分覆盖**：Remit 没有“同一类型内多套模板”的概念；`ordinal` 表示派生出的第几张卡，不是模板下标 |
| Field | Field | **已覆盖命名标量语义**；Anki 把字段值存在 note 上，Remit 默认从 Block 现算，只有编辑过的字段落库 |
| Template | Template | **部分覆盖**：字段替换/条件段/cloze 对应；HTML/CSS/JS、特殊字段、过滤器不采纳 |
| Deck | 无直接模型；Review Scope + 笔记/标签组织 | **不采纳 deck 容器**；见 2.3 |
| Collection | Library | **已覆盖“全部材料”语义**；不采纳单文件 SQLite 形态 |

ADR 0008 的 derive 契约是 `deriveBlock(block, types) → [{typeId, ordinal, fields}]`；
host 校验类型与字段，写入 `type/ordinal/producer`，字段只在用户编辑时覆盖。
因此 Remit 表达“一条 note 生成多张卡”的方式不是“一个 note type 多个模板”，而是
“一个 Block 由插件派生出多个 `(type, ordinal)`”。内置例子：

- `::` → `remit.basic` / ordinal 0；
- `:::` → `remit.bidirectional` / ordinal 0 和 1（交换 Front/Back）；
- `{{...}}` → `remit.cloze` / 每个空一个 ordinal；
- `::: type <id>` → 指定类型 / ordinal 0。

来源：`docs/adr/0008-card-types-templates.md`、
`internal/ext/builtin/cards/main.js`、`internal/cards/resolver.go`。

**判定**：Anki 的 note/card 一对多和字段/模板模型，Remit 已经在更细的 Block 层
覆盖；最大的模型差异是 **Anki note type 可以含多个 card type，而 Remit Card Type
只有一对模板**。如果未来要一比一吃下 Anki 模板，要么把 Anki 的每个 card type
拆成独立 Remit 类型，要么让 derive 插件为同一类型生成多个 ordinal；不要悄悄把
多个模板塞进一个 `Type` 而不改契约。

### 2.2 Cloze deletion 的语义、ordinal 与普通模板的差异

Anki 的 cloze 是 note type 的特殊种类，不是普通模板的一个过滤器那么简单：

- 用户在 Text 字段里选中文字，点 `[...]`，得到 `{{c1::1913}}`；
- `c1` 是用户可见的 1-based cloze 编号；
- 不同的 `cN` 各生成一张 card；
- 同一段文字里重复使用同一个 `cN`，只生成一张 card，并把该编号的所有位置一起
  遮住；
- 可以写 `{{c1::Canberra::city}}`，第二个 `::` 后面是 hint；
- 支持嵌套 cloze（有限层数）和新语法 `{{c1,2,3,4::feminine}}`（同一段文字在
  四张卡里分别被遮住）；
- Cloze note type 只有一套模板，不能像普通 note type 那样添加额外 card template；
  默认还有 `Extra` 字段，显示在答案面。

来源：[Cloze Deletion](https://docs.ankiweb.net/manual/editing#cloze-deletion)、
[Card Generation · Cloze Templates](https://docs.ankiweb.net/manual/templates/generation#cloze-templates)。

Anki 源码把用户编号转成内部 ordinal：`cloze_ord.saturating_sub(1).min(499)`，
也就是 `c1 → ord 0`、`c2 → ord 1`……；cloze note type 永远取第一套模板。
来源：[cardgen.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/cardgen.rs)、
[notetype/mod.rs · get_template](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/mod.rs)、
[cloze.rs](https://github.com/ankitects/anki/blob/main/rslib/src/cloze.rs)。

Remit 的 cloze 是另一种设计：

- 文本里写 `{{blank}}`，不是 `{{c1::blank}}`；
- 每个 `{{...}}` 出现就是一张卡，ordinal 按出现顺序 0-based；
- 一张卡只遮一个空；没有“同号多空合并成一张卡”；
- 没有 per-blank hint 语法；
- 不支持嵌套/重叠/`c1,2,3`；
- `remit.cloze` 只有 `Text` 字段；`Extra` 之类的附加信息要靠别的 Card Type 或
  `::: type` 字段块；
- 卡片模板的 `{{cloze:Text}}` 在 front 遮当前 ordinal、back 全揭示；源文本里的
  `{{...}}` 由内置 derive 插件识别。

来源：`internal/ext/builtin/cards/main.js`、
`internal/core/template/template.go`、`docs/adr/0008-card-types-templates.md`。

| 场景 | Anki | Remit | 判定 |
|---|---|---|---|
| 一句话一个空 | `{{c1::Paris}}` → 1 张卡 | `{{Paris}}` → 1 张卡 | **核心 / 已覆盖** |
| 一句话多个不同空 | `{{c1::A}} ... {{c2::B}}` → 2 张卡 | 两个 `{{...}}` → 2 张卡 | **核心 / 已覆盖** |
| 同一个 `cN` 出现在多处 | 1 张卡，遮住所有同号位置 | 每个 `{{...}}` 各一张卡 | **不采纳原生语义；导入需扁平化** |
| cloze hint `{{c1::X::hint}}` | front 显示 `[hint]` | 无 per-blank hint；导入会剥掉 hint | **不采纳原生语义；导入需报告** |
| 嵌套/部分重叠 / `c1,2,3,4` | 支持（有限） | 不支持 | **不采纳** |
| 空 cloze / 删除后的空卡 | 需 Tools → Empty Cards 清理 | 卡片不再被派生时会 suspend，历史保留 | **已覆盖且更安全**：`reconcileCards` 把不再被问到的卡置为 suspended，不删除 |
| Cloze 的 Extra 字段 | note type 的第二字段 | 用自定义 Card Type 的字段或字段块 | **可选 / 需补齐**（若要求 Anki 级布局） |

**ordinal 的关键区别**：Anki 的 `ord` 在普通 note type 里是**模板下标**，在 cloze
note type 里是**cloze 编号减一**；Remit 的 `ordinal` 统一表示“这个 Block 派生出的
第几张卡”，既用于双向卡方向，也用于 cloze 空位。两者都是 0-based，但语义空间
不同，迁移时不能只搬数字。

### 2.3 deck / collection / browser / stats / media / add-on：各自解决什么

来源：[Getting Started · Decks / Collection](https://docs.ankiweb.net/manual/getting-started)、
[Deck Options](https://docs.ankiweb.net/manual/deck-options)、
[Browsing](https://docs.ankiweb.net/manual/browsing)、
[Card Info, Graphs and Statistics](https://docs.ankiweb.net/manual/stats)、
[Media](https://docs.ankiweb.net/manual/media)、
[Add-ons](https://docs.ankiweb.net/manual/addons)、
[Managing Files and Your Collection](https://docs.ankiweb.net/manual/files)、
[Searching](https://docs.ankiweb.net/manual/searching)。

| Anki 东西 | 解决的问题 | 关键事实 | Remit 替代与判定 |
|---|---|---|---|
| **Deck** | 把 card 分组，按组复习；可设每 deck 的每日限额、学习步骤、调度参数；deck 可树状嵌套（`::`） | 一张 card 属于一个 deck；同一条 note 的不同 card 可进不同 deck；deck 选项可共享 preset；notes/note types 不属于 deck | **不采纳 deck 容器**。**Review Scope 已覆盖“本次复习范围”**（全库/笔记/标签，会话内不落库；`internal/core/queue/queue.go`）；按范围不同调度参数是可选能力，不是 deck 模型 |
| **Collection** | 装下所有 notes、cards、decks、note types、deck options | 桌面端是 `collection.anki2`（SQLite）+ `collection.media/` + `prefs.db`；不要在 Anki 打开时复制/移动 | **已覆盖“全部材料”语义**为明文库（notes/cards/types/assets/reviews.jsonl，见 ADR 0003）；**不采纳单文件 SQLite 形态** |
| **Browser** | 全局搜索、查看、编辑 cards/notes；可切换 Cards/Notes 表模式 | 支持搜索语法、侧栏、批量操作、编辑区 | **已覆盖基本能力**：Remit 有 Browse cards、搜索、过滤、打开（`internal/ui/screens/browse`）；批量编辑/保存搜索是**可选/需补齐** |
| **Stats** | 看复习量、到期预测、保留率、card info、revlog | 数据源是 revlog；图表是视图，不是调度真相 | **已覆盖基本能力**：Remit stats 有热力图、保留率、forecast（`internal/ui/screens/stats`）；更深图表**可选** |
| **Media** | 存图片/音频/视频，检查缺失/未使用文件，随 collection 备份 | 文件在 `collection.media/`；支持 mp3/mp4 等；模板不扫描媒体引用 | **图片 Asset 已覆盖**（内容寻址、随库走；`internal/core/store/assets.go`）；音频/视频/PDF **需补齐**；模板媒体引用不采纳 |
| **Add-on** | 用 Python 扩展 Anki 的功能，可改 UI、调度、导入等 | Anki 启动时加载 Python 模块；可修改任意代码；升级 Anki 常破坏旧 add-on | **不采纳该形态**；Remit 的能力插件（manifest + hook + capability、exec/js、宿主绘制）**已覆盖扩展诉求** |
| **Tags** | note 级分类，支持层级/搜索 | tag 作用于 note，sibling cards 共享；一张 card 只能有一个 flag | Remit tag 是 **Block 级**；**已覆盖基本组织**，但作用粒度不同；flags **不采纳** |
| **Deck options / presets** | 每 deck 的调度策略、每日限额、新卡顺序 | deck options 不回改历史；preset 可共享 | Remit 用库级 FSRS + 插件 policy；**不采纳 per-deck preset**；按 scope 的参数是可选 |
| **Search** | 用查询语言找 card/note | `deck:`、`tag:`、`is:due`、字段限定等 | Remit 有子串搜索；**已覆盖基本查找**；Anki 查询语言**可选/需补齐** |
| **Shared decks / AnkiWeb sync** | 分发和跨设备同步 | 云/包分发层 | **不采纳为核心**；导入 `.apkg` 是迁移，不是运行时依赖 |

核心判断：Anki 的 **deck 是“持久容器 + 调度参数”**，而 Remit 的 Review Scope 是
**“一次会话的过滤范围”**。两者只在一部分需求上重叠：想“今天只复习某篇笔记/
某个标签”时，Review Scope 已覆盖；想“这个 deck 永久独立、每天限额不同、参数
不同”时，Remit 有意不提供 deck，应该由库级设置或插件策略承担，而不是把 card
塞进一个容器。

Collection、Browser、Stats、Media、Add-on 也是同一逻辑：**数据/索引是核心，
容器和视图是宿主能力，第三方代码是受限扩展层**。Remit 已经有 library、browse、
stats、Asset、plugin，不需要照搬 Anki 的文件形态或 Python API。

### 2.4 从“读到一句话”到“生成一张可复习卡”的最短操作链

Anki 官方的最短链不是“在笔记里读到一句话”，而是从 Add 窗口开始：

1. 点 **Add**；
2. 选 note type（左上）和 deck（右上）；
3. 填写 Front/Back 等字段；
4. 点 **Add** 或 `Ctrl/Cmd+Enter`；Anki 自动生成 card 并放进 deck。

如果做 cloze：选 Cloze note type → 在 Text 里输入句子 → 选中要遮住的文字 →
点 `[...]` → Add。Anki 根据 `cN` 自动生成多张卡。
来源：[Adding Cards and Notes](https://docs.ankiweb.net/manual/editing#adding-cards-and-notes)、
[Cloze Deletion](https://docs.ankiweb.net/manual/editing#cloze-deletion)。

Remit 的最短链更短，而且发生在笔记里：

1. 在笔记段落里写 `提示 :: 答案`（或 `{{空}}`、`:::`、`::: type <id>` 字段块）；
2. 保存笔记。`reconcileCards` 调用 derive 插件，发现该 Block 的新 card 并写入。

或者把光标放在段落里，运行 **Make card** 命令。来源：
`internal/ui/cards.go`（`reconcileCards`、`makeCard`）、
`internal/ext/builtin/cards/main.js`。

| 交互 | Anki | Remit | 判断 |
|---|---|---|---|
| 制卡入口 | 独立 Add 窗口/表单 | 笔记内联标记 + 保存；或 Make card 命令 | **Remit 更短、更贴近阅读上下文；保留** |
| 选类型 | 必须先选 note type | `::` 默认 basic；`:::` 双向；`{{...}}` cloze；`::: type` 显式指定 | **Remit 的隐式默认 + 显式字段块更好** |
| 选 deck | 必须选 deck（或默认） | 无 deck；Review Scope 在复习时选范围 | **不采纳强制 deck 选择** |
| cloze 操作 | 选中文字点 `[...]` | 手写 `{{...}}` | **可借鉴**：给编辑器加“选中文字 → 包成空”的手势；核心语法不变 |
| 模板预览 | Cards 窗口有 front/back 预览 | 类型编辑页已有同引擎的 front/back 预览（`internal/ui/screens/settings/types_preview.go`） | **已覆盖**：不必再借 |
| 空卡处理 | Empty Cards 工具；新 note 可能建空白卡 | 不再被派生就 suspend，历史保留 | **Remit 更好**：不要引入“空白卡”历史包袱 |
| 多模板/ordinal | note type 内多套模板 | 一个类型一对模板，插件派生多卡 | **不采纳 Anki 的多模板层级**；用 derive 表达 |

**值得借鉴**：选中文字生成 cloze 的手势、“哪些字段该拆开”的字段设计引导
（Anki 手册明确建议把音频/图片/提示拆成字段，
而不是塞进一个 Front）。**历史包袱**：强制 deck、独立 Add 表单、note type 内多
模板的 ordinal 管理、HTML/CSS/JS 模板、add-on monkey-patching、cloze 的 `cN`
分组/嵌套/空卡清理。来源：
[Adding a Note Type](https://docs.ankiweb.net/manual/editing#adding-a-note-type)、
[Using Decks Appropriately](https://docs.ankiweb.net/manual/editing#using-decks-appropriately)。

### 2.5 与 ADR 0008 及 `::` / `{{...}}` / `::: type` 的映射

#### 模型映射

| Remit（ADR 0008） | Obsidian 类比 | Anki 类比 | 边界 |
|---|---|---|---|
| Note = 有序 Block 的 Markdown 文档 | Obsidian note（Markdown 文件） | **没有直接类比**；Anki note 是字段记录 | 不要把 Anki note 直接叫 Remit Note 的等价物 |
| Block = 内联 ID 的可寻址单位 | Obsidian block（`^id`） | Anki note/field 的“卡源”部分 | Remit 的 Block 是 card 的唯一起点；Anki 的起点是 note+模板 |
| Card = 引用一个 Block 的调度实体 | 无（Obsidian 不调度） | Anki card | 两者都有独立历史；来源粒度不同 |
| Card Type = 字段表 + 一对正/反模板 | 无 | Anki note type（字段+模板），但 Anki 可多套 card type | Remit 只有一对模板；多模板要靠插件派生 |
| Field = 具名标量，默认现算 | Obsidian property（但不是同一模型） | Anki field（值存 note） | Remit 默认不落库；Anki 全量落库 |
| Template = 纯数据方言，宿主渲染 | 无 | Anki front/back HTML 模板 | 字段替换相近；HTML/CSS/JS/过滤器不采纳 |
| Asset = 内容寻址图片 | Obsidian attachment 的图片子集 | Anki media 的图片子集 | 音频/视频/PDF 是缺口 |
| Review Scope = 会话内取卡范围 | 无 | Anki deck / filtered deck 的“范围”部分 | Remit 不落库、不做持久容器 |

#### 语法映射

| Remit 语法 | 语义 | Obsidian 里是什么 | Anki 对应 | 判定 |
|---|---|---|---|---|
| `提示 :: 答案` | `remit.basic`，Front/Back | 普通文本 | Basic note type 的两个字段 | **核心 / 已覆盖** |
| `提示 ::: 答案` | `remit.bidirectional`，ordinal 0/1 交换 | 普通文本 | Basic (and reversed card) 的两张卡 | **核心 / 已覆盖**；注意 `:::` 必须先于 `::` 判断 |
| `{{空}}` | `remit.cloze`，每个空一张卡，0-based | 普通文本（可能和模板语法混淆） | Cloze 的简单子集 | **核心 / 已覆盖**；同号分组/hint/嵌套不支持 |
| `::: type <typeId>` + `Field: value` 行 + `:::` | 任意 Card Type 的字段块 | 普通文本/代码块 | Add 窗口里选 note type 并填字段 | **核心 / 已覆盖**；未知字段丢弃并记日志 |
| `[[Note]]` / `[[Note#^block_id]]` | 笔记间链接/引用 | `[[wikilink]]` / 块引用 | 无 | **核心导航 / 基本已覆盖；alias/heading/路径需补齐** |
| `#tag` | Block 级标签 | 行内 tag / YAML tags | note 级 tag | **核心组织 / 已覆盖基础；层级/大小写/YAML 需补齐** |
| `![](assets/<sha>.<ext>)` | 库内图片 Asset | Markdown 图片 / `![[image]]` | `<img src>` / media | **核心图片 / 已覆盖存储；导入重写需补齐** |
| `{{Field}}` | 模板字段替换 | 无 | `{{Field}}` | **已覆盖** |
| `{{cloze:Text}}` | 模板按 ordinal 遮空 | 无 | `{{cloze:Text}}` | **已覆盖简单语义** |
| `{{hint:Field}}` / `{{type:Field}}` | 捕获 hint / 键入答案 | 无 | `{{hint:}}` / `{{type:}}` | **已覆盖捕获与复习屏（代码现状）；过滤器仍空** |
| `{{Field\|filter}}` | 预留过滤器 | 无 | `text:`/`furigana:`/`tts:` 等 | **不采纳 Anki 过滤器生态；Remit 预留但注册表为空** |

ADR 0008 的边界要再说清楚：

- **字段默认不持久化**：卡片只存 `type/ordinal/producer`，字段从 Block（live）或
  快照（frozen）现算；只有用户编辑过的字段存为覆盖。因此 Anki 那种“改 note 字段，
  所有 sibling 卡一起变”的行为，在 Remit 里是“改 Block，所有 live 卡一起变；
  某张卡编辑过该字段后，它就不再跟随”。
- **模板是用户可编辑的库数据**，但方言是纯数据，不允许 HTML/CSS/脚本。Anki 模板
  的 HTML 布局、CSS 样式、特殊字段和过滤器不能原样进入。
- **一个类型只有一对模板**。Anki note type 的多 card type 不能塞进 `Type.Front/
  Back`；要么拆类型，要么让 derive 插件产出多个 `(typeId, ordinal)`。

### 2.6 当前实现的缺口（按优先级）

| 优先级 | 缺口 | 现状 | 建议 |
|---|---|---|---|
| P0 | Obsidian frontmatter 导致标题/首块错位 | `ParseNoteSource` 只认第一行 `# `；导入回退文件名 | 导入前剥离 frontmatter、记录原文、提取 tags/aliases；不要让 YAML 混进 Block |
| P0 | Obsidian 链接语义覆盖不足 | 只解析 `[[标题]]`；alias/heading/路径/Markdown 链接不索引 | 明确支持子集并生成“未解析链接”报告；或扩展 `Links` 解析 |
| P0 | Obsidian 块 ID 不能复用 | Remit 只认 `^block_...`，Obsidian 是 `^37066d`/自定义短横线 ID | 导入时为被引块铸造 Remit ID 并重写链接；导出时反向映射 |
| P0 | Obsidian 附件路径/类型 | 只导入 `assets/` 子树，图片按哈希存，但不重写引用 | 读附件配置、收图片、重写为 `assets/<sha>.<ext>`；音频/PDF 明确列为不支持 |
| P0 | Obsidian vault 的忽略规则 | 递归收所有 `*.md`，包括 `.obsidian`、`.trash` | 导入器默认跳过隐藏目录与回收站，或让用户显式选择要导入的子树 |
| P1 | Obsidian tags 语义 | 行内、大小写敏感、无 `/`；YAML tags 不读 | 至少做大小写归一/层级保留策略；导入报告冲突 |
| P1 | Anki 多 card type | Remit 类型只有一对模板 | 保持“一类型一模板 + derive 多卡”；文档明确不承诺 Anki 模板 1:1 |
| P1 | 复杂 cloze 导入 | Anki 同号分组/hint/嵌套在 Remit 无原生语义 | 导入为 basic override 并警告；不要静默伪装成原生 cloze |
| P1 | Anki 媒体字节 | 只保留 `[img: name]`/`[sound: name]` 文本，不复制 media | 与 Asset 结合，至少图片；音频/视频列出损失 |
| P2 | 表格编辑 | 阅读面可渲染 GFM；richtext 编辑器降级为 tab 段落 | 宿主自建表格块或等 MyGo 表格节点；见既有研究 |
| P2 | Graph / 深 stats / 批量编辑 | 无 graph；stats 基础；browse 基础 | 可选视图，不进模型 |

## 3. 最终判断：什么进核心，什么留在边界

### Obsidian

**进核心**：

- 纯文本 Markdown 文档、文件夹式库、附件作为库资产；
- 简单 `[[标题]]` 链接与由文本派生的 backlinks；
- 行内 tags（作为 Block 级范围）；
- GFM 表格/代码/数学/图片的阅读与保存；
- 迁移时对 frontmatter、alias、heading、路径、块 ID、embed、附件的显式处理。

**可选**：

- callout 的样式化渲染；
- graph view；
- properties 的只读展示或提取；
- 音频/视频/PDF 的浏览；
- Canvas/Bases 的独立导入。

**不采纳**：

- `.obsidian` 作为库内容；
- 任意代码插件 API 与社区插件信任模型；
- Sync/Publish 云服务；
- graph 作为持久数据；
- 主题/CSS/Publish 属性进入核心模型；
- note/block transclusion 作为核心渲染语义。

### Anki

**进核心**：

- note/note type 的“字段 + 模板 → 多张 card”生成思想；
- card 独立调度与复习历史；
- 简单 cloze（每空一张）；
- 图片 media 作为 Asset；
- 浏览器式检索与统计作为宿主视图；
- 导入时保留评分历史并明确降级。

**可选**：

- 多 card type 的完整布局；
- 批量编辑、保存搜索、更深统计；
- 音频/视频 media；
- 按范围不同的调度参数。

**不采纳**：

- deck 作为 card 的持久容器；
- 单文件 SQLite collection；
- HTML/CSS/JS 模板与 Anki 过滤器生态；
- Python add-on / monkey-patching；
- cloze 同号分组、per-blank hint、嵌套/部分重叠；
- 强制 Add 表单与 deck 选择；
- AnkiWeb/shared deck 作为运行时依赖。

## 4. 开放问题与未能验证的点

1. **本报告没有找到 Obsidian 的可执行、版本化方言规范**。官方帮助逐项列支持面，
   但没有找到 grammar/conformance suite；本报告只能按官方页面列举，无法保证
   所有边角解析一致。若要做迁移器，需要自建测试语料。
2. **Obsidian 块 ID 对下划线的实际接受度**：官方说只能是拉丁字母、数字、短横线；
   Remit 的 `^block_...` 含下划线。文档层面可判定为不兼容，但没有在 Obsidian
   应用里实测“下划线是否真的被拒绝”。导出器应按文档生成合法 ID。
3. **Obsidian 附件/相对路径的精确解析**：官方文档区分 vault 内部链接、Markdown
   链接、附件位置设置，但没有给出所有组合的解析优先级；迁移器应以实测补齐。
4. **Remit 内联语法的歧义**：`::`/`:::`/`{{...}}` 目前会在整段文本上匹配，
   没有看到对代码块/行内代码的跳过规则；正文里的 `C++::` 或代码里的 `{{...}}`
   可能被误判成制卡。这是产品设计问题，不是外部事实，但会直接影响“读到一句话”
   的体验。来源：`internal/ext/builtin/cards/main.js`。
5. **Remit 标题链接与 Obsidian 路径链接的冲突**：Remit 按归一化标题匹配，不保留
   文件夹路径；Obsidian 允许同名文件在不同目录，并用路径/最短唯一路径区分。
   迁移器需要定义“同名多笔记”的策略（拒绝、加后缀、保留路径为标题的一部分）。
6. **Anki 新格式导入**：Remit 当前 `importanki` 只读 legacy schema 11
   （`collection.anki2`/`collection.anki21`），拒绝 zstd 压缩的
   `collection.anki21b` 和新 schema；若用户用最新 Anki 导出，可能无法直接导入。
   来源：`internal/core/importanki/doc.go`。
7. **Review Scope 是否需要“范围级调度参数”**：GLOSSARY 明确不引入 deck；如果
   用户需要“不同范围不同每日限额/保留率”，那是宿主设置或插件 policy 的新能力，
   不是把 deck 复活。当前没有 ADR 决定这一点。

## 来源清单

### Obsidian 官方

- [Obsidian Flavored Markdown](https://obsidian.md/help/obsidian-flavored-markdown)
- [Basic formatting syntax](https://obsidian.md/help/syntax)
- [Advanced formatting syntax](https://obsidian.md/help/advanced-syntax)
- [Callouts](https://obsidian.md/help/callouts)
- [Properties](https://obsidian.md/help/properties)
- [Tags](https://obsidian.md/help/tags)
- [Internal links](https://obsidian.md/help/links)
- [Embed files](https://obsidian.md/help/embeds)
- [Attachments](https://obsidian.md/help/attachments)
- [Accepted file formats](https://obsidian.md/help/file-formats)
- [Backlinks](https://obsidian.md/help/plugins/backlinks)
- [Graph view](https://obsidian.md/help/plugins/graph)
- [How Obsidian stores data](https://obsidian.md/help/data-storage)
- [Configuration folder](https://obsidian.md/help/configuration-folder)
- [Manage vaults](https://obsidian.md/help/manage-vaults)
- [Settings · Files and links](https://obsidian.md/help/settings)
- [Community plugins](https://obsidian.md/help/community-plugins)
- [Plugin security](https://obsidian.md/help/plugin-security)
- [Introduction to Obsidian Sync](https://obsidian.md/help/sync)
- [Local and remote vaults](https://obsidian.md/help/sync/vault-types)
- [Sync settings and selective syncing](https://obsidian.md/help/sync/settings)
- [Introduction to Obsidian Publish](https://obsidian.md/help/publish)
- [Publish limitations](https://obsidian.md/help/publish/limitations)
- [Publish security and privacy](https://obsidian.md/help/publish/security)
- [Import Markdown files](https://obsidian.md/help/import/markdown)
- [Glossary](https://obsidian.md/help/glossary)
- [Build a plugin](https://docs.obsidian.md/Plugins/Getting+started/Build+a+plugin)
- [Anatomy of a plugin](https://docs.obsidian.md/Plugins/Getting+started/Anatomy+of+a+plugin)
- [Vault](https://docs.obsidian.md/Plugins/Vault)
- [Markdown post processing](https://docs.obsidian.md/Plugins/Editor/Markdown+post+processing)
- [Events](https://docs.obsidian.md/Plugins/Events)
- [Manifest](https://docs.obsidian.md/Reference/Manifest)
- [MetadataCache](https://docs.obsidian.md/Reference/TypeScript+API/MetadataCache)
- [obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)

### Anki 官方

- [Getting Started](https://docs.ankiweb.net/manual/getting-started)
- [Adding/Editing](https://docs.ankiweb.net/manual/editing)
- [Card Templates](https://docs.ankiweb.net/manual/templates/intro)
- [Field Replacements](https://docs.ankiweb.net/manual/templates/fields)
- [Card Generation](https://docs.ankiweb.net/manual/templates/generation)
- [Styling & HTML](https://docs.ankiweb.net/manual/templates/styling)
- [Deck Options](https://docs.ankiweb.net/manual/deck-options)
- [Browsing](https://docs.ankiweb.net/manual/browsing)
- [Card Info, Graphs and Statistics](https://docs.ankiweb.net/manual/stats)
- [Media](https://docs.ankiweb.net/manual/media)
- [Add-ons](https://docs.ankiweb.net/manual/addons)
- [Add-on Introduction](https://docs.ankiweb.net/addons/intro)
- [Managing Files and Your Collection](https://docs.ankiweb.net/manual/files)
- [Searching](https://docs.ankiweb.net/manual/searching)
- [schema11.sql](https://github.com/ankitects/anki/blob/main/rslib/src/storage/schema11.sql)
- [notetype/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/mod.rs)
- [notetype/cardgen.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/cardgen.rs)
- [notetype/fields.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/fields.rs)
- [notetype/templates.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/templates.rs)
- [card/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/card/mod.rs)
- [cloze.rs](https://github.com/ankitects/anki/blob/main/rslib/src/cloze.rs)
- [notes/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notes/mod.rs)

### Remit 内部依据

- `GLOSSARY.md`
- `docs/adr/0002-content-model.md`
- `docs/adr/0003-storage-layout.md`
- `docs/adr/0006-extension-substrate.md`
- `docs/adr/0007-plugin-system.md`
- `docs/adr/0008-card-types-templates.md`
- `docs/specs/plugins/system.md`
- `docs/research/richtext-tables-and-images.md`
- `internal/core/model.go`、`internal/core/types.go`、`internal/core/links.go`、
  `internal/core/tags.go`
- `internal/core/template/template.go`
- `internal/core/queue/queue.go`
- `internal/core/store/note.go`、`internal/core/store/assets.go`
- `internal/core/importmd/import.go`、`internal/core/importmd/assets.go`
- `internal/core/importanki/doc.go`
- `internal/cards/resolver.go`
- `internal/ext/builtin/cards/main.js`
- `internal/ui/cards.go`
