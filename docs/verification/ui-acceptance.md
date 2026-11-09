# UI 验收 · 逐屏（t23 part 1）

对象：`docs/specs/ui-redesign-v2.md` §3 的 52 个面（S01–S52），一屏一节。核对基准是 `master`
`7b83d5f` 的代码与测试；本文件只记录**今天仓库里能用测试证明什么**，不改代码、不补测试。
每个测试名都用 `grep -q "^func <名字>(" --include='*_test.go' internal/` 核过，没核到的不写。

状态三值：

| 状态 | 判据 |
|---|---|
| `covered` | §9.1 点名的函数在仓库里存在，且断言的就是这屏的行为 |
| `missing` | 点名函数不存在，或这屏的元素 / 动作在代码里不存在 |
| `manual` | 有 headless 测试，但只证明了 fake（对话框、真实端口、网络、桌面外观）；结论要人上真平台 |

尺寸栏只写这屏自己的要求。通用三档（轨道 240/64、顶栏 60、子导航 44、状态行 30、720×520
无横向滚动）在「全局验收」里一次写完，逐屏不抄；§2.5 的折叠规则逐条在那一节核对。

# 全局验收

### 外壳四件与折叠

| 规则（spec） | 常量（`internal/ui/design/scale.go`） | 守卫测试 |
|---|---|---|
| 轨道 240，≤980 收成 64 图标条，计数仍在 | `RailWidth` / `RailCollapsed` / `BreakpointNarrow` | `TestRailCollapsesBelow980`（`internal/ui/shell_test.go`）、`TestRailChoosesAnArea`、`TestRailFoldsFromItsOwnControls`、`TestFoldedRailOffersItsWayBack`（`internal/ui/shell/shell_test.go`） |
| 顶栏 60，窄窗口尾部动作进 overflow | `FrameHeight` | `TestAppBarAtMinimumSize`、`TestMinimumSizeControls`（`internal/ui/app_test.go`）、`TestOverflowMenuDrawsTheToolbarAndNoteSlots`（`internal/ui/receivers_test.go`） |
| 子导航 44，`ui.TabsBase` 自己横向滚动；无子页的区域整行不建 | `SubnavHeight` | `TestSubnavKeepsTheAreaOnReload`（`internal/ui/shell_test.go`）、`TestSubnavHiddenForAnAreaWithoutPages`（`internal/ui/shell/shell_test.go`） |
| 状态行 30，`TypeCaption`，不可聚焦 | `StatusbarHeight` | `TestStatusLineOnlyWhenItSaysSomething`（`internal/ui/shell/shell_test.go`） |
| 720×520 整窗无横向滚动 | — | `TestNoHorizontalScrollAt720`（`internal/ui/shell_test.go`） |
| 面包屑回前段 | — | `TestBreadcrumbGoesBackToAnEarlierSegment`（`internal/ui/shell/shell_test.go`） |
| 焦点顺序：轨道 → 顶栏 → 子导航 → 舞台 → 浮层 | — | `TestPalette`（焦点还回原处）、`TestPanelBecomesAnOverlayWhenNarrow`、`TestBlockHandleAtCaretIsReachable` |

§2.5 五条折叠规则的实际状态：

| 条 | spec 要求 | 代码 | 判定 |
|---|---|---|---|
| ① | ≤980 轨道 64、label 传空、Tooltip 补名、Badge 仍显示 | `shell/rail.go` | 已实现，有测试 |
| ② | ≤980 文档面板收起、顶栏面板按钮改开 `ui.Overlay` | `notes/notes.go` + `notes/panel.go` | 已实现（`TestPanelBecomesAnOverlayWhenNarrow`、`TestNarrowWindowDropsThePanel`） |
| ③ | ≤760 纸面边距 48 → 20、`TypeDisplay 34 → 30` | 只有 `DisplayNarrow` 用在 `notes/read.go:166`；`GutterNarrow` 定义了但没有任何视图引用 | **部分未实现**（见缺口 #6） |
| ④ | ≤760 复习评分改两列 `ui.Grid` | `grep BreakpointSmall internal/ui/screens/review` 为空，四档仍是单行 `ui.Row`（`review.go:499`） | **未实现**，无测试（缺口 #7） |
| ⑤ | ≤760 块沟槽从左侧 48 沟槽改为段落上方一行 | 代码保留 48 沟槽，改由 `TestPaperKeepsGutterAtMinimumSize` / `TestBlockHandlesAtMinimumSize` 守「最小窗仍可点」 | **口径改判**（缺口 #8） |

### 组件闸门（`docs/specs/ui-composition.md` §4 + §9.1）

| 闸门 | 断言的能力 | 测试 | 状态 |
|---|---|---|---|
| Split 拖动 | 拖分隔条 → 笔记列宽变；窗口变化后记住 | `TestSplitResizesTheNotesColumn`、`TestThreeColumnsAtTheDefaultSize`（`internal/ui/notes_layout_test.go`）、`TestPanelTakesAColumnOnAWideWindow`（`internal/ui/workbench_test.go`） | 有 |
| Outline 当前节 | 光标进入某标题 → 大纲把它标为当前 | `TestOutlineFollowsTheCaret`、`TestOutlineJumpsToItsParagraph`（`internal/ui/panel_test.go`） | 有 |
| Table 排序 | 点表头 → 行序变化，再点反向 | `TestCardTableSortsByColumn`（`internal/ui/recall_test.go`） | 有（spec 旧名 `TestBrowseSortsByColumn` 不存在，缺口 #4） |
| Autocomplete 收窄 | 输入收窄候选、Enter 选中、Esc 取消 | `TestRelinkNarrowsAsYouType`、`TestRelinkPickerAtMinimumInChinese`（`internal/ui/recall_test.go`、`internal/ui/relink_test.go`）；候选本身 `TestCandidates`/`TestQuery`/`TestChoose`/`TestClose`/`TestEmpty`（`internal/ui/screens/relink/relink_test.go`） | 有 |
| ToggleGroup | 一组筛选里改一个 → 队列随之变化，且只留一个 | `TestNotesListFilterKeepsOnlyNotesWithCards`（`internal/ui/write_test.go`）、`TestRecallFilterNarrowsTheQueue`（`internal/ui/recall_test.go`）、`TestScopeDialogChoosesATag`（`internal/ui/review_scope_test.go`） | 有 |
| 命令面板 | 输入收窄、键盘选中并执行、焦点还回 | `TestPalette`、`TestPaletteGroupsCommandsNotesAndCards`、`TestPaletteListsPluginCommandsLast`（`internal/ui/palette_test.go`） | 有 |
| 最小窗控件可点 | 面板行在 720×520 可点且点了会执行 | `TestMinimumSizeControls`（`internal/ui/app_test.go`）、`TestOverlaysAtTheMinimumSizeInChinese`（`internal/ui/shortcuts_test.go`） | 有 |
| 页只报卡数不报块数 | 页面数字不说实现词 | **不存在**（缺口 #5） | 缺 |
| 纸面填满工作区 | 纸面填满，而非停在内容高度 | `TestNotebookFillsTheWindowHeight`（`internal/ui/notes_layout_test.go`） | 有 |

### i18n

- 目录：`locales/en.json` + `locales/zh-Hans.json`，生成 `locales/catalog_gen.go`；`go tool mygo i18n check .` 是闸门。
- 语言不得由第二条代码路径提供：`TestLocaleSwitchesTheLanguage`（`internal/ui/app_test.go`）、`TestNotesListDayFollowsTheFrameLanguage`（`internal/ui/write_test.go`）、`TestContributedPageFollowsTheLocale`（`internal/ui/contrib_test.go`）。
- 最小窗 + zh-Hans 的分面覆盖：`TestChineseAtTheMinimumSize`（`internal/ui/screens/reading/reading_test.go`）、`TestReadAreaInChineseAtTheMinimumSize`（`internal/ui/reading_screen_test.go`）、`TestTodayInChineseAtTheMinimumWindow`（`internal/ui/today_test.go`）、`TestWritePagesInChinese`（`internal/ui/write_test.go`）、`TestRecallPagesAtMinimumInChinese`（`internal/ui/recall_test.go`）、`TestStatsPagesZhHansAtMinimum`（`internal/ui/stats_test.go`）、`TestSettingsPagesAtMinimumInChinese`（`internal/ui/screens/settings/settings_test.go`）、`TestTypesPageInChineseAtMinimum`（`internal/ui/screens/cardtypes/cardtypes_test.go`）、`TestPluginSurfacesAtTheMinimumSizeInChinese`（`internal/ui/receivers_test.go`）、`TestSummaryKeepsItsActionAtTheSmallestWindow`（`internal/ui/screens/review/review_test.go`）。

### 暗色

`TestDark`（`internal/ui/app_test.go`）把每个页面在墨色下各画一遍；`TestEveryModeInBothAppearances`（`internal/ui/theme_test.go`）、`TestEveryControlInBothAppearances`（`internal/ui/kit/kit_test.go`，控件与焦点环）；设计层的取值由 `internal/ui/design/theme_test.go` 的 `TestThemeResolvesAppearance`、`TestThemeIsComplete`、`TestSemanticContrast`、`TestTextContrast` 守。

### 减少动效

`TestReducedMotionSwitchesAtOnce`、`TestMotionSettles`、`TestHoverAndPressKeepTheLayout`（`internal/ui/motion_test.go`）；`design.Press` 0.98 / `design.Hover` 由 `internal/ui/design/motion_test.go` 的 `TestOnlyTheEntranceBounces`、`TestMomentDurations` 定口径。降动效下选择性控件不得因动画不收敛而卡住：`narrowFilter` 用固定宽度而非 Grow，注释写在 `internal/ui/screens/recall/cards.go:519`。

### 人工交接（headless 测不到）

按 `docs/verification/handoff.md` 跑，并追加 §9.4 的条目：

1. 轨道折叠：拖到 960 宽 → 64 图标条、图标可点、tooltip 有名字；拉回 1200 恢复。
2. 白板：点节点跳到阅读态对应段落；从块菜单回白板，节点高亮。
3. 插件失败面板：改坏 `examples/plugins/leeches` 的页面数据 → 页面错误态；三次后导航项消失，设置页仍有行且「重新启用」能恢复。
4. 归档 / 不再出现：A/D 后到归档页恢复，笔记回今日；重启后状态保持。
5. 预设：改保留率 → 关窗重开，值还在；手改 `remit.json` 加未知键，重开不丢。
6. 深浅外观：系统切换 → 窗口即时跟随；显式选「纸面」后不再跟随。
7. 无障碍：读屏读出轨道计数、表头、评分键位；焦点环深浅两套都可见。
8. 本文件标 `manual` 的三面（S39 / S44 / S45）连同 §4.1 的原生对话框（打开库、导入、导出、选图）在这里各走一次。

不做像素基线；`tt.Image()` 只供 PR 正文附图（§9.5）。

# 逐屏

## S01 · 欢迎

- 代码：`internal/ui/screens/start/start.go`、`internal/ui/library.go`（`startView`、`libraryErrorView`）。
- 目的：没有库时的窗口——说清库会放哪，给开始的路。
- 元素：图标 + 标题 + 一句 + 目录 + 两个按钮（新建库 / 打开库）。**缺**：导入门、最近库列表（`settings.recent` 字段）、卡内 danger callout、「零遥测」页脚、`WelcomeWidth` 卡（常量在 `design/scale.go:53`，仅注册进 Tokens）。
- 动作：`Deps.Create` / `Deps.Choose`；库打不开时是**另一页** `libraryErrorView` 的「重试」，不是欢迎卡里的 banner。
- 键：无。
- 尺寸：1024×720 居中 `MessageWidth`；720×520 与 zh-Hans 均无测试。
- 守卫：`TestNoLibraryYet`、`TestPlaceComesFromTheApp`、`TestActionsGoToTheApp`（`internal/ui/screens/start/start_test.go`）；`TestFirstRun`、`TestOpenLibraryPicker`、`TestOpenLibraryPickerCancelled`、`TestStartViewWithNoSetting`（`internal/ui/onboarding_test.go`）。
- 状态：**missing**。§9.1 的 `TestWelcomeOffersThreeDoors` / `TestWelcomeShowsWhyALibraryFailed` 都不存在，且 §4.1 的三门、最近库、卡内 callout 在代码里没有。

## S02 · 导入向导

- 代码：`internal/ui/import.go`（`importView`、`importSteps`、`importPreviewStep`）。
- 目的：把三种来源（`.remitlib` / Markdown 目录 / Anki `.apkg`）带进库，并在写之前给人看计数。
- 元素：`ui.DialogBase` + 步骤条（`ui.Badge` + `ui.Divider` 组合，理由：`ui.Stepper` 是数值步进）+ 三个来源行 + 预览 `ui.Table`（项 / 值）+ 两个 `ui.Switch` + 完成页。
- 动作：`ChooseArchive` / `ChooseMarkdown` / `ChooseAnki` → 预览 → 下一步（计数 0 时 `Disabled` 并写原因）→ 写库 → 重开库。
- 键：`Esc` 关。
- 尺寸：`SettingsWidth` 内的对话框；720×520 无专门测试。
- 守卫：`TestImportWizardWalksThreeSteps`、`TestImportWizardRefusesAnEmptySource`、`TestImportWizardReportsAFailure`、`TestImportWizardUnpacksAnArchive`（`internal/ui/import_test.go`）；Markdown 路径 `TestImportMarkdownFromTheWizard`、`TestImportMarkdownFailure`（`internal/ui/importmd_test.go`）。
- 状态：**covered**。口径差：spec 第二个开关是「保留现有笔记」，代码是 `ImportQueue`「加入阅读队列」（`import.go:447`，缺口 #9）。

## S03 · 导出

- 代码：`internal/ui/export.go`。
- 目的：把库写成 Markdown 目录或 `.remitlib`，并说清哪一个不能写。
- 元素：`ui.DialogBase`（`SummaryWidth`）+ `ui.RadioGroup` 三项（Markdown / `.remitlib` / 卡片 CSV）+ 每项下的一行说明或禁用原因 + 取消 / 主按钮。
- 动作：`runExport` → `pickFolder` → `importmd.Export` 或 `writeArchive`；成功 `c.Toast`；空库两项都禁用并写原因，CSV 永久禁用。
- 键：`Esc` 关。
- 尺寸：`SummaryWidth` 对话框；两档宽无专门测试。
- 守卫：`TestExportWritesAndReports`、`TestExportRefusesTheCSVItDoesNotHave`（§9.1 点名）；另 `TestExportWritesTheLibraryArchive`、`TestExportDisabledOnAnEmptyLibrary`（`internal/ui/export_test.go`）。
- 状态：**covered**。导出中的 `ui.Spinner` 未实现、无测试（§4.1 只要求「按钮禁用并显示」，主按钮已禁用）。

## S04 · 今日

- 代码：`internal/ui/screens/today/today.go`、`internal/ui/today.go`。
- 目的：一屏回答「今天做什么、还有多久、从哪开始」。
- 元素：页头（H1 + 日期与预算副行 + 「命令」按钮）、预算卡（`TypeDisplay` 分钟 + `ui.Meter` 三档）、每日笔记行、三个 `kit.Stat`、队列 `ui.List`（读 / 摘 / 忆三类行，超预算加 `ui.Badge`）、「读过但还没摘」区块、`today.widget` 插件段、明日一行；空 → `kit.Empty`，错 → callout + 重试。
- 动作：行 → `OpenReading` / `OpenReadingAt` / `StartReview`；「命令」→ S49；预算卡不可点。
- 键：`⌘K`。
- 尺寸：纸面 `PaperWidth 760`；720×520 与 zh-Hans 由 `TestTodayInChineseAtTheMinimumWindow` 一并压。
- 守卫：`TestTodayOrdersReadingBeforeReview`、`TestTodayMarksWhatIsOverBudget`、`TestTodayCountsMatchTheRail`、`TestTodayOffersTheDailyNote`（`internal/ui/today_test.go`）；另 `TestTodayOpensAReadingLineAtItsParagraph`、`TestTodayControlsGoWhereTheySay`；屏级 `TestTodayDrawsTheDay`、`TestTodayEmptyDayOffersANote`、`TestTodaySaysWhyItCouldNotLoad`、`TestTodayDoesNotClaimAnUnloadedDayIsEmpty`、`TestTodayDrawsAPluginSectionOnlyWhenItHasItems`（`internal/ui/screens/today/today_test.go`）；插件槽 `TestTodayDrawsTheTodayWidgetSlot`（`internal/ui/receivers_test.go`）。
- 状态：**covered**。

## S05 · 写 · 笔记

- 代码：`internal/ui/screens/notes/list.go`、`internal/ui/write.go`（`loadNoteList`）。
- 目的：笔记的入口列表——标题、首行、读到哪里、喂了几张卡。
- 元素：页头（H1 + 「N 篇 · M 张卡」+ 新建）、工具行（`ui.SearchField` + `ui.ToggleGroup` 四个 `ui.Toggle` + 标签 chip + 排序 `ui.PopoverMenuButton`）、`ui.List` 行（标题 + `摘录` badge + 摘要 + 进度 / 卡数 badge + 相对时间 + 「阅读」「⋯」）；空 → `kit.Empty` + 清空筛选。
- 动作：行 → 打开文档；「阅读」→ 阅读态；`⋯` → S52；排序 / 筛选 → `Deps.Set…`；新建 → `Deps.NewNote`。
- 键：无专属（`⌘N` 新建，全局）。
- 尺寸：`ListWidth 860`；720×520 由 `TestWritePagesAtTheMinimumSize` 守，zh-Hans 由 `TestWritePagesInChinese` 守。
- 守卫：`TestNotesListShowsSummaryAndCards`、`TestNotesListFiltersByTag`、`TestNotesSortCyclesAndSurvivesReload`（`internal/ui/write_test.go`）；另 `TestNotesListFilterKeepsOnlyNotesWithCards`、`TestNotesList`（`internal/ui/sidebar_test.go`）。
- 状态：**covered**。

## S06 · 写 · 搜索

- 代码：`internal/ui/screens/notes/search.go`、`internal/ui/write.go`（`loadSearch`）。
- 目的：一次输入，两组结果（笔记 / 卡片），片段带强调。
- 元素：页头副行「『q』的 N 条结果」、`ui.SearchField`（自动聚焦）、笔记 / 卡片两个 `ui.List`、`SearchHit.Snippet` 转 `ui.RichText` 强调 span；空查询一句说明匹配范围。
- 动作：笔记行 → 打开；卡片行 → 打开它渲染自的笔记（S19 的入口）。
- 键：`⌘F`（其他面跳到这里）。
- 尺寸：`ListWidth`；720×520 由 `TestWritePagesAtTheMinimumSize` 覆盖。
- 守卫：`TestSearchGroupsNotesAndCards`、`TestSearchFromAnotherAreaPutsTheCaretInTheField`（`internal/ui/write_test.go`）；另 `TestSearchLink`。
- 状态：**covered**。

## S07 · 写 · 标签

- 代码：`internal/ui/screens/notes/tags.go`、`internal/ui/write.go`（`loadTagRows`）。
- 目的：标签是笔记的属性——回答「我有哪些标签、各多少篇」。
- 元素：`ui.List` 行（kind 图标 + `#tag` + 两个示例标题 + 「N 篇」）；空 → `kit.Empty` + 写法一句。
- 动作：行 → `Deps.OpenTag(tag)`（= S05 的 `tag=` 筛选）。
- 键：无。
- 尺寸：`ListWidth`；最小窗由 `TestWritePagesAtTheMinimumSize` 覆盖。
- 守卫：`TestTagPageOpensTheFilteredList`（`internal/ui/write_test.go`）；列表侧 `TestTagFilterNarrowsTheList`（`internal/ui/tags_test.go`）。
- 状态：**covered**。

## S08 · 写 · 资产

- 代码：`internal/ui/screens/notes/assetspage.go`、`internal/ui/screens/notes/assets.go`。
- 目的：文件随库走、按内容命名——回答「库里有哪些文件、被谁引用」。
- 元素：`ui.Table`（类型 badge / 文件 `TypeMono` / 大小 / 被引用 / 格式）、页头「导入」；空 → `kit.Empty` + 导入；孤儿文件仍列出并写「未被引用」。
- 动作：「导入」→ `Deps.ImportAssets`；行 → 打开第一篇引用它的笔记。
- 键：无。
- 尺寸：`BrowseWidth 1080`（代码与 §4.3 一致，与 §4.10 的 `ListWidth` 一行相抵，见缺口 #14）；最小窗由 `TestWritePagesAtTheMinimumSize` 覆盖。
- 守卫：`TestAssetsListNamesTheReferencingNotes`（`internal/ui/write_test.go`）；存图路径 `TestSVGAssetReachesTheEditor`、`TestDroppedPictureIsStoredAndInserted`、`TestPastedPictureClipboardIsStoredAndInserted`。
- 状态：**covered**。

## S09 · 写 · 模板

- 代码：`internal/ui/screens/notes/templates.go`、`internal/ui/notes.go`（`newFromTemplate`）。
- 目的：带 `#template` 的普通笔记只在新建时出现——这一页是它们的入口。
- 元素：页头（H1 + `#template` 写法一句 + 「空白笔记」）、`ui.List` 行（标题 + 「N 个块 · 首行」+ 「从模板新建」）；空 → `kit.Empty`。
- 动作：行 → `Deps.NewFromTemplate`（副本剥标记、块 ID 重铸）；「空白笔记」→ `Deps.NewNote`。
- 键：无。
- 尺寸：`ListWidth`；最小窗由 `TestWritePagesAtTheMinimumSize` 覆盖。
- 守卫：`TestTemplateStartsANoteWithoutTheMarker`（`internal/ui/write_test.go`）——§9.1 点名；既有 `TestNewNoteFromTemplate`、`TestNoTemplatesNoMenu`（`internal/ui/note_template_test.go`）。
- 状态：**covered**。

## S10 · 写 · 摘录树

- 代码：`internal/ui/screens/notes/tree.go`、`internal/ui/write.go`（`loadExcerptTree`）。
- 目的：摘录挂在被摘的笔记下，源没了的单列一段。
- 元素：`ui.Tree` + `ui.TreeItem`（折叠、键盘、层级、读屏在组件里）、每项「N 张卡 · M 条再摘」、孤儿段、页头「继续读」。
- 动作：点项 → 打开该摘录；「继续读」→ S23。
- 键：树自己的（↑↓ / Home / End）。
- 尺寸：`ListWidth`；最小窗由 `TestWritePagesAtTheMinimumSize` 覆盖。
- 守卫：`TestExcerptTreeNestsChildren`、`TestExcerptTreeListsOrphans`（`internal/ui/write_test.go`）。
- 状态：**covered**。

## S11 · 文档 · 编辑

- 代码：`internal/ui/screens/notes/notes.go`（`ui.Split` + `SplitReset`）、`editor.go`、`blocks.go`、`selection.go`。
- 目的：写。块是位置，身份在块 ID 里。
- 元素：笔记列 `ui.Split`（`NotesColumnWidth 260`）→ 编辑器 `ui.Split`（面板 `PanelWidth 288`）、标题 `ui.TextInput`（无边框 `TypeDisplay`）、正文 `richtext.EditorBase`、沟槽 48（`+` / `⋮⋮` / 状态徽章）、选区动作条（S15）。
- 动作：写回经 `Deps.SaveDocument`；块插入 / 移动 / 删除按块 ID；徽章 → S20；保存失败 → 就地提示 + 重试 + toast。
- 键：`⌘F` 查找、`/` 插入菜单、`⌘⇧P` 面板、`⌘Z`/`⇧⌘Z` 撤销重做。
- 尺寸：纸面 `PaperWidth 760`；720×520 保沟槽可点（`TestPaperKeepsGutterAtMinimumSize`、`TestBlockHandlesAtMinimumSize`），**未按 §2.5 ⑤ 改到段上一行**（缺口 #8）。
- 守卫：`TestDocGutterBadgeOpensTheBlockInspector`、`TestPaperKeepsGutterAtMinimumSize`（`internal/ui/screens/notes/workbench_test.go`）、`TestSelectionBarClozesTheBlockAndMakesACard`（`internal/ui/workbench_test.go`）；既有 `TestBlockMenuMovesParagraphWithItsIdentity`、`TestBlockGripDragReorders`（`internal/ui/screens/notes/block_actions_test.go`）；另 `TestSaveFailureOffersRetry`、`TestGutterBadgeSaysWhenACardStoppedFollowing`、`TestBlockHandleAtCaretIsReachable`。
- 状态：**covered**。

## S12 · 文档 · 阅读

- 代码：`internal/ui/screens/notes/read.go`、`internal/ui/screens/notes/images_test.go`（图片缓存）。
- 目的：读同一篇，但不改它。
- 元素：`ui/markdown.View` 只读正文、块编号常显、当前块高亮（`Accent.Alpha(0.10)` + 左侧 2px）、每块右标记列（N 卡 / 已摘 / 上次停在这里）、标题用 `DisplayNarrow`（≤760）。
- 动作：点块 → 设为当前块；`E` → `Deps.Excerpt`；`M` → 回编辑。
- 键：`E`、`⌘F`。
- 尺寸：`PaperWidth`；≤760 用 `DisplayNarrow`（`read.go:166`）。
- 守卫：`TestReadModeMarksCardsAndExcerpts`（`internal/ui/screens/notes/workbench_test.go`）、`TestReadModeExcerptsTheCurrentBlock`（`internal/ui/workbench_test.go`）；另 `TestReadModeChoosesItsParagraph`、`TestReadViewLoadsAnImageOnce`。
- 状态：**covered**。

## S13 · 文档 · 分栏

- 代码：`internal/ui/screens/notes/notes.go:421`（`ui.SplitPercent` + `SplitReset(defaultReadShare)`）。
- 目的：左写右读，中线可拖，50/50 可复位。
- 元素：`ui.SplitPercent`（`readShare`）、左 `richtext.EditorBase`、右 `ui/markdown.View`、中线 1px。
- 动作：拖中线；双击复位；两侧各自滚动。
- 键：无专属。
- 尺寸：每侧各自 measure；两档宽无专门测试。
- 守卫：`TestViewModesSwitchPanes`（`internal/ui/format_test.go`）、`TestViewModes`（`internal/ui/screens/notes/notes_test.go`）。
- 状态：**covered**。缺口（已记录在 `editor.go`）：`richtext` 一个 `Document` 只能一个编辑器，右侧只读。

## S14 · 查找条

- 代码：`internal/ui/screens/notes/editor.go:135`（`ui.FindBar`）。
- 目的：在打开的文档里找，数清命中，回车走一条。
- 元素：`ui.FindBar(c, &open, &query, matches, &current)`，占自身高度压在纸面上方；无命中「0 个结果」。
- 动作：下一个 / 上一个 / 关闭；`↵` 走到下一条并回卷。
- 键：`⌘F` 开、`Esc` 关。
- 尺寸：随纸面；720×520 由 `TestWritePagesAtTheMinimumSize` 覆盖。
- 守卫：`TestFindBarCountsMatches`（`internal/ui/workbench_test.go`）；另 `TestFindInTheOpenNote`、`TestFindStaysInTheOpenDocument`、`TestFindCloses`。
- 状态：**covered**。

## S15 · 选区动作条

- 代码：`internal/ui/screens/notes/selection.go`。
- 目的：选中一段字之后，就地能挖空、制卡、摘录整段。
- 元素：`ui.PopoverBase`（无箭头，贴当前段落）、三个图标按钮（挖空 / 制卡 / 摘录整段），`Role(RoleToolbar)`；选区 <2 字不出现。
- 动作：挖空 → `Deps.ClozeSelection(noteID, blockID, text)`（写回 `{{text}}` → 保存 → 制卡 → reload）；制卡 → `OpenCardEditor`；摘录整段 → `Deps.Excerpt`。
- 键：`Esc` 关（选区消失即关）。
- 尺寸：随段落；720×520 无专门测试。
- 守卫：`TestSelectionBarExcerptsTheWholeBlock`（`internal/ui/screens/notes/workbench_test.go`）、`TestSelectionBarClozesTheBlockAndMakesACard`、`TestSelectionBarClozesTheSelection`。
- 状态：**covered**。锚点是已记录的组件缺口：`richtext` v0.8.1 不暴露选区矩形，用 `VisibleParagraphs()` 的 `ParagraphBox`。

## S16 · 块菜单

- 代码：`internal/ui/screens/notes/blocks.go`。
- 目的：一段的原地动作：插入 / 复制 / 移动 / 转换 / 摘录 / 制卡 / 删除 / 检查器。
- 元素：`ui.Menu`（分组 + 分隔线 + `note.block` 插件项）、删除走 `ui.AlertDialog`（写明「N 张卡转为断链」）、最后一块删除项 `Disabled` + 菜单底部一行原因。
- 动作：上移 / 下移 / 复制 / 插到下方 / 八种转换 / 摘录 / 制卡 / 检查器 / 删除。
- 键：`Esc` 关。
- 尺寸：随段落；`TestSlashMenuAtMinimumSize` 守同类浮层的最小窗。
- 守卫：`TestBlockMenuOffersTheInspector`、`TestBlockMenuKeepsTheLastBlock`（`internal/ui/screens/notes/workbench_test.go`）、`TestDeleteBlockWarnsBeforeCardLoss`、`TestBlockMenuConvertsParagraph`、`TestBlockMenuDuplicatesParagraph`；插件槽 `TestBlockMenuDrawsTheNoteBlockSlot`（`internal/ui/receivers_test.go`）。
- 状态：**covered**。

## S17 · 插入菜单

- 代码：`internal/ui/screens/notes/slash.go`。
- 目的：段首 `/` 之后不再记 Markdown 语法。
- 元素：`ui.Popover` + `ui.List` 过滤命令、底部 `editor.slash` 插件项、无匹配「没有这个命令」。
- 动作：输入过滤、`↵` 插入（正文 / 标题 / 引用 / 代码 / 表格 / 公式 / 图片 / 卡片）。
- 键：`/` 开、`Esc` 关、上下选择。
- 尺寸：最小窗仍可用（`TestSlashMenuAtMinimumSize`）。
- 守卫：`TestSlashMenuInsertsTable`、`TestSlashMenuFiltersAndInsertsFormula`、`TestSlashMenuAtMinimumSize`、`TestSlashMenuInsertsPickedImage`（`internal/ui/screens/notes/block_actions_test.go`）；插件槽 `TestSlashMenuDrawsTheEditorSlashSlot`。
- 状态：**covered**。

## S18 · 卡片编辑器

- 代码：`internal/ui/cards.go`。
- 目的：为一段造卡或改卡，字段由类型模板决定。
- 元素：`ui.DialogBase`、每字段一个 `ui.TextArea`、类型 `ui.Select`（库里所有类型，含插件供给）、引用态 `ui.Badge`、「冻结」`ui.Switch`、删除走 `ui.AlertDialog`；无卡时字段空、删除禁用并写原因。
- 动作：`Deps.SaveCard` / `DeleteCard` / `FreezeCard`。
- 键：`Esc` 关。
- 尺寸：`SettingsWidth`；两档宽无专门测试。
- 守卫：`TestCardEditorWritesTheFieldsOfItsType`（§9.1）、`TestCardEditorFreezesAndDeletesTheCard`（`internal/ui/cards_test.go`）。
- 状态：**covered**。

## S19 · 卡片详情

- 代码：`internal/ui/cards.go`。
- 目的：这张卡从哪里来、被问过几次、下次什么时候。
- 元素：`ui.DialogBase`、来源块预读（`ui.callout` neutral）+「回到原文」、调度史 `ui.List`（时间 + 评分 + 间隔变化）、冻结 / 解冻、编辑卡面；无历史「还没有复习记录」。
- 动作：回原文 / 冻结 / 解冻 / 编辑 / 重链入口。
- 键：`Esc` 关。
- 尺寸：`SummaryWidth`；两档宽无专门测试。
- 守卫：`TestCardDetailListsTheScheduleHistory`（§9.1）、`TestCardBadgeReachesTheCardDetail`、`TestSeparatorMakesACard`（`internal/ui/cards_test.go`）。
- 状态：**covered**。

## S20 · 块检查器

- 代码：`internal/ui/screens/notes/inspector.go`。
- 目的：一段的三种角色（正文 / 卡片 / 摘录）在一处看清。
- 元素：三段（只读预览、卡片 N 张 +「为这一段制卡」、摘录 N 条）、页脚「在笔记里定位」/「在白板里定位」（仅当画布有该块节点）/「关闭」；三者皆无时三行说明 + 制卡入口。
- 动作：卡片行 → S19；摘录行 → 打开；制卡 → S18；定位 → 阅读态 / 白板高亮。
- 键：`Esc` 关。
- 尺寸：`SettingsWidth`；两档宽无专门测试。
- 守卫：`TestInspectorShowsTheThreeRoles`、`TestInspectorOffersTheWhiteboardWhenANodeExists`（`internal/ui/screens/notes/workbench_test.go`、`internal/ui/board_test.go`）。
- 状态：**covered**。

## S21 · 大纲面板

- 代码：`internal/ui/screens/notes/outline.go`、`internal/ui/screens/notes/panel.go`。
- 目的：这篇有哪些节，以及现在写在哪一节。
- 元素：`ui.Split` 第二栏（`PanelWidth 288`）、顶部 `ui.Segmented`（大纲 / 反链 N）+ 收起按钮（`.Tooltip`）、`ui.Outline`（当前节用强调色）；空 →「这篇还没有标题」+ 一句。
- 动作：点行 → `pendingJump` 滚到该段；分段切换 → 写 `PanelTab`；`⌘⇧P` 开关；≤980 → `ui.Overlay`。
- 键：`⌘⇧P`、`Esc`（Overlay）。
- 尺寸：288；≤980 收进 Overlay（`TestPanelBecomesAnOverlayWhenNarrow`、`TestNarrowWindowDropsThePanel`）。
- 守卫：`TestOutlineFollowsTheCaret`、`TestOutlineJumpsToItsParagraph`、`TestPanelScrollsOnItsOwn`（`internal/ui/panel_test.go`）、`TestPanelTakesAColumnOnAWideWindow`（`internal/ui/workbench_test.go`）。
- 状态：**covered**。

## S22 · 反链面板

- 代码：`internal/ui/screens/notes/panel.go`。
- 目的：谁引用了我。
- 元素：同一栏的第二个分段；`ui.List` 行三段（标题 / 首行 /「N 张卡 · 可再摘」）；空 →「还没有人摘录这篇」。
- 动作：行 → `Deps.OpenNote`；分段 ← → S21。
- 键：随面板。
- 尺寸：288 / Overlay 同上。
- 守卫：`TestPanelSwitchesToBacklinks`（`internal/ui/screens/notes/workbench_test.go`）、`TestBacklinkOpensTheSourceAtItsParagraph`（`internal/ui/panel_test.go`）、`TestBacklinkOpensItsNote`（`internal/ui/screens/notes/notes_test.go`）。
- 状态：**covered**。

## S23 · 读 · 队列

- 代码：`internal/ui/screens/read/read.go`、`internal/ui/reading.go`。
- 目的：今天该读的笔记排成一列，从哪继续一目了然。
- 元素：页头右「加入队列」（次）/「开始阅读」（主，队列空时禁用）、`ui.List` 行（标题 + 「第 5 / 9 段 · 上次停在…」+ `ui.Meter` 120 + 「继续」）、底部「节奏」三格；空 → `kit.Empty`。
- 动作：行 → 进入会话；「开始阅读」→ 第一篇；「加入队列」→ S05。
- 键：`Esc`（会话入口处）。
- 尺寸：`ListWidth`；zh-Hans 最小窗由 `TestReadAreaInChineseAtTheMinimumSize` 守。
- 守卫：`TestReadingQueueOpensTheSession`、`TestReadingQueueAddsANoteFromTheList`（`internal/ui/reading_screen_test.go`）；屏级 `TestQueuePageListsWhatsDue`、`TestQueuePageActions`、`TestEmptyQueueIsNotASession`、`TestLoadStates`（`internal/ui/screens/read/read_test.go`）。
- 状态：**covered**。

## S24 · 读 · 归档

- 代码：`internal/ui/screens/read/read.go`。
- 目的：归档与不再出现的笔记留在这里，能捞回来。
- 元素：两段（归档 / 不再出现），每行「打开」「恢复」；空 →「归档里还是空的」+「阅读会话里按 A」。
- 动作：恢复 → `Deps.QueueNote(id)`（写 `queued`，清 archived/dismissed）。
- 键：无专属。
- 尺寸：`ListWidth`；zh 最小窗同上。
- 守卫：`TestArchiveRestoresANoteToTheQueue`、`TestDismissedNotesStayOutUntilRestored`（`internal/ui/reading_screen_test.go`）；屏级 `TestArchivePageListsBothSections`、`TestArchiveEmptySaysHowANoteArrives`。
- 状态：**covered**。

## S25 · 阅读会话

- 代码：`internal/ui/screens/reading/reading.go`、`state.go`。
- 目的：全屏读一篇，逐段消化，并把难易写回。
- 元素：三行（会话栏：第 N / M 篇 + 标题 + 段进度 `ui.Meter` + 退出；舞台 `ReadingWidth` 居中、当前块高亮、右标记列；动作行：三档难度 + 「摘录」/「跳过」/「归档」/「不再出现」/「推后」/「在库里打开」）。
- 动作：三档难度写位置；A / D / P 之后 `Advance(false)` 并 toast 带「撤销」（撤销归档 = `QueueNote`）。
- 键：`J`/`K`/`↓`/`↑`、`1`/`2`/`3`、`E`、`S`、`X`、`A`、`D`、`P`、`M`。
- 尺寸：`ReviewWidth 680`；≤760 动作行换行（`TestActionRowAtTheMinimumSize`）。
- 守卫：`TestReadingSessionArchivesANote`、`TestReadingSessionPostponesToTomorrow`、`TestReadingSessionDismissesAndTells`（`internal/ui/reading_screen_test.go`）；既有 `TestReadingResumesWhereItStopped`；屏级 `TestActionsReachTheApp`、`TestEndingANoteTellsAndUndoes`、`TestParagraphsCarryTheirMarks`、`TestChineseAtTheMinimumSize`。
- 状态：**covered**。

## S26 · 会话 · 摘录并制卡

- 代码：`internal/ui/screens/reading/reading.go:289`（`ui.Checkbox`）、`state.go:178`（`ExcerptWithCard`）。
- 目的：摘录这一步顺手把卡也做出来，勾一次管一段会话。
- 元素：动作行里与「摘录」紧邻的一个 `ui.Checkbox`。
- 动作：勾上后 `Deps.Excerpt(..., withCard=true)`。
- 键：`E`（带勾时同时制卡）。
- 尺寸：随动作行；≤760 一起换行。
- 守卫：`TestExcerptLeavesTheReaderInPlace`（`internal/ui/screens/reading/reading_test.go`，勾选框 → `f.excerpts[1].card`）、`TestExcerptFromTheReadingScreen`（`internal/ui/reading_screen_test.go`，键盘 + 勾选同一条路）。
- 状态：**covered**。

## S27 · 忆 · 队列

- 代码：`internal/ui/screens/recall/recall.go`、`heat.go`、`filter.go`。
- 目的：今天该忆多少、由什么组成、按哪篇分。
- 元素：页头右（范围按钮 + 开始复习）、复习量卡（`TypeDisplay` 分钟 + 三段占比 + 图例）、6 个 `ui.Toggle`（`ui.ToggleGroup`）+「保存为筛选」+ 可删 chip、「按笔记分」`ui.List`、「最近 26 周」热力缩略。
- 动作：行 → 以该笔记为范围开复习；筛选 → `Deps.SetFilter`；保存 / 删除筛选 → 写 `settings.json`；范围 → S28。
- 键：无专属。
- 尺寸：`ListWidth`；zh-Hans 最小窗由 `TestRecallPagesAtMinimumInChinese` 守。
- 守卫：`TestRecallQueueGroupsByNote`、`TestRecallFilterNarrowsTheQueue`、`TestSavedFilterSurvivesRestart`（`internal/ui/recall_test.go`）；另 `TestRecallLostFilterOffersTheRepair`；屏级 `TestFiltersRoundTrip`、`TestHeatWeeksFoldsTheLogIntoCalendarColumns`（`internal/ui/screens/recall/recall_test.go`）。
- 状态：**covered**。

## S28 · 复习范围

- 代码：`internal/ui/screens/recall/scope.go`。
- 目的：这次复习取哪一批——全库 / 某篇 / 某标签。
- 元素：`ui.DialogBase`、`ui.Segmented` 三选、对应选择器（笔记 `ui.List`、标签 `ui.ToggleGroup`）、页脚「本次取 N 张」、开始（0 张禁用 + 说明）。
- 动作：选范围 → 计数变化 → 开始。
- 键：`Esc` 关。
- 尺寸：`SettingsWidth`；zh 最小窗由 `TestRecallPagesAtMinimumInChinese` / `TestRelinkPickerAtMinimumInChinese` 同类路径覆盖。
- 守卫：`TestScopeDialogNarrowsTheSession`（§9.1）；另 `TestScopeDialogOpensFromTheQueuePage`、`TestScopeDialogChoosesATag`、`TestScopeDialogRefusesAnEmptyScope`（`internal/ui/review_scope_test.go`）、`TestScopeCountsAnswerEachScope`（`internal/ui/screens/recall/recall_test.go`）。
- 状态：**covered**。

## S29 · 复习会话

- 代码：`internal/ui/screens/review/review.go`、`compare.go`、`typing.go`。
- 目的：一张卡问一次，答案之后一个评分，走之前还能看它从哪来。
- 元素：题面 → 答案（公式走 `maths.Render`）+ 比对（键入时）、来源行（「《标题》· 第 N 段 · 同块还有 M 张」+「回到原文」+「展开上下文」）、四档评分（`kit.Keycap` + 间隔预览）+ `review.card` 插件项 +「撤销」「编辑卡片」。
- 动作：`Space` 显示答案、`1`–`4` 评分、`U` 撤销、`E` 编辑、回原文、展开上下文。
- 键：§2.8 复习那一组。
- 尺寸：`ReviewWidth 680`；720×520 **仍是单行四档**（§2.5 ④ 未实现，缺口 #7）。
- 守卫：`TestReviewShowsTheSourceOfTheCard`、`TestReviewExpandsTheContext`（`internal/ui/screens/review/review_test.go`）；屏级 `TestRevealAndGrade`、`TestGradeKeysShowIntervals`、`TestGradeKeysWithoutPreview`、`TestIntervalSitsInsideItsKey`、`TestFooterActions`、`TestReviewFooterDrawsTheReviewCardSlot`（`internal/ui/receivers_test.go`）。
- 状态：**covered**。

## S30 · 复习总结

- 代码：`internal/ui/screens/review/review.go`（summary 段）。
- 目的：这一轮做了什么、没想起来的是哪些、下一步去哪。
- 元素：`kit.Card` + 三个 `kit.Stat` + 插件文案 + 主按钮；「没想起来的按来源聚合」（每行「标题 · N 张没想起来」+「回到原文」）；全对时没有失败分组。
- 动作：回今日 / 继续读 / 回原文。
- 键：`Esc`。
- 尺寸：`SummaryWidth 520`；`TestSummaryKeepsItsActionAtTheSmallestWindow` 守最小窗。
- 守卫：`TestSummaryGroupsFailedCardsByNote`（§9.1）、`TestSummaryCardIsCentered`（§9.1 既有）、`TestSummaryWithoutMissesDrawsNoGroups`；App 侧 `TestReviewSummaryCarriesTheMissedCards`（`internal/ui/recall_test.go`）。
- 状态：**covered**。

## S31 · 忆 · 卡片

- 代码：`internal/ui/screens/recall/cards.go`、`filter.go`。
- 目的：一张表看全库的卡，能排序、能筛、能批量。
- 元素：`ui.Split`（左 `FilterPaneWidth 200` 筛选栏；≤窄窗换成顶部 `ui.Select`）、`ui.Table`（首列勾选 `Fixed`、`Sortable`、`CellMenu`、列开关）、工具行（`ui.SearchField` 带 `is:`/`type:`/`tag:`/`note:` + 语法帮助 + 列开关 + `browse.row` 插件动作）、批量条（暂停 / 改到期 / 冻结 / 加标签）。
- 动作：点表头排序、右键批量、多选批量、语法筛选、开原笔记。
- 键：表自己的（↑↓ / Home / End / 空格选中）。
- 尺寸：`BrowseWidth 1080`；≤ 阈值切 `ui.Select`（`cards.go:504`），zh 最小窗由 `TestRecallPagesAtMinimumInChinese` 守。
- 守卫：`TestCardTableSortsByColumn`、`TestCardSearchSyntaxNarrows`、`TestCardTableBulkSuspends`（`internal/ui/recall_test.go`）；另 `TestCardTableOpensTheNote`、`TestCardTableDrawsTheBrowseRowSlot`（`internal/ui/receivers_test.go`）。
- 状态：**covered**。

## S32 · 忆 · 类型

- 代码：`internal/ui/screens/cardtypes/cardtypes.go`（从设置区旧的 types 页迁出，见 §4.6）。
- 目的：一种卡有哪几面、拿什么模板生成。
- 元素：主从 `ui.Split`（左 `ui.List` 类型名 + 卡数；右标题行 + 内置 / 插件 `ui.Badge` + 字段 chips + 模板只读代码块 + 两张卡面预览 + 回退说明）；只有内置三件时列表照常 + 一句说明。
- 动作：`SaveType` / `DeleteType`（内置禁用并写原因）。
- 键：随列表。
- 尺寸：无 measure 上限（§4.10 未给 S32 定值），`ui.Split` 左欄 `NotesColumnWidth 260`；zh 最小窗由 `TestTypesPageInChineseAtMinimum` 守。
- 守卫：`TestTypesPagePreviewsTheTemplate`、`TestTypesPageKeepsBuiltinsUndeletable`（§9.1）；另 `TestTypesPageCreatesAType`（`internal/ui/screens/cardtypes/cardtypes_test.go`）。
- 状态：**covered**。

## S33 · 忆 · 预设

- 代码：`internal/ui/screens/recall/presets.go`。
- 目的：库自己的节奏——学习步、每日上限、失败口径、断链策略。
- 元素：四组 `ui.Form` + `ui.Fieldset`（学习步只读 + 一行说明；上限两个 `ui.StepSlider`；失败 `ui.Select` + `ui.StepSlider`；断链 `ui.Segmented`）、「恢复默认」走 `ui.AlertDialog`、写入失败就地 callout。
- 动作：每个控件 → `Deps.SetPresets`（一次写 `remit.json`）；恢复默认 → `ResetPresets`。
- 键：无专属。
- 尺寸：`SettingsWidth 660`；zh 最小窗由 `TestRecallPagesAtMinimumInChinese` 守。
- 守卫：`TestPresetsWriteTheLibraryConfig`、`TestPresetsResetToDefaults`（`internal/ui/recall_test.go`）。
- 状态：**covered**。

## S34 · 忆 · 历史

- 代码：`internal/ui/screens/recall/history.go`。
- 目的：今天评过什么、答得怎么样。
- 元素：四个数字、评分分布四行横条（`internal/ui/screens/recall/history.go:160` 的 `gradeBar`：`ui.Row` + 两个 `ui.Box` 组合，`ui/chart` 对四项过重）、今天的时间线 `ui.List`、本周表 `ui.Table` 四列；空 →「今天还没有记录」。
- 动作：时间线行 → 回原文 / 回卡片。
- 键：无专属。
- 尺寸：`ListWidth`；zh 最小窗由 `TestRecallPagesAtMinimumInChinese` 守。
- 守卫：`TestHistoryListsTodaysReviews`、`TestHistoryCountsGrades`（`internal/ui/recall_test.go`）。
- 状态：**covered**。

## S35 · 忆 · 断链

- 代码：`internal/ui/screens/relink/page.go`、`relink.go`。
- 目的：被删块留下的卡，逐张重新指到活着的段上。
- 元素：两栏 `ui.List`（左「路断了的卡片」，右「重新指向《笔记》的哪一段」）、`ui.Autocomplete` 收窄候选、页头「全部重新指向」走 `ui.AlertDialog`、下方「已冻结」区 +「解冻」；空 →「没有断链」+ 因果一句。
- 动作：选卡 → 选块 → `Deps.Relink`；`Freeze` / `Thaw`；批量 `RelinkAll`。
- 键：无专属。
- 尺寸：`BrowseWidth 1080`（两欄要宽；§4.10 未列 S35）；zh 最小窗由 `TestRelinkPickerAtMinimumInChinese` 守。
- 守卫：`TestRelinkReattachesALostCard`、`TestRelinkNarrowsAsYouType`、`TestRelinkThawsAFrozenCard`（`internal/ui/recall_test.go`）；既有 `TestCandidates`/`TestQuery`/`TestChoose`/`TestClose`/`TestEmpty`（`internal/ui/screens/relink/relink_test.go`）、`TestRelinkFromReview`、`TestRelinkPickerAtMinimumInChinese`。
- 状态：**covered**。

## S36 · 统计 · 总览

- 代码：`internal/ui/screens/stats/overview.go`、`stats.go`、`internal/ui/library.go`（`statsSnapshot`）。
- 目的：库的产出、复习与阅读的配比、保留率与预测。
- 元素：四个数字、产出三格、26 周热力缩略 + 图例、时间配比三段条 + 图例、保留率曲线（`ui/chart` `Plot` + `NewTime` + `Area`/`Line`）、预测 14 天柱（`chart.Rect`）、`stats.block` 插件段；新库数字 0 + 一句。
- 动作：只读；插件块可动作。
- 键：无。
- 尺寸：纸面 `PaperWidth`；zh 最小窗由 `TestStatsPagesZhHansAtMinimum` 守。
- 守卫：`TestStatsShowsProductionAndRatio`、`TestStatsBlocksComeFromPlugins`（`internal/ui/stats_test.go`）；屏级 `TestOverviewDrawsTheNumbersTheProductionAndTheShare`、`TestBlockSectionsComeFromTheSlot`、`TestEmptyPagesSayWhatWouldFillThem`（`internal/ui/screens/stats/stats_test.go`）。
- 状态：**covered**。

## S37 · 统计 · 热力

- 代码：`internal/ui/screens/stats/heatmap.go`、`internal/ui/kit/heatmap.go`。
- 目的：半年里哪几天真的动手了，以及这个数从哪来。
- 元素：三个数字（范围 / 保留率 / 最长连续）、`kit.Heatmap` 大网格 + 图例、说明段（数据来自 append-only 日志，删索引不丢）。
- 动作：悬停 / 聚焦读某天。
- 键：网格自己的。
- 尺寸：纸面 `PaperWidth`；zh 最小窗同 S36。
- 守卫：`TestHeatmapExplainsItsSource`（§9.1，`internal/ui/stats_test.go`）；屏级 `TestHeatmapNamesItsRangeAndSource`；组件级 `TestHeatmapNamesItsCountedDays`、`TestHeatLevelCoversTheScale`（`internal/ui/kit/kit_test.go`）。
- 状态：**covered**。

## S38 · 统计 · 预测

- 代码：`internal/ui/screens/stats/forecast.go`。
- 目的：接下来 14 天会压过来多少，以及为什么读与忆合在一起算。
- 元素：柱状（`chart.Rect`）、三格（今天 / 峰值 / 明日）、「为什么合并」说明段；无到期一句。
- 动作：无。
- 键：无。
- 尺寸：纸面 `PaperWidth`；zh 最小窗同 S36。
- 守卫：`TestForecastMergesReadingAndCards`（§9.1）、`TestForecastNamesTheThreeDaysAndWhyTheyMerge`（`internal/ui/screens/stats/stats_test.go`）。
- 状态：**covered**。

## S39 · 设置 · 外观

- 代码：`internal/ui/screens/settings/appearance.go`、`internal/ui/prefs.go:231`（`a.tokens`：三选折成 `dark bool` 的唯一解析点）、`internal/ui/view.go:29`（`c.SetTheme`）。
- 目的：跟系统走，还是钉在纸面或墨色；纸面多宽。
- 元素：`ui.Segmented` 三选（随系统 / 纸面 / 墨色）、纸面宽度 `ui.StepSlider`（620–860 step 20）、正文尺寸显式只读 + 一行「跟随桌面文本缩放」+「打开系统设置」。
- 动作：三选 → `settings.Appearance` + `c.SetTheme`；滑杆 → `settings.PaperWidth`。
- 键：无。
- 尺寸：`SettingsWidth 660`；zh 最小窗由 `TestSettingsPagesAtMinimumInChinese` 守。
- 守卫：`TestSettingsAppearanceWritesTheOverride`（§9.1）、`TestSettingsAppearanceWritesTheLookAndThePaper`（`internal/ui/screens/settings/settings_test.go`）、`TestAppearanceFollowsTheDesktop`、`TestEveryModeInBothAppearances`（`internal/ui/theme_test.go`）。
- 状态：**manual**——测试跑的是 fake 的桌面外观与 `mygo.Shell.OpenExternal`，「系统切换时窗口即时跟随」「x-apple.systempreferences 打开正确面板」只有真桌面能证。

## S40 · 设置 · 复习

- 代码：`internal/ui/screens/settings/review.go`。
- 目的：保留率、最大间隔、评分键要不要显示下一次间隔、摘录后回不回原位。
- 元素：保留率 `ui.StepSlider`（80–97）、最大间隔 `ui.StepSlider`（30–365 step 30）、间隔预览 `ui.Switch`、摘录后回原位 `ui.Switch`。
- 动作：改一项 → 写 `presets.review` / `presets.reading`，调度随后用新值。
- 键：无。
- 尺寸：`SettingsWidth`；zh 最小窗同上。
- 守卫：`TestSettingsReviewWritesThePresets`（§9.1，`internal/ui/settings_test.go` 与 `internal/ui/screens/settings/settings_test.go` 各一条）、`TestGradeKeysWithoutPreview`（`internal/ui/screens/review/review_test.go`，关掉预览后键只显示评级）。
- 状态：**covered**。

## S41 · 设置 · 预算

- 代码：`internal/ui/screens/settings/budget.go`。
- 目的：一天给多少张卡、多少分钟，以及今天会被排成什么样。
- 元素：复习张数 `ui.StepSlider`（5–60 step 5）、可用时间（10–90 step 5）、今日分配预览（`kit.Card` + `ui.Meter`，读 S04 同一函数）、每日提醒 `ui.Switch`、托盘标题预览（等宽只读）。
- 动作：滑杆 → 预览即时变；开关 → `settings.Reminder`。
- 键：无。
- 尺寸：`SettingsWidth`；zh 最小窗同上。
- 守卫：`TestSettingsBudgetPreviewMatchesToday`（§9.1）、`TestSettingsBudgetShowsTheDayItWouldBuild`（`internal/ui/screens/settings/settings_test.go`）。
- 状态：**covered**。

## S42 · 设置 · 快捷键

- 代码：`internal/ui/screens/settings/keys.go`、`internal/ui/shortcuts.go`。
- 目的：键位一处能查；自定义明确不在 1.0。
- 元素：四组表（全局 / 阅读 / 复习 / 文档，每行说明 + `kit.Keycap`）、顶部禁用的「修改键位」+ 可见原因（#155）。
- 动作：只读；禁用的那一个不响应键盘。
- 键：无（它就是键位表）。
- 尺寸：`SettingsWidth`；zh 最小窗同上。
- 守卫：`TestSettingsKeysAreReadOnlyWithAReason`（§9.1）、`TestKeysHelpListsTheMap`（`internal/ui/shortcuts_test.go`）。
- 状态：**covered**。

## S43 · 设置 · 插件

- 代码：`internal/ui/screens/settings/plugins.go`、`internal/ui/screens/settings/pluginsettings.go`。
- 目的：装了什么、各自要什么能力、哪一件被宿主藏了、为什么。
- 元素：每插件一行（名 + 内置 badge + 描述 + capability badge 列 +「详情」+ 启用 `ui.Switch`）、「问题」段列 `ui.problems` 与三次隐藏的贡献、「页面贡献」段 +「打开」；无插件一句。
- 动作：启 / 停（与 S47 同一个授权写入）、详情、重新启用、打开页面。
- 键：无。
- 尺寸：`SettingsWidth`；zh 最小窗同上。
- 守卫：`TestPluginsShowCapabilitiesAndFailures`、`TestReenableBringsBackAHiddenContribution`（§9.1）；另 `TestPluginApproval`、`TestPluginsDegradeToAStatedEmptyState`、`TestContributionIssuesListDropsAndHidden`、`TestHiddenContributionStaysInSettingsWithItsReason`。
- 状态：**covered**。

## S44 · 设置 · Agent

- 代码：`internal/ui/screens/settings/agent.go`、`internal/agentapi`。
- 目的：把只读 / 带 token 的本地接口讲清楚，开关与 token 就在这一页。
- 元素：监听地址（等宽只读）、启用 `ui.Switch`、生成 / 轮换 token `ui.Button`（新 token 成功后只显示一次 + `SettingsTokenOnce` 一句）、路由表 `ui.Table`（方法 / 路径 / 说明，`Fixed` 前两列）；构建里没有该服务时一句说明。
- 动作：启 / 停（失败要说明原因并保持开关原状）、轮换 token。
- 键：无。
- 尺寸：`SettingsWidth`；路由表高度 `routeTableHeight 300`。
- 守卫：`TestAgentSwitchesNow`、`TestAgentFailureKeepsTheSwitch`（§9.1）、`TestSettingsAgentPageStatesTheSurface`（`internal/ui/screens/settings/settings_test.go`）。
- 状态：**manual**——测试换的是注入的监听器替身；真端口、真 token 落盘、真 401/403 要在机器上验。

## S45 · 设置 · 更新

- 代码：`internal/ui/screens/settings/update.go`。
- 目的：现在是什么版本、要不要自动查、把库重建成什么样、怎么整库带走。
- 元素：当前版本 + 格式版本、自动检查 `ui.Switch`、「检查」、重建索引、导出整库（打开 S03）；无库时仍可用。
- 动作：检查 → `update.Check`；重建索引；导出。
- 键：无。
- 尺寸：`SettingsWidth`；zh 最小窗同上。
- 守卫：`TestCheckNow`、`TestReindexKeepsTheLibrary`（§9.1）、`TestSettingsUpdatePageWithoutALibrary`、`TestSettingsWithoutALibrary`。
- 状态：**manual**——`TestCheckNow` 注入 check 函数，真请求、真签名校验、真重建耗时只有装上跑一次知道。

## S46 · 设置 · 关于

- 代码：`internal/ui/screens/settings/about.go`。
- 目的：这是什么、库在哪、里面有多少东西、凭什么许可。
- 元素：品牌卡（名 + 版本 + `AGPL-3.0 · 明文库 · 零遥测`）、库位置（等宽）、笔记 / 卡片 / 队列三个数、致谢段。
- 动作：打开许可证链接（可选的 `Deps.OpenLicense`，不在时为无按钮）。
- 键：无。
- 尺寸：`SettingsWidth`；zh 最小窗同上。
- 守卫：`TestAboutShowsTheLibraryFacts`（§9.1）、`TestAbout`（`internal/ui/screens/about/about_test.go`）、`TestSettingsAboutPageStatesTheBuild`。
- 状态：**covered**。

## S47 · 插件详情

- 代码：`internal/ui/screens/settings/plugindetail.go`。
- 目的：一件插件的能力、信任级别、失败计数和重新启用。
- 元素：`ui.DialogBase`、能力 chips、信任级别（内置 / 第三方）、启用 `ui.Switch`（与 S43 同一个写入）、失败计数 +「重新启用」；内置件不画开关（没有可授权的东西）。
- 动作：启 / 停、重新启用。
- 键：`Esc` 关。
- 尺寸：`SettingsWidth`；zh 由 `TestPluginSurfacesAtTheMinimumSizeInChinese` 覆盖同类浮层。
- 守卫：`TestPluginDetailShowsCapabilitiesAndFailures`（§9.1）、`TestPluginDetailsOpenOnTheirButton`、`TestPluginDetailOfABuiltinOffersNoSwitch`（`internal/ui/screens/settings/plugins_test.go`）。
- 状态：**covered**。

## S48 · 快捷键总表

- 代码：`internal/ui/shortcuts.go`。
- 目的：`⌘/` 一眼看全所有键，不再翻文档。
- 元素：`ui.DialogBase`、四组两列 `dl` 式列表（`ui.Grid` 两列）。
- 动作：关闭。
- 键：`⌘/` 开、`Esc` 关。
- 尺寸：`SettingsWidth`；zh 最小窗由 `TestOverlaysAtTheMinimumSizeInChinese` 守。
- 守卫：`TestKeysHelpListsTheMap`（`internal/ui/shortcuts_test.go`）、`TestOverlaysAtTheMinimumSizeInChinese`。
- 状态：**covered**。

## S49 · 命令面板

- 代码：`internal/ui/palette.go`。
- 目的：不离开键盘就能去任何地方、做任何事。
- 元素：`ui.DialogBase` + `ui.List`，三组（命令 / 笔记 / 卡片），插件命令在最后一组；空 →「没有匹配项」。
- 动作：输入收窄、`↑↓` 选、`↵` 执行、`Esc` 关并把焦点还回原处。
- 键：`⌘K`、`↑↓`、`↵`、`Esc`。
- 尺寸：最小窗行仍可点（`TestMinimumSizeControls`）。
- 守卫：`TestPalette`、`TestPaletteCommandsForReading`（§9.1 既有）、`TestPaletteGroupsCommandsNotesAndCards`、`TestPaletteListsPluginCommandsLast`（`internal/ui/palette_test.go`）。
- 状态：**covered**。

## S50 · 插件页 · 制卡脚本

- 代码：`internal/ui/screens/pluginpage/pluginpage.go`、`internal/ui/plugin.go`。
- 目的：插件自己的一页——页头动作、数据、上一次运行。
- 元素：`page.header` 槽动作、三个数字、「上一次运行」卡、「它能做什么」列表；无数据先骨架，失败进错误态 + 重试。
- 动作：重新运行、页头动作。
- 键：页内键盘路径（`TestContributedPageKeepsAKeyboardPath`）。
- 尺寸：`SettingsWidth`；窄窗收栈（`TestNarrowPageCollapsesItsLayouts`、`TestContributedPageDrawsAtBothSizes`），zh 由 `TestContributedPageFollowsTheLocale` 守。
- 守卫：`TestPluginPageShowsItsHeaderActions`（§9.1）、`TestLoadingPageShowsASkeleton`、`TestFailedPageSaysWhyAndRetries`、`TestEveryNodeTheHostKnowsDraws`、`TestUnknownNodeDrawsInPlace`（`internal/ui/screens/pluginpage/pluginpage_test.go`）、`TestContributedPageFailsWithARetry`、`TestAFailingCommandToastsThenHidesItself`、`TestFrameNeverCallsAPlugin`。
- 状态：**covered**。

## S51 · 插件页 · 白板

- 代码：`internal/ui/screens/pluginpage/board.go`、`internal/ui/board.go`、`internal/ui/kit/board.go`。
- 目的：把一页笔记的块摊成节点网格，点节点回原文。
- 元素：宿主画的节点网格（`kit.Board`，`ui.Grid` + `ui.List` 语义行，不是自由画布）、`page.header` 动作；空 →「这块画布还没有节点」。
- 动作：点节点 → 阅读态对应段落；居中 / 定位；从块菜单回白板时高亮。
- 键：随节点列表。
- 尺寸：窄窗收栈，720×520 不横向滚动（`TestWhiteboardOpensTheBlock` 的注释即此）。
- 守卫：`TestWhiteboardOpensTheBlock`、`TestWhiteboardSurvivesAChangedBlock`（§9.1）、`TestTheWhiteboardExampleOpensItsBlock`、`TestTheWhiteboardExampleLocatesItsNode`（`internal/ui/whiteboard_test.go`）、`TestInspectorOffersTheWhiteboardWhenANodeExists`（`internal/ui/board_test.go`）。
- 状态：**covered**。自由画布（坐标 + 连线）在 §10 推迟，`kit.Board` 的注释记了这一层选择。

## S52 · 笔记行菜单

- 代码：`internal/ui/screens/notes/list.go:258`（行尾 `ui.ButtonBase` + `PopoverMenuButton`）。
- 目的：不打开笔记也能对它做事。
- 元素：菜单里今天有「阅读」「加入阅读队列」+ 分隔线 + `sidebar.note` 插件项。**缺**：「重命名」「删除」（含 `ui.AlertDialog` 确认）；代码里没有 `RenameNote` / `DeleteNote`（`internal/ui/screens/notes/notes.go` 的 `Deps` 也没有这两个字段）。
- 动作：阅读 / 加入队列 / 插件项。
- 键：`Esc` 关。
- 尺寸：随行；720×520 与 zh-Hans 都没有验证。
- 守卫：`TestNoteRowMenuDrawsTheSidebarNoteSlot`（`internal/ui/receivers_test.go`，只守插件槽那一半）。
- 状态：**missing**。§9.1 的 `TestNoteRowMenuOffersReadingAndThePluginSlot` 不存在；「阅读 / 加入队列」这两行没有任何测试守着。

# 缺口

### §9.1 点名而不存在的测试

| # | 名字 | 该红的 bug | 现状 / 替代 |
|---|---|---|---|
| 1 | `TestWelcomeOffersThreeDoors` | 少一个入口 | 不存在；S01 代码只有两道门 |
| 2 | `TestWelcomeShowsWhyALibraryFailed` | 错误被吞 | 不存在；失败是另一页 `libraryErrorView`（`internal/ui/library.go:491`），没有覆盖 |
| 3 | `TestNoteRowMenuOffersReadingAndThePluginSlot` | 行菜单少了「阅读」或插件槽 | 不存在；插件槽一半由 `TestNoteRowMenuDrawsTheSidebarNoteSlot` 守，「阅读 / 加入队列」无测试 |
| 4 | `TestBrowseSortsByColumn`（`ui-composition.md` §4 闸门） | 点表头行序不变 | 不存在；表排序由 `TestCardTableSortsByColumn` 守（`internal/ui/recall_test.go`） |
| 5 | `TestThePageCountsCardsNotBlocks`（`ui-composition.md` §4 闸门） | 页面报块数不报卡数 | 不存在；近一半由 `TestNotesListShowsSummaryAndCards` 守 |

### spec 的面在代码里不存在或缺零件

| # | 面 / 规则 | 缺什么 | 证据 |
|---|---|---|---|
| 6 | §2.5 ③ 纸面边距 48 → 20 | `GutterNarrow` 定义了但没有任何视图引用；`DisplayNarrow` 只在 `internal/ui/screens/notes/read.go:166` 用 | `grep -rn GutterNarrow internal/ui` 只有 `design/` |
| 7 | §2.5 ④ ≤760 复习评分两列 | `internal/ui/screens/review` 里没有 `BreakpointSmall` 分支，四档仍是单行 `ui.Row`（`review.go:499`） | `grep -rn BreakpointSmall internal/ui/screens/review` 空 |
| 8 | §2.5 ⑤ 沟槽改到段上一行 | 代码保留左侧 48 沟槽（口径改判，已由测试固定） | `TestPaperKeepsGutterAtMinimumSize`、`TestBlockHandlesAtMinimumSize` |
| 9 | §4.1 S02 步骤 2 开关 | spec 写「保留现有笔记、导入后重建索引」，代码是「重建索引、加入阅读队列」 | `internal/ui/import.go:446-447` |
| 10 | §4.1 S01 三件元素 | 导入门、最近库列表（`settings.recent`）、卡内 danger callout 全无 | `internal/ui/screens/start/start.go` |
| 11 | §4.9 S52 | 重命名、删除（含确认）无实现，也没有对应 `Deps` | `grep -rnE 'Rename\|DeleteNote' internal/ui` 无命中 |
| 12 | §3 状态行「只在文档面出现」 | 代码按内容判断（有话才画），不按面 | `internal/ui/shell/status.go:15`、`TestStatusLineOnlyWhenItSaysSomething` |
| 13 | §4.14 `kit.GradeBar` | 没有这个 `kit` 组件；实现是 `internal/ui/screens/recall/history.go:160` 的局部 `gradeBar` | `grep -rn GradeBar internal/ui` 无命中 |
| 14 | §4.10 把 S05–S10 全归 `ListWidth 860` | S08 在 §4.3 写的是 `BrowseWidth`，代码也是 `BrowseWidth 1080` | `internal/ui/screens/notes/assetspage.go:47` |

### 无行为测试的屏

13 个 `internal/ui/screens/` 下的包与 `internal/ui/shell`、`internal/ui/kit`、`internal/ui/design` 各有 `_test.go`，没有「整包零测试」的屏。
逐面看，只有 S01 与 S52 两面没有守住自己行为的测试；S39 / S44 / S45 有测试但只证明 fake，按 `manual` 记。

### 尺寸口径

§9.1 要求「每个能力测试在 1024×720 与 720×520 各一遍」。实际是**按面**而不是**按测试**满足：每面一条默认尺寸的测试，加一条该面的最小窗 / zh-Hans 测试（列表见「全局验收 · i18n」），整窗的 720×520 由 `TestNoHorizontalScrollAt720` 兜底。逐条测试跑到两档宽这件事没做。

### 计数

52 个面：`covered` 47、`missing` 2（S01、S52）、`manual` 3（S39、S44、S45）。
§9.1 表里出现 100 个不重复的测试名，97 个在仓库里存在，缺的 3 个就是上表 #1–#3；`ui-composition.md` §4 另有两个名字不存在（#4、#5）。
