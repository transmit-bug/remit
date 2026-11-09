# UI 重设计 v2 · 从 40 屏原型到可实现的切片

> 状态：**accepted（2026-11-10）**。用户已把实现细节授权给工程判断；本稿是 3 合 1 信息架构与
> 界面的实现口径，§10.2 的三项口径按默认值冻结。
> 基线：`master`（合并已落地），MyGo **v0.8.1**（`go list -m -f '{{.Dir}}' github.com/egoist/mygo`）。
> 原型：`ref/prototype/`（`DESIGN.md` 说明书 + `index.html`/`styles.css`/`app.js` 可点稿），
> 主路由静帧在 `/tmp/remit-shots/*.png`（`ref/prototype/shots/` 的 23 张为重复/损坏，不采用）。
> 决策地图：[#201](https://github.com/transmit-bug/remit/issues/201)。UI 地图：[#128](https://github.com/transmit-bug/remit/issues/128)。
> 执行顺序：[#158](https://github.com/transmit-bug/remit/issues/158)。打磨清单：[#164](https://github.com/transmit-bug/remit/issues/164)。

目标：把 UI 重建到与原型一致的形态，按 **功能 → 内容 → 测试** 三段推进；每个实现切片一个 PR，
每个切片先读技能再动手（§8 每条工作流都写明这条规则）。

---

## 0. 状态与关系

### 0.1 本稿与既有文档的关系

| 文档 | 处理 | 具体哪一段 |
|---|---|---|
| `docs/specs/ui-redesign.md` | **部分作废** | §2 要素清单（2.1–2.8）、§3 视觉系统数值、§4 屏幕布局、§5 实现映射、§6 分期 P0–P3 → 全部由本稿取代。§1「为什么上一轮不够」保留为背景，不再作为规范。§7「不做」并入本稿 §10。 |
| `docs/specs/ui-inventory.md` | **重写** | 主键从「屏」改为「面」：本稿 §3 的 `S01–S52` 一行一面，列为 `面 / 需求 / 官方组件 / MyGo 版本 / 状态（slice 号或缺口）/ 验收测试名`。旧的分节（Shell / Notebook / Review / Browse / Stats and settings）作废；`Known gaps` 一节保留，改为「缺口条目必须写明查过哪些组件」。 |
| `docs/specs/ui-composition.md` | **规则保留，清单重排** | §1 规则、§2 失效机制、§4 闸门测试继续有效；§3「现状 → 目标」与 §5 迁移切片 S1–S3 由本稿 §8 的工作流取代；§6 验收命令沿用（本稿 §9 引用）。 |
| `docs/ui-layout-rules.md` | **保留骨架，替换骨架数量与宽度** | 三种骨架（Paper / Workspace / Session）保留；应用栏一节由本稿 §2.2 取代（顶栏不再是 brand + mode rail + context 的单条，而是 rail + 顶栏 + 子导航 + 状态行四件）；§2 宽度表按本稿 §7.4 更新。 |
| `docs/specs/incremental-reading.md` | **扩展** | §3 事件形状、§7 阅读会话屏、§16 差异说明由本稿 §1.5/§5.2/§4 扩展（新增三种终态事件、推后、归档页）；§4 重放、§6 摘录、§8 双态入口保留。 |
| `docs/specs/block-tools.md` | **保留 + 补入口** | 手柄、`+`、拖动、块菜单、`/` 插入、表格与图片的边界不变；新增「选区动作条」（S15）与「块检查器」（S20）两个入口，写在本稿 §4。 |
| `docs/specs/plugins/ui.md` | **扩展，不破坏** | §4 槽位表新增 `today.widget`、`stats.block`；`status.item` 登记为预留（不展示）。§3 上下文键新增 `area`、`sub`（新增键 = 兼容变更），`mode` 的取值表不变。 |
| `docs/specs/three-in-one.md` | **回填一句** | 1.0 范围表「平台」行与 §6「队列」缺口：本稿 §1.2 把「统一 Today」判为 1.0 内，实现后回填；§2 注里「统一 Today 仍是开放决策」改为指向本稿。 |
| `GLOSSARY.md` | **新增 6 条** | 见 §1.7（Today / 预算 / 归档 / 不再出现 / 推后 / 摘录树）。 |
| `docs/verification/handoff.md` | **追加项** | 追加：rail 折叠、白板页、插件失败面板、归档/不再出现恢复、预设写盘。 |
| `docs/adr/*` | **新增与修订** | 新增 ADR 0010（六区导航与 shell）、ADR 0011（库格式 v3：终态事件与预设）；修订 ADR 0002（归档不删源文）与 ADR 0003（事件种类与配置版本）。0004/0009 照旧；不引入新实体。 |
| 其余决策 | **不写 ADR** | §1.5（捕获/AI 拒绝）、§1.6（parity 矩阵推迟）、§1.10（原型即依据）、§1.11（SOP 推迟）与 §1.4 的 UI 交互部分只在本文与 GLOSSARY 记录：它们不改数据模型与存储格式，ADR 只收不可逆的模型/格式决定。 |

### 0.2 明文与一条日志的硬约束

1. **只有一份真相**：`notes/*.md`、`cards/*.json`、`types/*.json`、`assets/<sha>.<ext>`、
   `reviews.jsonl`（ADR 0003）。本稿新增的任何状态只能是「日志里的一条事件」或
   「`remit.json` 里的一个键」，不能是只存在于索引的第三种来源。
2. **索引可丢**：除 agent API 的只读路由外，一切派生（今日队列、阅读位置、归档标记、难卡判定、
   统计）必须能从明文重建；`internal/core/store/index.go:24` 的 `schemaVersion` 变动只许动派生表。
3. **时间注入**：core 不读时钟（`store.Options.Now`）；UI 读 `c.Now()` 或已传入的时刻。
   「今天」「明日」「N 天后」全部由 `now` 算出。
4. **UI 不碰 store**：帧只读快照或 `Deps`；写入一律走 handler → App → store → reload
   （ADR 0004/0009，`internal/ui/screens/*` 不 import `internal/ui` 与兄弟包）。
5. **隐私**：不新增出站请求。原型里 `.remitlib` / Markdown / Anki 三种导入都是本地路径。

### 0.3 当前代码与原型的主要差异（谁关掉它）

| 差异 | 今天 | 原型 | 关掉的切片 |
|---|---|---|---|
| 导航形态 | 单条 `ui.Toolbar`（`internal/ui/window.go:19`）：brand + 5 个 tab + context | 左 rail 240 + 顶栏 60 + 子导航 44 + 状态行 30 | W1 |
| 主页面数量 | 5（笔记 / 复习 / 浏览 / 统计 / 设置），`mode` 枚举 | 6 区域 + 插件页；子页 30+ | W1 |
| 首屏 | 复习队列（决策 #11，`internal/ui/library.go` apply） | 今日（混合队列 + 预算） | W2 + B1 |
| 字阶 | 22 / 17 / 15 / 13（`design/scale.go`） | 34 / 27 / 22 / 18 / 13 / 12 / 11 | W1（§7.2） |
| 顶栏高度 | `FrameHeight 64` | 60 + 子导航 44 + 状态行 30 | W1 |
| 文档面板 | `ui.SplitPercent` 比率（`screens/notes/notes.go`） | 固定 288 的 `ui.Split` | W3b |
| 状态行 | 纸面内 11px 一行（`screens/notes/editor.go:141`） | 窗口底部通栏 30，含标签与第 N 段 | W3b |
| 卡片徽章 | 文档上方徽章条（`screens/notes/cards.go:11`） | 段落左沟槽的徽章（带状态形状） + 块检查器 | W3b |
| 阅读 | 只有会话，无队列页/归档页 | 读 · 队列 + 读 · 归档 + 会话 | W4 + B1 + B2 |
| 浏览 | `ui.List` + `ListState` 手拼行（`screens/browse`） | `ui.Table` + 左侧三组筛选 + 批量动作 | W5 |
| 类型 | 在设置页里（`screens/settings/types.go`） | 忆 · 类型（主从工作台） | W5 |
| 设置 | 一页 `Form` + 四组 | 8 个子页 | W7 |
| 统计 | 热力 + 保留率 + 预测，一页 | 总览（含产出、时间配比）/ 热力 / 预测 三页 | W6 + B4 |
| 归档 / 不再出现 / 推后 | 无 | 阅读会话的 A / D / P 与归档页 | W4 + B2 |
| 预设 | 保留率在插件策略里，无界面 | 忆 · 预设（写库配置） | W5 + B3 |
| 插件面 | 8 个槽位，2 个有接收方 | 11 个槽位全有接收方，失败面板可见 | W9 + B6 |

---

## 1. 已冻结决策

逐条对应 [#205](https://github.com/transmit-bug/remit/issues/205)–[#216](https://github.com/transmit-bug/remit/issues/216)。
「采用 / 调整 / 推迟 / 拒绝」是本稿对原型的裁定；每条后面写清对 store / schedule / queue / UI 的后果。

### 1.1 #205 统一知识单元 — **调整：Note 是阅读单位，Block 是位置，不引入 Reading Item**

阅读队列的成员继续是**笔记**（`store.Reading{Note, State, Position, Queued}`，
`internal/core/store/reading.go`），位置是块 ID，摘录是子笔记。理由是原型的每一面都能用这套
表达：今日队列一行是笔记 + 第几段，会话按块推进，摘录树按「子笔记指向源块」成层。引入
first-class Reading Item 会带来第二个身份体系（与块 ID 并存的引用）、第二套反链、第二套删除语义，
而它要解决的两件事——「一段比一篇更细的注意力单位」与「优先级」——可以由块位置与队列排序给出。

后果：`reviews.jsonl` 的事件形状不变（`note` + `position`）；`reading` 索引表只加终态列（§5.2）；
卡片的 `StatusReading` 继续是预留位，不启用。#208 的字符区间摘录因此**推迟**（§1.4）。

### 1.2 #206 统一队列 — **采用：今日是混合队列，按时间预算排序**

首屏改为**今日**：一篇待读、一行「读过还没摘」、一段卡片复习聚合，三者按 §5.1 的顺序排，
总时长对着预算（默认 30 分钟）。这否掉了今天的「复习队列是默认首屏」（决策 #11），
是 1.0 范围内的一次产品改判，理由：原型的核心流是「今日 → 开始 → 读 → 摘 → 忆 → 回原文」，
而以复习开局的界面无法把「读」放到同一张待办上。

后果：`internal/core/queue` 新增 `today.go`（§5.1）；`internal/ui` 新增 `screens/today`；
`App` 的 `mode` 由「区域 + 子页」取代；托盘 / 提醒的两个计数不变（`App.queueCounts`）。

### 1.3 #207 调度与优先级 — **采用：FSRS 定间隔，优先级只做排序与限额**

FSRS 继续是唯一的间隔来源（`schedule.Step` / `Applier`，`internal/core/schedule/schedule.go`）。
原型里的「priority」不是存储字段，而是**队列排序层**：今天的顺序由「到期 → 入队先后 → 预算」
算出，不写任何 priority 数值到卡或笔记。SuperMemo 的 A-Factor / 第二套调度器**拒绝**
（与 `three-in-one.md` §5 一致）。每日上限落在库配置的 `presets`（§5.3），不是调度事件。
「难卡」不是页面也不是类型，是 `lapses ≥ presets.hard.lapses` 的**筛选条件**（默认 3）；
lapses 由重放数出（§5.5），不写进卡片文件。

后果：`queue.Options` 加预算字段；`schedule.State` 加 `Lapses`（派生）；索引 `card_state` 加列；
卡片表、队列 chip、搜索语法 `is:hard` 三处读同一个数。

### 1.4 #208 阅读态制卡 — **调整：块级摘录立即做，字符区间推迟；选区动作条落在编辑区**

摘录的粒度继续是**块**（与 `incremental-reading.md` §16.1 一致）：会话里「当前块即选区」。
原型中「选中一句话 → 挖空」不需要区间引用——cloze 是把选中的文字改写成 `{{…}}` 写回同一个块，
卡片仍引用这个块，所以**采用**。原型中「摘录整段」= 现有 `Library.Excerpt`（块级），**采用**。
字符区间（`note_…` + 起止偏移）作为第四个引用维度**推迟**到 #208 的后续票：它会让引用随编辑漂移，
与 ADR 0002「移动与重排不改变引用」冲突，需要单独的模型决定。

后果：编辑器新增选区动作条（S15，`internal/ui/screens/notes`）；`core.Cloze` 新增（§5.6）；
不新增字段、不改日志。

### 1.5 #209 文章捕获与 AI — **拒绝（本轮）**

捕获是 importer 与插件的地盘（#153 已关，`contributes.importers`），不在本稿范围。
原型里的导入向导只覆盖 `.remitlib` / Markdown / Anki 三种**本地**来源，本稿照此实现。
网络抓取、HTML/EPUB 转换、AI 修复与出题一律**不做**（`three-in-one.md` §7、#204 的结论）。

后果：导入向导（S02）只有三个来源；`internal/ui/import.go` 的三个既有入口被重排成向导，
不新增 importer。

### 1.6 #210 全功能 parity matrix — **推迟**

矩阵是产品层的总表（#210），本稿只交付 UI 的一种形态。本稿不声称追平任何产品；
凡原型中出现、而本稿 §10 列为「不做」的条目，都是下一张矩阵票的输入。

### 1.7 #211 术语 — **调整：不引入 Topic / Item / Read Point / Priority；新增 6 个界面词**

`GLOSSARY.md` 只收「会进数据或改变语义」的词。所以：

| 词 | 处置 | 定义（写入 GLOSSARY 的原文要点） |
|---|---|---|
| **Today**（今日） | 新增 | 默认首屏；今日待读、读过未摘、今日到期卡片合成的一张队列，顺序与截断由预算决定。_Avoid_: 仪表盘, dashboard |
| **Budget**（预算） | 新增 | 今日队列的时间上限与卡片张数上限，写在库配置 `presets.budget`；它不是提醒，是排序与截断的输入。_Avoid_: 目标, goal |
| **Archived**（归档） | 新增 | 读完收起的笔记：不再进队列，仍可搜索、仍可被引用、仍可制卡。由 `archived` 事件表示。_Avoid_: done（暗示删除）, 完成 |
| **Dismissed**（不再出现） | 新增 | 明确不再排进今日的笔记；比归档更强，只能手动恢复。由 `dismissed` 事件表示。_Avoid_: 删除, drop（drop 是移出队列） |
| **Postponed**（推后） | 新增 | 保持队列成员、把到期推到下一次的笔记；不改评分史。由 `postponed` 事件表示。_Avoid_: snooze, 延期 |
| **Excerpt Tree**（摘录树） | 新增 | 以摘录的「子笔记 → 源块」引用构成的层级视图；不是新实体。_Avoid_: 大纲（那是标题树） |

拒绝入库的词：`Topic` / `Item`（SuperMemo 的单元，Remit 用 Note 与 Card 表达）、
`Reading Item`（§1.1）、`Read Point`（= Reading Position 的近义词，已有词）、
`Priority`（= 队列排序，不是模型名词）。`增量阅读` 保留，含义升级为「note-level 队列 +
块位置 + 整块摘录 + 终态」，与本稿一致。

### 1.8 #212 信息架构 — **采用：一个工作台、六个区域、一条会话层**

一级是「我要做哪一类事」：今日 / 写 / 读 / 忆 / 统计 / 设置，加一条插件页分组（左下）。
二级是「这一类里的哪一面」（写：笔记 / 搜索 / 标签 / 摘录树 / 资产 / 模板；忆：队列 / 卡片 /
类型 / 预设 / 历史 / 断链；统计：总览 / 热力 / 预测；设置：外观 / 复习 / 预算 / 快捷键 /
插件 / Agent / 更新 / 关于）。会话（阅读 / 复习）与浮层不占区域，全屏接管，`Esc` 回到进入处。
顶栏只放当前上下文（面包屑 + 该面的动作），导航不再和动作抢一行。

后果：`internal/ui/app.go` 的 `mode` 枚举改为 `area` + `sub` + `session` + `overlay`（§2.7）；
插件 `when` 的 `mode` 取值**不变**（见 §6.4）；`internal/deeplink` 新增三种路由（§5.7）。

### 1.9 #213 数据格式 — **调整：日志加三种事件，库配置升到 `remit/3`，笔记与卡片文件不变**

笔记与卡片的明文形态不动；`reviews.jsonl` 增加 `archived` / `dismissed` / `postponed`
三种事件（§5.2）；库配置文件 `remit.json` 从 `remit/2` 升到 `remit/3`，新增 `presets` 段并
保留未知键（§5.3）。索引 `schemaVersion 10 → 11`，只加派生列与一张 `assets` 表。

后果：`store.FormatVersion`（`internal/core/store/library.go:58`）改值；打开 v2 库时迁移并写
`remit.json.bak`；v2 读者会拒绝 v3 库（格式值不同），回滚靠备份文件（迁移与回滚口径见 §5.3）。

### 1.10 #214 低保真原型 — **采用：`ref/prototype` 即验收依据**

原型的 40+ 面、深链接、可点交互作为本稿的视觉与交互依据；票 #214 由本稿收口，
实现票从 §4 的屏规格拆出，不再单独做第二份原型。

### 1.11 #215 SOP — **推迟**

发布节奏与 parity 维护流程不属 UI 重设计；本稿只遵守既有的「一票一 PR」与
`docs/engineering/conventions.md`。

### 1.12 #216 终态语义 — **采用：归档代替破坏性 Done；四种终态各有一条事件**

| 动作 | 语义 | 写什么 | 可恢复 | 影响 |
|---|---|---|---|---|
| 读完（1/2/3） | 记难度与位置，按 FSRS 安排下次 | `reviewed`（既有） | 撤销 = `void` 那条 | 离开今日，到期后回来 |
| 跳过（S） | 本篇今天不再问，不产生记录 | 无 | — | 会话内前进，换会话重来 |
| 归档（A） | 收起，不进队列 | `archived` | 归档页「恢复」→ `queued` | 仍可搜索 / 引用 / 制卡 |
| 不再出现（D） | 明确不再排进今日 | `dismissed` | 归档页「恢复」→ `queued` | 同上，且不会被自动加回 |
| 推后（P） | 保持成员，顺延到明天 | `postponed`（带 `until`） | 撤销 = 再 `postponed` 到今日 | 今天不再出现 |
| 移出队列（X） | 取消成员资格（既有 `dropped`） | `dropped` | 重新「加入阅读队列」 | 历史与位置保留 |

绝不因为「读完」删除正文：`three-in-one.md` §5 已把破坏性 Done 判为 reject。
归档与不再出现的笔记仍可被卡片引用；源块被删除时走既有的 lost 语义，与归档无关。

---

## 2. 应用骨架

### 2.1 四件套

```
┌───────────┬──────────────────────────────────────────────────────────────┐
│ ◇ Remit   │ [⧉]  Remit / 忆 / 卡片          [搜索 ⌘K]        [⋯]         │ 60  FrameHeight
│           ├──────────────────────────────────────────────────────────────┤
│ ▣ 今日  22│ 队列  卡片 25  类型  预设  历史  断链                          │ 44  SubnavHeight
│ ✎ 写     8├──────────────────────────────────────────────────────────────┤
│ ▤ 读     3│                                                              │
│ ▥ 忆    19│                        舞台（骨架见 §2.4）                    │ Grow(1)
│ ▨ 统计    │                                                              │
│ ⚙ 设置    │                                                              │
│           │                                                              │
│ ⚡ 制卡脚本 │                                                              │
│ ⬚ 白板    │                                                              │
│           ├──────────────────────────────────────────────────────────────┤
│ ‹ 收起    │ 1,204 字 · 14 段 · 6 张卡 · 已保存 · #方法论 · 第 7 段         │ 30  StatusbarHeight
└───────────┴──────────────────────────────────────────────────────────────┘
     240                         （窗口宽 >980；≤980 收成 64 图标条）
```

### 2.2 每一件用什么组件（MyGo v0.8.1）

| 件 | 结构 | 组件 | 备注 |
|---|---|---|---|
| 导航轨 | `Column` 固定宽，右 1px hairline，`Surface` 底 | `ui.Column` + 品牌行 + `ui.Sidebar`（`Grow(1)`）+ `ui.Divider` + 收起按钮 | `ui.Sidebar` 是官方 source list：图标/标签/计数/分组/键盘/读屏角色都在它里面。品牌行用 `ui.Row`（26 强调色方标 + `Remit` 字标，非按钮）。 |
| 轨道项 | 一行：图标 + 标签 + `ui.Badge` | `ui.SidebarItem(c, area, icon, label).Children(Badge)` | 当前区域用 `.Background(tk.Accent.Alpha(0.10)).TextColor(tk.Accent)`，与 `Sidebar` 自己的选择态同色系，不改它的键盘语义。计数为 0 时不加 Badge。`Sidebar` 的 `*string` 选择值是**区域 id**（`"today"`…`"settings"`），由 `area` 推出，不另存一份。 |
| 轨道分组（插件页） | 底部一道 hairline 之后的第二组 | `ui.SidebarSection(c, msgs.PluginPages(), nil, …)` | 无页面时整段不建。 |
| 收起/展开 | 轨道底部 34 高图标按钮 | `ui.Button`（图标 + `.Label` + `.Tooltip`） | 见 §2.5。 |
| 顶栏 | 高 60，`Surface` 底，下 hairline | `ui.Toolbar` | 官方 overflow：窗口变窄时尾部控件自动进菜单（`docs/ui/toolbar.md`）。 |
| 顶栏：轨道开关 | 左起第一个 | `ui.Button` 图标-only | 窄窗口下由顶栏的它代替轨道底部按钮。 |
| 顶栏：面包屑 | `Remit / 忆 / 卡片` | `ui.Breadcrumbs(c, items, &chosen)` | `chosen` 必须是有效指针：组件点击非末段会写它。初始化 `len(items)-1`；末段是 `CurrentPage`，不可点；点击前段 = 回到该区域，`Changed()` 后调 `setArea`。 |
| 顶栏：上下文动作 | 分段 + 图标按钮 + `⋯` | `ui.SegmentedBase`（紧凑外观，见 `segmentedChoice`）+ 图标 `ui.Button` + `ui.MenuButton`/`Button.Menu` | 沿用 `internal/ui/window.go` 已有的 `segmentedChoice` 与 `iconButton`。 |
| 子导航 | 文字 tab + 2px 下划线 + 计数徽章 | `ui.TabsBase` + `ui.Badge` + `DrawOver` 画下划线 | 与 `modeTabs`（`internal/ui/window.go`）同一写法，从顶栏搬到独立一行。 |
| 舞台 | `Scroll` → 居中 `Column(MaxWidth measure)` | `ui.Scroll` + `ui.Column` | 三类骨架见 §2.4。 |
| 文档分支 | 左编辑 / 右只读 / 右面板 | `ui.Split`（宽度，非百分比） + `ui.SplitPercent`（分栏 50/50） | 面板宽度默认 `PanelWidth 288`，`SplitReset`。 |
| 状态行 | 高 30，`Surface` 底，上 hairline，`TypeCaption` | `ui.Row` + `ui.Text` + `ui.Divider`（作为间隔） | 只在文档面（S11–S13）出现，其余面隐藏。 |
| 面板 | 288 宽的一列，左侧细线，与纸面同级（不浮起） | `ui.Split` 第二栏 + `ui.Segmented`（大纲/反链） + `ui.Outline` / `ui.List` | 窄窗口收进 `ui.Overlay`（§2.6）。 |
| 会话 | 全屏三行：会话栏 / 舞台 / 动作行 | `ui.Column` ×3 + 既有 `screens/review`、`screens/reading` 的骨架 | 隐藏 rail/顶栏/子导航/状态行；`Esc` 退出。 |
| 浮层 | 菜单 / 对话框 / 面板 | `ui.Popover`、`ui.PopoverMenuButton`、`ui.MenuButton`；`ui.DialogBase`、`ui.Modal`、`ui.AlertDialog`；`ui.Overlay` | 只有浮层带阴影（`tk.ShadowPanel`）。 |

### 2.3 交互状态口径

| 状态 | 规则 | 出处 |
|---|---|---|
| hover | 面变 `tk.SurfaceHover`，120ms（`design.Hover`）；不移动、不缩放布局 | 组件的 `Hovered()`；`design.Hover` |
| press | `PressScale 0.98`，90ms；不改变占位 | `.AnimatePose` + `design.Press` |
| selected | 轨道/子导航/列表：`Accent.Alpha(0.10)` 底 + `tk.Text` 字（轨道项用 `tk.Accent` 字）；表格行用 `ListState.Selected()` | 组件自己的选择态 |
| focus | 由控件自己画 2px 环（`FocusRing`），宿主不另画；`FocusWithin` 只用于「面板打开时把焦点送进面板」 | `ui.Handle` |
| disabled | 半透明 + 不响应键盘；**不显示 tooltip**，原因写在可见文字或 `Field.Description` 里 | `docs/ui/toolbar.md`、`mygo-components` §「Feedback」 |
| 危险动作 | `AlertDialog` 确认，按钮文案带后果（「删除，3 张卡转为断链」） | S16 / S20 |

### 2.4 三种页面骨架（沿用 `docs/ui-layout-rules.md`，数值按本稿）

| 骨架 | 面 | 结构 |
|---|---|---|
| Paper | S04 今日、S06 搜索、S36–S38 统计、S39–S46 设置、S50 制卡脚本 | `Scroll` → `Column(FillWidth, AlignItems Center, Padding(SpaceXL))` → `Column(MaxWidth measure)` |
| List | S05 笔记、S07 标签、S08 资产、S09 模板、S10 摘录树、S23 队列、S24 归档、S27 忆队列、S31 卡片、S32 类型、S33 预设、S34 历史、S35 断链 | `Scroll` → `Column(MaxWidth ListWidth/BrowseWidth)`；行用 `ui.List`，表格用 `ui.Table` |
| Workspace | S11–S13 文档 | `ui.Split`（笔记列 260） → `ui.Split`（面板 288）/ `SplitPercent`（分栏） |
| Session | S25 阅读会话、S29 复习、S30 总结 | 三行：会话栏（`ReviewWidth` 内居中）/ 舞台（`Center()`）/ 动作行 |

### 2.5 折叠规则

| 断点 | 条件 | 动作 |
|---|---|---|
| `BreakpointNarrow = 980` | 窗口宽 ≤980 | ① 轨道收成 64：`SidebarItem` 的 label 传空、`.Tooltip(label)` 补名字，计数 Badge 仍显示（只显示一位数，超过 9 显示 `9+`）。② 文档面板收起；顶栏面板按钮改为打开 `ui.Overlay`（同内容，`Esc` 关）。 |
| `BreakpointSmall = 760` | 窗口宽 ≤760 | ③ 纸面左右边距 `Gutter 48 → GutterNarrow 20`，`TypeDisplay 34 → DisplayNarrow 30`；measure 不缩。④ 复习评分两列（`ui.Grid` 两列），阅读动作行换行。⑤ 块沟槽从纸面左侧的 48 沟槽改为**段落上方一行**（`+` / `⋮⋮` / 徽章），因为左侧没有余量。 |
| 720×520（最小窗口） | 由 ①–⑤ 全部生效 | 顶栏尾部动作进 `ui.Toolbar` 的 overflow；子导航仍常驻（`ui.TabsBase` 自己横向滚动）；**整窗不出现横向滚动**（测试 `TestNoHorizontalScrollAt720`）。 |

声明：折叠是**产品规则**，不是布局计算，因此写在 `design`（断点常量）与屏（`panelShown` 同形）里，
不写在各屏的 `Width()` 里。

### 2.6 焦点顺序

1. 轨道（`ui.Sidebar` 是一个 tab stop，内部 ↑↓/Home/End/首字母）。
2. 顶栏（`ui.Toolbar` 是一个 tab stop，内部 ←→）。
3. 子导航（`ui.TabsBase` 一个 stop，内部 ←→）。
4. 舞台：按文档顺序；`ui.List`/`ui.Table` 各自是一个 stop；文档内是编辑器 → 沟槽手柄 → 面板。
5. 状态行不可聚焦。
6. 浮层打开时焦点进入浮层，`Esc` 关闭并把焦点还给打开它的控件（`ui.DialogBase` 已实现）；
   会话打开时焦点进舞台，`Esc` 回到进入处的那个控件。

### 2.7 路由模型

`App` 的 `mode` 枚举（`internal/ui/app.go`）拆成四个字段：

```go
type areaID uint8   // areaToday, areaWrite, areaRead, areaRecall, areaStats, areaSettings, areaPlugin
type subID  uint8   // 每个区域自己的子页；areaToday 无子页
// App 新增字段
area  areaID
sub   subID
session sessionKind // sessionNone / sessionReading / sessionReview
overlay overlayKind // overlayNone / overlayPalette / overlayKeys / overlayImport / …（也可用已有的 open 布尔）
```

- `mode` 不再是状态，而是**派生**：`App` 仍保留 `mode` 字段（既有代码与测试读它），由
  `syncMode()`（**新增**，在 `setArea`/`setSub`/会话开关里唯一的写入点）从 `area`+`sub`+`session` 算出；
  它的取值仍是 `notes / review / browse / stats / settings / ""`（§6.4），供插件 `when` 使用。
- 深链接与 hash：`#/area/sub[/id][?mode=|tag=|q=]`；`internal/deeplink` 新增三种
  （`today`、`read`、`recall`），既有的 `note` / `card` / `queue` / `search` 保持。
- `App.setArea(area, sub)` 与 `App.setSub(sub)` 是唯一的导航入口；任何控件不得直接改 `area`。

### 2.8 键盘地图

| 键 | 动作 | 作用域 |
|---|---|---|
| `⌘K` | 命令面板（S49） | 全局 |
| `⌘/` | 快捷键总表（S48） | 全局 |
| `⌘,` | 设置 ·（进入进入前的子页） | 全局 |
| `⌘N` | 新建笔记 | 全局 |
| `⌘F` | 文档面：查找条；其余面：跳搜索页 | 全局 |
| `⌘⇧P` | 文档面：面板开关 | 文档 |
| `⌘1`…`⌘6` | 今日 / 写 / 读 / 忆 / 统计 / 设置 | 全局 |
| `Esc` | 关浮层 → 退出会话 → 设置回上一区域 | 分层 |
| `J` / `K`、`↓` / `↑` | 阅读：下/上一段 | 阅读会话 |
| `1` / `2` / `3` | 阅读：太难 / 刚好 / 太浅 | 阅读会话 |
| `E` | 阅读：摘录当前段；复习：编辑卡片 | 会话 |
| `S` | 阅读：跳过 | 阅读会话 |
| `X` | 阅读：移出队列 | 阅读会话 |
| `A` / `D` / `P` | 阅读：归档 / 不再出现 / 推后 | 阅读会话 |
| `M` | 阅读：在库里打开 | 阅读会话 |
| `Space` | 复习：显示答案 | 复习会话 |
| `1`–`4` | 复习：评分（只在答案显示后；键入作答时数字进输入框） | 复习会话 |
| `U` | 复习：撤销上一次评分 | 复习会话 |
| `/` | 文档：插入菜单 | 文档 |
| `⌘Z` / `⇧⌘Z` | 撤销 / 重做 | 文档 |

`⌘1…⌘3` 的含义从「复习 / 笔记 / 浏览」改为「今日 / 写 / 读」，是既有键位的**改判**：
`internal/ui/window.go` 的 `shortcuts` 与 `internal/ui/window_test.go` 需同改，验收测试
`TestAreaShortcutsSwitchAreas`。

---

## 3. 区域 / 路由 / 屏幕清单

形态：**页**（区域子页）/ **会话**（全屏接管）/ **浮层**（菜单、对话框、面板内嵌层）。
数据源一列写「谁提供」，全部经 `Deps` 或快照，帧不读 store。

| id | 面 | 区域 | 子 | 路由 | 形态 | 数据源 | 主动作 | 键 | 空态 |
|---|---|---|---|---|---|---|---|---|---|
| S01 | 欢迎 | — | — | 启动（无库） | 页 | `App.start` / `openErr` / 最近库（新） | 打开库、新建库、导入 | — | 无最近库时只留三个入口；库打不开时顶部 banner + 「换一个」 |
| S02 | 导入向导 | 浮层 | — | 顶栏 `⋯` → 导入 | 浮层 | `importmd` / `importanki` / `.remitlib` 报告 | 选来源 → 预览 → 完成 | `Esc` | 空目录：预览计数全 0，下一步禁用并写明原因 |
| S03 | 导出 | 浮层 | — | 顶栏 `⋯` → 导出 | 浮层 | `Library.Notes` + `SaveNote` 渲染；`.remitlib` 走 `transfer.Write` | Markdown 目录 / `.remitlib` / 卡片 CSV | `Esc` | 无笔记时两项禁用（CSV 永久禁用并写明原因） |
| S04 | 今日 | 今日 | — | `#/today` · `remit://today` | 页 | `queue.Today`（B1） | 开始、打开某篇、复习 | `⌘K` | 全空：图标 + 一句 + 「加入一篇笔记」 |
| S05 | 写·笔记 | 写 | 笔记 | `#/write/notes?tag=` | 页 | `QueryNoteRows`（B5） | 新建、打开、阅读、行菜单 | — | 无匹配：一句 + 清空筛选 |
| S06 | 写·搜索 | 写 | 搜索 | `#/write/search?q=` · `remit://search` | 页 | `Library.Search` | 打开笔记 / 卡片 | `⌘F` | 未输入：一句说明匹配范围 |
| S07 | 写·标签 | 写 | 标签 | `#/write/tags?tag=` | 页 | `QueryTags`（B5） | 点标签 → 筛过的 S05 | — | 「还没有标签」+ 一句写法 |
| S08 | 写·资产 | 写 | 资产 | `#/write/assets` | 页 | `AssetRows`（B5） | 导入、点被引用笔记 | — | 「还没有资产」+ 导入 |
| S09 | 写·模板 | 写 | 模板 | `#/write/templates` | 页 | `App.noteTemplates` | 从模板新建、空白新建 | — | 「还没有模板」+ 一句「加 `#template` 就是模板」 |
| S10 | 写·摘录树 | 写 | 摘录树 | `#/write/tree` | 页 | `ExcerptTree`（B5） | 打开节点、继续读 | — | 「还没有摘录」+ 「开始读一篇」 |
| S11 | 文档·编辑 | 写 | 文档 | `#/write/doc/<id>` | 页 | 草稿 + `Deps` | 写、格式、沟槽、`/`、选区动作 | `⌘F`/`/` | 空笔记：一行 placeholder |
| S12 | 文档·阅读 | 写 | 文档 | `#/write/doc/<id>?mode=read` | 页 | 同上 + `markdown.View` | 跳到块、摘录 | `E` | 同上 |
| S13 | 文档·分栏 | 写 | 文档 | `#/write/doc/<id>?mode=split` | 页 | 编辑器 + 只读 `ui/markdown` | 对照、拖动中线 | — | 同上 |
| S14 | 查找条 | 文档内 | — | `⌘F` | 内嵌层 | `richtext.Find`（既有） | 下一个/上一个、关闭 | `⌘F`/`Esc` | 「0 个结果」 |
| S15 | 选区动作条 | 文档内 | — | 选中文字 | 浮层 | 选区文本 + 当前块 | 挖空、制卡、摘录整段 | — | 选区 <2 字不出现 |
| S16 | 块菜单 | 浮层 | — | `⋮⋮` 或沟槽菜单键 | 浮层 | 当前块 + 它的卡片 | 上移/下移/复制/转换/摘录/制卡/删除/检查器 | `Esc` | — |
| S17 | 插入菜单 | 浮层 | — | 段首 `/` | 浮层 | 命令表（`screens/notes/slash.go` 既有） | 正文/标题/引用/代码/表格/公式/图片/卡片 | `Esc` | 无匹配命令：一行「没有这个命令」 |
| S18 | 卡片编辑器 | 浮层 | — | 徽章 / 块菜单 | 浮层 | 卡片 + 类型字段 | 改提示/答案/类型、冻结、删除 | `Esc` | 该块无卡：字段空白，删除禁用 |
| S19 | 卡片详情 | 浮层 | — | 卡片表行 | 浮层 | `Library.History`（既有） | 回原文、冻结/解冻、编辑 | `Esc` | 无历史：「还没有复习记录」 |
| S20 | 块检查器 | 浮层 | — | 沟槽徽章 | 浮层 | 块 + 卡片 + 摘录子笔记 + 插件动作 | 制卡、开卡、开摘录、定位、白板定位 | `Esc` | 三者皆无：三行说明 + 制卡入口 |
| S21 | 大纲面板 | 文档内 | — | 顶栏面板 | 面板 | `Document.Headings`（既有） | 跳转、折叠 | `⌘⇧P` | 「这篇还没有标题」 |
| S22 | 反链面板 | 文档内 | — | 面板分段 | 面板 | `Deps.Backlinks` | 打开来源、开摘录 | — | 「还没有人摘录这篇」 |
| S23 | 读·队列 | 读 | 队列 | `#/read/queue` | 页 | `queue.Reading` | 开始阅读、加入队列、继续 | `Esc` | 「今天的阅读队列空了」+ 加入 |
| S24 | 读·归档 | 读 | 归档 | `#/read/archive` | 页 | `Reading` 中 archived/dismissed（B2） | 打开、恢复 | — | 「归档里还是空的」 |
| S25 | 阅读会话 | — | — | 进入（无路由） | 会话 | `screens/reading` | J/K、1–3、E、S、X、A、D、P、M | 见 §2.8 | 队列空：toast「今天没有待读的笔记」 |
| S26 | 会话·摘录并制卡 | 会话内 | — | 复选框 | 会话控件 | `State.ExcerptWithCard`（既有） | 勾选后 E 同时制卡 | — | — |
| S27 | 忆·队列 | 忆 | 队列 | `#/recall/queue` · `remit://queue` | 页 | `queue.Today` + `queue.CountsOf` | 开始复习、改范围、筛选、保存筛选 | — | 无到期：「今天没有到期的卡片」+ 一句 |
| S28 | 复习范围 | 浮层 | — | S27 的范围按钮 | 浮层 | `App.scopeNotes` / `scopeTags` | 全库/笔记/标签、开始 | `Esc` | 该范围 0 张：开始禁用 + 写明 |
| S29 | 复习会话 | — | — | 进入（无路由） | 会话 | `screens/review` | 空格、1–4、U、E、回原文、上下文 | 见 §2.8 | 队列空：toast |
| S30 | 复习总结 | — | — | 会话末尾 | 会话 | `review.Summary` + 未记住的卡（新） | 回今日、继续读、回原文 | `Esc` | 全对：无失败分组，只有三个数字 |
| S31 | 忆·卡片 | 忆 | 卡片 | `#/recall/cards` | 页 | `QueryCards`（B5 扩展） | 排序、筛选、搜索语法、多选、批量 | — | 无命中：一句 + 语法提示 |
| S32 | 忆·类型 | 忆 | 类型 | `#/recall/types` | 页 | `Library.Types`（既有） | 选类型、编辑字段/模板、预览、新建 | — | 只有内置三件：列表照常，说明「内置无特权」 |
| S33 | 忆·预设 | 忆 | 预设 | `#/recall/presets` | 页 | `Library.Presets`（B3） | 改上限/阈值/断链策略、恢复默认 | — | — |
| S34 | 忆·历史 | 忆 | 历史 | `#/recall/history` | 页 | `LiveReviews`（既有）+ `stats.HistoryOf`（B4） | 点事件回原文/回卡片 | — | 今天没有记录：一句 |
| S35 | 忆·断链 | 忆 | 断链 | `#/recall/relink` | 页 | `Library.LostCards`（既有） | 逐块重连、冻结/解冻、批量 | — | 「没有断链」+ 一句因果 |
| S36 | 统计·总览 | 统计 | 总览 | `#/stats/overview` | 页 | `stats.Summary` + `Production` + `TimeShare`（B4） | 无（只读）；插件块可动作 | — | 新库：数字 0 + 一句「复习或读一次就有了」 |
| S37 | 统计·热力 | 统计 | 热力 | `#/stats/heatmap` | 页 | `stats.ReviewsPerDay`（既有） | 悬停/聚焦读某天 | — | 无记录：空网格 + 图例 |
| S38 | 统计·预测 | 统计 | 预测 | `#/stats/forecast` | 页 | `stats.ForecastAll`（既有） | — | — | 无到期：一句 |
| S39 | 设置·外观 | 设置 | 外观 | `#/settings/appearance` | 页 | `settings.Settings` + `mygo.Theme` | 明暗三选、纸面宽度 | — | — |
| S40 | 设置·复习 | 设置 | 复习 | `#/settings/review` | 页 | `presets.review` | 保留率、最大间隔、间隔预览、摘录后回原位 | — | — |
| S41 | 设置·预算 | 设置 | 预算 | `#/settings/budget` | 页 | `presets.budget` | 张数、分钟、每日提醒、托盘标题预览 | — | — |
| S42 | 设置·快捷键 | 设置 | 快捷键 | `#/settings/keys` | 页 | 常量表（§2.8） | 只读；自定义入口禁用并写明 #155 | — | — |
| S43 | 设置·插件 | 设置 | 插件 | `#/settings/plugins` | 页 | `App.pluginList` + `ui.problems` | 启用/停用、详情、重新启用、打开页面 | — | 无插件：一句 |
| S44 | 设置·Agent | 设置 | Agent | `#/settings/agent` | 页 | `a.agentAPI` | 启停、生成 token、看路由表 | — | 构建无此服务：一句 |
| S45 | 设置·更新 | 设置 | 更新 | `#/settings/update` | 页 | `a.checkUpdate` | 检查、自动检查开关、重建索引、导出全库 | — | — |
| S46 | 设置·关于 | 设置 | 关于 | `#/settings/about` | 页 | 常量 + 计数 | 打开许可证链接 | — | — |
| S47 | 插件详情 | 浮层 | — | 插件行「详情」 | 浮层 | manifest + 失败计数 | 看能力、信任级别、启停 | `Esc` | — |
| S48 | 快捷键总表 | 浮层 | — | `⌘/` | 浮层 | §2.8 表 | 关闭 | `Esc` | — |
| S49 | 命令面板 | 浮层 | — | `⌘K` | 浮层 | 命令表 + `Search` + 插件命令 | 选一条执行 | `↑↓` `↵` `Esc` | 「没有匹配项」 |
| S50 | 插件页·制卡脚本 | 插件 | — | `#/plugin/remit.cards` | 页 | 页面数据命令（缓存） | 重新运行、页头动作 | — | 无数据：骨架 → 错误态 + 重试 |
| S51 | 插件页·白板 | 插件 | — | `#/plugin/acme.whiteboard` | 页 | 页面数据命令 | 点节点回块、居中 | — | 「这块画布还没有节点」 |
| S52 | 笔记行菜单 | 浮层 | — | S05 行 `⋯` | 浮层 | 笔记 + `sidebar.note` 槽 | 阅读、加入队列、重命名、删除 | `Esc` | — |

---

## 4. 每屏规格

规则（每一屏都适用）：

- 只用 MyGo 组件承载含义；没有组件承载的记在 `internal/ui/kit` 的注释里（§4.14）。
- 一切尺寸来自 `design.Tokens`；新常量只加在 `internal/ui/design/scale.go`（§7.4）。
- 每个可见控件都有 handler；禁用项必须写明原因（§4.13）。
- 文案走 `locales`（en + zh-Hans 同改），`go tool mygo generate` 重生成。
- 每屏的 Deps 是**一个函数一件事**（ADR 0004）；屏不读 store、不读时钟、不 import 兄弟包。

### 4.1 启动与导入（S01–S03）

**S01 欢迎**
- 版式：Paper 骨架，`MessageWidth` 加宽到 `WelcomeWidth 640`（新常量，只此一处）；一张卡（`kit.Card`）。
- 元素：品牌块（方标 + `Remit` + 一句「本地优先 · 明文库 · 无账号」）；三行大按钮
  （`ui.Button` 高 56，图标 + 标题 + 副行）；`ui.Divider`；「最近」列表（`ui.List` + 行：
  路径 + 可选 `ui.Badge("打不开")`）；页脚 `TypeCaption` 一句「零遥测 · 零网络请求」。
- 动作：打开库 → `Deps.OpenLibrary`；新建库 → `Deps.CreateLibrary`；导入 → 打开 S02；
  最近项 → `Deps.OpenLibraryAt(path)`；打不开的最近项仍可点（重试）。
- 数据：`App.start`、`App.openErr`、最近库列表（**新增** `App.recentLibraries`，来自
  `internal/settings` 新增字段 `recent []recentLibrary`，最多 5 条，写在 `settings.json`）。
- 状态：无最近库 → 「最近」整段不建；`openErr` → 卡内 `ui.callout`（danger tone）写明原因与路径 +「换一个」。
- 验收：`TestWelcomeOffersThreeDoors`、`TestWelcomeShowsWhyALibraryFailed`；既有
  `TestFirstRun`、`TestNoLibraryYet`、`TestCorruptLibrary` 继续通过。

**S02 导入向导**
- 版式：`ui.DialogBase`，宽 `SettingsWidth`；三步（步骤条用 `ui.Badge`+`ui.Divider` 组合，
  没有官方 stepper 视觉 → 记为一次组合，理由：`ui.Stepper` 是数值步进不是流程步骤）。
- 元素：步骤 1 三个来源行（`.remitlib` / Markdown 目录 / Anki `.apkg`）；步骤 2 计数表
  （`ui.Table` 两列：项 / 值）+ 两个 `ui.Switch`（保留现有笔记、导入后重建索引）；步骤 3 空态式完成页。
- 动作：`Deps.ChooseArchive` / `ChooseMarkdown` / `ChooseAnki` → `Deps.Import…`：Markdown 与 Anki 走
  既有 `importmd` / `importanki`；`.remitlib` 走既有 `transfer.Read`
  （`internal/core/transfer/transfer.go:78`）解到新目录后重开库（`Deps.OpenLibraryAt`）。
  步骤 2 的 `Next` 在计数为 0 时 `Disabled(true)`，下方 `ui.Text` 写明「没有可导入的内容」。
- 数据：`importmd.Report` / `importanki.Report`（既有类型）。
- 状态：导入中 `ui.Progress`（不定长）；失败 → 就地 `ui.callout` danger + 「重试」，不关对话框。
- 验收：`TestImportWizardWalksThreeSteps`、`TestImportWizardRefusesAnEmptySource`。

**S03 导出**
- 版式：`ui.DialogBase` 宽 `SummaryWidth`；`ui.RadioGroup` 三项（Markdown 目录 / `.remitlib` /
  卡片 CSV）+ 一句说明 + 主按钮。
- 动作：`Deps.ExportMarkdown`（既有 `exportAllMarkdown` / `exportNoteMarkdown` 的重组）与
  `Deps.ExportLibrary`（**新增** handler，包 `transfer.Write`，`internal/core/transfer/transfer.go:22`）；
  卡片 CSV 是**禁用**项，原因见 §4.13（1.0 不做）。
- 状态：导出中按钮禁用并显示 `ui.Spinner`；完成 `c.Toast` + 动作「打开文件夹」。
- 验收：`TestExportWritesAndReports`、`TestExportRefusesTheCSVItDoesNotHave`。

### 4.2 今日（S04）

- 目的：一张屏回答「今天做什么、还有多久、从哪开始」。
- 版式：Paper 骨架，measure `PaperWidth`。
- 元素（自上而下）：
  1. 页头：`H1`「今日」+ 副行「9月18日 周四 · 预算 30 分钟」（`TypeLabel`）+ 右「命令」按钮（开 S49）。
  2. **预算卡**（`kit.Card`）：`TypeDisplay` 的数字「29 分钟」+ 一行说明 +
     `ui.Meter`（`MeterLevels` 三档：≤80% 正常 / 80–100% 提醒 / >100% 危险）。
  3. **每日笔记行**（`ui.Button` 行）：标题「今天 · 9月18日」+ 摘要（最近一次会话的一句话）+
     「打开」。数据来自 `dailyTitle(now)`（既有 `internal/ui/notes.go`）。
  4. 三个统计（`kit.Stat`）：篇待读 / 张待忆 / 今日已忆（`queue.CountsOf` + 今日已忆的计数，后者
     由 `Library.CountReviewed(dayStart, dayEnd)` 推出）。
  5. **队列**（`ui.Column` + section head）：行由 `ui.List` 承载，一件一件：
     - 待读行：kind 方块「读」+ 标题 + 「第 7 / 14 段 · 上次停在 4 天前」+ 「约 6 分钟」；
     - 待摘行：kind「摘」（虚边框）+ 标题 + 「读到第 5 / 6 段 · 还没有摘」+「约 2 分钟」；
     - 复习行：kind「忆」+「复习 · 全库」+「19 张待忆 · 其中 4 张是新的」+「约 11 分钟」。
     超预算的行加 `ui.Badge`「顺延到明天」，仍可点。
  6. 「读过但还没摘」区块：最多 3 行，与待摘行同一数据源的另一视图（没有则不建）。
  7. `today.widget` 插件块（§6.3）：每个有该槽项的插件一个 section。
  8. 「明日」区块：一行 `TypeLabel` 数字（`stats.ForecastAll` 的第 2 天）。
- 动作：行 → `Deps.OpenReading(noteID)` / `Deps.OpenReadingAt(noteID, blockID)` / `Deps.StartReview`；
  「开始」→ 队列第一条；预算卡不可点。
- 数据（Deps）：`Today func() queue.Today`、`StartReview`、`OpenReading`、`OpenNote`、`OpenPalette`、
  `Daily func() (core.Note, bool)`、`Tomorrow func() int`、`Widgets func() []contrib.MenuRef`。
- 状态：空 → `kit.Empty`（图标 + 「今天没有待读也没有到期」+「加入一篇笔记」）；载入 → 三行骨架；
  错（库读失败）→ 就地 callout + 重试。
- 验收：`TestTodayOrdersReadingBeforeReview`、`TestTodayMarksWhatIsOverBudget`、
  `TestTodayCountsMatchTheRail`、`TestTodayOffersTheDailyNote`。

### 4.3 写（S05–S10）

**S05 写·笔记**
- 版式：List 骨架，measure `ListWidth 860`。
- 元素：页头（`H1`「笔记」+ 副行「N 篇 · M 张卡 · 明文目录」+ `PrimaryButton`「新建笔记」）；
  工具行（`ui.SearchField` Grow + 四个 `ui.Toggle`（全部/在读/有卡/摘录）组成 `ui.ToggleGroup` +
  当前标签 chip + 排序 `ui.PopoverMenuButton`（最近编辑/标题/卡数））；
  行列表 `ui.List`：标题行（含可选 `ui.Badge("摘录")`）+ 首行摘要（`TypeLabel`，一行截断）+
  右尾（阅读位置 `ui.Badge`、卡数 `ui.Badge`、相对时间、`hover` 出现的「阅读」「⋯」）。
- 动作：行 → 打开文档；「阅读」→ `?mode=read`；`⋯` → S52；排序改变 → `Deps.SetNoteSort`；
  筛选 → `Deps.SetNoteFilter`；清空筛选 → 重置三项；新建 → `Deps.NewNote`。
- 数据：`Notes func() []store.NoteRow`、`Tags func() []store.TagRow`、`Filter`/`Sort` 状态、
  `Deps.Set…`、`NewNote`、`OpenNote`、`OpenNoteRead`、`AddReading`、`RenameNote`、`DeleteNote`。
- 状态：无匹配 → `kit.Empty`「没有匹配的笔记」+「清空筛选」；无笔记 → 「写下第一篇」+ 新建。
- 验收：`TestNotesListShowsSummaryAndCards`、`TestNotesListFiltersByTag`、
  `TestNotesSortCyclesAndSurvivesReload`。

**S06 写·搜索**
- 版式：List；页头副行「『q』的 N 条结果」。
- 元素：`ui.SearchField`（自动聚焦）+ 两个分组（笔记 / 卡片），各自 `ui.List`；命中片段由
  `SearchHit.Snippet` 直接显示（`[` `]` 标记改成 `ui.RichText` 的强调 span）。
- 动作：笔记行 → 打开；卡片行 → 打开卡片详情（S19）。
- 数据：`Results func(q string) ([]store.SearchHit, int)`（既有 `Library.Search`）。
- 状态：空查询 → 一句说明匹配范围；无命中 → 两句（笔记 0 / 卡片 0）。
- 验收：`TestSearchGroupsNotesAndCards`、`TestSearchFromAnotherAreaPutsTheCaretInTheField`。

**S07 写·标签**
- 目的：标签是笔记的属性，不是容器——这一页回答「我有哪些标签、各自多少篇」。
- 版式：List；`ui.List` 行：`row-kind` 图标 + `#tag` + 两个示例标题（`TypeLabel`）+ 右「N 篇」。
- 动作：行 → `Deps.OpenTag(tag)`（= S05 的 `?tag=`）；页头无按钮（新建在 S05）。
- 数据：`Tags func() []store.TagRow`。
- 状态：空 → `kit.Empty`「还没有标签」+ 一句「在笔记里写 `#标签`，它就会出现在这里」。
- 验收：`TestTagPageOpensTheFilteredList`。

**S08 写·资产**
- 目的：资产随库走、按内容命名——这一页回答「库里有哪些文件、被谁引用」。
- 版式：List（`BrowseWidth` 内）；`ui.Table` 列：类型（`ui.Badge`）/ 文件（`TypeMono`）/ 大小 /
  被引用（N 篇 + 第一篇标题）/ 格式。行可点（开第一篇引用它的笔记）。
- 动作：页头「导入」→ `Deps.ImportAssets()`（走既有 `putAsset` 的批量版，**新增**）；
  行 → `Deps.OpenNote(firstRef)`。
- 数据：`Assets func() []store.AssetRow`。
- 状态：空 → 「还没有资产」+「导入」；无引用（孤儿文件）仍列出，引用列写「未被引用」。
- 验收：`TestAssetsListNamesTheReferencingNotes`。

**S09 写·模板**
- 目的：带 `#template` 的普通笔记只在新建时出现——这一页是它们的入口。
- 版式：List；每行：标题 + 「N 个块 · 首行」+ 右「从模板新建」；页头右「空白笔记」。
- 动作：行 → `Deps.NewFromTemplate(c, id)`；「空白笔记」→ `Deps.NewNote`。
- 数据：`Templates func() []core.Note`（既有 `App.noteTemplates`）。
- 状态：空 → 「还没有模板」+ 一句「给一篇笔记加 `#template` 标签，它就是模板」。
- 说明段（常驻）：副本剥掉 `#template`、块 ID 重新铸造，模板本体可以无限次开工（#154）。
- 验收：`TestTemplateStartsANoteWithoutTheMarker`（既有 `TestNewNoteFromTemplate` 的 UI 面）。

**S10 写·摘录树**
- 元素：`ui.Tree` + `ui.TreeItem`（**官方树**：折叠、键盘、层级、读屏）。每项：标题 +
  「N 张卡 · M 条再摘」。根是无源笔记的笔记；孤儿摘录（源笔记已删）单列一段。
- 动作：点项 → 打开该摘录；页头「继续读」→ S23。
- 数据：`Tree func() []store.ExcerptNode`。
- 状态：空 → 「还没有摘录」+「开始读一篇」。
- 验收：`TestExcerptTreeNestsChildren`、`TestExcerptTreeListsOrphans`。

### 4.4 文档（S11–S22）

**S11 文档·编辑**
- 版式：Workspace：`ui.Split`（笔记列 `NotesColumnWidth 260`，`SplitReset` 同一个值）→ `ui.Split`
  （面板 `PanelWidth 288`，`SplitReset(PanelWidth)`）。
- 元素：
  - 纸面：`title` 用 `ui.TextInput`（无边框，`TypeDisplay`），正文 `richtext.EditorBase`（ADR 0005），
    `richtext.Theme` 提供 18/1.85 与三级标题。
  - 沟槽（48 宽，纸面左侧）：当前/悬停段落的 `+` 与 `⋮⋮`（`ui.ButtonBase` + `Element.Drag/Drop`，既有
    `block-tools.md` 口径）；上方徽章 `ui.ButtonBase`（形状编码状态：`due` 实边、`new` 虚边、
    `later` 淡边、`lost` 前缀 `!`），点击 → S20。
  - 卡片徽章不再放在文档上方：那段 `ScrollHorizontal` 徽章条删除（差异表 §0.3）。
  - 选区动作条（S15）在选中 ≥2 字时浮出。
- 动作：一切写回经 `Deps.SaveDocument`；块插入/移动/删除按块 ID（既有）；徽章 → `OpenInspector`。
- 数据：既有 `notes.Deps` + `Inspector`/`CardsOf(block)`。
- 状态：保存失败 → callout + 「重试」（`App.report` 已有 toast，这里加就地提示）。
- 验收：`TestDocGutterBadgeOpensTheBlockInspector`、`TestBlockMenuMovesParagraphWithItsIdentity`（既有）、
  `TestSelectionBarClozesTheBlockAndMakesACard`、`TestPaperKeepsGutterAtMinimumSize`。

**S12 文档·阅读**
- 与编辑同屏不同态：无沟槽工具、块编号常显、当前块高亮（`Accent.Alpha(0.10)` + 左侧 2px 竖线），
  每个块右侧一列标记（`N 卡` / `已摘` / `上次停在这里`）。正文用 `ui/markdown.View`（只读，图片缓存复用）。
- 动作：点块 → 设为当前块；`E` → 摘录当前块（`Deps.Excerpt`）；`M` → 编辑态。
- 验收：`TestReadModeMarksCardsAndExcerpts`、`TestReadModeExcerptsTheCurrentBlock`。

**S13 文档·分栏**：`ui.SplitPercent`（50/50，`SplitReset(50)`），左编辑器右 `ui/markdown.View`；
中线 1px。`richtext` v0.8.1 一个 `Document` 只能有一个编辑器（已记录缺口），只读侧不能编辑。
验收：`TestViewModesSwitchPanes`（既有，扩到三态）。

**S14 查找条**：`ui.FindBar(c, &open, &query, matches, &current)`（官方），占用自身高度，压在纸面上方；
无命中显示「0 个结果」。验收：`TestFindBarCountsMatches`。

**S15 选区动作条**
- 版式：`ui.Popover`（无箭头，贴当前段落上方）；三个按钮：挖空 / 制卡 / 摘录整段。
- 锚点：`richtext` v0.8.1 不暴露选区矩形。选区文本用 `Document.Selection()` 的
  `anchor/caret` 经 `Document.Text().Slice(start, end).String()` 取；块 ID 用段落 → 块的
  既有映射；浮层贴该段的 `ParagraphBox`（`Document.VisibleParagraphs()`）。这是已记录的
  组件缺口，不手写"选区命中测试"。
- 动作：挖空 → `Deps.ClozeSelection(noteID, blockID, text)`（**新增** `core.Cloze` + App handler）：
  把选中文字改写成 `{{text}}` 写回块、保存文档、按 cloze 类型制卡、reload；制卡 →
  `Deps.OpenCardEditor(blockID)`；摘录整段 → `Deps.Excerpt(blockID, blockText)`。
- 状态：选区消失即关；写入失败 → toast + 保持选区。
- 验收：`TestSelectionBarClozesTheBlockAndMakesACard`、`TestSelectionBarExcerptsTheWholeBlock`。

**S16 块菜单**（既有 `block-tools.md` + 两项新增）
- 菜单项：在下方插入、复制、上移、下移、删除、—、转为正文/标题 1/2/3/无序/有序/引用/代码、
  —、摘录整段、为这段制卡、—、检查器…、`note.block` 插件项。
- 删除：有 live 卡片时 `ui.AlertDialog`，写明「N 张卡转为断链，复习历史保留」，按钮文案带 N；
  最后一块不可删（禁用 + 可见说明）。
- 验收：既有 `TestDeleteBlockWarnsBeforeCardLoss` + 新增 `TestBlockMenuOffersTheInspector`、
  `TestBlockMenuKeepsTheLastBlock`。

**S17 插入菜单**：`ui.Popover` + `ui.List`（既有 `screens/notes/slash.go`）；底部接 `editor.slash` 插件项。
验收：既有 `/` 菜单测试。

**S18 卡片编辑器**：`ui.DialogBase`；字段按**类型模板**生成（`ui.TextArea` per 字段）；类型用
`ui.Select`（库里所有类型，含插件供给）；引用态用 `ui.Badge` 显示；「冻结」`ui.Switch`；
删除走 `ui.AlertDialog`。动作 → `Deps.SaveCard` / `DeleteCard` / `FreezeCard`。
验收：`TestCardEditorWritesTheFieldsOfItsType`。

**S19 卡片详情**：`ui.DialogBase`；来源块预读（`ui.callout` neutral）+ 「回到原文」+ 调度史
（`ui.List`，每行时间 + 评分 + 间隔变化，来自 `Library.History`）+ 冻结/解冻 + 编辑卡面。
验收：`TestCardDetailListsTheScheduleHistory`。

**S20 块检查器**（原型 §11 的「三种角色」）
- 三段：正文（只读预览）/ 卡片（N 张，行→S19，「为这一段制卡」）/ 摘录（N 条，行→打开）。
- 页脚：在笔记里定位、在白板里定位（仅当画布有该块节点时出现）、关闭。
- 数据：`Inspector func(blockID) (notes.Inspector, bool)`（**新增** `notes.Inspector` 结构：
  Block、Cards、Excerpts、IsReadingPosition、BoardNodes）。
- 验收：`TestInspectorShowsTheThreeRoles`、`TestInspectorOffersTheWhiteboardWhenANodeExists`。

**S21 / S22 面板**
- 目的：纸面旁边的一块可切换视图——大纲回答「这篇有哪些节」，反链回答「谁引用了我」。
- 版式：`ui.Split` 的第二栏，默认 `PanelWidth 288`；顶部一行 `ui.Segmented`（大纲 / 反链 N）+ 收起
  `ui.Button`（图标，`.Tooltip`）。
- 大纲：`ui.Outline`（既有；光标所在的节是当前项），空态「这篇还没有标题」+ 一句「输入 / 可以插入」。
- 反链：`ui.List`（替换现有 `ButtonBase` 循环，`ui-composition.md` §3 的待迁项），每行三段：
  标题 / 首行 / 「N 张卡 · 可再摘」。
- 动作：大纲行 → `Deps.ShowPanel` 的跳段（既有 `pendingJump`）；反链行 → `Deps.OpenNote`；
  分段切换 → 写 `notes.State.PanelTab`。
- 数据：大纲条目由笔记的块派生（既有 `outlineEntries(blocks []core.Block) []outlineEntry`，
  `internal/ui/screens/notes/outline.go:27`），进 `ui.OutlineState[int]`；反链读既有
  `notes.Deps.Backlinks func() []notes.Backlink`（`internal/ui/screens/notes/notes.go` 的 `Deps`）。
- 状态：`≤980` 收进 `ui.Overlay`（同内容，`Esc` 关）；面板打开时焦点进入面板，关闭时还给顶栏的开关。
- 验收：`TestPanelSwitchesToBacklinks`、`TestPanelBecomesAnOverlayWhenNarrow`、
  既有 `TestOutlineFollowsTheCaret`、`TestOutlineJumpsToItsParagraph` 保持绿。

### 4.5 读（S23–S26）

**S23 读·队列**
- 版式：List（`ListWidth`）；页头右两个按钮：加入队列（次）、开始阅读（主，队列空时禁用 + 说明）。
- 行：标题 + 「第 5 / 9 段 · 上次停在今天」+ `ui.Meter`（段落进度，宽 120）+「继续」。
- 底部「节奏」三格：篇今日待读 / 篇在队列 / 三档消化难度（说明性，不是控件）。
- 数据：`Reading func() []store.Reading` + `Note func(id) (notes.Note, bool)`。
- 空态：`kit.Empty`「今天的阅读队列空了」+「加入一篇」（打开 S05）。
- 验收：`TestReadingQueueOpensTheSession`、`TestReadingQueueAddsANoteFromTheList`。

**S24 读·归档**
- 两段：归档（`archived`）与不再出现（`dismissed`），各一行表格/列表；每行「打开」「恢复」。
- 动作：恢复 → `Deps.QueueNote(id)`（写 `queued`，清 archived/dismissed）。
- 空态：「归档里还是空的」+ 一句「阅读会话里按 A」。
- 验收：`TestArchiveRestoresANoteToTheQueue`、`TestDismissedNotesStayOutUntilRestored`。

**S25 阅读会话**（在既有 `screens/reading` 上扩展）
- 版式：Session 三行。会话栏：「读 · 第 N / M 篇」+ 标题 + 右（第 X / Y 段 + `ui.Meter` + 退出）。
- 舞台：`ReadingWidth` 内居中；块列表，当前块高亮；每块右标记列（N 卡 / 已摘 / 上次停在这里）。
- 动作行：三档难度（`ui.Button` + `kit.Keycap`）在一组；「摘录并制卡」`ui.Checkbox` + 「摘录」「跳过」
  「归档」「不再出现」「推后」「在库里打开」在另一组；`≤760` 换行。
- `Deps` 扩展（**新增**）：`Archive func()`, `Dismiss func()`, `Postpone func()`；`reading.State` 新增
  会话内的已处理记录（与既有「跳过的笔记本会话不再回来」同形，不写日志）。
- 语义：A/D/P 之后 `Advance(false)`（不写评分）；`c.Toast` 报告结果并给「撤销」动作
  （撤销归档 = `Deps.QueueNote`）。
- 验收：`TestReadingSessionArchivesANote`、`TestReadingSessionPostponesToTomorrow`、
  `TestReadingSessionDismissesAndTells`（既有会话测试保持通过）。

**S26 会话·摘录并制卡**：既有 `State.ExcerptWithCard` + `ui.Checkbox`（`internal/ui/screens/reading/reading.go:256`）
+ `Deps.Excerpt(..., withCard)`；本稿只重排它在动作行里的分组与文案（与「摘录」按钮同组，紧邻）。
验收：既有 `TestExcerptLeavesTheReaderInPlace` 扩到带卡。

### 4.6 忆（S27–S35）

**S27 忆·队列**
- 版式：List；页头右：范围按钮（显示当前范围）+ 开始复习。
- 复习量卡（`kit.Card`）：`TypeDisplay` 分钟数 + 一行「今天的复习量」+ 三档占比条（`ui.Meter` 三段
  组合，记为组合：`ui/chart` 没有堆叠条，`ui.Meter` 只有一档）+ 图例（新 / 学习中 / 复习）。
- 筛选行：6 个 `ui.Toggle`（全部/今天到期/新/学习中/断链/难卡）+「保存为筛选」；保存的筛选是可删的 chip
  （`ui.ButtonBase` chip，本稿**新增**持久化：写 `settings.json` 的 `savedFilters`）。
- 「按笔记分」：`ui.List`，每行标题 + 「14 段 · 7 张卡 · 3 张反复答错」+ 右「N 张到期」。
- 「最近 26 周」：热力缩略（S37 的同一组件，`kit` 内）。
- 动作：行 → 以该笔记为范围开复习（`Deps.SetScope(ScopeNote)`）；筛选 → `Deps.SetFilter`；
  范围 → S28。
- 数据：`Counts func() queue.Counts`、`ByNote func() []queue.NoteDue`（**新增**类型，落
  `internal/core/queue/today.go`，不要放进屏包：它与今日队列共用同一份派生）。
- 空态：「今天没有到期的卡片」+ 一句「明天 8 张」。
- 验收：`TestRecallQueueGroupsByNote`、`TestRecallFilterNarrowsTheQueue`、`TestSavedFilterSurvivesRestart`。

**S28 复习范围**：`ui.DialogBase`；`ui.Segmented`（全库/某篇笔记/某个标签）+ 对应选择器
（笔记用 `ui.List`，标签用 `ui.ToggleGroup`）+ 页脚「本次取 N 张」+ 开始（0 张时禁用 + 说明）。
数据：`scopeNotes` / `scopeTags`（既有 `internal/ui/review.go`）。验收：`TestScopeDialogNarrowsTheSession`。

**S29 复习会话**（在既有 `screens/review` 上扩展）
- 新增：题面下方一条**来源行**——「《标题》· 第 N 段 · 同块还有 M 张」+「回到原文」+「展开上下文」；
  展开时显示前后各一段（`ui.callout` neutral）+ 同块兄弟卡的 `ui.Badge`。
- 底部：四档评分（`kit.Keycap` + 间隔预览，既有）+ `review.card` 插件项 + 「撤销」「编辑卡片」。
- 验收：`TestReviewShowsTheSourceOfTheCard`、`TestReviewExpandsTheContext`（既有评分/撤销测试保持）。

**S30 复习总结**
- 既有 `kit.Card` + 三个 `kit.Stat` + 插件文案 + 主按钮；**新增**：「没想起来的按来源聚合」一段——
  按笔记分组，每行「标题 · N 张没想起来」+「回到原文」。
- 数据：`review.Summary` 加 `Failed []review.FailedCard`（**新增**字段，App 在会话内累积）。
- 验收：`TestSummaryGroupsFailedCardsByNote`。

**S31 忆·卡片**
- 版式：List + 左侧筛选栏（`ui.Split`，左 `FilterPaneWidth 200`（新增常量），右表格）——或窄窗口改为顶部 `ui.Select`。
- 筛选栏三组（状态 / 类型 / 引用），每行「名 + 计数」，用 `ui.ButtonBase` 行组成的**筛选列表**——
  记一次组合：`ui.Sidebar` 能承载选择与计数但会与主列表抢焦点，`ui.ToggleGroup` 不能带计数换行，
  所以用一列 `ButtonBase` 行并加 `Role("button")` + `aria-pressed`。
- 表格：`ui.Table` + `Columns`（首列是勾选框 `ui.Checkbox`，不是 `RowHeader`）+ `Sortable`
  （提示/笔记/类型/状态/间隔/重来/到期）+ `CellMenu`（右键批量动作）+ `Fixed`（勾选列）。
- 工具行：`ui.SearchField`（支持 `is:` / `type:` / `tag:` / `note:`）+ 语法帮助（`ui.Button` 开 callout）
  + 列开关 `ui.PopoverMenuButton` + `browse.row` 插件动作（无项时禁用 + 说明）。
- 批量动作条：选中数 + 「暂停」「改到期」「冻结」「加标签」。
- 数据：`Cards func() ([]store.CardSummary, int)`、`Selection` 状态、`Deps.Bulk…`。
- 空态：无命中 → 一句 + 语法提示；无卡片 → 「还没有卡片」+「在笔记里用 `::` 或选中文字制卡」。
- 验收：`TestCardTableSortsByColumn`、`TestCardSearchSyntaxNarrows`、`TestCardTableBulkSuspends`。

**S32 忆·类型**（从 `screens/settings/types.go` 迁出为 `internal/ui/screens/cardtypes`）
- 主从：左 `ui.List`（类型名 + 卡数）；右：标题行（名 + 内置/插件 `ui.Badge` +「编辑模板」）、
  字段 chips、模板（两段等宽只读代码块，`ui/editor` 只读或 `ui.Text` 等宽）、预览
  （`ui/markdown.View` 两张卡面，既有 `types_preview.go`）、回退说明。
- 动作：`SaveType` / `DeleteType`（既有 Deps，带错误文案）。
- 验收：`TestTypesPagePreviewsTheTemplate`、`TestTypesPageKeepsBuiltinsUndeletable`。

**S33 忆·预设**
- 四组（`ui.Form` + `ui.Fieldset`）：学习步（只读：当前「重来后次日再问」+ 一行说明，
  无「加一步」，理由见 §10）；每日上限
  （新卡 `ui.StepSlider`、复习 `ui.StepSlider`）；失败（重学间隔 `ui.Select`、难卡阈值 `ui.StepSlider`）；
  断链（`ui.Segmented`：保留为断链 / 自动转快照）。
- 动作：每个控件 → `Deps.SetPresets`（一次写 `remit.json`，失败就地 callout）；「恢复默认」→
  `Deps.ResetPresets`（`AlertDialog` 确认）。
- 数据：`Presets func() store.Presets`。
- 验收：`TestPresetsWriteTheLibraryConfig`、`TestPresetsResetToDefaults`。

**S34 忆·历史**
- 四个数字（今天评过 / 用时 / 重来 / 今日正确率）；评分分布（四行横条，`ui.Row` + `Box` 组合；
  `ui/chart` 的条形图对四项太重，记为组合）；今天的时间线（`ui.List`，每行 time + 节点 + 内容 + 元信息；
  节点颜色按 kind）；本周表（`ui.Table` 四列）。
- 数据：`History func() stats.History`（B4：`LiveReviews` + 派生）。
- 空态：「今天还没有记录」+ 一句「复习或读一次就有了」。
- 验收：`TestHistoryListsTodaysReviews`、`TestHistoryCountsGrades`.

**S35 忆·断链**
- 两栏：左「路断了的卡片」`ui.List`（当前项高亮）；右「重新指向《笔记》的哪一段」`ui.List`（块行，
  段号 + 首行）。页头「全部重新指向」→ `AlertDialog`（不可撤销，写明条数）。
- 下方「已冻结」区：行 + 「解冻」。
- 动作：选卡片 → 选块 → `Deps.Relink(cardID, blockID)`；`Deps.Freeze(cardID)` / `Deps.Thaw(cardID)`。
- 组件：卡片列表与块列表都用 `ui.List`；查询收窄用 `ui.Autocomplete` 换掉今天的手写 overlay
  （`ui-composition.md` §3 的待迁项），`TestRelinkNarrowsAsYouType` 是本片要新建的那个闸门
  （`ui-composition.md` §4 表里今天标着「待建」）。
- 空态：「没有断链」+ 一句因果（删掉被引用的块才会出现）。
- 验收：`TestRelinkReattachesALostCard`、`TestRelinkNarrowsAsYouType`、`TestRelinkThawsAFrozenCard`；
  既有 `relink` 包的 `TestCandidates`/`TestQuery`/`TestChoose`/`TestClose`/`TestEmpty` 保持绿。

### 4.7 统计（S36–S38）

**S36 统计·总览**
- 四个数字：保留率（目标见预设）/ 今日到期 / 篇待读 / 今日卡片。
- 「产出」三格：字数 / 条摘录（每条平均 N 张卡）/ 张卡片（来自 M 篇笔记）→ **新增** `stats.Production`。
- 「复习与阅读」26 周热力缩略 + 图例。
- 「时间配比」：读 / 忆 / 摘 三段条 + 图例 → **新增** `stats.TimeShare`；口径一句话说明。
- 「保留率」曲线（`ui/chart`：`chart.Plot` + `chart.NewTime` + `chart.Area`/`Line`）。
- 「预测」14 天柱（`chart.Rect`）。
- `stats.block` 插件块（§6.3）。
- 验收：`TestStatsShowsProductionAndRatio`、`TestStatsBlocksComeFromPlugins`.

**S37 统计·热力**：三个数字（范围/保留率/最长连续）+ 大网格（`kit.Heatmap`，§4.14 的缺口组件）+
图例 + 说明段（来自 append-only 日志，删索引不丢）。验收：`TestHeatmapExplainsItsSource`。

**S38 统计·预测**：柱状（`chart.Rect`）+ 三格（今天/峰值/明日）+ 「为什么合并」说明。验收：
`TestForecastMergesReadingAndCards`。

### 4.8 设置（S39–S46）

统一版式：Paper 骨架，measure `SettingsWidth 660`；每页 `ui.Form` + `ui.Fieldset` + `ui.Field`，
`Field.Description` 承担说明；组间 `SpaceL`。页头 `H1` + 一句副行。

- **S39 外观**：外观三选 `ui.Segmented`（随系统 / 纸面 / 墨色）→ **新增** `settings.Appearance`。
  解析只有一处：`App.View` 把三选折成 `dark bool`（system → `c.Theme().Dark`，paper → false，
  ink → true），用 `design.NewTokens` 构造后 `c.SetTheme`（`internal/ui/view.go` 既有的两步）；
  九个 `design.NewTokens(c.Theme())` 调用点因此不用改。启动时再调一次 `mygo.Theme.SetSource`
  让首帧就是对的底色，避免闪白。纸面宽度 `ui.StepSlider`（620–860，step 20）
  → `settings.PaperWidth`；正文尺寸：只读，写明「跟随桌面文本缩放」，并给一个 `ui.Button`「打开系统设置」
  （`mygo.Shell.OpenExternal("x-apple.systempreferences:…")` 在 macOS / 平台空实现）——**显式只读**，
  理由：第二套字号会与 `c.Preferences()` 抢真相（#228 未定）。
- **S40 复习**：保留率 `ui.StepSlider` 80–97（写 `presets.review.retention`，覆盖插件策略，见 §5.3）；
  最大间隔 30–365 step 30；间隔预览 `ui.Switch`（**新增** `presets.review.preview`：关掉时
  `review.Deps.Preview` 返回 `false`，评分键只显示评级，不显示下一次间隔）；摘录后回原位 `ui.Switch`
  （**新增** `presets.reading.returnAfterExcerpt`，默认开）。
- **S41 预算**：复习张数 5–60 step 5；可用时间 10–90 step 5；今日分配预览（`kit.Card` + `ui.Meter`，
  读 S04 的同一函数）；每日提醒 `ui.Switch`（既有）；托盘标题预览（等宽只读）。
- **S42 快捷键**：四组表（全局 / 阅读 / 复习 / 文档），每行说明 + `kit.Keycap`；顶部一行
  **禁用**的「修改键位」按钮 + 可见说明「自定义键位不在 1.0（#155）」。
- **S43 插件**：每插件一行（名 + 内置 `ui.Badge` + 描述 + capability `ui.Badge` 列 +「详情」+ 启用
  `ui.Switch`）；「问题」段列 `ui.problems` 与三次隐藏的贡献（来源 `App.ui`，§6.4）；「页面贡献」段
  列可用插件页 +「打开」。
- **S44 Agent**：监听地址（等宽只读）、启用 `ui.Switch`、生成 token `ui.Button`（只在成功后显示一次）、
  路由表 `ui.Table`（方法 / 路径 / 说明）。
- **S45 更新**：当前版本 + 格式版本（读 `store.FormatVersion`）、自动检查 `ui.Switch`、「检查」、重建索引、
  导出整库（打开 S03）。
- **S46 关于**：品牌卡（名 + 版本 + `AGPL-3.0 · 明文库 · 零遥测`）、库位置（等宽）、
  笔记 / 卡片 / 队列三个数、致谢段。

验收：`TestSettingsAppearanceWritesTheOverride`、`TestSettingsKeysAreReadOnlyWithAReason`、
`TestSettingsBudgetPreviewMatchesToday`、`TestPluginsShowCapabilitiesAndFailures`。

### 4.9 浮层与插件页（S47–S52）

- **S47 插件详情**：`ui.DialogBase`；能力 chips、信任级别（内置 / 第三方）、启用 `ui.Switch`
  （与 S43 列表上的开关是同一个授权写入）、失败计数与「重新启用」。不给一个「启动时加载」
  开关：契约里只有 `activation` 记录，启用列表才是加载时机（`plugins/ui.md` §1），多一个开关就是死控件。
- **S48 快捷键总表**：`ui.DialogBase`；四组 `dl` 式两列表（`ui.Grid` 两列）。
- **S49 命令面板**：既有 `internal/ui/palette.go`（`ui.DialogBase` + `ui.List`）扩为三组：
  命令 / 笔记 / 卡片；插件命令在最后一组（既有 `paletteActions`）；空态「没有匹配项」。
- **S50 制卡脚本**（插件页）：`page.header` 槽动作；三个数字；「上一次运行」卡；「它能做什么」列表。
- **S51 白板**：见 §6.5（宿主画的节点网格，不是自由画布）。
- **S52 笔记行菜单**：`ui.Popover`/`Element.Menu`：阅读、加入阅读队列、重命名、删除（`AlertDialog`）、
  `sidebar.note` 插件项。

### 4.10 页面骨架与测量值

| 面 | measure |
|---|---|
| S04 今日 / S36–S38 统计 | `PaperWidth 760` |
| S05–S10、S23、S24、S27、S34 | `ListWidth 860` |
| S31 卡片 | `BrowseWidth 1080` |
| S11–S13 文档 | `PaperWidth 760`（分栏时每侧各自 measure） |
| S25 阅读 / S29 复习 | `ReviewWidth 680`；总结 `SummaryWidth 520` |
| S39–S46 设置 / S50 | `SettingsWidth 660` |
| S01 欢迎 | `WelcomeWidth 640`（新增） |
| S02/S03/S18/S19/S20/S28/S47/S48/S49 | `SettingsWidth` 或 `SummaryWidth`（按内容，见各行规格） |

### 4.11 空 / 载入 / 错

| 状态 | 统一做法 |
|---|---|
| 空 | `kit.Empty`：图标 20 + 一句 + 一个动作（与 `mygo-components` 的「空态」行一致） |
| 载入 | 首屏骨架（三行 `Box`，`tk.Sunken`）；重取数据用 `ui.Spinner` + 保留旧内容 |
| 错 | 就地 `ui.callout`（danger）+ 一句原因 + 一个动作（重试 / 换一个）；同时 `c.Toast` 报一次 |
| 部分失败 | 页面照常渲染，坏的那一块换成 callout（插件失败即此形） |

### 4.12 文案与 i18n

- 所有新文案进 `locales/en.json`（带 `@key.description`）与 `locales/zh-Hans.json`，再由
  `go tool mygo generate` 生成 `locales/catalog_gen.go`；`go tool mygo i18n check .` 必须绿。
- 数字、日期、相对时间用 `i18n.FormatRelativeTime` / `i18n.NumberOptions`，不手写格式串。
- 键名按面分组：`today.*`、`write.*`、`read.*`、`recall.*`、`stats.*`、`settings.*`、`plugins.*`；
  旧键（`modeNotes` 等）保留到导航改判的 PR 里一并改。

### 4.13 显式禁用清单（可见，带原因）

| 控件 | 面 | 禁用条件 | 可见原因 |
|---|---|---|---|
| 「删除最后一块」 | S16 | 笔记只有一块 | 菜单项 `Disabled(true)` + 菜单底部一行「至少留一段」 |
| 「开始复习」 | S27/S28 | 该范围 0 张 | 按钮旁「这个范围今天没有到期」 |
| 「开始阅读」 | S23 | 队列空 | 空态里就是「加入一篇」 |
| 「下一步」 | S02 | 预览计数为 0 | 一行「没有可导入的内容」 |
| Markdown 目录 | S03 | 库为空 | 一行「库里还没有笔记」 |
| 卡片 CSV | S03 | 永久（1.0 不做，见 §10.1） | 选项下方一行「1.0 不做卡片 CSV」 |
| 删除卡片 | S18 | 该块无卡 | 一行「这一段还不是卡」 |
| 「插件动作」 | S31 | 该槽无项（含全部隐藏） | 紧跟一行「没有插件提供这一动作」 |
| 「修改键位」 | S42 | 永久（1.0 不含 #155） | 一行「自定义键位不在 1.0，见 #155」 |
| 正文尺寸 | S39 | 永久（跟随系统） | 一行「跟随桌面文本缩放」，并给系统设置入口 |
| 「加一步」（学习步） | S33 | 永久（无 per-card step counter） | 一行「当前重来后次日再问；自定义学习步不在本轮」 |
| `status.item` 槽 | 状态行 | 永久（预留槽） | 不展示；在插件设置页「问题」里说明「宿主暂无接收方」 |

### 4.14 `kit` 组合与缺口（每个缺口都要写明查过哪些组件）

| 组件 | 承载什么 | 查过而不用 | 结论 |
|---|---|---|---|
| `kit.Empty` | 空态图标 + 句 + 动作 | `ui.Box` 拼、`ui.AlertDialog` | 组合保留（无官方空态组件），文件内注明 v0.8.1 |
| `kit.Stat` | 大数字 + 小标签 + 细节 | `ui.Meter`、`ui.Badge` | 组合保留（数字块不是 meter） |
| `kit.Keycap` | 键位提示 | `ui.Badge`、`ui.Tooltip` | 组合保留（badge 语义是计数） |
| `kit.Heatmap` | 日历热力格 | `ui/chart`（v0.8.1 无 calendar heatmap）、`ui.GridView`、`ui.Grid` | 按 `screens/stats/stats.go` 既有口径保留组合；`GridView` 不提供等宽列与周对齐 |
| `kit.ProgressRatio` | 新/学习中/复习三段占比条 | `ui.Meter`（单值）、`ui/chart`（无堆叠条 mark） | 组合；`chart` 的 stack 只对 column/area 有效 |
| `kit.GradeBar` | 四项评分分布条 | `ui/chart` 条形 | 组合；四项用图表过重 |
| `kit.Paper` | 纸面（居中 measure + gutter） | `ui.Scroll` + `ui.Column` | 组合保留（`ui-layout-rules.md` 已定） |
| `kit.PageHead` | 页头：H1 标题 + Label 副行 + 页级动作 | `ui.Toolbar`（窗口条，一个 tab stop 与 overflow）、`ui.Fieldset`（表单组，无自用标题） | 组合保留（v0.8.1 无页头组件）；§4.10 的 measure 由调用方给 |
| `kit.SectionHead` | 节头：UI 13/600 名 + Label 提示 + 右侧动作 | `ui.Fieldset`（无计数与动作）、`ui.TabsBase`（选页而非标题） | 组合保留；替代屏内自绘的 H2 节标题 |
| `kit.Notice` | 就地提示：图标 + 一句 + 可选动作，语义色 12% 底 | `ui.Badge`（计数）、`ui.AlertDialog`（提问） | 组合保留（v0.8.1 无 callout/banner）；错误与"写不进去"共用一处 |
| `kit.Board` | 白板节点网格 | `ui.NewCanvas`/`Draw`（自由画布）、`ui.GridView` | v1 用 `ui.Grid` + `ui.List` 语义行；自由画布（坐标 + 连线）推迟（§10） |

---

## 5. 数据语义增量

标注约定：**新增** = 本稿要求的新函数/字段/常量；**改** = 既有实现的形状变化；
未标注的签名是今天真实存在的（可在给出的文件里核对）。

### 5.1 今日：顺序、预算、完成

**落点**：`internal/core/queue/today.go`（**新增**文件，包 `queue`）。

```go
// 新增
type EntryKind uint8
const (
    EntryRead EntryKind = iota   // 一篇到期待读的笔记
    EntryExcerpt                 // 读过但还没有摘录的笔记
    EntryReview                  // 今天到期的卡片（聚合一行）
)

type Entry struct {
    Kind     EntryKind
    Note     core.ID   // EntryRead / EntryExcerpt
    Title    string
    Position core.ID   // 上次停下的块
    Blocks   int
    Read     int       // 读到的段号
    Cards    int       // EntryReview：到期张数
    New      int       // EntryReview：其中新卡
    Minutes  int
    Over     bool      // 超出预算
    LastSeen time.Time // EntryExcerpt：最近一次阅读
}

type Today struct {
    Entries  []Entry
    ReadMinutes, ReviewMinutes, TotalMinutes, Budget int
}

type TodayOptions struct {
    Budget        time.Duration // 0 = 不截断
    ExcerptSlots  int           // 0 = 默认 2
    Reading       Options       // 复用既有的 Limit/Scope
    Cards         Options
}

func Entries(lib *store.Library, now time.Time, opts TodayOptions) (Today, error) // 新增
func ExcerptCandidates(lib *store.Library, now time.Time, limit int) ([]store.Reading, error) // 新增
func Estimate(readings, cards int) time.Duration // 新增：MinutesPerReading*readings + MinutesPerReview*cards

const (
    MinutesPerReading = 6.0  // 新增：一次阅读会话的估算
    MinutesPerReview  = 0.6  // 新增：一张卡的估算
)
```

**顺序（冻结）**：① 到期笔记按 `queue.Reading` 的既有顺序（`Due` 升序、同刻按 ID）；
② 读过未摘按最近一次 `reviewed` 的 `at` 倒序，取前 `ExcerptSlots`（默认 2）；
③ 卡片聚合一行永远最后，只在 `CountsOf(...).Remaining > 0` 时出现。

**预算（冻结）**：`Budget` 来自 `presets.budget.minutes`；前 ①② 按 `MinutesPerReading` 累加，
从第一条让累计超过预算的开始 `Over = true`（仍显示、仍可点）；③ 的分钟数单独计入
`ReviewMinutes`，也参与 `TotalMinutes`。`ui.Meter` 显示 `TotalMinutes/Budget`，封顶 100%。

**完成（冻结）**：
- `EntryRead`：写下 `reviewed`（或 `postponed`/`archived`/`dismissed`/`dropped`）后，因到期变化而离开今日；
- `EntryExcerpt`：该笔记出现子笔记（`ExcerptTree` 有它）即离开，无需事件；
- `EntryReview`：卡片被评分或暂停后，`CountsOf` 变小。

**读数**：`App.loadToday`（**新增**，`internal/ui/today.go`）在每次 reload 与进入今日时计算一次；
托盘与提醒继续用 `App.queueCounts`（既有，`internal/ui/route.go`）。

### 5.2 阅读事件：四种终态 + 推后

**事件种类**（`internal/core/schedule/schedule.go` 的 `Kind`，**改**：新增三个值 + `String`/`ParseKind`）：

| `kind` | 含义 | 载荷 | 重放效果 |
|---|---|---|---|
| （省略） | 一次评分（卡片或笔记） | `card` 或 `note` + `rating` [+ `position`] | 既有 |
| `queued` | 入队 | `note` | `queued=true`，清 `archived`/`dismissed`；未读过的置 due=at |
| `dropped` | 移出队列 | `note` | `queued=false`，历史保留 |
| `archived` | 归档 | `note` | `archived=true`，`queued=false` |
| `dismissed` | 不再出现 | `note` | `dismissed=true`，`queued=false` |
| `postponed` | 推后 | `note` + `until` | `due=until`，成员不变，评分史不变 |

**精确行形状**（`reviews.jsonl`，一行一条；顺序即写入顺序）：

```jsonl
{"seq":412,"note":"note_000000000000000000000000000000a1","kind":"archived","at":"2026-09-18T09:31:07Z"}
{"seq":413,"note":"note_000000000000000000000000000000a1","kind":"dismissed","at":"2026-09-18T09:32:00Z"}
{"seq":414,"note":"note_000000000000000000000000000000a1","kind":"postponed","until":"2026-09-19T00:00:00Z","at":"2026-09-18T09:33:10Z"}
{"seq":415,"note":"note_000000000000000000000000000000a1","kind":"queued","at":"2026-09-20T08:00:00Z"}
{"seq":416,"card":"card_000000000000000000000000000000b2","rating":"good","at":"2026-09-18T09:35:00Z","scheduler":"remit.builtin/1"}
```

规则（与 `incremental-reading.md` §3 同一套）：
- `kind` 缺省是评分；非评分事件必须带 `note`、不带 `card`、不带 `rating`。
- `until` 只出现在 `postponed` 上，RFC 3339；缺失或早于 `at` 视为坏行（打开失败）。
- 日志只手写可改；坏行让打开失败；末行缺换行视为写了一半（既有 `streamLog`）。
- 新字段 `until` 是**改** `logRecord`（`internal/core/store/log.go`）：加 `Until string \`json:"until,omitempty"\``，
  `marshalRecord`/`DecodeLogLine` 各加一条分支。

**撤销（冻结）**：
- `void` 仍只撤销一条**评分**事件（卡片或笔记），不撤销成员变更与终态——与既有语义一致。
- 终态的恢复是写**反向事件**：`archived`/`dismissed` → `queued`；`postponed` → 再写一条
  `postponed`（`until` 为今日）或 `queued`（重排到当前到期）。
- **新增** `func (l *Library) UndoReading(noteID core.ID, at time.Time) error`
  （`internal/core/store/reading.go`）：对一条笔记 void 掉最新一条存活的评分事件，重放该笔记的
  due/position。UI 的「撤销」在阅读会话里调它，在复习会话里继续用既有 `UndoLast`。

**索引**（`internal/core/store/index.go`，**改**：`schemaVersion "10" → "11"`）：
`reading` 表加 `archived INTEGER NOT NULL DEFAULT 0`、`dismissed INTEGER NOT NULL DEFAULT 0`、
`until INTEGER NOT NULL DEFAULT 0`；`card_state` 加 `lapses INTEGER NOT NULL DEFAULT 0`。
`store.Reading` 加 `Archived`、`Dismissed` 两个字段（**新增**）。

### 5.3 库配置 v3 与预设

**落点**：`internal/core/store/library.go`（`FormatVersion`、`configFile`、`Presets`）。

```json
{
  "format": "remit/3",
  "fsrs": { "weights": [21 floats], "desired_retention": 0.9 },
  "extensions": ["remit.cards.standard"],
  "roles": { "cards.derive": "remit.cards.standard" },
  "presets": {
    "budget":  { "cards": 20, "minutes": 30, "excerptSlots": 2 },
    "review":  { "newPerDay": 10, "maxIntervalDays": 180, "retention": null, "preview": true },
    "reading": { "newPerDay": 3, "returnAfterExcerpt": true },
    "hard":    { "lapses": 3 },
    "broken":  { "policy": "keep" }
  }
}
```

- `retention: null` = 跟随调度插件的策略（今天的行为）；非 null = 用户覆盖，写进
  `store.Options.Params.DesiredRetention`（**改** `App.storeOptions`，`internal/ui/library.go`）。
- **新增** `type Presets struct`、`func (l *Library) Presets() Presets`、
  `func (l *Library) SetPresets(Presets) error`、`func DefaultPresets() Presets`，
  `store.Options.Presets`。
- **迁移（改）**：打开 `remit/2` 时读 `fsrs`/`extensions`/`roles`，`presets` 取默认值，先把原文件复制到
  `remit.json.bak`，再写 `remit/3`；未知顶层键原样保留在 `"unknown"` 里再写回（#226 的要求）。
  迁移失败不改文件，只报错（库仍打开，预设用默认值 + 一条日志 + 设置页一行提示）。
- 卡片 `newPerDay` 走既有 `queue.Options.NewPerDay`；阅读 `newPerDay` 是**新增**
  `queue.Options` 语义（未读条目每日引入上限），由 `queue.Reading` 支持。
- **学习步不在本稿**：当前调度用 FSRS `NextState`，没有 per-card step counter
  （`internal/core/schedule/schedule.go` 文件头已写明）；`presets.review` 不存步长，
  S33 的「学习步」是只读说明。自定义学习步等调度契约（#227 之后）另开。

**作用域分界（重要）**：预设、保留率、上限、断链策略 **随库**走（`remit.json` 的 `presets`）。
下面这些是 **app 偏好**，进 `internal/settings` 的 `settings.json`，**不得**写进库配置：
`appearance`（系统/纸面/墨色，#228）、`paperWidth`、`recent`（最近库，最多 5 条）、
`savedFilters`（忆队列的保存筛选）。理由：它们随机器变（外观、窗口尺寸、最近路径），而节奏随库走。

### 5.4 历史查询（无新增存储）

- 一天的事件：`Library.LiveReviews(from, to)`（既有，`internal/core/store/library.go:1455`）。
- 一张卡的历史：`Library.History(cardID)`（既有，`internal/core/store/log.go:230`）。
- 一条笔记的阅读史：`Library.ReadingEvents(noteID)`（既有，`internal/core/store/log.go:237`）。
- **新增** `internal/core/stats/history.go`：`HistoryOf(lib, from, to) History`，其中
  `History{Events []HistoryEvent, Grades [4]int, Total, Again int}`，
  `HistoryEvent{At, Kind, NoteTitle, CardID, Rating, IntervalFrom, IntervalTo}`（名字与 `queue.Entry`
  区分开）——间隔变化由相邻评分事件的 `schedule.State` 差算出。

### 5.5 产出统计与时间配比

**新增** `internal/core/stats/production.go`：

```go
type Production struct { Characters, Notes, Excerpts, Cards int; CardsPerExcerpt float64 }
func ProductionOf(lib *store.Library) Production // 新增

type TimeShare struct { Read, Review, Excerpt int } // 分钟
func TimeShareOf(lib *store.Library, from, to time.Time) TimeShare // 新增
const MinutesPerExcerpt = 2.0 // 新增
```

字数与块数走索引的 `blocks` 表（`COUNT(*)` / `SUM(length(text))`），**不逐篇读文件**；摘录数与卡片数
走 `notes`/`cards` 表的计数（`Excerpt` 由 `links` 的 `[[标题#^块]]` 命中判出）。

口径：读 = `reviewed` 且有 `note` 的事件 × `queue.MinutesPerReading`；忆 = 卡片评分事件 ×
`queue.MinutesPerReview`；摘 = 摘录笔记数 × `MinutesPerExcerpt`。三者共用一份存活日志，故与热力、
预测、托盘同一来源（`incremental-reading.md` §11 的口径）。

**难卡**：`schedule.State` 加 `Lapses int`（**新增**），重放时遇 `Again` 累加；
`card_state.lapses` 存它；`presets.hard.lapses` 是比较阈值。三处读同一个数：
S27 的 chip、S31 的侧栏与「重来」列、S31 搜索语法的 `is:hard`。

### 5.6 查询与写入口

**落点**：`internal/core/store/query.go`（**新增**类型与函数）、`internal/core/store/index.go`（**新增**表）。

| 用途 | 函数 | 说明 |
|---|---|---|
| 笔记列表行 | **新增** `QueryNoteRows(q NoteRowQuery) ([]NoteRow, int, error)` | `NoteRow{ID, Title, Summary, Blocks, Cards, ReadingPos, ReadingBlocks, Updated, Excerpt bool}`；`NoteRowQuery{Text, Tag, Reading, HasCards, Excerpt, Sort, Limit, Offset}`（三个 bool 是 S05 的筛选，`Sort` 加 `"cards"`）；`Summary` 是首块文本压成一行；索引 join `blocks`/`cards`/`reading`，一页只读它返回的行 |
| 标签页 | **新增** `QueryTags(limit int) []TagRow` | `TagRow{Tag, Count, Sample []string}`；`Sample` 取 2 个标题 |
| 资产页 | **新增** `AssetRows() ([]AssetRow, error)` | `AssetRow{Name, Ext, Bytes, Notes []core.ID}`；新增索引表 `assets(name TEXT PRIMARY KEY, ext TEXT, bytes INTEGER, notes TEXT)`，引用关系由 `[[…]]`/相对路径扫描写入 |
| 摘录树 | **新增** `ExcerptTree() []ExcerptNode` | `ExcerptNode{Note, Title, Cards int, Children []ExcerptNode}`；由 `links` 表（`[[标题#^块]]`）反查 |
| 读过未摘 | **新增** `UnExcerptedNotes(limit int) []Reading` | 有阅读位置且没有子笔记的笔记；**B1 的 PR 里先落这一条**（今日队列要用），其余查询归 B5 |
| 卡片表 | **改** `CardQuery` 加 `Type core.ID`、`Ref *core.LinkState`、`MinLapses int`；`CardSummary` 加 `Note core.ID`、`Type core.ID`、`Ref core.LinkState`、`Lapses int`、`IntervalDays int`；**新增** `CardSortInterval`、`CardSortLapses` | 排序键加两列 |
| 批量动作 | **新增** `SuspendCards([]core.ID, time.Time)`、`SetDue([]core.ID, time.Time)`、`FreezeCards([]core.ID, time.Time)`、`AddTagToNotes([]core.ID, string)` | 每个都写既有卡片文件/笔记明文，不新增格式 |
| 搜索语法 | **新增** `queue.ParseCardQuery(raw string) store.CardQuery`（`internal/core/queue/syntax.go`） | `is:` / `type:` / `tag:` / `note:`，空格为「与」；未知键忽略并原样当文本 |

**正文改写的唯一入口**（S15 挖空）：**新增** `core.Cloze(text, selected string) string`
（`internal/core/excerpt.go` 旁新增 `cloze.go`，保持 core 无 UI）：第一次命中包成 `{{…}}`；已含同文本的
`{{…}}` 则不重复包裹。

### 5.7 路由入口

- **新增** `internal/deeplink` 的三种 `Kind`：`KindToday`（`remit://today`）、`KindRead`
  （`remit://read/queue`）、`KindRecall`（`remit://recall/queue`）。既有 `note` / `card` / `queue` /
  `search` 不变；`queue` 继续落到复习队列（兼容旧链接）。
- **新增** `func (a *App) ShowArea(area, sub)`（`internal/ui/route.go`）：深链接与命令面板共用。

### 5.8 agent API 的计数

**落点**：`internal/agentapi/server.go`（`getLibrary`、`getReading`）。

- `GET /v1/library` 的计数加三项（**新增**）：`today`（今日队列条数）、`archived`、`unexcerpted`。
- `GET /v1/reading` 的每项加 `archived`、`dismissed`、`due`（**新增**字段），既有字段不变。
- 不加路由、不加写入口；删除仍不暴露（`three-in-one.md` §7）。

---

## 6. 插件面

契约与本文的分工：`docs/specs/plugins/ui.md` 冻结构件（manifest、`when`、槽位、页面节点、失败隔离）；
本文只补「宿主在哪里接收」。改动全部是**兼容添加**（新槽位、新上下文键、新页面数据），
`apiVersion` 不动（新增 = 兼容，`ui.md` §10）。

### 6.1 轨道上的插件页分组

- 位置：轨道底部，一条 `ui.Divider` 之下，`ui.SidebarSection(c, msgs.PluginPages(), nil, …)`；
  每页一项，图标取 manifest 的 `icon`（`design.HasIcon` 校验过），名取 `contributes.pages[].title` 的
  `%key%` 译文。
- 顺序：宿主先按 `nav`（`plugins` 组），组内按页面 id 字典序——**排序归 #129，本稿只保证「有一组」**。
- 无页面时整段不建（不出现空标题）。页面被三次失败隐藏时，项从轨道消失，设置页仍保留一行。
- `nav: "hidden"` 的页面不进轨道，只可从命令（`navigate` 结果）进入。

### 6.2 设置页的插件列表（S43）

每插件一行：名 +（内置时）`ui.Badge("内置")` + 描述 + capability `ui.Badge` 列 +「详情」+ 启用 `ui.Switch`
（开关就是授权：写库的 `extensions`，`App.setPluginEnabled` 既有）。下面「问题」段列两件事：

1. manifest 里被丢弃的贡献（`App.ui.problems[name]`，来自 `contrib.Parse` 的 `[]error`）；
2. 三次失败被隐藏的贡献（`App.ui.failures` / `App.ui.hidden`，键是 `plugin/id`），每行「来源 + 原因 +
   `已隐藏` 徽章 + 重新启用」（`Deps.Reenable` 既有）。

### 6.3 每个槽位的接收方

| 槽位 | 接收方（本稿落点） | `when` 里可用的上下文 |
|---|---|---|
| `toolbar.overflow` | 区域顶栏的 `⋯`（`internal/ui/shell`）；槽项由 W9 经 `shell.Deps.OverflowItems` 送进 | `area`、`sub`、`library.open`、`mode` |
| `note.overflow` | 文档面的 `⋯` | 上面 + `note.open` |
| `note.block` | 块菜单底部 + 块检查器「插件」段（S16/S20） | 上面 + `note.selection`、`card.present` |
| `editor.slash` | `/` 插入菜单底部（S17） | 上面 + `note.selection` |
| `review.card` | 复习会话动作行（S29） | `card.present`、`card.status`、`mode` |
| `browse.row` | 卡片表工具行「插件动作」（S31），需要 `browse.selection` | `browse.selection`、`card.present` |
| `sidebar.note` | 笔记行菜单（S52） | `note.open`、`mode` |
| `page.header` | 插件页页头（S50/S51） | `page.id`、`plugin.id` |
| **`today.widget`（新增）** | 今日页的一段（S04 第 7 块）：标题 + 卡片 + 槽项按钮 | `library.open`、**`area`**（值 `today`） |
| **`stats.block`（新增）** | 统计·总览的一段（S36） | `library.open`、`area`（值 `stats`） |
| `status.item`（预留） | 状态行 —— **不展示** | 留待 `apiVersion` 升级；登记在设置页「问题」里 |

新增上下文键（`internal/ext/contrib/when.go`，`Context` 加 `Area`、`Sub` 两个字段 + 两条 `contextReads`）：
`area` ∈ `today|write|read|recall|stats|settings|plugin`；`sub` 是子页的稳定 id（`queue`、`cards`…）。
`mode` 的取值表**不变**（`notes|review|browse|stats|settings|""`），由 §2.7 的映射给出——这样既有
`when` 表达式不会因为 IA 改动而静默变真/变假。

### 6.4 白板页（S51）

白板是「插件提供数据、宿主绘页面」的第一个实例。契约不动（不加 `board` 节点、不加坐标字段）：

- 插件 `acme.whiteboard` 声明一个页面 + 一个数据命令，返回节点树：
  `section` → `grid(columns: 2)` → 若干 `card`，每个 `card` 里是 `text`（笔记标题）、`markdown`（块首行）、
  `actions`（「回到这一段」）。节点与块的对应由宿主从 `actions` 项的 `value` 里读 `note`/`block`
  （`contrib.Item{Label, Value, Command}`：`Command` 是槽命令 id，`Value` 是它要的参数）。
- 宿主把它画成 `kit.Board`：`ui.Grid` 的卡片网格，每张卡可点（命令回填 `navigate`，宿主跳到
  `#/write/doc/<note>?mode=read` 并把光标放到该块）。
- 块的 `⋮⋮` 菜单与块检查器各有一项「在白板里定位」（仅当该块出现在当前页面数据里），跳回
  `#/plugin/acme.whiteboard` 并给节点加一次 `design.ModeSwitch` 高亮。
- 不做：自由画布（坐标、拖拽、连线）。理由写进 `kit.Board` 注释：`ui.NewCanvas` + `Draw` 能画，
  但节点会失去键盘、焦点、读屏与 720×520 的降级——那正是 `mygo-components` 禁止的手写理由。
  自由画布的裁决见 §10。

### 6.5 失败隔离（与 `ui.md` §8 对齐，本文只补可见性）

| 情况 | 宿主行为 | 用户看到 |
|---|---|---|
| 一条贡献非法 | 只丢那一条 | 设置页「问题」一行 |
| 页面数据抛错/超时 | 页面错误态 + 重试 | 页面内 callout + 重试按钮；第 3 次后导航项消失 |
| 命令抛错/超时 | toast + 计数 | toast + 设置页一行 |
| 未知节点/叶 | 该节点占位 | 占位块，含 `kind` 名 |
| 插件页数据超限 | 错误态 | callout 说明上限 |
| 钩子失败 | 不影响 UI 贡献，反之亦然 | 各自计数分别显示 |

**永不在帧里调插件**：贡献在打开库时解析一次（`App.loadContributions` 既有）；页面数据只在进入、
显式刷新与 `onEvent` 时取，结果按 `(plugin, page, context-hash)` 缓存（既有 `pagesFor`）。
新增的两个槽位只读缓存里的 `MenuRef`，不触发调用。

---

## 7. 视觉与动效

### 7.1 颜色角色（`internal/ui/design/theme.go` 的 `Palette`，不另起名字）

| 角色 | token | 用途 |
|---|---|---|
| Page | `Palette.Page` / `theme.Background` | 窗口底 |
| Paper | `Palette.Card` / `theme.Surface` | 顶栏、子导航、纸面、状态行、卡面 |
| Sunken | `Palette.Sunken` | 输入底、代码底、分段槽、表头 |
| Hairline | `theme.Border` | 所有 1px 分隔与容器边 |
| Ink / Pencil / Faint | `theme.Text` / `theme.TextMuted` / `Tokens.TextFaint` | 正文 / 次要 / 提示 |
| Mark | `theme.Accent` | 光标、当前块、当前导航项、进度、可点；可作线，也可作浅底（`Accent.Alpha(0.10)`） |
| Review 四色 | `Tokens.ReviewAgain/Hard/Good/Easy` | 只有评分与历史节点用；**颜色不单独承担意义**（每处都有文字） |

规则：相邻面用 1px 细线 + 至多一级明暗差；只有浮层用阴影（`Tokens.ShadowPanel`）；
**没有渐变、没有玻璃、没有装饰性卡片叠层**。深浅两套各自满足正文对比度（`Palette` 已有）。

### 7.2 字阶（`design/scale.go`，**改**）

| 名称 | 常量 | 字号 / 行高 | 用途 |
|---|---|---|---|
| Display | `TypeDisplay` | 34 / 1.34，600 | 文档标题，一篇一个 |
| H1 | `TypeH1`（新） | 27 / 1.3，600 | 页面标题、卡面题面 |
| H2 | `TypeH2`（原 `TypeTitle`） | 22 / 1.45，600 | 面板标题、对话框标题、统计数字 |
| Body | `TypeBody`（名字保留，值改成 18） | 18 / 1.85，400 | **只有**纸面正文与卡面答案 |
| UI | `TypeUI`（原 `TypeSmall 13` / `TypeChrome 13` 与**旧的** `TypeBody 15` 合并） | 13 / 1.45，400 | 控件、菜单、表格、列表 |
| Label | `TypeLabel` | 12 / 1.45，400 | 说明、时间、计数、次级按钮 |
| Cap | `TypeCaption` | 11 / 1.45，400 | 状态行、注脚、键位 |
| Mono | `TypeMono`（改 14→13） | 13 / 1.45，400 | 路径、模板、日志 |

行高常量：`LineTight 1.3`（H1）、`LineDisplay 1.34`、`LineHeading 1.45`、`LineHeight 1.45`（chrome）、
`TextLineHeight 1.85`（正文）。旧名（`TypeTitle`/`TypeHeading`/`TypeSmall`/`TypeChrome`）在同一个 PR 里
删除，机械替换映射写进 PR 描述：`TypeTitle→TypeH2`、`TypeHeading→TypeH1`、`TypeSmall→TypeUI`、
`TypeChrome→TypeUI`、**旧的** `TypeBody 15 → TypeUI`（改名后的 `TypeBody` 指文档正文 18，别搬错方向）。

**正文宽度**：`PaperWidth 760 − 2×Gutter 48 = 664 ≈ 37 个汉字`；文档字号继续跟随桌面文本缩放
（`Tokens.TypeDisplay × scale`，`NewTokens` 既有），#164 的「正文不跟随」由本稿一并关掉。

### 7.3 间距、形状、图标

- 间距：4 / 8 / 12 / 16 / 24 / 32（`SpaceXS…SpaceXXL`）；组内 8–12，组间 16–24。
- 圆角：`RadiusControl 6`、`RadiusCard 10`、`RadiusPanel 14`、`RadiusBar 3`。
- 控件：命中区 ≥ 34（`Hit = ControlHeight + 2`，新常量），输入 ≥ 32。
- 图标：Lucide（`design/icons`，v0.460.0），2 描边，`IconSmall 16` / `IconMedium 20` / `IconLarge 24`，
  颜色随文字。
- 分隔：`ui.Divider`；留白用 `ui.Spacer`，不用定高空 `Box`。

### 7.4 尺寸常量（全部落在 `internal/ui/design/scale.go`）

| 常量 | 值 | 备注 |
|---|---:|---|
| `RailWidth` | 240 | 导航轨 |
| `RailCollapsed` | 64 | ≤980 |
| `BreakpointNarrow` / `BreakpointSmall` | 980 / 760 | 折叠 |
| `FrameHeight` | 60 | **改**（原 64） |
| `SubnavHeight` | 44 | 新 |
| `StatusbarHeight` | 30 | 新 |
| `PanelWidth` | 288 | 新 |
| `PaperWidth` | 760 | 不变 |
| `Gutter` / `GutterNarrow` | 48 / 20 | 后者新 |
| `ListWidth` | 860 | 新 |
| `SettingsWidth` | 660 | **改**（原 640） |
| `BrowseWidth` | 1080 | **改**（原 1040） |
| `ReviewWidth` / `SummaryWidth` / `MessageWidth` | 680 / 520 / 520 | 不变 |
| `WelcomeWidth` | 640 | 新 |
| `FilterPaneWidth` | 200 | 新（S31 左筛选栏） |
| `NotesColumnWidth` | 260 | **迁移**：从 `screens/notes` 的 `defaultNotesWidth` 搬进 design（用户可拖的初始值） |
| `ControlHeight` / `Hit` | 32 / 34 | 后者新 |
| `TypeDisplay 34` / `TypeH1 27` / `TypeH2 22` / `TypeBody 18` / `TypeUI 13` / `TypeLabel 12` / `TypeCaption 11` / `TypeMono 13` | — | 见 §7.2 |
| `DisplayNarrow` | 30 | ≤760 |

屏内不得再出现字面宽度：`internal/ui/screens/notes/notes.go` 的 `defaultNotesWidth 260` 也搬成
`design.NotesColumnWidth`（W3b 的 PR 里删掉那个私有常量）。`ui.SplitPercent` 用的**比率**（72 / 50）
不是长度，留在屏里。

### 7.5 动效

| 预设 | 时长 / 曲线 | 用在哪 | 落点 |
|---|---|---|---|
| `Hover` | 120ms ease-out | hover、按下反馈、chip、开关 | 既有 `design.Hover` |
| `Press` | 90ms | 按下缩放 `PressScale 0.98` | 既有 |
| `ModeSwitch` | 180ms ease-out，上移 6px + 淡入 | 换区域、换子页、浮层内容展开 | 既有；子导航下划线 `scaleX` 也用 180ms |
| `Row` | 180ms，每行 +24ms，最多 6 行 | 队列、笔记行、大纲进场 | `Row` + `EnterStagger` |
| `Reveal` | 180ms，上移 6px + 淡入 | 答案展开、总结、查找条 | 既有 `Reveal`（260ms 改为 180ms） |
| `Panel` | 180ms，`PanelScale .96 → 1` | 菜单、对话框、面板浮层 | 既有（200→180） |
| `Progress` | 260ms，宽度 0 → 目标 | 阅读进度、预算条、热力载入 | 新增 `Progress` 预设 |

只动 `opacity` 与 `transform`（进度条是唯一例外，且只在挂载时跑一次）。`c.Preferences().ReduceMotion`
时全部归零，布局不变；`tt.FramePending()` 必须能为 false。预设是包级命名值（`design/motion.go`），
不在屏里写时长字面量。

---

## 8. 并行工作分解

### 8.0 每个切片开工前的硬规则（逐字执行）

> **load all six mygo skills first**: `.agents/skills/mygo-usage/SKILL.md`,
> `mygo-components`, `mygo-motion`, `mygo-drawing`, `mygo-desktop`, `mygo-maintenance`.
> 然后读本稿相关小节 + `docs/specs/ui-composition.md` + 目标屏的既有测试。

其余共同规则：
- 一个切片一个 PR（squash）；PR 正文写「用了哪个组件、删了哪段手写、哪些测试证明是组件在干活」。
- 只用 MyGo v0.8.1 存在的 API（`go list -m -f '{{.Dir}}'` 指到的那份源码）；缺的能力升级，不包壳。
- 屏包 = `State`/`Deps`/`View`（ADR 0004/0009）；屏不 import `internal/ui`、兄弟屏、store。
- 时间注入；core 不 import `mygo/ui`；`internal/core` 的改动不带任何 UI 概念。
- 文案 en + zh-Hans 同改；`go tool mygo i18n check .` 与 `go tool mygo generate` 干净。
- 不写像素基线；断言文本、盒子、焦点、能力。

### 8.1 前端工作流（UI 包所有权互不重叠）

| # | 工作流 | 拥有（文件/包） | 依赖 | PR 边界 | 关键接缝 |
|---|---|---|---|---|---|
| **W1** | 骨架 | `internal/ui/design/{scale,tokens,motion,theme}.go`、`internal/ui/shell/`（**新**）、`internal/ui/{app,view,route,window}.go`、`internal/deeplink` | — | 一个 PR：字阶与常量、shell 包、`area/sub/session` 状态、键盘地图、深链接 | `App.setArea`、`shell.Deps`、`design.Tokens` 字段（W2–W9 全部依赖它） |
| **W2** | 今日 | `internal/ui/screens/today/`（**新**）、`internal/ui/today.go`（**新**） | W1、B1、B6（槽位） | 一个 PR：S04 + `today.widget` 接收方 | `today.Deps.Today`（B1 的类型）、`kit.Stat`、`kit.Empty` |
| **W3a** | 写（列表类子页） | `internal/ui/screens/notes/{search,tags,tree,assets,templates}.go`（新）、`internal/ui/screens/notes/sidebar.go` | W1、B5 | 一个 PR：S05–S10 | `notes.Deps` 的列表侧新函数、`ui.Table`（S08）、`ui.Tree`（S10）、`ui.ToggleGroup`（S05 的筛选） |
| **W3b** | 文档 | `internal/ui/screens/notes/{editor,blocks,slash,outline,panel,cards,document,tables}.go`、`internal/ui/notes.go` 的文档侧 | W1、W3a（共用 `notes.Deps`）、B5 | 一个 PR：S11–S17、S20–S22（沟槽徽章、选区动作条、块检查器、状态行搬迁、面板浮层）；S18/S19 两个卡片浮层归 W8 | 与 W3a 共用 `notes.State`：W3a 先合，W3b 只碰文档侧文件；`window.go` 归 W1（文档键位在 W1 的 map，动作经 `Deps`），W3b 不改它；`design.NotesColumnWidth` 的迁移在这一片 |
| **W4** | 读 | `internal/ui/screens/read/`（**新**）、`internal/ui/screens/reading/*`、`internal/ui/reading.go` | W1、B1、B2 | 两个 PR：① 队列与归档页（S23/S24）；② 会话扩展（S25/S26：A/D/P、动作行重排） | `reading.Deps` 新函数、`store.Reading.Archived/Dismissed` |
| **W5** | 忆 | `internal/ui/screens/recall/`（**新**：队列/预设/历史）、`internal/ui/screens/browse/*`、`internal/ui/screens/cardtypes/`（**新**，从 settings 迁入）、`internal/ui/screens/review/*`、`internal/ui/screens/relink/*` | W1、B1、B2、B3、B4、B5 | 三个 PR：① 队列 + 范围（S27/S28）；② 卡片表 + 类型（S31/S32，`ui.Table` 迁移）；③ 预设 + 历史 + 断链 + 会话扩展 + 总结（S33–S35、S29/S30） | `browse.Deps`、`cardtypes.Deps`（从 `settings.Deps` 摘出类型四项）、`review.Summary.Failed` |
| **W6** | 统计 | `internal/ui/screens/stats/*` | W1、B4 | 一个 PR：三页（S36–S38）、`stats.block` 接收方 | `stats.State` 新字段、`kit.Heatmap`、`ui/chart` |
| **W7** | 设置 | `internal/ui/screens/settings/*`、`internal/ui/prefs.go`、`internal/settings` | W1、B3 | 一个 PR：8 个子页；类型表单迁出到 W5 的包 | `settings.Deps` 新函数、`settings.Settings` 新字段 |
| **W8** | 浮层 | `internal/ui/palette.go`、`internal/ui/shortcuts.go`（**新**，S48）、`internal/ui/cards.go`（S18）、
`internal/ui/import.go`（S02/S03）、`internal/ui/screens/pluginpage/*`（S50/S51） | W1、B6 | 两个 PR：① 命令面板 + 快捷键总表 + 卡片编辑器/详情；② 导入向导 + 导出 + 插件页与白板 | `paletteState`、`pluginpage.Deps`、`design` 的键位表常量 |
| **W9** | 插件面 | `internal/ui/contribs.go`、`internal/ext/contrib/*`（只加槽位与键）、`examples/plugins/{whiteboard,leeches}` | W1、W8（页面渲染） | 一个 PR：槽位接收方 + `area/sub` 上下文键 + 设置页问题段 + 白板示例插件 | `contrib.Slots`、`contrib.Context`、`App.ui.problems` |

### 8.2 后端通道（`internal/core`，与 W1 之后并行）

| # | 通道 | 拥有 | 依赖 | PR 边界 | 说明 |
|---|---|---|---|---|---|
| **B1** | 今日队列 | `internal/core/queue/today.go`（新）、`queue.go`；`store.UnExcerptedNotes` 的**第一份**实现（`internal/core/store/query.go`，只读） | — | 一个 PR + 表驱动测试 | §5.1；`Options` 加阅读每日上限；`UnExcerptedNotes` 先落在这里，B5 只做其余查询 |
| **B2** | 终态事件 | `internal/core/schedule/schedule.go`、`internal/core/store/{log,reading,index,library}.go` | — | 一个 PR + 日志/重放/撤销测试 | §5.2；索引 schema 11；`UndoReading` |
| **B3** | 预设与配置 v3 | `internal/core/store/{library,index}.go` | — | 一个 PR + 迁移与未知键保留测试 | §5.3；`remit/3` + `.bak`；app 偏好（`settings.json`）归 W7，不在本片 |
| **B4** | 历史与产出 | `internal/core/stats/{history,production}.go`（新） | B2（读新事件） | 一个 PR | §5.4/§5.5；难卡 `Lapses` 在 B2 的重放里 |
| **B5** | 查询 | `internal/core/store/{query,search,index,links,tags}.go` | B1/B2（读 `reading` 新列） | 一个 PR | §5.6；`assets` 表 |
| **B6** | 插件宿主 | `internal/ext/contrib/{contrib,when,index}.go`、`internal/ui/contribs.go` | — | 一个 PR（与 W9 同一 PR 亦可，但文件不重叠的话分开更小） | §6；新增槽位与 `area`/`sub` 键 |

**接缝（不得在切片里私自改）**：
1. `App` ⇄ 屏：屏只经 `Deps`；`Deps` 的签名在 W1 的 PR 里冻结成一份 `internal/ui/deps.go` 的清单注释。
2. `App` ⇄ store：写入一律 `a.lib.<Method>` + `a.reload()`；帧内只读快照。
3. B* ⇄ W*：B 的类型定义（`queue.Today`、`store.Presets`、`stats.Production`）在 B 的 PR 里一次定好；
   W 只消费，需要加字段时开 B 的小 PR，不在 UI 侧加临时结构。
4. `design` 常量：只有 W1 能加/改；其他工作流发现缺常量时在 PR 里提 `design` 的小改动并在描述里写明理由。
5. `locales`：每条工作流只加自己的键前缀，冲突在合并时按前缀解决。

### 8.3 每个工作流的 subagent brief（可直接粘给实现 agent）

> **W1 / 骨架**
> load all six mygo skills first（mygo-usage、mygo-components、mygo-motion、mygo-drawing、mygo-desktop、
> mygo-maintenance）。然后读 `docs/specs/ui-redesign-v2.md` §0、§2、§7 与 `docs/ui-layout-rules.md`。
> 交付：`design/scale.go` 的新常量与字阶改名、`internal/ui/shell`（rail + 顶栏 + 子导航 + 状态行，
> `State`/`Deps`/`View`）、`App` 的 `area/sub/session` 状态与 `setArea`、键盘地图、`deeplink` 三种新路由。
> 约束：只用 v0.8.1 组件（`ui.Sidebar`/`SidebarItem`/`SidebarSection`/`Badge`/`Toolbar`/`TabsBase`/
> `Breadcrumbs`/`Divider`）；不写新的手写导航；`≤980` 折叠与 `720×520` 无横向滚动；
> 测试 `TestRailCollapsesBelow980`、`TestRailCountsMatchTheQueues`、`TestSubnavKeepsTheAreaOnReload`、
> `TestAppBarAtMinimumSize`（既有，扩写）、`TestAreaShortcutsSwitchAreas`、`TestChineseAtTheMinimumSize`。
> 不实现任何子页内容：舞台留给后续工作流的空骨架。

> **W2 / 今日、W3a / 写、W3b / 文档、W4 / 读、W5 / 忆、W6 / 统计、W7 / 设置、W8 / 浮层、W9 / 插件面**：同一开头
> （load all six mygo skills first + 读本稿 §4 对应小节），各自把 §4 的屏规格逐条落成 `State`/`Deps`/`View`，
> 每个屏写 §4 给出的验收测试名；不得改动 `design` 常量与 `internal/core`（缺东西时开 B 的 PR）。

> **B1–B6**：开头同样是 load all six mygo skills（core 也要守 `mygo-maintenance` 的「框架常量与线程」规则），
> 然后读本稿 §5 的对应小节与 `docs/specs/incremental-reading.md` / `docs/adr/0002,0003`。表驱动测试放
> `internal/core/<pkg>`；写入路径必须有「写入 → 重开 → 删索引重建」三者一致的测试。

---

## 9. 验收

### 9.1 每屏能力测试（1024×720 与 720×520 各一遍，另加一次 zh-Hans 最小窗）

| 面 | 测试（"红"的那个 bug） |
|---|---|
| S01 | `TestWelcomeOffersThreeDoors`（少一个入口就红）、`TestWelcomeShowsWhyALibraryFailed`（错误被吞就红） |
| S02 | `TestImportWizardWalksThreeSteps`、`TestImportWizardRefusesAnEmptySource` |
| S03 | `TestExportWritesAndReports`（导出成功却无反馈就红） |
| S04 | `TestTodayOrdersReadingBeforeReview`、`TestTodayMarksWhatIsOverBudget`、`TestTodayCountsMatchTheRail`、`TestTodayOffersTheDailyNote` |
| S05 | `TestNotesListShowsSummaryAndCards`、`TestNotesListFiltersByTag`、`TestNotesSortCyclesAndSurvivesReload` |
| S06 | `TestSearchGroupsNotesAndCards`、`TestSearchFromAnotherAreaPutsTheCaretInTheField` |
| S07 | `TestTagPageOpensTheFilteredList` |
| S08 | `TestAssetsListNamesTheReferencingNotes` |
| S09 | `TestTemplateStartsANoteWithoutTheMarker`（既有 `TestNewNoteFromTemplate` 的 UI 面） |
| S10 | `TestExcerptTreeNestsChildren`、`TestExcerptTreeListsOrphans` |
| S11 | `TestDocGutterBadgeOpensTheBlockInspector`、`TestSelectionBarClozesTheBlockAndMakesACard`、`TestPaperKeepsGutterAtMinimumSize`；既有 `TestBlockMenuMovesParagraphWithItsIdentity`、`TestBlockGripDragReorders` 继续绿 |
| S12 | `TestReadModeMarksCardsAndExcerpts`、`TestReadModeExcerptsTheCurrentBlock` |
| S13 | `TestViewModesSwitchPanes`（既有，扩到三态） |
| S14 | `TestFindBarCountsMatches` |
| S15 | `TestSelectionBarExcerptsTheWholeBlock`（另见 S11 的 cloze 测试） |
| S16 | `TestBlockMenuOffersTheInspector`、`TestBlockMenuKeepsTheLastBlock`；既有 `TestDeleteBlockWarnsBeforeCardLoss`、`TestBlockMenuConvertsParagraph`、`TestBlockMenuDuplicatesParagraph` 继续绿 |
| S17 | 既有 `TestSlashMenuInsertsTable`、`TestSlashMenuFiltersAndInsertsFormula`、`TestSlashMenuAtMinimumSize` 继续绿 |
| S18 | `TestCardEditorWritesTheFieldsOfItsType` |
| S19 | `TestCardDetailListsTheScheduleHistory` |
| S20 | `TestInspectorShowsTheThreeRoles`、`TestInspectorOffersTheWhiteboardWhenANodeExists` |
| S21/S22 | `TestPanelSwitchesToBacklinks`、`TestPanelBecomesAnOverlayWhenNarrow`、`TestOutlineFollowsTheCaret`（既有） |
| S23 | `TestReadingQueueOpensTheSession`、`TestReadingQueueAddsANoteFromTheList` |
| S24 | `TestArchiveRestoresANoteToTheQueue`、`TestDismissedNotesStayOutUntilRestored` |
| S25/S26 | `TestReadingSessionArchivesANote`、`TestReadingSessionPostponesToTomorrow`、`TestReadingSessionDismissesAndTells`；既有 `TestReadingResumesWhereItStopped`、`TestExcerptLeavesTheReaderInPlace` 保持绿 |
| S27 | `TestRecallQueueGroupsByNote`、`TestRecallFilterNarrowsTheQueue`、`TestSavedFilterSurvivesRestart` |
| S28 | `TestScopeDialogNarrowsTheSession` |
| S29 | `TestReviewShowsTheSourceOfTheCard`、`TestReviewExpandsTheContext` |
| S30 | `TestSummaryGroupsFailedCardsByNote`、`TestSummaryCardIsCentered`（既有） |
| S31 | `TestCardTableSortsByColumn`、`TestCardSearchSyntaxNarrows`、`TestCardTableBulkSuspends` |
| S32 | `TestTypesPagePreviewsTheTemplate`、`TestTypesPageKeepsBuiltinsUndeletable` |
| S33 | `TestPresetsWriteTheLibraryConfig`、`TestPresetsResetToDefaults` |
| S34 | `TestHistoryListsTodaysReviews`、`TestHistoryCountsGrades` |
| S35 | `TestRelinkReattachesALostCard`、`TestRelinkNarrowsAsYouType`、`TestRelinkThawsAFrozenCard` |
| S36 | `TestStatsShowsProductionAndRatio`、`TestStatsBlocksComeFromPlugins` |
| S37 | `TestHeatmapExplainsItsSource` |
| S38 | `TestForecastMergesReadingAndCards` |
| S39 | `TestSettingsAppearanceWritesTheOverride` |
| S40 | `TestSettingsReviewWritesThePresets` |
| S41 | `TestSettingsBudgetPreviewMatchesToday` |
| S42 | `TestSettingsKeysAreReadOnlyWithAReason` |
| S43 | `TestPluginsShowCapabilitiesAndFailures`、`TestReenableBringsBackAHiddenContribution` |
| S44 | 既有 `TestAgentSwitchesNow`、`TestAgentFailureKeepsTheSwitch` 扩到新页 |
| S45 | 既有 `TestCheckNow` 扩到新页；新增 `TestReindexKeepsTheLibrary` |
| S46 | `TestAboutShowsTheLibraryFacts` |
| S47 | `TestPluginDetailShowsCapabilitiesAndFailures` |
| S48 | `TestKeysHelpListsTheMap` |
| S49 | `TestPalette`（既有）、`TestPaletteCommandsForReading`（既有） |
| S50/S51 | `TestPluginPageShowsItsHeaderActions`、`TestWhiteboardOpensTheBlock`、`TestWhiteboardSurvivesAChangedBlock` |
| S52 | `TestNoteRowMenuOffersReadingAndThePluginSlot` |
| 全局 | `TestNoHorizontalScrollAt720`、`TestChineseAtTheMinimumSize`（既有，扩到新区域）、`TestMinimumSizeControls`（既有） |

规则（`docs/specs/tests.md`）：不测像素坐标、不测文案常量、不测实现细节；一条测试守一件事，
名字就是那件事；它红了要么改代码要么没有它存在的理由。

### 9.2 命令

```sh
go build ./...
go test ./...
CGO_ENABLED=0 go test ./...        # 发货构建无 cgo，这一条最重要
go tool mygo vet .
go tool mygo i18n check .
go tool mygo generate              # locales/ 必须无 diff
```

### 9.3 组件审计（对着 v0.8.1 的源码跑）

```sh
go list -m -f '{{.Version}}' github.com/egoist/mygo   # 必须是 v0.8.1
MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)
UI=./internal/ui
# 字面尺寸：每一处都要么是 design 常量，要么写明理由
grep -rnE '\.(Width|Height|FontSize|Padding|Gap|MinWidth|MinHeight)\([0-9]' "$UI" --include='*.go'
grep -rnE '(Width|Height|FontSize|Padding|Gap)\s*(\w+)?\s*=\s*[0-9]' "$UI" --include='*.go'
# 手写选择器 / 表 / 分栏的痕迹
grep -rnE 'ButtonBase\(c\.Key|Width\(side\)|Width\(panelWidth\)' "$UI" --include='*.go'
# 反馈：toast 在，且没有只进日志的失败路径
grep -rn 'Toast' "$UI" --include='*.go'
# 用到的组件名都在这一份源码里（防止照着别的版本写 API）
grep -rhoE '^func [A-Z][A-Za-z0-9_]*' "$MYGO"/ui/*.go | sed -E 's/func //' | sort -u > /tmp/mygo-ctors.txt
```

每条命中要么修好，要么在 PR 描述里解释（`ui-composition.md` §3 的纪律）。

### 9.4 人工交接（headless 测不到的部分）

按 `docs/verification/handoff.md` 跑，并追加：

- 轨道折叠：拖窗口到 960 宽，轨道变 64、图标可点、tooltip 有名字；拉回 1200 恢复。
- 白板：点节点跳到阅读态的对应段落；从块的菜单回到白板，节点被高亮。
- 插件失败面板：把 `examples/plugins/leeches` 的页面数据改坏，页面进错误态；三次后导航项消失，
  设置页仍有行且「重新启用」能恢复。
- 归档 / 不再出现：A/D 之后在归档页恢复，笔记回到今日；重启后状态保持。
- 预设：改保留率 → 关窗 → 重开，值还在；手改 `remit.json` 加一个未知键，重开不丢。
- 深浅外观：系统切换外观时窗口即时跟随；显式选「纸面」后不再跟随。
- 无障碍：读屏能读出轨道计数、表格表头、评分按钮的键位；焦点环在深浅两套都可见。

### 9.5 不做的验收方式

不建像素基线；`tt.Image()` 只用于人工过目（PR 正文附图，不放仓库）。截图不得含真实笔记或 token。

---

## 10. 不做 / 已定口径

### 10.1 明确不做（本稿范围外，各自有出处）

1. **字符区间摘录 / 跨块选区**（#208 后续票；与 ADR 0002 的引用稳定性冲突，需单独模型决定）。
2. **文章捕获**（HTML / EPUB / URL / PDF）与 **AI 转换、AI 出题**（#209、#204；`three-in-one.md` §7）。
3. **deck / topic / 阅读条目等新实体**（#205、`three-in-one.md` §7）。
4. **第二套调度器 / A-Factor / 手写 priority 字段**（#207；priority 只是队列排序）。
5. **学习步自定义**（S33 的「加一步」禁用：当前 `schedule` 用 FSRS `NextState`，没有
   per-card step counter；改它需要先定调度契约，见 #227）。
6. **键位自定义**（#155，1.0 不含）。
7. **正文尺寸的第二套缩放**（#228 未定；本稿让它只跟随桌面）。
8. **自由画布白板**（坐标、拖拽、连线、缩放）：`ui.NewCanvas` 能画，但会丢掉键盘、焦点、
   读屏与最小窗降级；本轮白板是宿主绘制的只读卡片网格（§6.4），不是画布。
9. **卡片导出 CSV**（S03 里是禁用项）；**新的归档格式**：`.remitlib` 的进出直接复用既有
   `internal/core/transfer`（`Write`/`Read`），不新造格式、不加字段。
10. **设置文件的 v2 迁移与偏好注册表**（#221–#225）：本稿只动库配置 `remit.json`。
11. **`status.item` 槽的接收方**（需 `apiVersion` 升级）。
12. **复习屏的键入作答**（`{{type:}}`，#141 已关但 UI 未做）：不在本轮视觉重设计里。
13. **插件市场 / 签名 / `net:fetch`**（`plugins/ui.md` §13）。
14. **遥测、云同步、账号**（项目铁律）。

### 10.2 三项口径已冻结（2026-11-10）

1. **默认首屏 = 今日**（§1.2/§4.2）。`internal/ui/library.go` 的 `apply` 打开库后落
   `areaToday`；托盘与提醒不变。
2. **一级导航 = 六个区域 + 插件页分组**（§1.8/§2）。`mode` 保留为派生值，插件 `when`
   的 `mode` 取值不变。
3. **预设 = 库配置 `remit/3`**（§1.9/§5.3），随库走；app 偏好仍留 `settings.json`。

这三项与 §1 的其余条目一样已冻结；实现按本稿执行，不再等确认。
