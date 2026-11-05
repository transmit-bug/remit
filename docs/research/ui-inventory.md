# mygo UI 能力与惯例盘点（工单 #16）

调研日期：2026-11-06。mygo 版本：**v0.7.4**（`go list -m -f '{{.Dir}}' github.com/egoist/mygo` 实测解析路径 `/Users/pony/go/pkg/mod/github.com/egoist/mygo@v0.7.4`）。信息源：该模块内 `docs/ui/*` 与 `ui/` 源码、仓库内 `.agents/skills/mygo-components/SKILL.md` 等技能文件、`docs/i18n.md`。除标注「推断」外均为事实，逐条给来源。前置输入：#4 的 `docs/research/mygo-capabilities.md`（research/mygo-capabilities 分支），本文只深化 UI 侧，不重复其粗盘点。

来源缩写：模块根 `$M` = mygo@v0.7.4 安装目录；SKILL = `.agents/skills/mygo-components/SKILL.md`。

## 1. 组件映射：Remit UI 需求 × mygo 组件

判定档位：**官方组件**（直接用）、**官方组合**（2+ 现成 widget 拼装，无自绘核心）、**需 wrapper 的缺口**（无承载该语义的现成 widget，按 SKILL 步骤 4 应落在项目 widget 包里的一个小 wrapper，而非页内手写）。

| Remit 需求 | 落位 | 用什么 | 来源 |
|---|---|---|---|
| 复习队列列表 | 官方组件 | `ui.List` + `Key`（稳定行 ID）、`FollowEnd`、`Reorder`（拖拽调序）、`DragSelect`；百万行级（可见行 14.5µs/帧） | SKILL Collections 表；$M/docs/ui/list.md、performance.md |
| 翻卡视图 | 需 wrapper 的缺口（薄） | 无「翻面卡片」widget。积木齐全：`Element.Rotate`/`Scale`/`TransformOrigin` 做翻转几何，`Transition(ElementTransition{…})` 或 `AnimatePose` 做进出场，`Keyframes` 可做非对称翻面曲线；正面/反面各是普通 `Column`。属一个 FlipCard wrapper（~百行级）而非缺口级工作量（推断：几何+动效原语完备，见 §3） | $M/docs/ui/transforms.md、motion.md、transitions.md |
| 评分按钮组（FSRS Again/Hard/Good/Easy） | 官方组合 | 一排 `ui.Button`（`PrimaryButton` 标 Good）+ 每个按钮内嵌下次间隔预览文本；键盘 `1–4` 走 `OnShortcut`。注：`ui.Rating`/`ui.ToggleGroup`/`ui.Segmented` 语义都不对（星级/停留选中态/分段视图），评分按钮是「一次性动作排」——用 Buttons 是记录在案的组合决策，不是偷懒（推断，语义判定依据 SKILL「Name the need, not the shape」） | SKILL Actions 表、mygo-usage SKILL（OnShortcut） |
| 侧栏导航（牌组/过滤器） | 官方组件 | `ui.Sidebar` + `SidebarSection` + `SidebarItem` + `ui.Badge`（到期计数）；`ui.Router` 管页面历史，`Breadcrumbs` 给层级路径 | SKILL Navigation 表；$M/docs/ui/sidebar.md、navigation.md |
| 文档树 | 官方组件 | `ui.Tree`/`TreeItem`；大文档或需多列时 `ui.Outline`/`OutlineTable`。RTL 下树自动从右缩进、Left 键展开 | SKILL Collections 表；$M/docs/ui/tree.md、outline.md、right-to-left.md |
| 块编辑器 | 需 wrapper 的缺口（#4 已定性：交互层自建） | 基座：`richtext.NewDocument`+`richtext.Editor`（块内富文本）、`ui.List` `Reorder`（块拖拽排序）、`Tree`/`Outline`（块层级）、`MenuButton`/`Popover`（slash 命令菜单）、`markdown.View`（只读态）。无现成 slash/块拖拽嵌套/块折叠大纲 widget | $M/docs/ui/rich-text-editor.md、list.md、menu-button.md；#4 调研结论（见 mygo-capabilities.md §3） |
| 统计图表 | 官方组件 | `ui/chart`：`Plot`+`Axes`、`NewLinear`/`NewBand`/`NewTime` 刻度、`Rect`/`Line`/`Area` marks——复习曲线/遗忘曲线/热力条形均可；日历热图配 `ui.Calendar` | SKILL Status/charts 表；$M/docs/ui/chart.md、calendar.md |
| 设置表单 | 官方组件 | `ui.Form` + `ui.Field`（`.Description`/`.Error`、`Fieldset.Disabled`）+ `Switch`/`Select`/`NumberInput`/`Stepper`/`Slider` 等；错误提示由 Field 承载，勿手画 | SKILL Containers/Choices 表；$M/docs/ui/form.md |
| 搜索 | 官方组件 | `ui.SearchField`（Esc 清空）；牌组内全文查找 `ui.FindBar`；模糊跳转列表可用 `Autocomplete`/`Combobox` | SKILL Text 表；$M/docs/ui/search-field.md、find-bar.md |
| toast / 对话框 | 官方组件 | `c.Toast`/`c.ToastAction`/`c.AddToast`（自动 announce）；确认类 `ui.AlertDialog`；自定义弹层 `DialogBase`/`Modal`/`Popover`；桌面级另可走 `mygo.Dialog.Message` | SKILL Dialogs 表；$M/docs/ui/toast.md、alert-dialog.md、dialog.md |

组合层提醒（SKILL「Feedback lands where the action is」）：复习完成/保存成功必须落 toast 或就地状态，只写日志 = 「点了没反应」bug。

## 2. 主题体系

`Theme` 结构体全量字段（$M/ui/theme.go:7-57，事实）：`Dark`、`Background`、`Surface`/`SurfaceHover`/`SurfacePressed`、`Border`、`Text`/`TextMuted`、`Accent`/`AccentHover`/`AccentPressed`/`AccentText`、`Danger`、`Warning`、`Success`、`Selection`、`Focus`、`Inverse`/`InverseText`（toast/tooltip 反色面，留零=Text/Background 互换）、`Scrollbar`/`ScrollbarWidth`、`Radius`、`Spacing`、`FontSize`、`Font`。

- **定制惯例**：拷贝默认主题改字段后 `c.SetTheme(&t)`；默认主题跟随系统外观与桌面设置，改了即脱离跟随（$M/docs/ui/styling.md「Themes」）。例：`t.Accent, t.Radius = ui.Hex("#7c3aed"), 8`。
- **token 化**：自绘元素尺寸一律 `t.Space(n)`（n×Spacing）与 `t.Rem(n)`（n×FontSize），字面量宽度是需具名说明的例外（SKILL「Lay it out without literal geometry」；styling.md）。
- **密度**：`Spacing` 4 默认、3 紧凑、5 宽松，全部 widget 内距/间隙/控件尺寸随之缩放——密度是主题级决策，不是逐 widget padding（styling.md；SKILL）。`FontSize` 受桌面 `TextScale` 自动放大。
- **深浅色**：窗口主题跟随系统并随系统切换热更；测试侧 `tt.SetDark(bool)`；局部主题 `ui.Themed(c, t)` 可在同一窗口嵌套明暗（如主题预览）（styling.md「A theme for part of the window」）。
- **桌面偏好**：`c.Preferences()` → `Accent`/`HighContrast`/`TextScale`/`ReduceMotion`，默认主题自动跟随（styling.md「The desktop's preferences」）。Remit 提供自身主题选项时应从 `*c.Theme()` 出发保留这些跟随，只覆盖想改的字段（推断，styling.md「set a copy」语义）。
- **颜色体系**：`ui.Hex/RGB/RGBA/Oklch`（Oklch 保留广色域真值+sRGB 回退）、`Mix`/`Alpha`；渐变四族 + 边框渐变 + `Stripes`；`Blur`/`Cache`/Material（glass 插件）均可用（styling.md）。无样式表、无状态选择器——状态即视图代码（`if row.Hovered() { … }`）。

## 3. 动效原语与 reduced-motion

全部来源：$M/docs/ui/motion.md、transitions.md（事实）：

- **值动画**：`AnimateTo/Animate/Spring(key, target, anim)`；spring 预设 `ui.Smooth/Snappy/Bouncy`，可按 `Duration+Bounce`（感知式）或 `Stiffness/Damping/Mass`（物理式）描述；弹簧精确解析，掉帧不改路径；中途改目标保留速度。`AnimateColor` 颜色插值。
- **布局过渡**：`Element.Transition(ElementTransition{Enter/Exit *Motion, Position/Size/Colors, Animation, Delay})`；进出可 `Collapse`；`Match`（shared element，SwiftUI matchedGeometryEffect 等价）——翻卡视图/列表行进出场的主原语。
- **只绘不动视图**：`AnimatePose(ui.Pose{X,Y,Scale,Rotate,Fade}, anim)` 最廉价（不重跑视图）；`ContentTransition{Numeric:true}` 数字滚动（统计页计数器）。
- **Keyframes/Timeline**：`Keyframes` 定值序列（Repeat/Alternate/Delay）；`Timeline` 多 tween 一条播放头（`AfterPrevious/WithPrevious/AtLabel/AtTime` 定位，`Paused/Reverse/Seek/Progress` 控制）——「复习完成庆祝」「卡片进场编排」用 timeline。
- **手势动效**：`Draggable`（Axis/Bounds 橡皮筋/Snap 抛掷落点/`ui.Project`/`ui.Rubberband`），`DragOffset/Dragging/Transitioning`——滑卡式复习交互（左滑 Again）可用 `Draggable`+`Snap` 实现，官方 swipe-to-remove 范式即此（motion.md「Gestures」示例，推断适用）。
- **reduced-motion**（关键惯例）：`c.Preferences().ReduceMotion` 时——值/pose 直达目标、序列停在末值；颜色过渡、Fade、内容淡入淡出**仍平滑**；`Draggable` 跟手但回位即时；timeline/keyframe 直达 To/From；`Loop`（spinner）照常。即**框架已替你降级，无需自写双路径**（motion.md「Less motion and frames」、transitions.md「When nothing moves」）。`c.DisableAnimations()`/`c.Instant()` 可主动关帧内动画（恢复窗口状态时）。
- 性能：帧只在有东西动时画；窗口不可见不画帧（motion.md「Out of sight」——自定义动画要从状态变更时刻计时，勿数帧）。

## 4. 无障碍与 i18n（UI 侧惯例）

来源：$M/docs/ui/accessibility.md、right-to-left.md、i18n.md、locale.go（事实）：

- **命名三件套**：`.Label()`（无文本元素必须给，兼作测试定位键）、`.Role()`（自绘控件声明 `RoleSwitch`/`RoleHeading`+`Level`/`RoleMenu*` 等）、`.Description()`/`.Error()`。Field 会把下方文字自动挂到控件。禁用控件的 tooltip 不显示——原因要放可见文字或 `Field.Description`（SKILL）。
- **状态**：`Checked/Mixed/Expanded/Value/Range/Step/ActiveDescendant`，ARIA 语义等价。
- **播报**：异步结果用 `c.Announce(text)`；toast 自带播报，router 自动播报页标题。读屏器只在被请求时才付无障碍开销。
- **键盘**：widget 自带完整键盘行为（List 上下键+「第 5/共 10000」播报、Table 排序调整、菜单 accelerators）；`IgnoreInput(true)` 让 hover 按钮对指针透明但读屏可达。
- **i18n**：catalog JSON（ARB 格式，CLDR 复数）→ 生成类型化函数 `locales.FilesSelected(n)`；`mygo.json` 配 `i18n.dir/source`；`SetLocale` 持久化并触发 `OnLocaleChanged`，`MYGO_LOCALE` 可覆盖。整应用单一显示语言（含 mygo 控件与菜单文案）。`Element.Lang(tag)` 标记局部外语文本（影响字形与字体回退，如中日 Han 消歧）；`c.Region`/`i18n` 包管数字日期本地化格式。英语 source + `zh-Hans.json` 正是「英文优先+简中一等」既定决策的原生形态（事实：机制即如此；适配判断为推断）。
- **RTL**：窗口方向随应用语言；逻辑属性 `MarginStart/End`、`AnchorTopStart` 等自动镜像，Row/滚动/侧栏/返回键/树缩进/表格列全部镜像；代码、图表、图标、`Draw` 内容**不**镜像。开发期 `MYGO_DIRECTION=rtl mygo dev` 全局试 RTL，inspector 有开关。Remit 现阶段语言（en/zh-Hans）非 RTL，但只要坚持逻辑属性就免费保留未来 RTL 面（推断）。

## 5. ui.NewTester 面（设计验证用法）

来源：$M/docs/ui/testing.md、$M/ui/headless.go:372-442（事实）：

- 构造：`ui.NewTester(view, w, h)`；定位靠文本或 `Label`（又一个给 icon 按钮 `.Label()` 的理由）。
- 输入：`Click/ClickAt/ClickWith(mod, "Row 3")/Press/Move/Release/Scroll/Wheel`、`Key/TypeKey/Type/Compose`（输入法）/`Command("copy")`、`RightClick`+`ChooseMenuItem`。
- 环境切换（UI 设计验证核心）：`SetDark`（明暗两版断言）、`SetPreferences`（accent/高对比/文字缩放/**ReduceMotion**）、`SetLocale`/`SetRegion`（语言与区域格式）、`SetDirection`（RTL 布局断言）、`SetSize/SetScale/SetZoom/SetFocused`。
- 断言/观察：`Texts/HasText/Find/Focused/Cursor/Clipboard/OpenedURLs/Announcements`（读屏播报可测！）、`Image()` 截帧快照。
- 时间：`tt.Now()/Advance(d)` 驱动动画/双击/撤销合并——**永不 sleep**；`FramePending` 断言动画收敛。
- 设计验证惯用法（推断，组合自上述 API）：每个核心界面写 4 组参数化 tester 用例（明/暗 × 默认/ReduceMotion），i18n 文案溢出用 `SetLocale("de")` 类长词语言测，RTL 用 `SetDirection`；toast 播报用 `Announcements` 断言。端到端（真窗口）走 automation 插件（testing.md「Testing the built app」）。

## 结论

组件映射十项中八项为官方组件/官方组合；唯二 wrapper 级：**翻卡视图**（薄：transform+transition 原语齐全）与**块编辑器交互层**（与 #4 结论一致，richtext.Editor+List/Tree 之上自建）。主题（含密度/深浅色/桌面偏好跟随）、动效（含自动 reduced-motion 降级）、无障碍（Label/Role/Announce 全链路+可测）、i18n（catalog+zh-Hans 一等）均有官方惯例背书。与既定决策（mygo 主体、英文+简中一等、桌面端）无冲突事实。
