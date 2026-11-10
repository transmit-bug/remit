# 应用级布局规则

这套规则回答一个窗口怎么排，而不是某一屏怎么拼。屏只声明自己属于哪种骨架，
宽度、居中、边距和最小窗口下的让位顺序都从 `internal/ui/design` 取。
窗口的外壳（轨道 / 顶栏 / 子导航 / 状态行）由 `internal/ui/shell` 一次画完，屏不画边框。

口径来源：`docs/specs/ui-redesign-v2.md` §2.1–§2.6、§7.4；实现在 `internal/ui/shell`，
逐屏核对在 `docs/verification/ui-acceptance.md`。

## 1. 三种骨架

| 骨架 | 用途 | 结构 |
|---|---|---|
| Paper | 笔记、统计、阅读、插件页 | `Scroll` → `Column(FillWidth, AlignItems Center, Padding)` → `Column(MaxWidth measure)` |
| Workspace | 文档（写 / 分栏 / 读） | 纸面（居中 measure）+ 可选面板：舞台放得下纸面 + 面板 + 两条边时面板是 `PanelWidth` 的列（`ui.Row` + `ui.Divider`），放不下就是浮在纸上的 `ui.Overlay`；文档里没有笔记列 |
| Session | 复习、阅读会话 | 一条居中的会话栏 + 一个 `Grow(1).Center()` 的舞台；舞台内容用同一 measure |

列表类页面（笔记、标签、资产、模板、摘录树、读队列、忆队列、卡片表、设置）是 Paper 骨架换一个
measure（§2 的 `ListWidth` / `BrowseWidth` / `SettingsWidth`），不另立第四种骨架。

全屏接管（`takingOver`：复习、阅读会话）与无库的欢迎页**不画外壳**，只保留会话自己的会话栏；
设置区没有库也画外壳——它是 app 的页，不是库的页（`App.framed`，`internal/ui/view.go:63`）。

空态、卡片、进度和总结都在同一个 measure 上，不再各自漂移。

## 2. 唯一宽度与高度来源

宽度、高度、断点都是 `internal/ui/design/scale.go` 的语义常量（数值同 v2 §7.4）：

| 常量 | 值 | 用途 |
|---|---:|---|
| `RailWidth` / `RailCollapsed` | 240 / 64 | 导航轨；≤`BreakpointNarrow` 收成图标条 |
| `FrameHeight` | 60 | 顶栏 |
| `SubnavHeight` | 44 | 区域子导航 |
| `StatusbarHeight` | 30 | 状态行 |
| `BarHeight` | 44 | 格式条（文档面） |
| `BreakpointNarrow` / `BreakpointSmall` | 980 / 760 | 折叠：轨道 / 纸面与评分（面板不随断点，见 §3） |
| `PaperWidth` | 760 | 笔记纸面、阅读、插件页 |
| `Gutter` / `GutterNarrow` | 48 / 20 | 纸面左右边距；读写两态在 ≤760 都取 `GutterNarrow`（那时块沟槽已改为段上一行，左边距不再给它让位） |
| `ReadingWidth` | 760 | 整篇阅读与分栏右侧 |
| `ReviewWidth` | 680 | 复习卡、会话栏、状态栏 |
| `SummaryWidth` | 520 | 复习完成总结卡 |
| `ListWidth` | 860 | 行式列表页：笔记、搜索、标签、模板、摘录树、读队列、忆队列、历史 |
| `BrowseWidth` | 1080 | 表与两栏页：卡片表、资产、断链 |
| `SettingsWidth` | 660 | 设置八页、预设页、复习范围对话框 |
| `MessageWidth` | 520 | 库错误页与空态的短句 |
| `WelcomeWidth` | 640 | 欢迎页的卡（只此一处） |
| `PanelWidth` | 288 | 文档右侧面板（大纲 / 反链），成列或 `ui.Overlay` |
| `NotesColumnWidth` | 260 | 主从页的左侧列（现在的用处是忆·类型 `ui.Split` 的初始宽；文档面已没有笔记列） |
| `FilterPaneWidth` | 200 | 卡片表左筛选栏 |
| `DisplayNarrow` | 30 | ≤760 的页面标题（`TypeDisplay 34` 降一档） |

屏内不得再写 `const width`。需要新宽度时先加语义常量，再只在一个骨架里使用。
`About` 的 360 是独立小窗口的尺寸，不是页面 measure，留在窗口装配处。

## 3. 外壳四件

```text
┌───────────┬──────────────────────────────────────────────────────────────┐
│ ◇ Remit   │ [⧉]  Remit / 忆 / 卡片          [搜索 ⌘K]        [⋯]         │ 60  FrameHeight
│           ├──────────────────────────────────────────────────────────────┤
│ ▣ 今日  22│ 队列  卡片 25  类型  预设  历史  断链                          │ 44  SubnavHeight
│ ✎ 写     8├──────────────────────────────────────────────────────────────┤
│ ▤ 读     3│                                                              │
│ ▥ 忆    19│                        舞台（§1 的骨架）                      │ Grow(1)
│ ▨ 统计    │                                                              │
│ ⚙ 设置    │                                                              │
│           │                                                              │
│ ⚡ 制卡脚本 │                                                              │
│ ⬚ 白板    │                                                              │
│           ├──────────────────────────────────────────────────────────────┤
│ ‹ 收起    │ 1,204 字 · 14 段 · 6 张卡 · 已保存 · #方法论 · 第 7 段         │ 30  StatusbarHeight
└───────────┴──────────────────────────────────────────────────────────────┘
     240                         （窗口宽 ≤980：64 的图标条）
```

- **轨道** 240（≤980 为 64）：`ui.Sidebar` + `ui.SidebarItem` + `ui.Badge`，插件页在底部一道
  hairline 之后的 `ui.SidebarSection`，没有插件页时整段不建。品牌行是 `ui.Row`（26 方标 +
  `Remit`），不是按钮。当前项的脸是 Mark 的浅底 + 强调色文字，键盘语义仍归 `Sidebar`
  （理由写在 `internal/ui/shell/shell.go` 的包注释）。
- **顶栏** 60：`ui.Toolbar`——轨道开关、`ui.Breadcrumbs`（`Remit / 忆 / 卡片`，末段不可点）、
  当前页自己的动作、`⋯` 溢出（`toolbar.overflow` 与 `note.overflow` 在这里合并）。窄窗口由
  `Toolbar` 把尾部控件收进 overflow。
- **子导航** 44：`ui.TabsBase` + 计数徽章，选中项 2 DIP 强调色下划线（`design.ModeSwitch`）。
  没有子页的区域（今日）整行不建，舞台拿回这 44。
- **状态行** 30：`ui.Row` + `ui.Text`（`TypeCaption`），段间 `ui.Divider`，整行不可聚焦。
  有话才画；没话不占高度。整行只在文档面（S11–S13）出现：`App.docStatus`
  只在「写」区域的文档里给内容，其余面返回空，整行不建。

折叠规则（断点见 §2）：

1. 窗口 ≤980：轨道收成 64 图标条，标签进 `.Tooltip`、计数 Badge 仍在；这个宽度下两个折叠控钮都不画——
   它们改不动任何东西（`shell.collapsed`、`shell.narrow`）。文档面板成不成列不随这个断点：它由
   舞台宽度算出（`panelFits`），1024 的舞台放不下纸面 + 面板，所以那里就是浮层。
2. 窗口 ≤760：纸面边距 `Gutter 48 → GutterNarrow 20`，页面标题 `TypeDisplay 34 →
   DisplayNarrow 30`（读写两态一样）；写作态的块沟槽从纸面左侧的 48 改到段落上方一行
   （`compactBlockRow`），复习四档评分排成 `ui.Grid` 两列。
3. 720×520：以上全部生效，顶栏尾部进 overflow，子导航仍在，整窗不得横向滚动
   （`TestNoHorizontalScrollAt720`）。

## 4. 复习与会话

- 会话栏（Review / Reading、Scope、阅读等待数）在 `ReviewWidth` 内居中。
- 复习状态栏（剩余、进度、键位）也在 `ReviewWidth` 内居中。
- 完成总结卡 `MaxWidth(SummaryWidth)` + `AlignSelf(Center)`；不得再以
  `ReviewWidth` 的左边为起点。
- 卡面、评分、总结共用同一水平轴；垂直方向由可用空间 `Center()`，窄窗口只
  收紧外边距，不改变轴心。
- 会话不画四件外壳；`Esc` 退出，焦点回到进入处的控件。

## 5. 最小窗口 720×520

让位顺序固定：

1. 轨道收成 64：图标可点、tooltip 补名、计数仍在；折叠控钮让位（§3）。
2. 顶栏尾部动作进 `ui.Toolbar` 的 overflow；面包屑与搜索按钮保留。
3. 子导航仍常驻，`ui.TabsBase` 自己横向滚动；没有子页的区域整行不建。
4. 文档面板放不下就成 `ui.Overlay`（连 1024 都是这一档，见 §3）；S05 的笔记列表自成一页，文档里没有笔记列。
5. 纸面：读写两态边距 48 → 20、标题降一档；measure 不缩。
6. 编辑器纸面的块沟槽改到段落上方一行（v2 §2.5 ⑤，`compactBlockRow`），菜单高度按行下余量收，
   不越出窗口底部。
7. 复习四档评分排成两列，四档仍都在窗内且可点；再窄时才由组件自身换行。
8. 浏览表宽于窗口时由页面自身滚动，不让整窗出现横向滚动。

## 6. 组件映射

| 需要 | 组件 |
|---|---|
| 导航轨（区域、插件页、计数） | `ui.Sidebar` + `SidebarItem` + `SidebarSection` + `Badge` |
| 路径回前段 | `ui.Breadcrumbs` |
| 窗口动作行、overflow | `ui.Toolbar` |
| 区域子页 | `ui.TabsBase` + `Badge` |
| 视图 / 队列切换 | `ui.SegmentedBase`（自定义紧凑外观） |
| 可拖栏 | `ui.Split` / `SplitPercent`（S13 分栏、忆·类型主从）；文档面板不是可拖的列 |
| 页面滚动 | `ui.Scroll` |
| 纸面 / 表单 measure | `Column.MaxWidth` + `AlignItems(Center)` |
| 状态行 | `Row` + `Text`（`TypeCaption`），不可聚焦 |
| 浮层 | `ui.DialogBase` / `Modal` / `Popover` / `Overlay` |

## 7. 现状与保留例外

已由外壳与 design 收口：

- 单条顶栏（brand + mode rail + context）已拆成轨道 / 顶栏 / 子导航 / 状态行四件；
  导航从 `TabsBase` 换成 `ui.Sidebar`（一个 tab stop、方向键、读屏角色都在组件里）。
- 状态行从纸面内 11px 一行搬到窗口底部通栏 30，段间细线。
- 七个页面宽度来源散落在各屏；现在统一到 `design`（§2）。
- 复习完成总结曾在 680 列内左对齐，视觉中心偏左约 84 DIP；现在是 `SummaryWidth` 居中
  （`TestSummaryCardIsCentered`）。
- 浏览页宽窗口下曾贴左；现在在 `BrowseWidth` 内居中。

保留例外：

- `About` 是独立 360×… 小窗，不是主窗口页面。
- 表格单元格编辑是内容能力缺口（#218），不是布局问题。
- 1024 的文档面是纸面 + 浮层：纸面保住 760 的 measure，面板是窗口右缘的 `ui.Overlay`；
  约 1384 以上面板才是列（`TestPaperKeepsItsMeasureAt1024`、`TestPanelBecomesAnOverlayAt1024`、
  `TestPanelTakesAColumnOnAWideWindow`）。
- 文档面板是固定 288 的列（`ui.Row` + `ui.Divider`），不是可拖的 `ui.Split`；可拖的只有分栏
  （`ui.SplitPercent`）。
- 评分在 ≤760 是两列，不是四档一行。

## 8. 验收

- `TestRailCollapsesBelow980`、`TestRailChoosesAnArea`、`TestRailFoldsFromItsOwnControls`、
  `TestFoldedRailOffersItsWayBack`：轨道 240 / 64、按区域选择、两个折叠方向。
- `TestSubnavKeepsTheAreaOnReload`、`TestSubnavHiddenForAnAreaWithoutPages`：子导航 44 与
  「无子页不建行」。
- `TestStatusLineOnlyWhenItSaysSomething`、`TestBreadcrumbGoesBackToAnEarlierSegment`：状态行
  与面包屑。
- `TestAppBarAtMinimumSize`、`TestOverflowMenuDrawsTheToolbarAndNoteSlots`：顶栏 60 与 720×520
  的 overflow。
- 笔记列的三列测试随笔记列删除，改由 `TestDocumentHasNoNotesColumn`、`TestPaperKeepsItsMeasureAt1024`、
  `TestPanelBecomesAnOverlayAt1024`、`TestPanelTakesAColumnOnAWideWindow` 守「纸面 + 面板」与成列阈值；
  `TestGutterMovesAboveTheParagraphAt760`、`TestNarrowWindowDropsThePanel` 守 ≤760。
- `TestSummaryCardIsCentered`：完成总结中心与窗口中心的误差 ≤ 1 DIP，1024×720 与 720×520 各跑一次。
- `TestNoHorizontalScrollAt720`、`TestMinimumSizeControls`、`TestChineseAtTheMinimumSize`：最小窗
  的整窗约束。
