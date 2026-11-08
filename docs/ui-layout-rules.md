# 应用级布局规则

这套规则回答一个窗口怎么排，而不是某一屏怎么拼。屏只声明自己属于哪种骨架，
宽度、居中、边距和最小窗口下的让位顺序都从 `internal/ui/design` 取。

## 1. 三种骨架

| 骨架 | 用途 | 结构 |
|---|---|---|
| Paper | 笔记、统计、阅读、插件页 | `Scroll` → `Column(FillWidth, AlignItems Center, Padding)` → `Column(MaxWidth measure)` |
| Workspace | 笔记列表 / 文档 / 面板 | `ui.Split` / `SplitPercent` 填满窗口；文档自身仍是 Paper |
| Session | 复习、阅读会话 | 一条居中的会话栏 + 一个 `Grow(1).Center()` 的舞台；舞台内容用同一 measure |

全屏接管（`takingOver`）隐藏应用栏，只保留 Session 自己的会话栏。空态、卡片、
进度和总结都在同一个 measure 上，不再各自漂移。

## 2. 唯一宽度来源

宽度是 `internal/ui/design/scale.go` 的语义常量：

| 常量 | 值 | 用途 |
|---|---:|---|
| `PaperWidth` | 760 | 笔记纸面、统计、插件页 |
| `ReadingWidth` | 760 | 整篇阅读与分栏右侧 |
| `ReviewWidth` | 680 | 复习卡、会话栏、状态栏 |
| `SummaryWidth` | 520 | 复习完成总结卡 |
| `SettingsWidth` | 640 | 设置表单 |
| `BrowseWidth` | 1040 | 浏览表格 |
| `MessageWidth` | 520 | 开始页、库错误的短句 |

屏内不得再写 `const width`。需要新宽度时先加语义常量，再只在一个骨架里使用。
`About` 的 360 是独立小窗口的尺寸，不是页面 measure，留在窗口装配处。

## 3. 应用栏

应用栏是 `ui.Toolbar`，高 `FrameHeight = 64`，纸面底、下边 1px hairline：

```text
[brand] │ ……… [Notes Review Browse Stats Settings] ……… [context] [More]
```

- **Brand**：26px 强调色方标 + `Remit` 字标；不是按钮。
- **Mode rail**：`ui.TabsBase`，文字 tab + 选中项 2px 强调色下划线。它是主页面
  导航，不是第二层 segmented。
- **Context**：只放当前模式的动作。笔记模式是编辑 / 分栏 / 阅读、面板开关；
  其他模式留空或放该模式自己的动作。
- **Right rail**：插件页入口（有才出现）、`⋯` 溢出。窄窗口由 `Toolbar` 从尾部
  收入 overflow。

模式索引不得直接投射到 `mode` 枚举：`modeReading` 不在主 tab 中，浏览 / 统计 /
设置各自由自己的映射选择。插件页面暂不占主 tab，归 #129 的插件导航决定。

## 4. 复习与会话

- 会话栏（Review / Reading、Scope、阅读等待数）在 `ReviewWidth` 内居中。
- 复习状态栏（剩余、进度、键位）也在 `ReviewWidth` 内居中。
- 完成总结卡 `MaxWidth(SummaryWidth)` + `AlignSelf(Center)`；不得再以
  `ReviewWidth` 的左边为起点。
- 卡面、评分、总结共用同一水平轴；垂直方向由可用空间 `Center()`，窄窗口只
  收紧外边距，不改变轴心。

## 5. 最小窗口 720×520

让位顺序固定：

1. 应用栏尾部动作进入 Toolbar overflow；主 mode rail 保留。
2. 笔记页右侧面板收起；笔记列表仍是 `Split` 的第一栏。
3. Paper 的左右 padding 从 `SpaceXXL` 收紧到 `SpaceXL`；measure 不缩。
4. 编辑器纸面允许变窄，块沟槽仍留在纸面左侧。
5. 复习卡收窄到可用宽度，四个评分仍在一行；再窄时才由组件自身换行。
6. 浏览表宽于窗口时由页面自身滚动，不让整窗出现横向滚动。

## 6. 组件映射

| 需要 | 组件 |
|---|---|
| 窗口动作行、overflow | `ui.Toolbar` |
| 主页面导航 | `ui.TabsBase` |
| 视图 / 队列切换 | `ui.SegmentedBase`（自定义紧凑外观） |
| 可拖栏 | `ui.Split` / `SplitPercent` |
| 页面滚动 | `ui.Scroll` |
| 纸面 / 表单 measure | `Column.MaxWidth` + `AlignItems(Center)` |
| 浮层 | `ui.DialogBase` / `Modal` / `Popover` |

## 7. 当前违规清单

已修复：

- 复习完成总结在 680 列内左对齐，视觉中心偏左约 84 DIP。
- 七个页面宽度来源散落在各屏；现在统一到 `design`。
- 浏览页宽窗口下贴左；现在在 `BrowseWidth` 内居中。
- 顶栏是一串同级控件；现在分为 brand / mode rail / context 三段。

保留例外：

- `About` 是独立 360×… 小窗，不是主窗口页面。
- 表格单元格编辑是内容能力缺口（#218），不是布局问题。

## 8. 验收

- `TestSummaryCardIsCentered`：完成总结中心与窗口中心的误差 ≤ 1 DIP，
  1024×720 与 720×520 各跑一次。
- `TestAppBarAtMinimumSize`：brand、五个主 tab、视图切换和溢出入口都在窗口内；
  tab 可点击切换 Browse / Notes。
- 既有 `TestReviewTakesTheWindow`、`TestMinimumSizeControls` 与
  `TestReviewAtTheSmallestWindow` 继续通过。
