# 文章捕获与 AI 转换：HTML/EPUB/web → Markdown + Block + Asset

对应 [#204](https://github.com/transmit-bug/remit/issues/204)。基线：
`bad1b778eef862400caeab8f3b5e13422dc814d3`（2026-11-08 的 `origin/master`）。
本文只做事实基线与方案判断，不改产品实现。

## 结论摘要

**推荐：确定性的宿主侧导入管线，AI 只做显式开启的后处理。**

1. 网页/HTML 先解析成 DOM，做正文抽取、清洗、语义归一化，最后才序列化为
   Markdown；不要先让模型“整页转 Markdown”。
2. 推荐栈：`x/net/html`（HTML5 解析）+ Readeck 维护的
   `codeberg.org/readeck/go-readability/v2`（正文与元数据）+
   `bluemonday`（allowlist 清洗）+ `html-to-markdown/v2`（Markdown 序列化，
   含 table 插件）。四者分别是 BSD-3、MIT、BSD-3、MIT，符合本仓库的
   AGPL-3.0 依赖规则。
3. EPUB 是 OCF ZIP：`META-INF/container.xml` → OPF 包文档 →
   `manifest`/`spine` → XHTML。用标准库 `archive/zip`、`encoding/xml`
   和同一条 HTML→Markdown 管线即可，不必引入 EPUB 框架。
4. 浏览器剪藏不内置浏览器。推荐让用户在浏览器里用 SingleFile 存成单个
   HTML 文件，再由 Remit 离线导入；这样动态页面、图片和样式由浏览器侧解决，
   Remit 不承担 Chromium 的更新与攻击面。
5. URL 直接抓取是第二增量：它需要网络，而且没有 JS，动态站点会失败；
   失败时应明确提示“请用浏览器保存为单文件 HTML”，不要偷偷降级到 AI。
6. 来源元数据作为笔记末尾的**普通文本块**保存；不新增文章 JSON、文章表或
   第二套正文格式。块的 ID 由 `SaveNote` 一次性铸造，转换器绝不生成
   `^block_…`，也绝不从 URL、标题、正文哈希推导 ID。
7. AI 可以做标题/摘要建议、结构修复候选、翻译、公式/表格修复候选；
   它不能成为唯一转换器、不能直接写库、不能生成块 ID。所有 AI 输出先通过
   JSON Schema 和确定性不变量校验，失败就保留确定性结果。
8. 最小可行切片是“单个 HTML 文件（含 SingleFile 产物）→ 一篇普通笔记 +
   assets”，不做 URL、AI、EPUB、PDF。EPUB 和 URL 复用同一内核，是后续增量。

## 1. Remit 今天能承接什么

### 1.1 Note 与 Block 的硬约束

事实：

- `Note` 只有 `ID`、`Title`、`Blocks`、`Updated`，没有文章类型、HTML 正文或
  独立的来源字段（[`internal/core/model.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/model.go#L14-L30)）。
- 块是引用单位；ID 是 `kind_` + 128 位随机数的十六进制，铸造一次，永不从路径、
  标题或位置推导（[`internal/core/id.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/id.go#L9-L21)）。
- 注意文件是 H1 标题 + 空行分隔的块；块尾部写 ` ^block_<id>`。`SaveNote`
  给没有 ID 的块铸 ID，并明确要求“块的文本不能含空行”（[`internal/core/store/library.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/library.go#L761-L787)）。
- 解析器遇到任何空行就结束当前块；它不是 Markdown 围栏感知的
  （[`internal/core/store/note.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/note.go#L31-L72)）。
- 资产的真相是 `assets/<sha256>.<ext>`，按内容寻址，笔记用相对路径引用；
  同一字节只存一份（[`internal/core/store/assets.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/assets.go#L13-L53)）。
- 库的真相是 notes、cards、reviews、`remit.json`、types、plugins-data 和
  assets；`.remit/` 永远可删重建（[ADR 0003](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/adr/0003-storage-layout.md#L24-L48)）。

判断：文章捕获的产物必须是“`core.Note` + 若干 `core.Block` + `core.Asset`”，
而不是另一个 `Article` 结构。转换器只需要交付标题、块文本、资产字节和来源行；
ID 与文件落盘由 store 负责。

### 1.2 现有导入路径的边界

事实：

- `importmd` 只接受 `.md`/`.markdown`，把第一个 H1 当标题；
  它不解析 HTML 或 EPUB（[`internal/core/importmd/import.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/importmd/import.go#L19-L28)、[`markdownFiles`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/importmd/import.go#L143-L165)）。
- 插件 importer 通过 `importFile` 收到 `{name, data(base64), format}`，
  只能返回 `notes: [{title, blocks}]`、`cards` 和 `warnings`
  （[`remit.d.ts`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/remit.d.ts#L394-L413)）。
- 插件没有 `fs`、`net`、UI 或任意 SQL；联网能力是 v2 议题
  （[插件系统 §6](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/specs/plugins/system.md#L191-L214)）。
- 宿主的 `suggest.Apply` 只写 notes/cards，没有资产建议通道
  （[`internal/core/suggest/suggest.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/suggest/suggest.go#L38-L43)、[`applyNote`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/suggest/suggest.go#L127-L142)）。

判断：**今天的插件 importer 契约不足以承接带图片的文章。** 它不是“插件不能做”，
而是“插件只能在 1 秒/64 MiB 内把本地字节变成无资产的 notes/cards”
（[插件系统 §8](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/specs/plugins/system.md#L222-L236)）。带网络抓取、
图片落盘和 EPUB 资源映射的文章捕获应是宿主能力，像 `importmd`/`importanki`
一样由宿主实现；第三方格式仍可继续走插件 importer。若未来要让插件做完整文章导入，
需要新增“资产建议”契约（字节/媒体类型/来源）并重新评估时间与内存上限，不应在
本票偷偷扩展。

### 1.3 笔记语法会与文章正文碰撞

这是本票最重要的、容易被忽略的约束。导入的文章不是惰性文本：

- 任何包含 `::` 的段落会被内置制卡插件解释为 basic 卡片；`:::` 是双向卡，
  `{{…}}` 是 cloze（[`internal/ext/builtin/cards/main.js`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/ext/builtin/cards/main.js#L79-L97)）。
- 文档保存后会执行 `reconcileCards`，自动创建段落“要求”的卡片
  （[`internal/ui/cards.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/ui/cards.go#L38-L100)、[`internal/ui/notes.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/ui/notes.go#L83-L94)）。
- 因此，导入 C++ 的 `std::vector`、YAML/配置里的 `::`、代码里的 `{{…}}`，
  在用户下一次编辑并保存时可能变成卡片。文章捕获若直接调用 `saveDocument`，
  甚至会在导入时就制卡。
- `#template` 已经是代码感知的：围栏代码块和行内代码里的 `#template` 不算标签
  （[`internal/core/tags.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/tags.go#L68-L88)）。
  但 `::`、`:::`、`{{…}}` 的扫描没有同等的 Markdown 代码感知
  （[`internal/ext/builtin/cards/main.js`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/ext/builtin/cards/main.js#L79-L97)）；
  wiki link 的扫描也没有（[`internal/core/links.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/links.go#L27-L57)）。

判断：

1. 导入路径必须像 `importmd`/`suggest` 一样直接 `SaveNote`，**不能**调用
   `saveDocument` 或 `reconcileCards`。
2. 在宣称“代码可保真”之前，制卡插件的 `deriveBlock` 必须先忽略围栏代码块和
   行内代码里的卡片标记；否则 `std::vector` 仍会在下一次保存时变成卡片。
   这是文章捕获的依赖，不是可选的润色。
3. 代码块里的 `[[…]]` 仍可能被 `core.Links` 当成 wiki link。要么让链接扫描也
   做代码感知，要么接受“代码里的双链会产生反链”的已知缺陷并在导入报告中警告。

### 1.4 下游编辑器会折损表格

事实：阅读面 `ui/markdown` 的默认扩展是 GFM + 脚注 + 数学
（[MyGo parser](https://github.com/egoist/mygo/blob/v0.8.1/ui/markdown/parser/document.go#L23-L38)）；编辑面
`richtext.ParseMarkdown` 会把表格按行降级为 tab 分隔的段落，列对齐和跨行列丢失，
再次保存后无法恢复成表格（[`docs/research/richtext-tables-and-images.md`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/research/richtext-tables-and-images.md#L9-L19)、[§1 后果](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/research/richtext-tables-and-images.md#L202-L210)）。

判断：导入管线可以正确保存 GFM 表格，但“用户打开编辑器改一个字再保存”仍会破坏
表格。文章捕获不应为此新建文章格式；应把表格保真度标为“阅读态保真、编辑态当前
降级”，并在编辑器修复前避免宣称完整表格编辑。

## 2. 工具、能力与许可证

本仓库是 AGPL-3.0；工程规约允许 MIT/BSD/Apache-2.0，不接受 GPL-2-only、
专有或不明许可证（[`docs/engineering/conventions.md`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/engineering/conventions.md#L71-L79)）。
下表是按这条规则做的初筛，不是法律意见；真正分发前仍需复核。

| 工具 | 角色 | 许可证 | 离线 | 结论 |
|---|---|---|---|---|
| Remit | 宿主 | AGPL-3.0（[`LICENSE`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/LICENSE)） | 是 | 基线 |
| Readeck `go-readability/v2` | 网页正文与元数据抽取 | MIT（[README](https://codeberg.org/readeck/go-readability/src/branch/v2/README.md)、[LICENSE](https://codeberg.org/readeck/go-readability/src/branch/v2/LICENSE)） | 是 | 推荐；原 `go-shiori` 已归档并标注 deprecated（[README](https://github.com/go-shiori/go-readability#status)） |
| `html-to-markdown/v2` | HTML → CommonMark/GFM | MIT（[README](https://github.com/JohannesKaufmann/html-to-markdown#readme)、[LICENSE](https://github.com/JohannesKaufmann/html-to-markdown/blob/main/LICENSE)） | 是 | 推荐；table 插件处理对齐与单元格 |
| `bluemonday` | HTML allowlist 清洗 | BSD-3-Clause（[README](https://github.com/microcosm-cc/bluemonday#readme)、[LICENSE](https://github.com/microcosm-cc/bluemonday/blob/main/LICENSE.md)） | 是 | 推荐；Readability 明确说自己不是 sanitizer |
| `golang.org/x/net/html` | HTML5 解析 | BSD-3-Clause（[包文档](https://pkg.go.dev/golang.org/x/net/html)、[LICENSE](https://github.com/golang/net/blob/master/LICENSE)） | 是 | 已作为间接依赖存在 |
| Pandoc | HTML/EPUB/PDF → Markdown | GPL-2.0-or-later（[COPYRIGHT](https://github.com/jgm/pandoc/blob/main/COPYRIGHT)） | 是 | 可选外部工具，不作默认嵌入依赖 |
| Trafilatura | 正文抽取，输出 Markdown | Apache-2.0（[README](https://github.com/adbar/trafilatura#readme)、[LICENSE](https://github.com/adbar/trafilatura/blob/master/LICENSE)） | 是 | 备选；需要 Python 运行时 |
| SingleFile | 浏览器侧保存完整页面 | AGPL-3.0（[README](https://github.com/gildas-lormeau/SingleFile#readme)、[LICENSE](https://github.com/gildas-lormeau/SingleFile/blob/main/LICENSE)） | 浏览器侧需要网络，Remit 侧离线 | 推荐作为外部剪藏工具，不作为内置浏览器 |
| `github.com/ledongthuc/pdf` | PDF 纯文本读取 | BSD-3-Clause（[README](https://github.com/ledongthuc/pdf#readme)、[LICENSE](https://github.com/ledongthuc/pdf/blob/master/LICENSE)） | 是 | 只能拿纯文本，不能作为文章级 PDF 方案 |

## 3. 推荐管线：确定性优先

### 3.1 总览

```
输入
  ├─ .html / SingleFile HTML
  ├─ .epub
  ├─ URL（显式网络动作，后续）
  └─ 插件 importer（仅本地字节、无资产，保持现状）
        │
        ▼
解码/解析 DOM → 去脚本样式 → 正文抽取 → 语义 IR
        → 资产物化（PutAsset）→ Markdown 序列化
        → 块归一化 → SaveNote（铸 ID）→ 末尾 Source 块
        → 可选 AI 后处理（显式同意、结构化输出、校验、预览）
```

关键点是**中间语义 IR**。不要从 HTML 直接拼 Markdown 字符串：IR 才能同时完成
正文范围选择、链接绝对化、资产重写、脚注/数学的特殊处理、块边界控制和校验。

### 3.2 输入边界

| 入口 | 需要网络 | 动态页面 | 资产 | 判断 |
|---|---|---|---|---|
| `.html`/`.htm` 文件 | 否 | 取决于保存时是否渲染过 | 支持 `data:`、同目录相对文件 | MVP |
| SingleFile 保存的 HTML | Remit 不需要 | 是，页面已由浏览器渲染 | 已内联/打包，HTML 也能离线打开 | 推荐的浏览器剪藏路径 |
| URL 抓取 | 是，用户显式发起 | 否（Go `net/http` 不跑 JS） | 需另行抓取；默认不应在阅读时联网 | 后续增量 |
| EPUB 文件 | 否 | 不适用 | 包内图片/SVG，按 manifest 取用 | 后续增量 |
| 插件 importer | 否 | 不适用 | 当前契约没有资产 | 保持现状；不承接完整文章 |
| Pandoc/Trafilatura 等外部工具 | 用户自己决定 | 取决于工具 | 输出文件由用户负责 | 外部工具，不是默认依赖 |

SingleFile 的官方说明是：它把完整页面保存成单个 HTML，并会在需要时下载页面
引用但尚未缓存的资源；默认不上传数据到第三方（[README](https://github.com/gildas-lormeau/SingleFile#readme)、[FAQ](https://github.com/gildas-lormeau/SingleFile/blob/main/faq.md)、[隐私政策](https://github.com/gildas-lormeau/SingleFile/blob/main/privacy.md)）。
因此它最适合本产品：**网络与浏览器复杂性留在用户已信任的浏览器里，Remit 只读文件。**

URL 抓取不是“更自动的浏览器”。Go 的 HTTP 客户端不执行 JS、不处理登录态、
不解决反爬；对 SPA 和付费墙应明确失败，并给出“用 SingleFile 保存后导入”的
可执行退路。URL 抓取应设超时、重定向上限、最大字节数、内容类型检查和
`http/https` 协议限制；不抓 `file:`，不执行脚本。

### 3.3 HTML 阶段

1. **解析**：`x/net/html` 是 HTML5 兼容的 tokenizer/parser（[包文档](https://pkg.go.dev/golang.org/x/net/html)）。
2. **去噪**：在正文抽取前移除 `script`、`style`、`iframe`、`object`、`embed`、
   `form` 等；不要执行任何页面脚本。
3. **正文抽取**：网页用 Readeck 的 Readability fork。它返回清洗后的正文 HTML
   和标题、作者、站点、发布时间等元数据（[README](https://codeberg.org/readeck/go-readability/src/branch/v2/README.md)、[`article.go`](https://codeberg.org/readeck/go-readability/src/branch/v2/article.go#L34-L86)）。
   EPUB 的 XHTML 章节已经是“正文”，不要对它再做 Readability。
4. **兜底**：Readability 失败或正文过短时，按 `<article>` → `<main>` → `<body>`
   选择容器；仍失败则报错，不保存空壳。
5. **清洗**：Readability 明确声明自己不负责清除注入内容，建议在其后使用
   sanitizer（[Readability security](https://github.com/mozilla/readability#security)）。
   用 `bluemonday` 的 allowlist 策略只保留语义标签和安全的 URL/属性。
   数学公式若需要在清洗后保留，应在清洗前把 MathML/TeX 提取成占位节点，
   再在 IR 里换回，避免 sanitizer 改坏公式。

### 3.4 结构保真矩阵

| 结构 | 推荐做法 | 可保证的程度 |
|---|---|---|
| 标题 | 文章标题进入 `Note.Title`；去掉正文里重复的首个 H1 | 保真 |
| 标题层级 | 保留 H2–H6；源文档的 H1 若就是文章标题则移除 | 基本保真 |
| 段落 | 每个语义段落一个块 | 保真 |
| 列表 | 整个紧凑列表为一个块；嵌套层级保留 | 基本保真；松散列表要压紧 |
| 引用 | blockquote 为一个块 | 保真 |
| 代码 | 保留语言和缩进；用足够长的围栏 | 基本保真；空行见 §3.5 |
| 链接 | 相对 URL 按 `<base>`/最终 URL 绝对化；去掉 `javascript:` | 保真；EPUB 跨章锚点需映射 |
| 图片 | 物化为 asset，重写为 `assets/<sha>.<ext>`；保留 alt | 保真 |
| 表格 | 简单表格转 GFM；保留对齐 | 简单表保真；合并单元格/多行单元格降级 |
| 脚注 | `role=doc-noteref`/`doc-endnotes` 或 `epub:type` 映射为 `[^n]` | 基本保真；复杂多段脚注降级 |
| 数学 | 优先取 MathML `<annotation encoding="application/x-tex">`，输出 `$…$`/`$$…$$` | TeX 注解可用时保真；纯 MathML 需降级 |
| 音频/视频/附件 | 不是当前 Asset 的显示类型；保留链接/说明并警告 | 不保证内嵌 |

GFM 表格的规范只有 header row、delimiter row 和数据行，没有 `colspan`/`rowspan`
语法（[GFM §4.10](https://github.github.com/gfm/#tables-extension-)）。
`html-to-markdown` 的 table 插件能保留对齐，并把合并单元格展开为空/镜像单元格，
而不是恢复合并结构（[table 插件源码](https://github.com/JohannesKaufmann/html-to-markdown/blob/main/plugin/table/table.go#L12-L80)）。
因此复杂表格只能“文本不丢、结构降级”，并在导入报告中标注。

### 3.5 代码块与空行：必须在 MVP 里处理

`html-to-markdown` 能正确输出围栏代码块（包括块内空行）；问题在 Remit 的块解析：
任何空行都会结束当前块。一个带空行的代码围栏会被拆成多个 `Block`，
每个块都被追加独立的 `^block_…`。阅读时 `DocumentText` 会把块重新用空行连接，
所以整篇阅读可能仍然像原来的代码；但：

- 不能把整段代码当成一个可引用块；
- 对代码块制卡会只拿到碎片；
- 文件里的 `^block_…` 可能落在围栏代码内部，违背“任何 Markdown 工具都能安全
  保留”的初衷。

判断：文章捕获若不改解析器，就应把“含空行的代码块”记为已知降级并在导入报告
里明确提示。若要承诺代码保真与代码块制卡，必须先把 `parseNote`/`blocksOfDocument`
改成围栏感知（只在围栏外的空行分块）。这不需要第二套格式，只是让块边界理解
Markdown。

### 3.6 资产

事实：

- `PutAsset` 按 SHA-256 命名，重复字节只存一份；`Asset` 只允许读
  `assets/<name>`，拒绝路径、`..` 和 scheme（[`assets.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/assets.go#L25-L77)）。
- 阅读面只加载 `assets/` 引用；web URL、`data:` 和库外路径一律不联网，只显示
  alt（[`internal/ui/screens/notes/editor.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/ui/screens/notes/editor.go#L232-L260)）。
  `importmd` 也把 web URL/data URI/盘外路径当作不是库的资产
  （[`internal/core/importmd/assets.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/importmd/assets.go#L139-L148)）。

因此资产策略必须是：

1. **SingleFile/HTML 文件**：支持 `data:` 图片和同一输入目录内的相对文件。
   解析后拒绝 `..`、绝对路径、symlink 跳出目录和 `file:`；用 `PutAsset`
   落库，再把 Markdown 的图片目标重写为 `assets/<sha>.<ext>`。
2. **URL 抓取**：图片下载是用户显式抓取的一部分，但要显示“将下载 N 个远程
   资产”并在失败时保留 `alt` + 原链接，不把坏 URL 伪装成库内资产。只在
   `http`/`https` 下下载，限制单文件和总大小。
3. **EPUB**：只取 manifest 中实际被 XHTML 引用的图片，不解压无关资源；
   同一图片按内容去重。
4. **未取得的图片**：转成普通链接或保留 alt，正文不丢；绝不把远程图片 URL
   写进 `assets/` 形式，也绝不在阅读时自动联网。

“附件”不要扩展成一个大而全的媒体系统：`Asset` 今天的显示语义是图片。
音频、视频、PDF 内嵌文件可以作为链接/外部附件处理，并在报告里说明未内嵌。

### 3.7 块 ID、来源元数据与更新

**铸造规则**：

- 转换器只产出 `core.Block{Text: …}`，`ID` 留空。
- `SaveNote`/`normalizeNote` 给每个块铸 128 位随机 ID（[`note.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/note.go#L130-L145)）。
- 不从 URL、标题、段落哈希、DOM 位置或序号推导块 ID；哈希会随编辑变化，
  位置会随重排变化，都会破坏“ID 一次性铸造”的语义。
- 初次导入后，块 ID 在库内稳定；编辑、移动、重排由现有编辑器规则维护。

**来源元数据**：`Note` 没有元数据字段，也没有 YAML front matter 的约定；
术语表明确把 front matter 视为“块内容”，而不是笔记元数据
（[`GLOSSARY.md`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/GLOSSARY.md#L56-L58)）。
因此 MVP 把来源写成一个**普通尾部块**，例如：

```markdown
Source: <https://example.com/article> · 2024-01-02 · captured 2026-11-08
```

EPUB/本地文件可以是：

```markdown
Source: EPUB “Book title” · urn:isbn:… · captured 2026-11-08
Source: file “article.html” · sha256:ab12… · captured 2026-11-08
```

这一行仍然得到块 ID，能被链接、搜索、摘录；它没有新 schema，也不会变成文章
JSON。作者、站点、提取器版本和警告可以只进导入报告，避免把笔记变成调试记录。
生成这一行时要按纯文本规则避开 `::`/`{{…}}` 等卡片标记；URL 必要时做百分号编码。

**重新捕获**：

- 不要静默覆盖已有笔记。按规范化的 canonical URL（网页）、`dc:identifier`
  （EPUB）或文件哈希（本地文件）找到已有来源时，默认“打开已有 / 另存为新笔记”。
- 若以后做“更新已有笔记”，只对文本完全未变的块复用旧 ID；新块铸新 ID；
  被删除的块让引用进入 lost（ADR 0002 的正常状态），不要按位置把旧 ID
  强行分配给不同内容。
- 跨章节的 EPUB 锚点、逐块来源定位符和旧 ID 的映射属于“刷新”功能，需要
  额外的 sidecar 元数据；它不是 MVP，也不应把 sidecar 变成第二份正文。

## 4. AI 可以做什么，不能做什么

### 4.1 分工

| 步骤 | AI 适合度 | 原因 | 确定性兜底 |
|---|---|---|---|
| 正文抽取 | 低 | 规则/Readability 已能做；把整页发给模型昂贵且不可复现 | Readability → `<article>/<main>/<body>` |
| 结构恢复（标题层级、段落/列表边界） | 中 | HTML 语义缺失时可提出候选，但容易改内容 | 原始 DOM 语义；低置信度就保留纯文本 |
| 标题/摘要/标签 | 高，但非破坏性 | 不影响正文真相；用户可拒绝 | 页面标题/`dc:title`；没有摘要就不生成 |
| 翻译 | 高，但必须派生 | 译文无法用原文逐字验证；应生成新笔记 | 原文永久保留，不覆盖 |
| 公式/表格修复 | 中 | 从 MathML/图片恢复 LaTeX/GFM 有价值，但可能幻觉 | 保留原始 MathML/alt/资产并链接来源 |
| “整页转 Markdown” | 低，不推荐 | 模型可能丢链接、改代码、编造标题；不可审计 | 确定性管线 |

判断：AI 的默认位置是**在确定性结果之后**，而不是替代确定性管线。导入先成功，
AI 后处理失败不能阻止笔记落库。

### 4.2 输出协议与校验

不要把自由文本 Markdown 当作 AI 的写入格式。让模型返回结构化建议，例如：

```json
{
  "title": "…",
  "summary": "…",
  "headings": [{"block": 3, "level": 2}],
  "warnings": []
}
```

或对单块修复：

```json
{"block": 7, "markdown": "…", "reason": "…"}
```

校验顺序：

1. **Schema**：字段、类型、枚举、长度上限；未知字段拒绝。
2. **引用完整性**：`block` 必须在当前导入的块范围内、唯一、按输入生成的顺序；
   不允许模型造出块 ID。
3. **内容不变量**：结构修复不得改变块数、代码字节、链接目标和资产引用；
   表格修复的行列数必须与原文一致；公式必须能被宿主解析。
4. **分歧预览**：展示 before/after diff；用户确认后才应用。翻译是显式派生，
   不是“修复”。
5. **失败回退**：任何一步失败就保留确定性结果，记录原因，不做部分写入。

OpenAI 的 Structured Outputs 能让模型输出遵守提供的 JSON Schema，并且拒绝
会以单独的 `refusal` 字段出现；但 schema 合规只保证形状，不保证事实正确
（[官方文档](https://developers.openai.com/api/docs/guides/structured-outputs)）。
因此上面的不变量校验仍然必须存在。

### 4.3 隐私边界

事实：

- 本仓库的 CI 有“不得有 allowlist 之外的出站 URL”的隐私门
  （[`docs/engineering/conventions.md`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/engineering/conventions.md#L53)）。
- 当前库内图片渲染只读 `assets/`，不联网（[`editor.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/ui/screens/notes/editor.go#L232-L260)）。
- 插件没有网络能力；外部 AI 不能假装成一个普通 importer
  （[插件系统 §6](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/specs/plugins/system.md#L191-L214)）。

推荐规则：

- **默认关闭**。设置页按 provider 配置 base URL/model/key；关闭即无请求。
- 每次发送前显示：provider、model、将发送的笔记/块数、字符数、是否含图片；
  用户按次确认。不要“导入时顺手调用”。
- 只发送必要范围：标题/摘要只发正文文本；结构修复只发候选块；公式只发
  MathML/TeX/图片；不要上传整个库。
- 审计日志只记时间、provider/model、目的、块数/字节数、结果和错误；
  默认不记录请求正文。日志可清除，AI 开关可撤销。
- API key 放系统钥匙串，不进笔记、不进 `remit.json`、不进日志。
- 本地模型/本地网关优先作为敏感内容的选项；只要端点是 loopback，就不属于
  外部发送，但仍应显示实际端点。
- 外部 API 的数据政策必须原样告知用户。以 OpenAI 为例，官方说明 API 数据默认
  不用于训练（除非显式 opt-in），滥用监控日志默认最多保留 30 天，Zero Data
  Retention/Modified Abuse Monitoring 需要审批（[Data controls](https://developers.openai.com/api/docs/guides/your-data)）。
  这只是示例，不是对所有 provider 的保证；UI 应链接当前 provider 的政策。

## 5. 产物契约

### 5.1 必须

一次成功捕获产生：

- **一个普通 `Note`**：标题来自 `dc:title`/Readability 标题/首个 H1/文件名；
  正文是空行分隔的普通块。
- **若干普通 `Block`**：每个块由 `SaveNote` 铸随机 ID；代码、表格、列表等只是
  Markdown 文本，不是新的块类型。
- **若干 `Asset`**：`assets/<sha256>.<ext>`，笔记用相对路径引用。
- **来源元数据**：一个普通尾部块；无文章 JSON、无 HTML 正文真相、无来源表。
- **导入报告**：notes/assets/warnings/skipped；失败的图片、表格合并、
  空行代码、跨章锚点都要可见。

### 5.2 不能

- 不能新增 `Article`、`ArticleBlock`、`ArticleAsset` 之类的并行模型。
- 不能把原始 HTML 作为第二份正文真相；HTML 只应是内存中的中间态。
- 不能把 AI 生成的 Markdown 直接覆盖原文；翻译/摘要必须可见地派生。
- 不能让转换器生成 `^block_…` 或从内容哈希生成 ID。
- 不能把远程图片 URL 写进 `assets/` 命名空间；资产必须是已落库字节。

### 5.3 EPUB 的粒度

推荐 v1：**一本 EPUB = 一篇 Note**，按 `spine` 顺序拼接章节。理由：

- 阅读位置、块引用、跨章节内部链接都在同一篇笔记内；
- 产物契约最简单，不需要“书”这个新模型；
- `.remitlib` 只多一个 note 文件，资产仍按内容去重。

若一本书大到 `ui/markdown`/`richtext` 不可接受，再按 `spine` 或顶层标题拆成
多篇普通笔记，标题写成 `Book — Chapter`，每篇都带同一 book source key；
这仍是普通 Note，不是书格式。拆分阈值应由性能测试决定，不在本票拍脑袋。

EPUB 事实：包文档定义 manifest 和表示默认阅读顺序的 spine；XHTML 内容文档
使用 XML 语法；OCF 是 ZIP 容器；`META-INF/encryption.xml` 可声明加密资源
（[EPUB 3.3 §4.2.6.3.1](https://www.w3.org/TR/epub-33/#sec-container-metainf-container.xml)、
[§5.6](https://www.w3.org/TR/epub-33/#sec-pkg-manifest)、
[§5.7](https://www.w3.org/TR/epub-33/#sec-pkg-spine)、
[§6.1](https://www.w3.org/TR/epub-33/#sec-xhtml)、
[§4.2.6.3.2](https://www.w3.org/TR/epub-33/#sec-container-metainf-encryption.xml)）。
遇到 DRM/加密资源应明确失败，不做解密或规避。

### 5.4 PDF

PDF 不应进入 MVP。PDF 的内容模型是页面描述，而不是语义段落
（[PDF 1.7 规范](https://opensource.adobe.com/dc-acrobat-sdk-docs/pdfstandards/PDF32000_2008.pdf)）。
`ledongthuc/pdf` 的官方能力是“get plain text content (without format)”
（[README](https://github.com/ledongthuc/pdf#readme)）；用它实现只能得到文本流，
标题层级、列表、表格、图片、脚注和阅读顺序都要靠猜测。

推荐：

- 需要 PDF 时，作为**单独的插件/外部工具**，或让用户用 `pdftotext` 等外部工具
  先转成 Markdown 再导入；
- 不内嵌 OCR；
- 不默认把 PDF 页面截图当 Asset；
- 如果未来做 PDF 结构恢复，AI 只能做候选，原文/页码/截图必须保留并可回退。

## 6. 推荐 / 备选 / 不采纳

### 6.1 推荐

**宿主内置、确定性的 `importarticle` 管线。**

- HTML 文件（含 SingleFile）为第一个入口；
- DOM → Readability → sanitize → IR → html-to-markdown → blocks；
- 资产用 `PutAsset`；
- 末尾来源块；
- 直接 `SaveNote`，不触发制卡；
- 导入报告和 fixture 测试。

### 6.2 备选

**Pandoc 作为可选外部工具。**

Pandoc 能读 HTML/EPUB，输出 CommonMark/GFM，支持脚注、数学和 `--extract-media`
（[README](https://github.com/jgm/pandoc#readme)、
[MANUAL](https://pandoc.org/MANUAL.html)、
[`--extract-media`](https://pandoc.org/MANUAL.html#option--extract-media)）。
但它不是默认内置方案：

- 许可证是 GPL-2.0-or-later，不是 MIT 级依赖；嵌入/打包需要单独复核
  （[COPYRIGHT](https://github.com/jgm/pandoc/blob/main/COPYRIGHT)）；
- 用户机器上未必有 Pandoc；
- 它输出一个 Markdown 字符串，仍需 Remit 的块归一化、资产落库和来源块。

可把它当作“用户自装、在外部运行”的逃生口，不能作为产品默认路径。

**Trafilatura 作为抽取备选。**

它支持多种输出格式、Markdown、元数据和可选表格/图片，许可证 Apache-2.0
（[README](https://github.com/adbar/trafilatura#readme)）。但需要 Python 运行时，
更适合作为一个可选的 exec 插件/外部工具，而不是宿主核心依赖。

**SingleFile 作为浏览器剪藏路径。**

它已经把页面、样式和资源保存成一个 HTML；Remit 只做导入。这比内置浏览器
或浏览器扩展直连更小、更安全。官方说明它不上传数据，但保存时可能下载页面
引用的资源（[README](https://github.com/gildas-lormeau/SingleFile#readme)、
[FAQ](https://github.com/gildas-lormeau/SingleFile/blob/main/faq.md)）。

### 6.3 不采纳

- **AI 作为唯一 HTML→Markdown 转换器**：不可复现，可能丢链接、改代码、编造
  标题；不满足“捕获的产物是原文的普通笔记”。
- **保存 HTML 正文作为第二真相**：违反单一真相和可携带性；HTML 只做内存中间态。
- **新增 Article/ArticleBlock 表或 JSON**：违反本票的产物契约。
- **从 URL/标题/内容哈希生成块 ID**：URL 会变，标题会改，哈希会随编辑变化；
  与 ADR 0002/0003 的“一次性铸造”冲突。
- **阅读时加载远程图片**：当前渲染器和隐私门都不允许；捕获时必须落成本地资产，
  否则保留 alt/链接。
- **内置 Chromium 做网页抓取**：体积、更新、沙箱和攻击面都不划算；用
  SingleFile 或用户已有浏览器。
- **默认把所有页面交给云 AI**：违反“默认不外发”；即使 provider 声称不训练，
  仍应逐次显式同意。
- **把 Pandoc 或任何 GPL-2-only 库直接嵌进宿主**：Pandoc 是 GPL-2+，不是本
  仓库的 MIT/BSD/Apache 级依赖；GPL-2-only 则按规约直接不接受。若分发方式改变，
  Pandoc 仍需单独许可评审。
- **用正则拆 HTML/EPUB**：正文、表格、脚注和资源路径都需要 DOM/XML 结构；
  字符串手术会在真实页面上失败。

## 7. 最小可行切片

### MVP：单个 HTML 文件 → 一篇普通笔记

范围：

1. 文件选择器接受 `.html`/`.htm`，包括 SingleFile 产物。
2. 解析 HTML，运行 Readability；失败时 `<article>/<main>/<body>` 兜底。
3. 清洗；保留标题、段落、标题层级、紧凑列表、引用、围栏代码、简单表格、
   链接、`data:`/同目录图片。
4. 用 `html-to-markdown/v2` + table 插件序列化；对图片、脚注、MathML/TeX
   加自定义 pass。
5. 每个语义块调用 `SaveNote`（不调用 `reconcileCards`）；ID 由 store 铸。
6. 资产 `PutAsset`；相对引用重写为 `assets/<sha>.<ext>`。
7. 末尾写来源块；导入报告列出资产数、降级项和失败图片。
8. 不做 URL、不做 AI、不做 EPUB/PDF、不改插件 importer 契约。

验收 fixture 应至少包含：

- 标题 + 两级标题；
- 嵌套列表；
- 带语言标记、带空行的代码块（预期：空行代码降级为警告，或先修解析器）；
- 简单 GFM 表格与一个 `colspan`（预期：表格文字保留、合并结构警告）；
- 脚注引用与定义；
- MathML 带 TeX annotation 与不带 TeX annotation 各一个；
- `data:` 图片、同目录相对图片、一个远程图片（文件导入时预期不联网）；
- `javascript:` 链接与 `../` 路径（预期：拒绝）；
- 正文包含 `std::vector`、`{{…}}`（预期：导入不制卡；编辑保存前需制卡插件
  代码感知修复，否则作为已知风险）；
- 重复导入同一文件（预期：不重复创建，或明确询问）。

### 后续增量

1. **EPUB**：ZIP → container.xml → OPF → spine → 同一 HTML 管线；
   加密/DRM 明确失败；一本一篇，性能不够再按章节拆。
2. **URL**：显式输入；`net/http` 抓取；无 JS；失败引导 SingleFile；
   远程资产另行计数与同意。
3. **AI**：先做标题/摘要建议，再做结构/公式候选；结构化输出 + diff +
   显式确认；默认关闭。
4. **PDF**：外部工具/插件，不进入核心。

## 8. 测试与可验证性

好测试应能抓住这些回归：

- HTML 正文抽取把导航/页脚当正文 → 正文块集合变化；
- `javascript:` 链接或 `../` 文件被当作资产 → 安全测试变红；
- 两张同字节图片 → asset 只增 1；
- 相对图片未重写为 `assets/` → 阅读面不显示；
- 空行代码块被静默拆碎 → 报告/fixture 变红；
- 表格 `colspan` 被伪装成完整表格 → 保真声明变红；
- 脚注引用/定义丢失 → 渲染文本变化；
- 导入调用 `saveDocument` 导致 `std::vector` 制卡 → 卡片数变化；
- 转换器自己写 `^block_` 或从哈希造 ID → 块 ID 断言变红；
- AI 返回越界/缺字段/改代码 → 不变量校验拒绝，确定性笔记不变；
- AI 关闭时任何 outbound 请求 → 网络测试/隐私门变红。

测试数据应包含中文、英文、CJK 标点、相对链接、`data:` 图片和一段
`std::vector`；这些问题不靠真实网页才能复现。

## 9. 未决问题

1. **EPUB 粒度**：一本一篇，还是按 spine/章节拆？本票推荐前者，等性能数据再改。
2. **来源元数据的机器可读部分**：MVP 只有一个尾部 `Source` 块；若以后要
   无损刷新，是否允许一个仅含 source key/块锚点映射的 sidecar？不能让它成为
   第二份正文。
3. **浏览器剪藏配对**：是否要在 loopback agent API 上增加 token-gated
   capture 端点，还是长期只支持 SingleFile 文件导入？现有 API 是 loopback +
   bearer token 的只增写入（[`internal/agentapi/server.go`](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/agentapi/server.go#L1-L8)），
   但浏览器扩展拿 token 需要额外的配对 UX。
4. **块解析器**：是否先修“空行代码块”再发布代码捕获？不修就必须把代码保真
   标为降级。
5. **制卡插件代码感知**：`::`/`:::`/`{{…}}` 的扫描是否先忽略围栏代码块和
   行内代码？不修会让技术文章在编辑保存后产生意外卡片。
6. **AI provider/合规**：只支持 OpenAI 兼容 API，还是 provider-agnostic？
   审计日志的保存位置和保留期是什么？
7. **PDF**：是否真的在产品范围内？若在，OCR 与版面恢复是否接受“低保真、
   外部工具”定位？

## 10. 来源

项目内：

- [Note/Block 模型](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/model.go#L14-L30)
- [ID 铸造](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/id.go#L9-L21)
- [块标记](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/marker.go#L5-L37)
- [note 文件解析](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/note.go#L31-L72)
- [SaveNote](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/library.go#L761-L787)
- [资产](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/store/assets.go#L13-L77)
- [Markdown 导入](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/importmd/import.go#L19-L28)
- [插件 importer 的宿主侧](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/suggest/suggest.go#L1-L43)
- [wiki link 扫描](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/internal/core/links.go#L27-L57)
- [ADR 0002](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/adr/0002-content-model.md)
- [ADR 0003](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/adr/0003-storage-layout.md)
- [ADR 0005](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/adr/0005-note-editor.md)
- [插件系统](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/specs/plugins/system.md)
- [增量阅读](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/specs/incremental-reading.md)
- [工程规约](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/engineering/conventions.md)
- [richtext 表格与图片研究](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/docs/research/richtext-tables-and-images.md)
- [GLOSSARY](https://github.com/transmit-bug/remit/blob/bad1b778eef862400caeab8f3b5e13422dc814d3/GLOSSARY.md)
- [MyGo Markdown 扩展](https://github.com/egoist/mygo/blob/v0.8.1/ui/markdown/parser/document.go#L23-L38)

外部：

- [Readeck go-readability](https://codeberg.org/readeck/go-readability/src/branch/v2/README.md)
- [Go-Readability（已归档）](https://github.com/go-shiori/go-readability)
- [Mozilla Readability](https://github.com/mozilla/readability)
- [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown)
- [html-to-markdown table plugin](https://github.com/JohannesKaufmann/html-to-markdown/blob/main/plugin/table/table.go)
- [bluemonday](https://github.com/microcosm-cc/bluemonday)
- [x/net/html](https://pkg.go.dev/golang.org/x/net/html)
- [Pandoc manual](https://pandoc.org/MANUAL.html)
- [Pandoc COPYRIGHT](https://github.com/jgm/pandoc/blob/main/COPYRIGHT)
- [Trafilatura](https://github.com/adbar/trafilatura)
- [SingleFile](https://github.com/gildas-lormeau/SingleFile)
- [SingleFile FAQ](https://github.com/gildas-lormeau/SingleFile/blob/main/faq.md)
- [EPUB 3.3](https://www.w3.org/TR/epub-33/)
- [MathML 3 annotation](https://www.w3.org/TR/MathML3/chapter5.html#mixing.semantic.annotations)
- [GFM tables](https://github.github.com/gfm/#tables-extension-)
- [ledongthuc/pdf](https://github.com/ledongthuc/pdf)
- [PDF 1.7 规范](https://opensource.adobe.com/dc-acrobat-sdk-docs/pdfstandards/PDF32000_2008.pdf)
- [OpenAI data controls](https://developers.openai.com/api/docs/guides/your-data)
- [OpenAI Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs)
