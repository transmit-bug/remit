# SuperMemo 增量阅读的事实边界

基线：Remit `origin/master` `bad1b77`（2026-11-08）。本文只回答 #202，不做产品决策；
结论以 SuperMemo 官方帮助 / 官方文档为准，仓库现状用于最后一节映射。来源编号见文末。

先给结论：

- **topic / item 是两种学习行为，不是两种文件格式。** topic 用来被动读，item 用来主动回忆；
  extract 默认生成 topic，cloze 生成 item。[S1][S2][S3]
- **priority、A-Factor、interval、forgetting index 不是同一层东西。** priority 是队列里的
  相对重要性；A-Factor 是 topic 的间隔乘数（对 item 则是 SM-15 的难度/增长因子）；interval 是
  两次呈现之间的天数；forgetting index 是 item 在复习时允许遗忘的比例。[S2][S5][S8][S10]
- **read point 是一段“下次从这里继续”的选区，不是滚动位置。** extract、cloze、highlight
  会替用户自动设置它；没有它时，SuperMemo 会把 read point 留在最后一次 extract 或 highlight
  的位置。[S4][S20]
- **Postpone / Dismiss / Done / Mercy / overload 是五种不同的事。** 前三个分别是推迟、
  退出学习、结束文章；Mercy 是成批重排；overload 是“到期材料超过处理能力”的状态，不是按钮。
  [S11][S12][S13][S14][S15]
- **官方“最低定义”把 extract、cloze、read point、priority queue、auto-sort / auto-postpone、
  extract/cloze 层级、导入与引用列为增量阅读的起步条件；A-Factor 文章算法则明确列在“其它选项”。**
  [S16]
- **Remit 已经能映射大部分对象：** Note 扮演 topic，Card 扮演 item，Excerpt 扮演 extract，
  `remit.cloze` 卡片扮演 cloze，Block 级的 Reading Position 近似 read point，FSRS 负责卡片
  间隔。[GLOSSARY.md][docs/adr/0002-content-model.md][docs/specs/incremental-reading.md]
- **真正的缺口是优先级与过载策略、read point 的自动推进/显式操作、以及 Done 的终态语义。**
  A-Factor、Mercy、相对百分位 priority queue 和“Done 删除正文”不建议照搬。

## 1. topic 与 item：先看行为，不看名字

SuperMemo 的 collection 由 element 组成；与增量阅读直接相关的是两类 element：

| 维度 | topic | item |
|---|---|---|
| 学习方式 | 被动阅读 / 观看 / 听；不需要刺激—反应 | 主动回忆；通常有提示与答案 |
| 典型形态 | 文章、章节、段落、句子、图片、视频 | 问答、cloze、拼写、图片辨认等 |
| 目的 | 引入新知识，作为生成 item 的材料 | 让已经学过的知识保持可回忆 |
| 复习节奏 | 按递增 interval 反复呈现；可手动改下次日期 | 由 SuperMemo algorithm 依据评分等决定 |
| 处理终点 | 读完后通常删除或 dismiss | 长期保留并重复 |

来源：SuperMemo Help 的 “Items, topics, concepts, and tasks”。[S2]

因此，**topic / item 不是“笔记 / 卡片”的存储类型，而是 element 在学习过程中的角色**。
同一篇材料可以在处理过程中从 topic 变成多个 item 的来源；topic 读完以后通常不再需要。
[S2][S21][S22]

### extract 生成什么

在增量阅读里，选中一段文字执行 **Remember extract**（`Alt+X`），SuperMemo 会：

1. 把选中的文字放进一个新的 element；
2. 让这个 element 作为原 element 的 child 进入学习过程；
3. 把它当作一篇可继续处理的 mini-article / topic，而不是一张卡片。[S3]

官方 glossary 直接把 extract 定义为“从更大的文章中取出、作为独立 **topic** 复习和处理的
文本片段”。extract 可以嵌套：书 → 章 → 节 → 段 → 句。[S3][S27]

所以，**extract 默认是 topic；不是 item。** 它保留来源、引用和层级，之后还可以继续 extract，
或者在更短的片段上生成 cloze。

### cloze 生成什么

cloze deletion 是 **item**：它把句子中的关键词挖空，形成 fill-in-the-blank 的提示—答案对。
SuperMemo 的说明是：cloze 通常从 topic 中“已经是句子或短段落”的内容生成；执行
**Remember cloze**（`Alt+Z`）会把句子变成具体问题与答案，从被动阅读转入主动回忆。[S1]

因此：

- article / extract → **topic**；
- extract 中的句子 → **cloze item**；
- cloze 是 item，不是另一种 topic。[S1][S2][S26]

## 2. priority、A-Factor、interval、forgetting index

这四个词经常被混在一起，但官方文档把它们放在不同层：

| 概念 | 作用域 | 实际语义 | 是调度还是排序 / 负荷控制 |
|---|---|---|---|
| **priority** | 每个 element | `0%` 最高、`100%` 最低；按相对位置进入 priority queue。决定谁先被处理，以及 overload 时谁先被牺牲 | 排序 / 负荷控制 |
| **A-Factor（topic）** | topic / concept / task | 下一次 topic interval = 当前 interval × A-Factor；越小越频繁。`1.01` 最频繁 | topic 的间隔调度 |
| **A-Factor（item）** | item | SM-15 中的难度/间隔增长因子；SM-17 及以后 item 调度不再把它作为输入，只作为 SM-15 并行背景与诊断保留 | 历史算法 / 诊断 |
| **interval** | topic 与 item | 两次呈现或复习之间的天数；topic 由 A-Factor 乘出来，item 由算法算出 | 调度结果 |
| **forgetting index** | item | 复习时允许遗忘的比例（通常 3%–20%）；决定算法把下一次复习排到多晚。topic 不用它 | item 的调度目标 |

逐项说明：

### priority 是相对队列位置，不是间隔

SuperMemo 的 priority queue 把所有 element 按 priority 排成一个序列。最不重要（`100%`）的
element 在时间不够时最先被牺牲；最重要（`0%`）的先被呈现。priority 还可以用“队列中的
position”表示，`Position=1` 对应 `0%`。这个队列是**相对队列**：插入一个高优先级 element
会改变其它 element 的 percentile 位置。[S5][S25]

默认情况下，学习队列按 priority 从高到低 auto-sort；如果当天做不完，低 priority 材料先受影响。
[S5] 这里要注意：**priority 不是“下次到期日”**。到期日回答“什么时候该看”，priority 回答
“今天先看谁、做不完时牺牲谁”。[S5][S12]

### A-Factor 对 topic 是乘数，对 item 是历史难度参数

对 topic、concept、task，A-Factor 的语义是“下一次 interval 乘多少”：

```text
next topic interval = current interval × A-Factor
```

所以 A-Factor 越大，topic 越不常出现；`1.01` 表示几乎每次都尽快回来。用户可以直接编辑 topic
的 A-Factor；把 priority 调高时，SuperMemo 也会把 topic 的 A-Factor 往 `1.01` 方向压。
[S6][S7][S9][S17]

对 item，A-Factor 来自 SM-15：它反映 item 难度，数值越大通常越容易、interval 增长越快。
但 SuperMemo 17 及以后 item 的调度用新的记忆模型，不再把 A-Factor 当输入；A-Factor 只为
SM-15（并行运行）保留。[S8][S9][S17]

这就是为什么不能把两个 A-Factor 混为一谈：

- **topic 的 A-Factor 是频率旋钮**，直接参与 topic 间隔公式；
- **item 的 A-Factor 是旧算法的难度估计**，在新算法里主要作历史/诊断。

### interval 是结果，不是 priority

interval 是“距离下一次呈现多少天”。topic 的 interval 由 A-Factor 连乘；item 的 interval
由评分、当前记忆状态、elapsed time 和目标遗忘率共同决定。[S2][S8]

手动改 interval 会影响 priority：缩短 interval 通常会把 priority 推高，延后则把 priority
降低。官方帮助明确说“改 interval 会改 priority”，这说明二者相关，但仍是两个量。
[S5]

### forgetting index 是 item 的目标，不是 topic 的参数

forgetting index 是“复习时没想起来”的目标比例。`10%` 大致对应“在复习点 recall 约
90%”；由于间隔期间记忆从约 100% 下降，平均 retention 会更高。[S10]

关键边界：

- **topic 不使用 forgetting index**；Element parameters 明确说 forgetting index 对 topic /
  task 不生效，只会在 topic 生成 item 后，由 item 继承。[S7]
- 它不是一个替代 priority 的队列键；它是算法安排 item interval 的目标。overload 时低 priority
  item 被实际延后，measured forgetting index 会升高，但那是结果，不是队列排序规则。某些
  protection / Mercy 策略会参考 forgetting index，但主队列仍按 priority 组织。[S5][S10][S24]

### 与 FSRS 的关系

SuperMemo 20 Help 把 FSRS 列为可选算法之一（与 SM-2、SM-15、SM-19、SM-20 并列）。
这说明“用 FSRS 做 item 间隔”不是与 SuperMemo 体系不相容的选择；FSRS 的
`desired_retention` 与 forgetting index 是同一类目标（期望在复习点记住多少），只是模型与
参数化不同。[S18][S19]

但 **FSRS 不能自动替代 topic 的 A-Factor 语义**：SuperMemo 的 topic 是“被动读、按乘数拉长
间隔”，没有 item 的评分—难度—稳定性循环。Remit 现在用 FSRS 给阅读笔记排期，是官方
“A-Factor-based article review algorithm **or similar**”允许的改造，但要明确它不是逐字复刻
topic 的 A-Factor。[S16][S17]

## 3. read point：一段可恢复的选区

SuperMemo 的定义：

> read-point 是 SuperMemo 在文章中留下的 recurring selection；每次回到文章，选区会恢复到
> “下一次应该处理的片段”。[S20]

具体操作：

- `Ctrl+F7`：把当前选区设为 read-point；
- `Alt+F7`：跳到 read-point；
- `Shift+Ctrl+F7`：清除 read-point；
- 如果用户不手动设置，extract、cloze、highlight 等操作会自动把 read-point 放到刚处理的
  位置；没有手动设置时，read-point 会留在最后一次 extract 或 highlight 的位置。[S1][S4][S20]

因此 read point 不是“滚动到第几屏”，也不是“笔记打开位置”的完整替代品。它是 **topic 内部的
处理游标**：

- 它记录的是选区，而不是纯时间或 scroll offset；
- 它让中断阅读成为常态：下次打开 topic 时，用户从上次停下的句子继续；
- extract / cloze 既消耗当前选区，也推动 read point 前进；这样“读一点、摘一点、制一张卡”
  可以反复中断和恢复。[S1][S4]

read point 在官方最低定义里是起步条件之一，不是可选装饰。[S16]

## 4. postpone / dismiss / done / mercy / overload

这五个词描述的是不同层次，不能互相替代：

| 名称 | 它是什么 | 对 element 做什么 | 与其它动作的区别 |
|---|---|---|---|
| **Postpone** | 把到期材料推迟 | 只作用于 outstanding（已到期）element；按 delay factor / 上下限增加 interval | 不移出学习过程；不是 Dismiss |
| **Mercy / Spread** | 成批重排 | 把选中的 outstanding element 均匀分配到一段时间；可考虑未来复习、可提前或延后；有 priority / lateness / investment / easiness / recency / random 等排序准则 | 不是单条 postpone；可一次重排很多条 |
| **Dismiss** | 退出学习但保留 | element 仍在 collection 里，可搜索、可重新 Remember；priority 被置为 `100%` 并忽略 | 不删除内容；不是 Done |
| **Done** | 结束文章处理 | 终止 article 的 review；可删除已处理正文；有 child 时提议 dismiss 父文章，无 child 时提议删除 element；已生成的 extract 保留 | 不是普通 Dismiss；可能删除源正文 |
| **overload** | 一种状态 | 到期材料多于当天能处理的数量 | 不是按钮；用 priority queue / auto-postpone / postpone / mercy 处理 |

来源：Postpone、Mercy、Mercy criteria、Dismiss glossary、Learnbar、Element menu、Overload
glossary。[S11][S12][S13][S14][S15][S24]

### Postpone

Postpone 的关键语义是：**只影响已经 outstanding 的 element，把它们整体往后推**。
`Delay factor` 会乘到 interval 上，并有 `Maximum interval` / `Minimum interval` 限制；
它保证被 postpone 的条目至少离开 outstanding 集合。官方还特别警告：每次 Postpone 都偏离
当前设定的 retention 目标。[S12]

Auto-postpone 是每天开始学习前，对“之前几天遗留的 outstanding”自动执行 Postpone；
当天新到期的条目通常不被它推迟。[S12]

### Mercy

Mercy（在 subset 操作里也叫 Spread）不是简单延后，而是**把一批 outstanding 均匀铺到指定
天数 / 每天上限里**。它可以是把积压往后铺，也可以勾选 `Consider future repetitions` 把未来
复习提前；排序可按 priority、lateness、investment、easiness、recency 或 random。
[S11][S24]

它和 Postpone 的边界：

- Postpone：在每条自己的 interval 上做乘法，主要解决“今天到期太多”；
- Mercy：按目标天数重新分配一批条目，主要解决“假期后 / 考试前的整体重排”。[S12][S24]

官方历史页说得很直接：2006 年 priority queue、auto-sort、auto-postpone 出现后，Mercy
不再是解决 overload 的中心工具；它退回到“偶尔随机化 / 假期 / 考试前”的位置。[S11]

### Dismiss 与 Done

Dismiss 是“我不再把这条放进学习过程”，但 element 还在 collection 里，之后可以
`Learning : Remember` 重新加入；dismissed element 的 priority 被忽略并设为 `100%`。[S15]

Done 是“这篇文章读完了”的终态清理：

- 有 child（例如已经 extract 出的内容）时，SuperMemo 提议 Dismiss 父文章；
- 没有 child 时，提议删除整个 element；
- 正文可能被删除，但已经生成的 extract 不会被删。[S1][S13]

所以 **Done 不是 Dismiss 的同义词，也不是“从队列里移除”这么轻**。它包含“源材料处理完毕”
和“清理源正文”的产品决策。

### Overload

overload 不是一条操作记录，而是一个相对状态：outstanding 数量超过当天能处理的数量。
官方给过“每天 200 个 item 已经很难长期维持”的示例数字，但这不是普适阈值；真正的定义是
“超过你的处理能力”。[S14]

SuperMemo 的解法是组合：

1. priority queue 决定先做谁；
2. auto-sort 每天按 priority 重排；
3. auto-postpone 把前几天的低 priority 积压自动推后；
4. 必要时手动 Postpone / Mercy 做一次性调整。[S5][S12][S14]

## 5. 从“读一篇文章”到“形成可复习卡片”

SuperMemo 文档给出的典型链路是：

```text
导入文章（topic）
  → 从顶部或 read point 继续读
  → 选中重要片段，Remember extract（生成 child topic）
  → 在短句/短段落上选中关键词，Remember cloze（生成 item）
  → item 进入间隔复习
  → 源 topic 继续处理，最后 Done / Dismiss
```

每一步的边界：

1. **导入**：文章成为 topic；可以指定 priority 和第一次 interval。[S1][S5]
2. **继续读**：从顶部或上次 read point；中断时设置 read point。[S4]
3. **extract**：`Alt+X` 生成 child topic，保留来源层级，进入学习过程；不是 item。[S3]
4. **cloze**：`Alt+Z` 从短句/关键词生成 item；这是主动回忆的起点。[S1]
5. **复习**：item 按算法复习；topic 按自己的 interval 继续被呈现。[S2]
6. **结束源材料**：Done（清理正文、保留 extract）或 Dismiss（保留 element、退出学习）。
   [S13][S15]

但要注意两个不同的“最小”：

- **生成一张可复习卡片的最小操作链**其实可以是 `topic → cloze item`，extract 不是逻辑上
  必需的中间步；SuperMemo 允许直接从 topic 的短句生成 cloze。[S1]
- **官方仍愿意称之为“incremental reading”的最小闭环**则更宽：extract、cloze、read point、
  priority queue、auto-sort / auto-postpone、extract/cloze 层级都在最低定义里。[S16]

这就是事实边界：**“能制一张卡”与“构成增量阅读”不是同一个门槛。** 官方 3 分钟入门也把
操作压成同一条链：导入文章 → `Alt+X` extract → Learn 中 `Alt+Z` cloze。[S23]

## 6. 什么是核心，什么是产品历史 / 偶然

官方 “Minimum definition of incremental reading” 把功能分成三层：[S16]

| 官方层级 | 内容 | 判断 |
|---|---|---|
| First Steps | spaced repetition；extracts；cloze deletions；read points；priority queue；repetition auto-sort / auto-postpone；extract 与 cloze 的层级；网页导入；WYSIWYG；图片传播 | 这是 SuperMemo 官方给出的“增量阅读起步条件” |
| Vital | mid-interval review 与 spacing correction；early review；semantic review；overload tools（Mercy / Postpone 或类似物）；references；图片本地化 | 重要能力，但具体工具可替换 |
| Other | A-Factor-based article review algorithm（或类似物）；processing attributes；visual learning；批量导入；自动拆分；视频/音频/邮件；统计 | 产品扩展或实现选择，不是最低定义 |

据此可以把常见概念分成：

### 核心语义

- topic / item 的行为区分；[S2]
- extract 作为可继续处理的 child topic；[S3]
- cloze 作为 item；[S1]
- read point 与可中断、可恢复的阅读；[S4][S20]
- 优先级队列与 overload 处理；[S5][S14][S16]
- extract/cloze 与来源的层级 / reference；[S3][S16]
- 导入、WYSIWYG、图片随文传播等阅读载体的最低能力。[S16]

### 可替换的实现

- 用 A-Factor 还是别的 topic 节奏算法（官方明说 “or similar”）；[S16][S17]
- 用 SuperMemo algorithm、FSRS 还是别的 item 调度器（SuperMemo 20 自己也并列列出多种）；[S18][S19]
- 用 relative percentile priority queue 还是稳定的 priority band；
- 用 Mercy 还是其它 overload 重排工具；官方把 “Mercy / Postpone or similar” 放在 Vital，
  但历史页又说 2006 年后 Mercy 已非中心。[S11][S16]
- 用 HTML element / knowledge tree 还是 Markdown note + block + link。

### 产品历史 / 偶然

- A-Factor 作为 item 难度参数属于 SM-15 时代；SM-17+ 的 item 调度不再用它。[S8][S9]
- “A-Factor = 1.01 就等于最高优先级”是旧版拥挤问题的表现；2006 年的相对 priority queue
  正是为了把 priority 与 A-Factor 分开。[S5]
- Mercy 诞生于 1992，早于增量阅读；2006 年 priority queue 后被边缘化。[S11]
- Done 的“删除正文”是 SuperMemo 对 collection 体积和搜索死链的产品取舍，不是增量阅读的
  概念必要条件。[S13]
- concepts / tasks 是 SuperMemo 的其它 element 类型，不是 topic/item 阅读链的必需部分。[S2]

## 7. 映射到 Remit

### 7.1 对象映射

| SuperMemo | Remit 现状 | 贴合度 |
|---|---|---|
| topic | **Note**：Reading Queue 的成员是整篇 Markdown 笔记 | 语义接近；SuperMemo 的 topic 可以短到一句，Remit 的 Note 是整个文档 |
| item | **Card**：引用一个 Block 的主动回忆实体 | 直接对应；一个 Block 可以派生多张 Card |
| extract | **Excerpt**：从当前 Block 取出文本，成为子 Note 的首块，并写入指回原 Block 的 Reference | 很接近；SuperMemo 可跨选区，Remit 以 Block 为粒度 |
| cloze | **`remit.cloze` Card Type**：按空位 ordinal 生成卡片 | 直接对应 |
| topic 的 read point | **Reading Position**：Block ID；打开时回到上次会话停下的块 | 近似；缺少“选区级 read point”和自动推进 / 显式 set / go / clear |
| item 的 interval / forgetting index | **FSRS**：`desired_retention` 与卡片重放 | 目标同类；算法不同，不能声称等同 SM 算法 |
| topic 的 A-Factor | Remit 没有；阅读用独立 FSRS 参数与三档消化难度 | 可以替代，但不是 A-Factor 语义 |
| priority queue | Remit 没有 priority；Reading Queue 按 due / 入队先后 | 缺 priority 与 overload 策略 |
| Dismiss | 阅读侧 **Drop**；卡片侧 **Suspend** | 都是“退出队列、保留历史” |
| Done | Remit 没有终态 “finished / archived” | 缺；且不应照搬删除正文 |
| Postpone | Remit 的 **Skip** 只是本次会话跳过，不写日志；**Drop** 是移出队列 | 两者都不是 Postpone |
| Mercy | 没有 | 可作为历史工具不采纳 |
| overload | 没有显式策略 | 需要时再加 |

Remit 现状来源：[GLOSSARY.md]、[docs/adr/0002-content-model.md]、
[docs/adr/0003-storage-layout.md]、[docs/specs/incremental-reading.md]、`internal/core/queue/queue.go`、
`internal/core/schedule/schedule.go`、`internal/core/excerpt.go`、`internal/ui/screens/reading/state.go`。

### 7.2 关键差异

1. **SuperMemo 的 topic 是 element，Remit 的 topic 角色由 Note 扮演。**
   SuperMemo 可以把一篇文章 split 成多个 topic 并分别排期；Remit 的阅读队列成员是整篇 Note，
   只能通过 Excerpt 生成新的子 Note 来细分。官方最低定义没有把 split article 列为核心，
   所以这是可接受的改造，但“整篇 Note = 一个 topic”比 SuperMemo 粗。[S2][S16]

2. **Remit 的 extract 是 Block 级，SuperMemo 的 extract 是文本选区级。**
   Remit 的 Block 是引用单位；Excerpt 取当前 Block 的文本，子 Note 首块里带
   `[[标题#^块 ID]]` 引用。[docs/specs/incremental-reading.md][internal/core/excerpt.go]
   这保住了“extract 是 child topic + 可追溯来源”的核心，但放弃了同一 Block 内的任意字符区间。

3. **Remit 的 read point 现在只是 Block ID，不是选区。**
   Reading Position 只在评分时落库；移动位置或执行摘录不写日志；摘录后阅读位置不动。
   [docs/specs/incremental-reading.md][internal/ui/screens/reading/state.go]
   SuperMemo 则会在 extract / cloze / highlight 后自动设置 read point。这是最大的一处语义缺口。

4. **Remit 把阅读与复习分成两个队列，SuperMemo 把 topic 与 item 混在同一次学习流程中。**
   SuperMemo 的 “Learn” 会交错呈现文章与卡片，并用 topic 比例 / randomization 控制节奏；
   Remit 是“复习 | 阅读”两个模式。[S1][S5][docs/specs/incremental-reading.md]
   官方最低定义没有逐字要求一个统一 UI，但要求 priority queue、auto-sort / auto-postpone
   与 overload 能力；Remit 目前两者都还没有，所以不能仅凭“有阅读队列”就声称满足该定义。

5. **Remit 的 FSRS 阅读节奏是“类似物”，不是 A-Factor。**
   Remit 用三档消化难度映射 Again / Good / Easy，并用独立 FSRS 参数排期；
   [docs/specs/incremental-reading.md] 明确说阅读参数与卡片参数独立、调度插件不管阅读。
   这符合官方 “or similar” 的口径，但不要把 `desired_retention` 说成 topic 的
   forgetting index，也不要把 FSRS 的 interval 说成 A-Factor 乘出来的 interval。[S10][S16][S17]

### 7.3 可迁移 / 需改造 / 不采纳

| 机制 | 判断 | 在 Remit 的落法 / 理由 |
|---|---|---|
| topic 被动读、item 主动回忆 | **可迁移** | Note 扮演 topic，Card 扮演 item；不需要新增 Topic / Item 实体 |
| extract 生成 child topic | **可迁移** | 现有 Excerpt 子 Note + Reference 已经很接近 |
| cloze 生成 item | **可迁移** | 现有 `remit.cloze` Card Type；一个 Block 多张卡 |
| extract / cloze 与来源的层级 | **可迁移** | Block ID + link + 反链；不需要 SuperMemo knowledge tree |
| 阅读 position / read point | **需改造** | 现在只有 Block 级、只在评分时保存；若要更接近 SuperMemo，应加“下一个未处理 Block”语义，或显式 set / go / clear，并决定 extract 后是否推进 |
| item 的 forgetting index | **可迁移** | 映射为 FSRS `desired_retention`；只作用于 Card，不作用于 Note |
| item 的 FSRS 间隔 | **可迁移** | 已经是 Card 调度器；SuperMemo 20 也把 FSRS 列为算法之一 |
| topic 的 A-Factor | **不采纳** | 官方最低定义只要求 “or similar”；Remit 已有独立 FSRS 阅读节奏，A-Factor 只会增加第二套心智模型 |
| 相对百分位 priority queue | **不采纳** | 它是 SuperMemo 的 collection 级相对队列实现；Remit 更适合稳定的 priority band / 显式排序，便于明文日志重放 |
| 单条 priority | **需改造** | 需要一个 Note 级（或 Note + Card 级）的 priority 概念；这是 Remit 目前没有的新概念 |
| auto-sort / auto-postpone | **需改造** | 若要认真对齐官方最低定义，需要“到期 + priority + 每日上限 + 自动推迟”的策略；当前只有 due/FIFO |
| Postpone | **需改造** | 需要显式的“推迟到某日 / 今天不做”事件；现有 Skip 不落日志，Drop 是移出队列，语义不同 |
| Mercy / 批量重排 | **不采纳** | 历史工具；官方自己说 2006 年后已非 overload 中心。若将来有真实积压，再做一次性 rebalance 即可 |
| Dismiss | **可迁移** | 阅读侧 Drop、卡片侧 Suspend；保留 Note / Card 历史，可重新加入 |
| Done | **需改造** | 需要一个“文章处理完毕”的终态（finished / archived），但不要照搬删除正文；与 Drop 区分开 |
| overload | **需改造** | overload 不是按钮；需要 priority + 每日上限 + auto-postpone 的组合策略，先有可观测的“今天做不完”状态 |
| 统一 topic/item 学习队列 | **不采纳（字面形态）** | Remit 的“复习 / 阅读”双态可以保留；但若声称满足官方最低定义，需要补跨队列的 priority / overload 等价物 |
| 子 Block 字符区间 extract | **不采纳（当前模型）** | Block 是引用单位；跨块 / 块内区间需要新的 range reference，不是当前 Excerpt 的自然扩展 |
| Done 自动删除源正文 | **不采纳** | 与 Remit 的明文库、可追溯与不丢数据取向冲突；终态应可逆地隐藏/归档，而不是删除 |
| concepts / tasks / SuperMemo element tree | **不采纳** | 不是 topic/item 阅读链的必要部分；Remit 用 Note + tag + link 表达组织 |

## 8. 对 Remit 的最小结论

如果 Remit 只想做“按节奏读笔记”，当前 Reading Queue 已经足够；它不需要 A-Factor、Mercy
或 SuperMemo 的 element tree。

如果 Remit 要声称“增量阅读”，按官方最低定义，至少还缺三件事：

1. **一个可解释的 priority 概念**（Note 级即可，先不必做相对百分位）；
2. **overload 策略**：每日阅读上限、到期与 priority 的排序、自动/手动推迟；
3. **read point 的显式语义**：set / go / clear 或“下一个未处理 Block”，并说明 excerpt 后
   是否推进。

最小的新概念建议只有两个：

- `ReadingPriority`：Note（或 Reading Queue entry）上的稳定优先级，不复刻相对 percentile；
- `Finished` / `Archived`：文章处理完毕的终态，与 `Dropped`（暂时移出）分开。

其余都可以用现有 Note / Block / Card / Excerpt / FSRS / Reading Queue 表达。

## 9. 未决问题

1. Remit 要把自己称为 “incremental reading”，还是更保守地称为 “spaced reading queue”？
   前者按官方最低定义需要 priority / overload / read point，后者不需要。
2. priority 只放在 Note 上，还是 Note 与 Card 都要？SuperMemo 的 priority 是每个 element
   都有的；Remit 若只给阅读笔记排序，就不是完整的 cross-element priority queue。[S5]
3. 是否接受 Block 级 read point / extract？如果接受，应在词汇表里明确这是“块级增量阅读”，
   不要暗示支持 SuperMemo 的任意选区。
4. “复习 | 阅读”双态是否长期保留？SuperMemo 的 topic/item 交错是实践的一部分；Remit 若
   保留双态，需要另一套机制保证阅读不会挤掉高优先级卡片。
5. Done 的终态是否允许删除源 Note？本文建议不删除；若一定要删，应先有可恢复的归档/回收站，
   不能把 Done 直接映射为不可逆删除。

## 来源

所有来源均为 SuperMemo 官方帮助或 SuperMemo Guru 第一方文档；访问日期 2026-11-08。
未公开的内部权重与公式不在本文推断范围内。

| 编号 | 来源 |
|---|---|
| S1 | [SuperMemo Help: Incremental reading](https://help.supermemo.org/wiki/Incremental_reading) |
| S2 | [SuperMemo Help: Items, topics, concepts, and tasks](https://help.supermemo.org/wiki/Items,_topics,_concepts,_and_tasks) |
| S3 | [SuperMemo Help: Extract](https://help.supermemo.org/wiki/Extract) |
| S4 | [SuperMemo Help: Read toolbar](https://help.supermemo.org/wiki/Read_toolbar) |
| S5 | [SuperMemo Help: Priority queue](https://help.supermemo.org/wiki/Priority_queue) |
| S6 | [SuperMemo Help: Element menu](https://help.supermemo.org/wiki/Element_menu) |
| S7 | [SuperMemo Help: Element parameters](https://help.supermemo.org/wiki/Element_parameters) |
| S8 | [SuperMemo Help: Element data](https://help.supermemo.org/wiki/Element_data) |
| S9 | [SuperMemo Help: Glossary: A-Factor](https://help.supermemo.org/wiki/Glossary:A-Factor) |
| S10 | [SuperMemo Guru: Forgetting index in SuperMemo](https://supermemo.guru/wiki/Forgetting_index_in_SuperMemo) |
| S11 | [SuperMemo Help: Mercy](https://help.supermemo.org/wiki/Mercy) |
| S12 | [SuperMemo Help: Postpone](https://help.supermemo.org/wiki/Postpone) |
| S13 | [SuperMemo Help: Learnbar](https://help.supermemo.org/wiki/Learnbar) |
| S14 | [SuperMemo Help: Glossary: Overload](https://help.supermemo.org/wiki/Glossary:Overload) |
| S15 | [SuperMemo Help: Glossary: Dismiss](https://help.supermemo.org/wiki/Glossary:Dismiss) |
| S16 | [SuperMemo Guru: Minimum definition of incremental reading](https://supermemo.guru/wiki/Minimum_definition_of_incremental_reading) |
| S17 | [SuperMemo Guru: A-Factor](https://supermemo.guru/wiki/A-Factor) |
| S18 | [SuperMemo Help: SuperMemo Algorithm](https://help.supermemo.org/wiki/SuperMemo_Algorithm) |
| S19 | [SuperMemo Help: Algorithm SM-20](https://help.supermemo.org/wiki/Algorithm_SM-20) |
| S20 | [SuperMemo Help: Glossary: Read-point](https://help.supermemo.org/wiki/Glossary:Read-point) |
| S21 | [SuperMemo Guru: Topic](https://supermemo.guru/wiki/Topic) |
| S22 | [SuperMemo Guru: Item](https://supermemo.guru/wiki/Item) |
| S23 | [SuperMemo Guru: Incremental reading step by step](https://supermemo.guru/wiki/Incremental_reading_step_by_step) |
| S24 | [SuperMemo Help: Mercy criteria](https://help.supermemo.org/wiki/Mercy_criteria) |
| S25 | [SuperMemo Help: Glossary: Priority](https://help.supermemo.org/wiki/Glossary:Priority) |
| S26 | [SuperMemo Help: Glossary: Cloze deletion](https://help.supermemo.org/wiki/Glossary:Cloze_deletion) |
| S27 | [SuperMemo Help: Glossary: Extract](https://help.supermemo.org/wiki/Glossary:Extract) |
