# UI inventory · S01–S52

一面一行。规则与执法在 `docs/specs/ui-composition.md`；面的定义在
`docs/specs/ui-redesign-v2.md` §3/§4。本表是 v2 §0.1 要求的那份重写：主键从「屏」改成「面」，
旧的 Shell / Notebook / Review / Browse / Stats and settings 五节不再存在。

- 版本：`github.com/egoist/mygo` **v0.8.1**（`go list -m -f '{{.Version}}'` 是 v0.8.1）。
  每一行的组件都在这份源码里查过；「缺」的行写明查过哪几个组件（见 §已知缺口）。
- 状态列写**谁把它做成现在这样**：切片号（W1–W9，W3a/W3b、W5a/W5b、W8a/W8b；后端 B1–B6），
  或「既有 / 某片改」。切片定义见 v2 §8。
- 验收测试名都能 `grep -q '^func <名字>(' --include='*_test.go' internal/` 核到；v2 §9.1 点名的那些是同一份，
  各行后面并列的补充测试（批量、CSV、白板节点 / 边等）是逐面核对时加上的，不在 §9.1 表里。
- 改一行的同一次改动就改这张表；不攒。

## 面与组件

| 面 | 需求 | 官方组件 | MyGo | 状态 | 验收测试 |
|---|---|---|---|---|---|
| S01 欢迎 | 无库时的三条入口（新建 / 打开 / 导入）与最近库，库打不开时说原因 | `ui.ButtonBase`（门，高 `DoorHeight`）+ `ui.List`（最近库）+ `ui.Divider` + `kit.Notice` | v0.8.1 | 既有 `screens/start`；t23 p2 补三门与最近库 | `TestWelcomeOffersThreeDoors`、`TestWelcomeShowsWhyALibraryFailed` |
| S02 导入向导 | 选来源 → 预览 → 完成，空来源下一步禁用并写原因 | `ui.DialogBase` + `ui.Table`（项 / 值预览）+ `ui.Switch` + `ui.Badge`（步骤条，`ui.Stepper` 是数值步进） | v0.8.1 | W8a | `TestImportWizardWalksThreeSteps`、`TestImportWizardRefusesAnEmptySource` |
| S03 导出 | Markdown 目录 / `.remitlib` / 卡片 CSV，选项随选择变，不可写的写原因 | `ui.DialogBase` + `ui.RadioGroup` + `encoding/csv`（`cards.csv`） | v0.8.1 | W8a；卡片 CSV 与「保留现有笔记」同批（`003ab4f`） | `TestExportWritesAndReports`、`TestExportWritesTheCardCSV`、`TestCardCSVEscapesWhatItCarries`、`TestExportRefusesTheCSVWithoutCards` |
| S04 今日 | 今日混合队列（读 / 摘 / 忆）、预算、每日笔记 | `ui.List` + `kit.Progress`（预算条，`MeterLevels` 三档）+ `kit.StateBadge`（超预算）+ `kit.Stat` + `kit.Empty` | v0.8.1 | W2（队列类型来自 B1） | `TestTodayOrdersReadingBeforeReview`、`TestTodayMarksWhatIsOverBudget` |
| S05 写·笔记 | 列表 + 筛选 + 排序 + 行菜单 | `ui.List` + `ui.SearchField` + `ui.ToggleGroup` / `ui.Toggle` + `ui.PopoverMenuButton` | v0.8.1 | W3a | `TestNotesListShowsSummaryAndCards`、`TestNotesListFiltersByTag` |
| S06 写·搜索 | 跨笔记与卡片的搜索，命中标出片段 | `ui.SearchField` + `ui.List` + `ui.RichText` | v0.8.1 | W3a | `TestSearchGroupsNotesAndCards`、`TestSearchFromAnotherAreaPutsTheCaretInTheField` |
| S07 写·标签 | 标签与计数，点开进筛过的列表 | `ui.List` | v0.8.1 | W3a | `TestTagPageOpensTheFilteredList` |
| S08 写·资产 | 资产表、被谁引用、孤儿文件 | `ui.Table` | v0.8.1 | W3a | `TestAssetsListNamesTheReferencingNotes` |
| S09 写·模板 | 这库里能当模板的笔记，从它新建 | `ui.List` | v0.8.1 | W3a | `TestTemplateStartsANoteWithoutTheMarker` |
| S10 写·摘录树 | 摘录的层级、折叠、孤儿 | `ui.Tree` + `ui.TreeItem` | v0.8.1 | W3a | `TestExcerptTreeNestsChildren`、`TestExcerptTreeListsOrphans` |
| S11 文档·编辑 | 标题、正文、块沟槽、选区动作、纸面 + 可选面板 | `ui.Row`（纸面 + 固定列，或纸面 + `ui.Overlay`）+ `ui.TextInput`（无边框）+ `richtext.EditorBase` + `ui.ButtonBase`（沟槽 / 段上行）+ `ui.PopoverBase`（选区条） | v0.8.1 | W3b；文档返工（`8366602`）去掉笔记列与文档子导航，≤760 沟槽改段上一行 | `TestDocumentHasNoNotesColumn`、`TestPaperKeepsItsMeasureAt1024`、`TestGutterMovesAboveTheParagraphAt760`、`TestPanelShortcutIsCmdShiftP`、`TestDocGutterBadgeOpensTheBlockInspector` |
| S12 文档·阅读 | 只读正文、块标记、当前块 | `ui/markdown.View` + `ui.ButtonBase`（块行，`RoleListItem`） | v0.8.1 | W3b | `TestReadModeMarksCardsAndExcerpts`、`TestReadModeExcerptsTheCurrentBlock` |
| S13 文档·分栏 | 左写右读，中线可拖、可复位 | `ui.SplitPercent` + `richtext.EditorBase` + `ui/markdown.View` | v0.8.1 | W3b | `TestViewModesSwitchPanes` |
| S14 查找条 | 文档内查找、命中计数 | `ui.FindBar` | v0.8.1 | W3b | `TestFindBarCountsMatches` |
| S15 选区动作条 | 选中文字后挖空 / 制卡 / 摘录整段 | `ui.PopoverBase` + 图标 `ui.Button`（`RoleToolbar`） | v0.8.1（`richtext` 不给选区矩形 → 贴段落） | W3b | `TestSelectionBarExcerptsTheWholeBlock`、`TestSelectionBarClozesTheBlockAndMakesACard` |
| S16 块菜单 | 一段的原地动作，删除先问 | `ui.Popover` + `ui.ButtonBase`（`RoleMenuItem`，分组 + 分隔线）+ `ui.AlertDialog` | v0.8.1 | 既有（block-tools）；W3b 加检查器；W9 加 `note.block` 插件项 | `TestBlockMenuOffersTheInspector`、`TestBlockMenuKeepsTheLastBlock` |
| S17 插入菜单 | 段首 `/` 过滤命令并插入 | `ui.PopoverBase` + `ui.ButtonBase`（`RoleOption`） | v0.8.1 | 既有；W9 加 `editor.slash` 插件项 | `TestSlashMenuInsertsTable` |
| S18 卡片编辑器 | 字段随卡片类型，冻结与删除 | `ui.DialogBase` + `ui.TextArea` + `ui.Select` + `ui.Switch`（冻结）+ `ui.AlertDialog`（删除） | v0.8.1 | 既有；W8b 重做 | `TestCardEditorWritesTheFieldsOfItsType`、`TestCardEditorFreezesAndDeletesTheCard` |
| S19 卡片详情 | 来源块预读、调度史、冻结 | `ui.DialogBase` + `ui.List`（调度史）+ `ui.Text`（来源段，`Sunken` 底） | v0.8.1 | 既有；W8b 重做 | `TestCardDetailListsTheScheduleHistory` |
| S20 块检查器 | 块的三重角色：预览 / 卡片 / 摘录 | `ui.DialogBase` + `ui/markdown.View` + `ui.Badge` | v0.8.1 | W3b | `TestInspectorShowsTheThreeRoles`、`TestInspectorOffersTheWhiteboardWhenANodeExists` |
| S21 大纲面板 | 标题树，当前节随光标 | 固定列 `PanelWidth 288`（`ui.Row` + `ui.Divider`，不是可拖的 `ui.Split`；舞台不够时 `ui.Overlay`）+ `ui.Segmented` + `ui.Outline` | v0.8.1 | 既有面板；W3b 改宽度与窄窗浮层；文档返工（`8366602`）成列阈值改由舞台算 | `TestOutlineFollowsTheCaret`、`TestPanelTakesAColumnOnAWideWindow`、`TestPanelBecomesAnOverlayAt1024`、`TestPanelBecomesAnOverlayWhenNarrow`、`TestPanelShortcutIsCmdShiftP` |
| S22 反链面板 | 谁摘录了这篇，点开到段 | `ui.Segmented`（同栏第二段）+ `ui.List` | v0.8.1 | 既有面板；W3b | `TestPanelSwitchesToBacklinks`、`TestBacklinkOpensTheSourceAtItsParagraph` |
| S23 读·队列 | 排队、段落进度、开始 / 继续 | `ui.List` + `ui.Meter` + `kit.Empty` | v0.8.1 | W4 | `TestReadingQueueOpensTheSession`、`TestReadingQueueAddsANoteFromTheList` |
| S24 读·归档 | 归档 / 不再出现两段，逐条恢复 | `ui.List` + `ui.PrimaryButton` | v0.8.1 | W4（事件来自 B2） | `TestArchiveRestoresANoteToTheQueue`、`TestDismissedNotesStayOutUntilRestored` |
| S25 阅读会话 | 全屏三行：会话栏 / 舞台 / 动作行 | 会话骨架 + `kit.Progress`（段进度，`MeterWidth`）+ `ui.ButtonBase`（块行） | v0.8.1 | W4（B2 的终态） | `TestReadingSessionArchivesANote`、`TestReadingSessionPostponesToTomorrow` |
| S26 会话·摘录并制卡 | 勾选后 `E` 摘录同时制卡 | `ui.Checkbox` | v0.8.1 | W4 | `TestExcerptFromTheReadingScreen`、`TestExcerptLeavesTheReaderInPlace` |
| S27 忆·队列 | 按笔记分组的今日复习量、筛选、保存筛选 | `ui.ToggleGroup` / `ui.Toggle` + `ui.List` + `kit.Card` + `kit.StackBar`（三段占比）+ `kit.Heatmap` | v0.8.1 | W5a | `TestRecallQueueGroupsByNote`、`TestRecallFilterNarrowsTheQueue`、`TestSavedFilterSurvivesRestart` |
| S28 复习范围 | 全库 / 笔记 / 标签三选，0 张时禁用 | `ui.DialogBase` + `ui.Segmented` + `ui.RadioGroup` / `ui.Radio` | v0.8.1 | W5a | `TestScopeDialogNarrowsTheSession` |
| S29 复习会话 | 题面、答案、来源、四档评分 | `ui.Text`（`maths.Render` 转写）+ `ui.Grid`（评分）+ `kit.Keycap` + `ui.Progress` | v0.8.1 | 既有；W5a 加来源行与上下文 | `TestReviewShowsTheSourceOfTheCard`、`TestReviewExpandsTheContext` |
| S30 复习总结 | 三个数字 + 没想起来的按来源分组 | `kit.Card` + `kit.Stat` + `ui.PrimaryButton` | v0.8.1 | W5a | `TestSummaryGroupsFailedCardsByNote`、`TestSummaryCardIsCentered` |
| S31 忆·卡片 | 卡片表：排序、筛选栏、搜索语法、多选、批量（暂停 / 改到期 / 改类型 / 冻结 / 埋藏同类 / 删除 / 加标签 / 复习所选） | `ui.Table`（`Sortable`/`Fixed`/`CellMenu`）+ `ui.Split`（筛选栏）+ `ui.SearchField` + `ui.Select`（窄窗）+ `ui.PopoverMenuButton`（批量菜单） | v0.8.1 | W5b；批量（`10fb795`） | `TestBrowseSortsByColumn`、`TestCardSearchSyntaxNarrows`、`TestCardTableBulkSuspends`、`TestCardTableBulkMenuOffersEveryWrite`、`TestCardTableBulkChangesTheType`、`TestCardTableBulkBuriesTheSiblings`、`TestCardTableBulkDeletesAndKeepsTheHistory`、`TestCardTableReviewsOnlyTheRowsTicked` |
| S32 忆·类型 | 主从：类型列表 + 字段、模板与两张卡面预览 | `ui.Split` + `ui.List` + `ui.Form` / `ui.Field` + `ui.TextInput` / `ui.TextArea` + `ui/markdown.View` | v0.8.1 | W5b（从设置区迁出） | `TestTypesPagePreviewsTheTemplate`、`TestTypesPageKeepsBuiltinsUndeletable` |
| S33 忆·预设 | 四组参数写库配置，「恢复默认」先问 | `ui.Form` + `ui.Fieldset` + `ui.StepSlider` + `ui.Select` + `ui.Segmented` + `ui.AlertDialog` | v0.8.1 | W5b（写入来自 B3） | `TestPresetsWriteTheLibraryConfig`、`TestPresetsResetToDefaults` |
| S34 忆·历史 | 四个数字、评分分布、今天的时间线、本周表 | `ui.List` + `ui.Table` + 组合 `gradeBar`（`ui.Row` + `ui.Box`；`ui.chart` 对四项过重） | v0.8.1 | W5b（数据来自 B4） | `TestHistoryListsTodaysReviews`、`TestHistoryCountsGrades` |
| S35 忆·断链 | 两栏重连：断链卡 / 笔记段落，收窄候选 | `ui.List` + `ui.Autocomplete` + `ui.AlertDialog`（全部重连） | v0.8.1 | W5b | `TestRelinkReattachesALostCard`、`TestRelinkNarrowsAsYouType`、`TestRelinkThawsAFrozenCard` |
| S36 统计·总览 | 数字、保留率曲线、时间配比、插件块 | `ui/chart`（`Plot`/`Axes`/`Rect`/`Area`/`Line`）+ `kit.ProgressRatio` + `kit.Heatmap`（26 周缩略，经 `calendar`）+ `kit.Stat` | v0.8.1 | W6（数据来自 B4） | `TestStatsShowsProductionAndRatio`、`TestStatsBlocksComeFromPlugins` |
| S37 统计·热力 | 日历网格、图例、数据来源 | 组合 `kit.Heatmap`（`ui/chart` 无日历热力，`ui.GridView` 无等宽列） | v0.8.1（缺，见缺口 1） | W6 | `TestHeatmapExplainsItsSource` |
| S38 统计·预测 | 14 天柱，读与忆合并的理由 | `ui/chart`（`barPlot`：`Plot` + `Rect`）+ `kit.Stat` | v0.8.1 | W6 | `TestForecastMergesReadingAndCards` |
| S39 设置·外观 | 明暗三选、纸面宽度、正文尺寸只读 | `ui.Segmented` + `ui.StepSlider` | v0.8.1 | W7（`manual`：只证明 fake，桌面外观要人上真机） | `TestSettingsAppearanceWritesTheOverride` |
| S40 设置·复习 | 保留率、最大间隔、间隔预览、摘录后回原位 | `ui.StepSlider` + `ui.Switch` | v0.8.1 | W7 | `TestSettingsReviewWritesThePresets` |
| S41 设置·预算 | 张数、分钟、今日分配预览、每日提醒 | `ui.StepSlider` + `ui.Switch` + `kit.Progress` | v0.8.1 | W7 | `TestSettingsBudgetPreviewMatchesToday` |
| S42 设置·快捷键 | 只读键表，自定义禁用并写明 #155 | `ui.Fieldset` + `ui.Field` + `kit.Keycap`（四组键行，无输入框） | v0.8.1 | W7 | `TestSettingsKeysAreReadOnlyWithAReason` |
| S43 设置·插件 | 每插件一行：能力、启用、问题、重新启用 | `ui.Fieldset` + `ui.Field` + `ui.Switch` + `ui.Badge`（能力）+ `ui.RadioGroup`（插件角色与设置） | v0.8.1 | W7；W9 把三次失败隐藏与预留槽位列进「问题」 | `TestPluginsShowCapabilitiesAndFailures`、`TestReenableBringsBackAHiddenContribution` |
| S44 设置·Agent | 监听地址、开关、token、路由表 | `ui.Switch` + `ui.Button` + `ui.Table` | v0.8.1 | W7（`manual`：真端口与 token 要人上真机） | `TestAgentSwitchesNow` |
| S45 设置·更新 | 版本、自动检查、重建索引、导出整库 | `ui.Switch` + `ui.Button` | v0.8.1 | W7（`manual`：真实更新源） | `TestCheckNow`、`TestReindexKeepsTheLibrary` |
| S46 设置·关于 | 品牌、库位置、三个计数、致谢 | `kit.Card` + `kit.Stat` + `ui.Fieldset` | v0.8.1 | W7 | `TestAboutShowsTheLibraryFacts` |
| S47 插件详情 | 能力、信任级别、启停、失败计数 | `ui.DialogBase` + `ui.Badge` + `ui.Switch` | v0.8.1 | W9 | `TestPluginDetailShowsCapabilitiesAndFailures` |
| S48 快捷键总表 | 四组键表 | `ui.DialogBase` + `ui.Grid` | v0.8.1 | W8a | `TestKeysHelpListsTheMap` |
| S49 命令面板 | 分组命令、收窄、执行、焦点还回 | `ui.DialogBase` + `ui.List` | v0.8.1 | 既有；W8a 加分组与插件命令 | `TestPalette`、`TestPaletteCommandsForReading` |
| S50 插件页·制卡脚本 | 插件给数据，宿主画页头与节点 | 宿主节点：`ui.Grid`/`ui.List`/`ui.Table`/`ui.Tree`/`chart.Plot` 等 + `ui.Spinner` 骨架 | v0.8.1 | W8b 页头；W9 槽位 | `TestPluginPageShowsItsHeaderActions` |
| S51 插件页·白板 | 块摊成一张连接图，点节点回原文 | 组合 `kit.Board`（节点是真 `ui.ButtonBase`，`Absolute` 放在插件给的位置；边由 `Draw` 画在卡面下；最小窗落 `ui.List`；不用 `ui.NewCanvas`） | v0.8.1（见缺口 8） | W8b 起，W9 收口；连线（`58cf0be`） | `TestWhiteboardOpensTheBlock`、`TestWhiteboardSurvivesAChangedBlock`、`TestBoardPlacesItsNodesWhereThePagePutThem`、`TestBoardPaintsOneLinePerConnection`、`TestBoardNodesAreFocusableInDataOrder`、`TestBoardAtTheSmallestWindowIsAList`、`TestParsePageRefusesABrokenBoard` |
| S52 笔记行菜单 | 不打开笔记也能阅读 / 入队 / 重命名 / 删除 | `ui.PopoverMenuButton` + `ui.DialogBase` + `ui.TextInput` + `ui.AlertDialog` | v0.8.1 | W3a 行菜单；W9 插件项；t23 p2 重命名 / 删除 | `TestNoteRowMenuOffersReadingAndThePluginSlot` |

整窗三条：`TestNoHorizontalScrollAt720`、`TestChineseAtTheMinimumSize`、`TestMinimumSizeControls`。

## 已知缺口（不是违规）

组件缺口每条都写明查过哪些组件、哪个版本（一律 v0.8.1）；末尾两条不是组件缺口，是产品口径
与内容能力。

1. **日历热力** — 查过 `ui/chart`（无日历热力）、`ui.GridView`（按最小宽排，没有等宽列与周对齐）
   → `internal/ui/kit/heatmap.go` 的组合。
2. **占比条 / 堆叠条** — 查过 `ui.Meter`（一个值一个色）、`ui/chart`（stack 只对 column/area 有效）
   → `kit.ProgressRatio`、`kit.StackBar`（`internal/ui/kit/progress.go`、`bar.go`）。
3. **可为值增长的进度条** — 查过 `ui.Meter`（没有动画接口）→ `kit.Progress`（t22 起接线）。
4. **提示条（callout）** — 查过 MyGo 目录（无 callout / banner），`ui.Badge` 只装计数
   → `kit.Notice`。
5. **大数字 Stat、页头、节头** — 查过 `ui.Meter` / `ui.Badge` / `ui.Toolbar` / `ui.Fieldset` /
   `ui.TabsBase`（各自是别的意思）→ `kit.Stat`、`kit.PageHead`、`kit.SectionHead`。
6. **选区动作条的锚点** — `richtext`（v0.8.1）不暴露选区矩形 → 用 `VisibleParagraphs()` 的
   `ParagraphBox` 贴住当前段（`screens/notes/selection.go`）。
7. **分栏右侧只读** — `richtext`（v0.8.1）一个 `Document` 只能挂一个视图 → 右侧 `ui/markdown.View`。
8. **白板不是自由画布** — `ui.NewCanvas` / painter 能画，但节点会丢掉键盘、焦点与读屏，也会丢掉
   720×520 的收栈（v2 §10.1 第 8 条不做）→ `kit.Board` 的只读连接图：节点是宿主的 `ui.ButtonBase`，
   只有边是画的；拖拽 / 缩放 / 平移 / 就地增删都不提供（`TestBoardPlacesItsNodesWhereThePagePutThem`、
   `TestBoardNodesAreFocusableInDataOrder`）。
9. **导入向导的步骤条** — `ui.Stepper` 是数值步进 → `ui.Badge` + `ui.Divider` 组合（`internal/ui/import.go`）。
10. **评分分布条（S34）** — `ui.chart` 对四项过重、`ui.Meter` 只有一个值 → 局部 `gradeBar`
    （`internal/ui/screens/recall/history.go:160`）。
11. **卡片表的左筛选栏** — 查过 `ui.Sidebar`（会夺走表格的焦点）、`ui.ToggleGroup`（变成一个分段
    控件）→ `ui.ToggleBase` 行（`screens/recall/cards.go:440`）。
12. **窄窗筛选 Select 用固定宽** — v0.8.1 的 `ui.Select` 在首次布局时给增长的宽度做动画，降动效下
    不 settle → `narrowFilter` 固定 `NameColumnWidth`（`screens/recall/cards.go:521`）。
13. **`::` 制卡推导** — 产品口径（ADR 0002），不是渲染缺口。
14. **表格单元格编辑** — 内容能力缺口（#218），不是布局问题。

## 旧表已改的几行

旧的四节表里这几行与代码不符，重写时按代码改，不保留旧值：

| 旧行 | 旧值 | 现在 |
|---|---|---|
| 卡片面 | `ui/markdown.View` | `ui.Text` + `maths.Render` 的转写（`screens/review/review.go:463/492`） |
| 标签筛选 | `todo S3`（`ButtonBase` chip） | `ui.ToggleGroup` / `ui.Toggle`（W3a，`TestNotesListFiltersByTag`） |
| 卡片列表 | `todo S3`（`ListState` 手拼行） | `ui.Table`（W5b，`TestBrowseSortsByColumn`） |
| 反链面板 | `todo S3`（`ButtonBase` 循环） | `ui.List`（W3b，`TestPanelSwitchesToBacklinks`） |
| 重连选择器 | `todo S3`（手拼行） | `ui.Autocomplete`（W5b，`TestRelinkNarrowsAsYouType`） |
| 统计热力图 | gap | `kit.Heatmap`（缺口 1） |
| 状态行 | `Row` + 11px 文字 | `shell/status.go` 的整窗状态行（W1） |
| 窗口导航 | `ui.Toolbar` 单条（brand + tab + context） | rail + 顶栏 + 子导航 + 状态行（W1，见 `docs/ui-layout-rules.md` §3） |
