# richtext 里的表格与图片：编辑边界

基线：`github.com/egoist/mygo v0.8.1`（`go.mod`），仓库里的 richtext 接线在
`internal/ui/screens/notes`。本票只查事实，不做设计选择；每条结论给源码位置和一句判定。

引用路径中，`ui/...`、`docs/ui/...` 与 `internal/scene/...` 指 MyGo v0.8.1
模块内；`internal/ui/...`、`internal/core/...` 指 Remit 仓库。

## 结论摘要

| 问题 | 结论 | 判定 |
|---|---|---|
| GFM 表格在 `richtext` 里能否原样编辑 | 不能。表格按行降级成以 tab 分隔的普通段落，列对齐和跨行列丢失 | 没有表格结构 |
| 表格可否“导入再导出”回 GFM 表格 | 不能。第一次 `ParseMarkdown` 之后就没有表格语法可写回 | 没有 |
| `richtext` 会不会根据图片 URL 去读文件/联网 | 不会。`URL` 只是元数据；没有图片 loader | 没有 |
| 图片像素谁提供 | 宿主读盘/解码后塞 `Attachment.Image`，或提供 `Draw` | 宿主自塞 |
| `ui/markdown` 阅读面 | GFM 表格直接渲染；图片通过 `LoadImage` 回调，由宿主提供像素 | 表格直接用；图片薄封装 |
| `ui.Table` 可否直接当表格编辑器 | 它是虚拟化行列表格，单元格可放可编辑控件，但没有插行/插列/合并单元格 API | 只能做宿主自建表格的底座 |
| 格式条 44px 有没有官方组件 | `ui.Toolbar` 是官方组件；44 是 Remit 的 `BarHeight`，不是 MyGo 的固定高度 | 直接用 + 宿主定高 |

## 1. GFM 表格：实测往返

### 方法

用临时 Go 程序调用：

```go
t := richtext.ParseMarkdown(src)
out := t.Markdown()
again := richtext.ParseMarkdown(out)
```

输入覆盖表头/表体、左右对齐、行内粗体/斜体/代码/链接、转义管道、单元格内
`<br>`、缺列，以及带 `colspan`/`rowspan` 的 HTML 表格。下面的 `\t` 表示实际
制表符；代码块里的换行就是输出内容。

### 结果

对齐表：

```markdown
| Name | Size |
| :--- | ---: |
| alpha | 1 |
| beta | 2 |
```

输出：

```markdown
**Name\tSize**

alpha\t1

beta\t2
```

表头加粗、单元格文字和顺序都在；表格、管道、分隔行、列对齐都不在了。

行内标记与链接：

```markdown
| **Head** | [Link](https://example.com) |
| --- | --- |
| *em* | `code` |
```

输出：

```markdown
**Head**\t[**Link**](https://example.com)

*em*\t`code`
```

表头的整行权重是 700，因此链接文字也带粗体；行内标记本身保留。

转义管道：

```markdown
| A | B |
| --- | --- |
| a \| b | c |
```

输出：

```markdown
**A\tB**

a | b\tc
```

单元格里的 `\|` 变成普通 `|`；`escapeMarkdown` 的转义集合不含 `|`
（`ui/richtext/markdown.go:402-417`）。输出已经是普通段落，所以这个 `|`
不再有表格语义；若把它重新放进表格上下文，需要另做转义。

单元格内 `<br>`：

```markdown
| A | B |
| --- | --- |
| one<br>two | c |
```

输出：

```markdown
**A\tB**

one

two\tc
```

`<br>` 被读成 `LineBreak`，再变成换行；`inlinePara` 把换行拆成新的段落
（`ui/richtext/markdown_parse.go:142-165,199-202`）。所以单元格内的多行不是
“同一段的换行”，而是两个段落，第二段还带走了后面的 tab 和后续单元格。

缺列：

```markdown
| A | B | C |
| --- | --- | --- |
| 1 | 2 |
```

输出：

```markdown
**A\tB\tC**

1\t2\t
```

解析器按表头列数补齐空单元格（`ui/markdown/parser/block.go:1338-1352`），
`richtext` 为每个单元格写 tab，所以尾部出现一个 tab。把这段输出再喂给
`ParseMarkdown` 时，段尾空白被裁掉，尾 tab 消失；首轮之后的形状不再变化。

HTML 表格的跨行列：

```html
<table>
  <tr><th colspan="2">Head</th></tr>
  <tr><td rowspan="2">a</td><td>b</td></tr>
  <tr><td>c</td></tr>
</table>
```

输出：

```markdown
Head

a b

c
```

HTML 块进入 `ParseHTML`；它的元素分支只处理文本、`br`、`img`、`input` 等，
没有 `table`/`tr`/`td` 的结构（`ui/richtext/html.go:435-475`）。`colspan`、
`rowspan` 既不在 GFM 表格扩展里（`ui/markdown/parser/document.go:14-35`），
也没有 HTML 表格映射，所以整体拍平成段落。

图片附件往返：

```markdown
Before ![a cat](assets/abc.png "title") after
```

输出：

```markdown
Before ![a cat](assets/abc.png) after
```

图片的 alt 和 URL 保留，title 丢失；`Text.Markdown` 的附件分支只写
`Name` 和 `URL`（`ui/richtext/markdown.go:280-285`）。解析回来的
`Attachment.Image` 是 nil（`ui/richtext/markdown_parse.go:218-221`）。

### 源码解释

- `ParseMarkdown` 的文档注释直接写明：“a table's rows are paragraphs of their
  cells apart by tabs, the header's bold”
  （`ui/richtext/markdown_parse.go:9-16`）。
- 表格分支逐行生成 `Paragraph`，行内单元格之间插入 tab，表头设置
  `Weight: 700`，没有生成表格节点
  （`ui/richtext/markdown_parse.go:89-107`）。
- `Text.Markdown` 只按段落样式写 heading/list/quote/code/body，没有 table
  分支（`ui/richtext/markdown.go:11-16,17-98`）。
- 解析器确实解析了列对齐：`parser.Block.Align []Align`
  （`ui/markdown/parser/document.go:63-64,136-137`），但 `mdReader` 的表格分支
  没有读取 `Align`，生成的 `ParagraphStyle` 也没有表格列的概念。
- `ui/markdown` 的阅读视图相反：`parser.Table` 分派到 `st.table`
  （`ui/markdown/view.go:417-418`），后者按 `b.Align[k]` 设置
  `TextAlign`（`ui/markdown/table.go:102-109`）。

### 对导入/导出的直接后果

1. 表格文本和行内标记不会丢；表格结构、列对齐、HTML 跨行列、单元格内多行会丢。
2. 第一次 `ParseMarkdown` → `Text.Markdown` 后，输出是普通段落文本；再次往返
   不再有表格可恢复。除尾部空单元格的 tab 在第二次往返被裁掉外，结果稳定。
3. Remit 的 `saveDocument` 用 `s.Doc.Text().Markdown()` 写回笔记
   （`internal/ui/screens/notes/editor.go:386-398`）。表格行之间以空行分隔，
   `blocksOfDocument` 又按空行拆块（`internal/ui/screens/notes/document.go:28-49`），
   所以一个原本的表格块在正文保存后会变成多个块；按位置复用 ID 的规则会让
   后面的块拿到新 ID。表格里的卡片/链接若要稳定引用，不能只靠这次降级后的文本。
4. `importmd` 本身把源文件交给 `store.ParseNoteSource` 分块，不调用
   `richtext`（`internal/core/importmd/import.go:66-72`）；损失发生在编辑器
   正文保存/导出经过 `Text.Markdown` 的路径，而不是导入的第一步。

## 2. 图片像素从哪来

### richtext：没有 loader

`ParseMarkdown` 对图片只产生：

```go
&Attachment{Name: parser.PlainText(alt), URL: safeLink(dest)}
```

（`ui/richtext/markdown_parse.go:218-221`）。`Attachment` 的像素字段是
`Image ui.ImageSource`，`URL` 的注释是“where the attachment comes from, as a
file's or an image's, which HTML and Markdown write for it”
（`ui/richtext/text.go:77-98`）。

绘制时只有三条路：

1. `at.Draw` 由宿主画；
2. `at.Image != nil` 时画 `ImageSource`；
3. 否则画一个带 `Name` 的盒子，像“图片还没加载”一样。

（`ui/richtext/paint.go:222-250`；同一行为也写在
`docs/ui/rich-text-editor.md:327-342`。）

`EditorOptions` 没有 `LoadImage`、`URL` 解析或网络字段
（`ui/richtext/view.go:17-46`）。所以框架不会把相对路径、`file://` 或
`http(s)://` 变成像素，也不会因为图片 URL 发起网络请求。

**判定：宿主必须自己读盘、解码并塞 `ImageSource`；`richtext` 没有图片 loader。**

### Remit 现在怎么塞

- 编辑器构建文档时，`rehydrate` 只处理 `assets/` 引用，用
  `deps.GetAsset` 取字节，再 `ui.DecodeBitmap`，最后写回
  `at.Image`（`internal/ui/screens/notes/notes.go:288-334`）。
- 拖拽/粘贴图片时，`insertAsset` 同样先 `ui.DecodeBitmap`
  （`internal/ui/screens/notes/assets.go:188-207`）。
- 因此非 `assets/` 的 URL 永远停在名字盒子；不会读外部路径、不会联网。

这里有一个当前实现的不对称：拖拽入口接受 `svg`
（`internal/ui/screens/notes/assets.go:150-156`），阅读面的
`imageOptions` 对 `.svg` 调用 `ui.ParseSVG`
（`internal/ui/screens/notes/editor.go:237-258`），但编辑器的
`rehydrate`/`insertAsset` 只调用 `ui.DecodeBitmap`
（`internal/ui/screens/notes/notes.go:318-325`；
`internal/ui/screens/notes/assets.go:193-197`）。所以 SVG 资产在阅读面能显示，
在 richtext 编辑器里会显示名字盒子。这是实现事实，不是设计结论。

### 阅读面 `ui/markdown`：回调加载，不是自动联网

`markdown.Options.LoadImage` 的约定是：在独立 goroutine 上加载，未完成时显示
alt（`ui/markdown/view.go:53-56`）。`FileImages(dir)` 是官方文件加载器：

- 拒绝包含 `://` 的地址和 `data:`（`ui/markdown/image.go:22-34`）；
- 相对路径按 `dir` 解析，`file:` 前缀去掉后按文件读；
- 读到的字节用 `ParseSVG` 或 `DecodeBitmap`（`ui/markdown/image.go:38-41`）。

框架不提供 http 加载路径；Remit 的 `imageOptions` 也主动拒绝非 `assets/`
引用（`internal/ui/screens/notes/editor.go:232-270`）。所以当前两条阅读路径的
图片都只来自库内资产，没有出站图片请求。

### 缓存、驻留与卸载

`richtext` 的附件直接持有 `ImageSource`，没有图片缓存或卸载钩子。宿主塞进去的
`*ui.Bitmap` 保留：

- 预乘 RGBA 像素：`4 * W * H` 字节（`internal/scene/scene.go:452-460`）；
- 为了重新解码，`DecodeBitmap` 还保留原始压缩字节的闭包
  （`ui/bitmap.go:117-126`）；
- 显示变小时可生成降采样层（`ui/bitmap.go:219-243`）。

`Bitmap.ReleasePixels` 可以丢 CPU 像素，保留 GPU 已上传的副本，并在需要时
重新生成；它要求在主线程、图片已经显示后调用（`ui/bitmap.go:181-193`）。
`richtext` 和当前 Remit 的 `rehydrate` 都没有调用它。所以一篇有 20 张图的笔记，
CPU 侧驻留至少是 `Σ(4 * W_i * H_i)` 再加压缩源数据/降采样层；具体字节取决于
每张图的像素尺寸，不能只凭 DIP 数给一个固定值。没有自动淘汰或文档关闭时的
卸载回调；只有 GC 回收整个 `Attachment`/`Bitmap`，或宿主显式调用
`ReleasePixels`。

`ui/markdown` 的阅读面另有一套缓存：`Options.images` 是
`sync.Map`，URL 到 `*imageEntry`（`ui/markdown/view.go:92`），加载在
`imageEntry` 上异步完成（`ui/markdown/image.go:14-67`）。这个缓存没有大小上限
或淘汰 API。当前 Remit 的 `readView`/`secondPane` 每帧调用
`s.imageOptions()`，而该函数每次返回新的 `*markdown.Options`
（`internal/ui/screens/notes/editor.go:205,223,237-258`）；缓存在 `Options`
里，所以不是跨帧缓存。一次 headless 探针（同一 URL、两帧）的结果：

- 复用同一个 `Options`：loader 调用 1 次；
- 每帧新建 `Options`：两帧内 loader 调用 8 次。

这说明阅读面当前会把“缓存”退化成每帧重新发起加载。它是实现层事实；是否在真实
窗口里造成可感卡顿，需要再用真实图片和 GPU 后端测量，本票没有做这一步。

## 3. 编辑面与阅读面的不对称

一句话规格：

> 阅读/复习面用 `ui/markdown` 把 GFM 表格和图片渲染出来；编辑面用 `richtext`，
> 表格只剩按行排列、tab 分隔的段落，列对齐和跨行列丢失；图片只有宿主注入
> `ImageSource` 后才显示，否则显示 alt/name 盒子。

两条链路的具体差异：

| 维度 | 编辑面 `richtext` | 阅读面 `ui/markdown` |
|---|---|---|
| 表格结构 | 行 → 段落；单元格 → tab 文本 | `parser.Table` → 行/单元格节点 |
| 列对齐 | 丢失 | `b.Align` → `TextAlign` |
| 跨行列 | GFM 无；HTML 表格拍平 | GFM 无；HTML 表格是否渲染取决于 `Options.HTML`，本票未验证 |
| 单元格内 `<br>` | 拆成两个段落 | Markdown 视图按行内 line break 解析（本票未单独实测） |
| 图片像素 | `Attachment.Image`（宿主自塞） | `LoadImage` 回调（宿主提供） |
| 图片 URL | 不解析、不联网 | `FileImages` 只读文件，拒绝 scheme/data；自定义 loader 决定 |
| 图片未加载 | 名字盒子 | alt 文本 |

最小可接受的降级（基于以上事实）：

- 表格降级为“每行一段、tab 分列”的文本可以读，但不能声称仍是表格；列对齐和
  跨行列必须明确写出会丢。
- 图片在编辑器里可以先是名字盒子；阅读面可以在加载完成前显示 alt。两面的像素
  来源必须由宿主统一，不能一边读文件、一边让框架联网。

## 4. 官方控件：`ui.Table` 与格式条

### `ui.Table` 能做什么

`ui.Table` 是虚拟化行列表格：`TableColumn` 定义列标题、宽度、对齐、排序和
行头；`cell(row, col)` 由宿主构建每个单元格
（`ui/table.go:9-34,122-155`）。它支持行选择、单元格块选择、列拖动/缩放/排序、
右键菜单，以及用 `ui.EditableText` 做单行原地改名
（`docs/ui/table.md:209-215`）。

它没有：

- 插入/删除行或列的 API；
- 合并单元格、`rowspan`/`colspan`；
- 内置的“表格数据模型”（行和列由调用者用切片和回调提供）；
- 内置的单元格富文本编辑器（可以在 cell 回调里放任意控件，但文档、选区、
  焦点、撤销和保存都由宿主负责）。

**判定：`ui.Table` 可作为浏览/选择表格的底座；不是 Markdown 表格编辑器，也不能
替代 richtext 的表格结构。**

### 宿主自绘表格块的代价（事实清单）

若要在笔记里编辑真正的表格，而 `richtext` 只提供段落/tab 文本，宿主至少需要：

1. 自己拥有表格的行/列/单元格数据，并在保存时序列化成 GFM（或保留原始
   Markdown，不经过 `Text.Markdown`）；
2. 自己处理单元格的焦点、选区、输入法、撤销、复制粘贴；
3. 自己处理列宽、对齐、表头、插入/删除行列；
4. 自己决定 `ui.Table` 是否足够，还是另开一个 `richtext` 之外的编辑态；
5. 处理表格块与现有 block ID/卡片引用的映射（见 §1 的 `blocksOfDocument`）。

这些是当前接口下的实现成本，不是设计选择。

### 格式条 44px

`ui.Toolbar` 是官方组件：一行控件、溢出菜单、Tab/方向键焦点
（`ui/toolbar.go:12-35`；`docs/ui/toolbar.md:5-31`）。它没有固定的 44px
高度；`coreToolbar` 用 `Padding(t.Space(1))` 和 `AlignItems(Center)` 布局
（`ui/toolbar.go:31-35`）。

Remit 的 44 来自自己的 token：`BarHeight = 44`
（`internal/ui/design/scale.go:35-37`），格式条用 `ui.Toolbar(...).MinHeight(tk.BarHeight)`
设置（`internal/ui/screens/notes/editor.go:297-359`）。

**判定：直接用 `ui.Toolbar`；44px 是宿主 token/样式，不是 MyGo 组件自带属性。**

## 没能回答的问题

- 一张 20 图的真实笔记在目标平台上的精确 CPU/GPU 驻留字节数：本票给出的是
  每张 bitmap 的像素公式和“无自动卸载”事实，没有用真实图片做内存剖析。
- `ui.Table` 的单元格里放完整 `richtext.Editor` 的焦点、输入法和性能表现：
  接口上可以放任意控件，但没有做原型验证。
- `richtext` 未来版本是否会增加表格节点或图片 loader：本票只覆盖 v0.8.1。
- GFM 表格里含 tab、复杂转义或极端空单元格组合的全部边界：本票测了代表性样例，
  没有穷举。
