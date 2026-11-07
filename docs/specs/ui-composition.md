# UI 组合 · 官方组件优先

一条规则的落地稿：**UI 的每一个需求，都先问「哪个官方组件承载这个含义」；问不到才手写，并且记录。**
手写一个官方已有的组件是 bug，不是风格选择——它一次丢掉键盘、焦点、禁用态、无障碍角色。

> 状态列与测试名按今日代码和 `docs/specs/ui-inventory.md`（v0.8.1）校正：`已建` / `已合规` 是
> 今天的行为，`待迁` 是下一片 S3，测试名以仓库里真实存在者为准。本稿是规则，清单是逐面状态。

## 1. 规则

1. **需求，不是形状。**「两栏用户能拖」是需求，「一个 Width(220) 的 Column」是形状。
2. **先查清单。**`.agents/skills/mygo-components/SKILL.md` 的 Need → widget 表，对着
   `go list -m -f '{{.Dir}}' github.com/egoist/mygo` 指向的那份源码读，不凭记忆。
3. **读完整能力集。**`go doc` 之后再看 `type XState struct`：排序、拖拽、重排、就地改名、
   右键菜单、键盘、焦点、禁用原因、读屏角色通常都在那里。
4. **真正的缺口才包一层。**写在 `internal/ui/kit`，注释里列出「查过哪些组件、为什么都不是这个含义、
   核对的 MyGo 版本」。
5. **禁止**：在页面里手工搭 `ButtonBase` 行的选择器、`Row` 拼的表头、`Box` 拼的 tab 条、
   固定宽度的 Column 冒充分栏、`bool` 加箭头冒充折叠。

**判据**：这段 UI 如果换成一个官方组件，白拿到键盘/焦点/禁用/读屏中的任何一项，那它就是错的。

## 2. 为什么这条规则以前写了没生效

技能从首个 commit（`ac7e595`）就在；当时 pin 的 MyGo **v0.7.4 就已经有**
`Split` / `Outline` / `Table` / `Tabs` / `Combobox` / `PopoverMenuButton` / `Calendar` / `TokenField`。
所以不是版本落后，是规则本身没有约束力：

| 失效机制 | 证据 |
|---|---|
| 只记录决定，没有闸门 | 技能的 Done-when 全是自评；仓库能观测的只有 vet / i18n / 隐私 / 性能，组件选择不在其中 |
| 审计 grep 恰好对这几处失明 | 字面量检查 `\.Width([0-9]` 抓不到 `sidebarWidth = 260` + `Width(side)`；能力检查抓不到 `ListState` 拼表、`ButtonBase` 拼大纲 |
| 逃生口被用上 | 技能的「A literal width … name it and say why」，`shell.go` 的 `sidebarWidth` 注释正好用它把违规变成已解释决策 |
| 只对新需求触发 | 大纲（#119）、标签（#120）都追加进同一个三栏函数，没有人回头复审存量 |
| 没有屏幕级组合层 | `kit/` 只有 91 行（Card/Panel/Empty/Badge/Keycap/Stat），每个屏都退回 `Column`+`Row`+`Text` |

结论：**把「自评」换成「会失败的测试」**，规则才有效。

## 3. 现状 → 目标

| 界面 | 今天的实现 | 官方组件 | 状态 |
|---|---|---|---|
| 三栏布局（笔记/文档/面板） | `ui.Split` / `SplitPercent`，用户拖过的宽度被记住 | — | 已迁 |
| 文档大纲 + 当前节 | `ui.Outline`，光标所在的标题以强调色标出 | — | 已迁 |
| 反向链接 | `ButtonBase` 循环 | `ui.List`（`FollowEnd`/`Key`） | 待迁 |
| 命令面板 | `ui.List` 承载行：选择、键盘、读屏角色都在组件里 | — | 已迁 |
| 重链选择器 | 自绘 overlay + 行 + 键盘 | `ui.Autocomplete` | 待迁 |
| 浏览（卡片列表） | `ListState` + 手拼行 | `ui.Table` + `Columns`/`Sortable`/`CellMenu` | 待迁 |
| 笔记栏 | 视图 `Segmented`（编辑/分栏/阅读）+ `⋯` 菜单 + 面板开关，在窗口顶栏里，所以在三种视图里都在 | — | 已建 |
| 格式工具条 | `Toolbar` + `MenuButton` + `richtext` 的 `Formatting()`/`Toggle*`，含链接气泡 | — | 已建 |
| 段落手柄（+ / ⋮⋮） | 无（`::` 分隔符即制卡） | `Document.VisibleParagraphs()` 的 `ParagraphBox` + `MenuButton` | 待评估 |
| 标签筛选 | `ButtonBase` chip | `ui.ToggleGroup` | 待迁 |
| 笔记列表 | `ui.Sidebar` ✓ | — | 已合规 |
| 纸面 + 状态行 | 居中的 `Column(MaxWidth 760)` + `Row`/`Text` 11px | — | 已建（组合） |
| 卡片徽章条 | 文档上方的徽章 `Button` 行 + 字段编辑 `ui.DialogBase` | — | 已建（组合） |
| 编辑器 | `richtext.Editor` ✓ | — | 已合规 |
| 查找 | `ui.FindBar` ✓ | — | 已合规 |
| 搜索框 | `ui.SearchField` ✓ | — | 已合规 |
| 统计图表 | `ui/chart` ✓ | — | 已合规 |
| 复习页 | `Progress` + `Button` + `kit.Keycap` + 完成总结的 `kit.Card`/`kit.Stat` ✓ | — | 已合规 |
| 设置页 | `Form`/`Field`/`Switch`/`Segmented` ✓ | — | 已合规 |
| 反馈 | `c.Toast` ✓ | — | 已合规 |

### 明确的缺口（不算违规，注释里写明版本）

- **热力图**：v0.8.1 的 `ui/chart` 没有日历热力格（`stats.go` 已记录）。
- **同一文档双编辑器**：`richtext` 每个 `Document` 只允许一个 `Editor` 持有视图状态，
  分栏右侧只能是 `ui/markdown` 的阅读视图（`editor.go` 已记录）。
- **制卡由 `::` 派生**：产品设计（ADR 0002），不是渲染缺口。

## 4. 闸门：能力测试

手写的复制品过不了下面这些测试；官方组件天然通过。每条都要在默认宽度和最小宽度各跑一次。

| 测试 | 断言的能力 | 手写版为何失败 | 状态 |
|---|---|---|---|
| `TestSplitResizesTheNotesColumn` | 拖分隔条 → 笔记列宽变化；窗口变化后宽度被记住 | 固定宽度不响应拖动 | 有 |
| `TestOutlineFollowsTheCaret` | 光标进入某标题下 → 大纲把该项标为当前 | 手写列表没有当前节 | 有 |
| `TestFormatBarBoldsTheSelection` | 选中文本点 B → `doc.Formatting().Bold == FormatOn`，且落盘 | 没有格式条 | 有 |
| `TestViewModesSwitchPanes` | 编辑/分栏/阅读三个视图各自成立，切换器始终可达 | 面板里没有视图概念 | 有 |
| `TestMinimumSizeControls` | 命令面板的行在最小窗口可点，且点了会执行 | 手写行缺选择语义 | 有 |
| `TestPalette` | 输入收窄候选、键盘选中并执行 | 自绘行缺键盘/选择 | 有 |
| `TestNotebookFillsTheWindowHeight` | 纸面填满工作区，而不是停在内容高度 | 内容高的纸面留下死白 | 有 |
| `TestThePageCountsCardsNotBlocks` | 页面只报卡数，不出现“blocks” | 数块会多报，且说的是实现 | 有 |
| `TestBrowseSortsByColumn` | 点表头 → 行序变化 | 手拼行没有排序 | 待建（S3） |
| `TestRelinkNarrowsAsYouType` | 输入收窄候选、Enter 选中、Esc 取消 | 自绘行缺焦点/键盘 | 待建（S3） |

配套文件 `docs/specs/ui-inventory.md`：一屏一块，每行写「需求 → 组件 → 核对的 MyGo 版本」。
评审看这张表，不看注释。

## 5. 迁移切片

**S1 · 外壳**（已完成）
`ui.Split` / `SplitPercent` 替换三栏，`ui.Outline` 替换大纲；删掉 `sidebarWidth`/`sidebarNarrow`/
`panelWidth` 三个手写列宽，宽度改成 split 自己记住的 `notesWidth`/`editorShare`。
`narrowWindow` + `panelShown` 保留：窄窗口自动收起面板是产品规则，不是布局计算。
验收：`TestSplitResizesTheNotesColumn`、`TestOutlineFollowsTheCaret`，最小宽度下文档仍可输入。

**S2 · 笔记页工具**（已完成的部分：视图切换、格式条、`⋯` 菜单；未做：字号、导出）
顶栏：`编辑/分栏/阅读` 视图（分栏用 `Split` + `secondPane` 点亮现成的阅读视图）、
`⋯` 溢出菜单（导入 Anki / 在光标处制卡 / 设置）。标题与保存状态在纸面与状态行，不进
顶栏（`ui-redesign.md` §2.1 已改判）。
格式条：`Toolbar` + 分组 + `Divider` + `richtext.Formatting()` 驱动按下态。
验收：`TestFormatBarBoldsTheSelection`、`TestViewModesSwitchPanes`。

**S3 · 列表**（剩余：browse、relink、标签；palette 已随 S2 完成）
`browse` → `ui.Table`；`relink` → `ui.Autocomplete`；标签 → `ToggleGroup`。
验收：`TestBrowseSortsByColumn`、`TestRelinkNarrowsAsYouType`（两个都待建）；palette 的门
是已存在的 `TestPalette`。

每片各自一个 PR，squash 合并；PR 正文写清「用了哪个组件、删了哪段手写、哪些测试证明它真的是组件在干活」。

## 6. 验收

```sh
go build ./... && go vet ./...
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
go tool mygo i18n check .
```

外加组件审计（`UI_DIR` 指到被改的视图包）：

```sh
UI=./internal/ui
# 命名的字面宽度也算：常量里出现 260 这类值得复审
grep -rnE '(Width|Height|FontSize|Padding|Gap)\s*(\w+)?\s*(float32)?\s*=\s*[0-9]' "$UI" --include='*.go'
# 手写选择器/表/分栏的痕迹
grep -rnE 'ButtonBase\(c\.Key|Width\(side\)|Width\(panelWidth\)' "$UI" --include='*.go'
```

## 7. 不在范围

- 换掉原生 UI 改用 web 模式（那是一次产品重写，与本规格无关）。
- 拷贝任何外部应用的代码：参考布局与内容完整度，实现只来自 MyGo 组件。
