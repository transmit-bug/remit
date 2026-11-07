# 增量阅读 · 阅读队列、位置与摘录制卡

笔记写完或导入后，长文只有两种命运：一次读完（然后遗忘），或永远躺在库里。复习队列让
卡片按节奏回来，笔记本身却没有节奏——Remember it 缺了「把读过的长文按节奏消化」这条
腿。读的时候想留住的重点，要么当场读完就忘，要么手工拆卡打断阅读；笔记与闪卡之间没有桥。

本稿把这条腿补上：任意笔记可加入阅读队列，阅读会话按 FSRS 节奏呈现下一篇待读并回到上次
读到的块，阅读中摘录的段落直接成为携带引用的子笔记（可立即按既有 Card Type 制卡），队列、
统计、托盘与每日提醒把「待读」与「待复习」并列报告。

> **落地口径（#157，2026-11-08）**：本稿从 #156 的冻结契约搬来，按实现校正。标 **计划**
> 的是尚未建的部分，其余按今天的行为写；落点见 §12，与冻结稿的差异见 §16。

## 1. 原则

| 原则 | 出处 | 我们怎么落 |
|---|---|---|
| **阅读条目是笔记，不是卡片** | ADR 0002 给 `reading` 卡片状态留的位是「将来的阅读型卡片」 | 队列的成员是笔记本身；卡片状态机不动，`StatusReading` 仍是预留位 |
| **一份调度史** | ADR 0003：`reviews.jsonl` 是唯一调度来源 | 阅读事件与卡片事件追加进同一个日志，带条目种类；重放推导到期与位置，索引只缓存结果 |
| **节奏复用 FSRS** | ADR 0003 的重放原则 | 笔记条目走既有 `Step` / `Replay` / `Applier`，配宿主自己的参数默认值；调度插件 v1 仍只作用于卡片 |
| **摘录是子笔记** | ADR 0008：制卡只有一条路 | 选区成为子笔记的首块，块内携带指回原块的 Reference 链接；制卡走既有类型与模板 |
| **双态不分裂** | ADR 0004/0009：屏是一个包 | 复习态顶部「复习 \| 阅读」分段；阅读会话是 `internal/ui/screens/reading`，写回经 `Deps` 到 App |

## 2. 词

| 词 | 含义 | 不用 |
|---|---|---|
| **Reading Queue**（阅读队列） | 今日待读的笔记序列，按 FSRS 到期先进先出，与复习队列并列 | 待读列表、to-read |
| **Reading Position**（阅读位置） | 一次阅读会话停下的块；打开笔记时回到这里 | 进度条、滚动位置 |
| **Excerpt**（摘录） | 从某个块取出的文本 + 指回该块的引用，成为一个新笔记的首块 | 高亮、剪藏 |
| **Digestion**（消化难度） | 一次阅读的三档主观难度：太难 / 刚好 / 太浅，映射到 FSRS 的 Again / Good / Easy | 评分（那是卡片侧） |

## 3. 数据：一条调度史

阅读事件是 `reviews.jsonl` 的兼容追加（`remit/2` 不变），与卡片事件同形：

```jsonl
{"seq":41,"note":"note_…","kind":"queued","at":"2026-11-08T09:12:00Z"}
{"seq":42,"note":"note_…","rating":"good","position":"block_…","at":"2026-11-08T09:31:07Z","scheduler":"remit.builtin/1"}
{"seq":43,"note":"note_…","kind":"dropped","at":"2026-11-09T20:02:11Z"}
```

- `kind` 缺省是 `reviewed`：一条评分事件。`queued` / `dropped` 只改队列成员，没有 rating。
- 评分事件带 `card` 的是卡片；带 `note` 的是阅读会话。两者都不带、或都带，是坏行。
- `position` 只在阅读会话上有意义：本次读到哪个块。
- `void` 仍只撤销一条评分事件（撤销键是卡片侧的），不撤销成员变更。
- 日志只手写可改：坏行让打开失败，末行缺换行视为写了一半，与卡片事件同一套规则。

## 4. 重放

`internal/core/schedule` 的重放同时答卡片与笔记：

- **到期**：`Step` / `Applier` 不变，笔记条目与卡片条目共用一套 FSRS 数学。
- **参数**：阅读用 `DefaultReadingParams()`——发布默认值，与库里的卡片参数、与调度插件
  的策略相互独立。库的 FSRS 权重被插件改过，阅读节奏也不跟着变。
- **位置**：最新一条带 `position` 的存活评分事件给出位置；撤销那条会话会把位置退回前一条。
- **成员**：`queued` 置入、`dropped` 取出、阅读会话隐式入队（读过的笔记是要回来记的）。
  撤销一次会话不改变成员——撤销拿回的是答案，不是阅读。
- **未读条目的到期**：入队那一刻即到期，所以队列按入队先后先进先出；读过之后按 FSRS 到期。

索引的 `reading` 表缓存这些结果（note、stability、difficulty、due、last_review、reviews、
last_rating、last_seq、position、queued），随 `.remit/` 一起可删可重建。库级测试证明：写入、
重开、删索引重建三者答案一致（§14 缝 1）。

## 5. 队列

`internal/core/queue`：

```go
Reading(lib, now, Options) ([]store.Reading, error)   // 今日待读，最早到期在前，同刻按 ID
ReadingCount(lib, now) (int, error)                    // 托盘与复习入口报的数
```

- 只有到期的条目在队列里；没到的留在库的队列里、不在今天。
- 把读过的笔记再放回队列，节奏不变：到期日已过就立刻回来，没到就等它到。加入队列是「记得
  回来读」，不是重排节奏。
- `Options.Limit` 截断，与卡片队列同一个含义。
- 顺序：`Due` 升序，同刻按笔记 ID，所以两次调用给出同一个顺序（缝 2）。

## 6. 摘录

选区的粒度是**块**：块是笔记的引用单位（ADR 0002），跨块的字符区间不属于 v1 的模型。
读态里「当前块」就是选区，`J`/`K` 或点击换块，`E` 摘录。

`core.Excerpt(source, block, text)` 生成子笔记的首块：

```
Spacing beats cramming :: reviewing just before you forget strengthens recall.
[[Memory and recall#^block_00000000000000000000000000000011]]
```

- 第二行是 Reference：`[[笔记标题#^块 ID]]`。`core.Links` 把它读成 `Link{Title, Block}`，
  于是源笔记的反链面板里出现这条摘录（故事 14），读态也能从摘录一键打开原文（故事 5）。
- 引用写在同一个块里：块是引用单位，引用跟着段落走，不随块顺序变化。
- 正文不做引用装饰（不写 `> `）：摘录常常直接制卡，卡面不该以引用符号开头。
- 块里不能有空行（那是块的分界），所以引用另起一行而不是隔一段。

`Library.Excerpt(noteID, blockID, text, title)` 写子笔记并返回它；制卡走既有路径
（`deriveBlock` 的产物 + `CreateCards`），没有第二套制卡实现。金标准把三种选区与
basic / cloze 卡面逐字冻结（缝 5）。

## 7. 阅读会话屏

`internal/ui/screens/reading`，ADR 0004/0009 的 `State` / `Deps` / `View`：

| 部分 | 内容 |
|---|---|
| `State` | 会话的队列顺序、当前笔记的阅读位置（块 ID）、本会话已读完的笔记（含跳过的，见下）、摘录是否同时制卡 |
| `Deps` | `Note`（块与上次位置）、`Read`（难度 + 位置）、`Skip`、`Drop`、`Excerpt`、`Source`/`Open`（回到原文）、`EditNote`、`Review`、`Home` |
| `View` | 状态行（第几篇 / 第几段 / 键位）、笔记卡（标题 + 逐段正文，当前段落高亮并带走位焦点）、三档难度、动作行、原文入口 |

- 屏不读 store、不读时钟、不 import `internal/ui` 或兄弟屏；每次写回 `Deps`，App 在 UI 线程
  写库并 reload。
- `State.Sync(due)` 与复习屏同形：成员是 store 的，顺序是会话的。**跳过的笔记本会话不再回来**
  （它仍到期，若不留痕就会被 `Sync` 追加回来），换会话自然重来。
- 阅读位置：`State.pos` 是读者移动到的块；没移动过就用笔记上次停下的块，都没有就用首块。
- 摘录后焦点回到当前段落（故事 22），位置不动：摘录是旁注，不是翻页。
- 720×520 能完整走完一次会话：内容区滚动，动作行换行（故事 20）。

### 键位

| 键 | 动作 |
|---|---|
| `J` / `K`、`↓` / `↑` | 下/上一个段落（移动阅读位置） |
| `1` / `2` / `3` | 太难 / 刚好 / 太浅，记下这次阅读并前进 |
| `E` | 摘录当前段落（制卡与否看勾选） |
| `S` | 跳过：前进，不产生任何记录 |
| `X` | 移出阅读队列 |
| `M` | 在笔记本里打开这篇笔记 |
| `Esc` | 回到笔记本 |

## 8. 双态入口

复习与阅读是同一个入口的两面：窗口顶部一条分段「复习 | 阅读」，右侧常驻「N 篇待读」。
两个队列都占满窗口（不显示工具栏与笔记列表），分段是唯一的导航。阅读队列为 0 时仍然显示
（`Nothing to read`）。命令面板另有「阅读队列」与「加入阅读队列」两条命令，后者作用于笔记本
里打开的笔记——任意笔记都能进队列，入口就在用户已经在的地方。

## 9. agent API

只读暴露，加入队列是写：

| 路由 | 说明 |
|---|---|
| `GET /v1/reading` | 阅读队列：笔记、标题、位置、到期、上次评分与次数 |
| `GET /v1/reading/{id}` | 一条条目 + 它的事件（queued / reviewed / dropped，按序） |
| `POST /v1/reading` | `{"note": "note_…", "at"?: RFC 3339}`，需 bearer token |
| `DELETE /v1/reading` | 405：移出与写位置是窗口的 |

`GET /v1/library` 的计数加 `reading`（今日待读）。API 仍不暴露删除，也不接受位置写入：
位置只由读者自己的会话写下。

## 10. 插件

- 没有新钩子：`deriveBlock` / `policy` / `summary` / `on_event` 契约不变，阅读不向插件开面。
- 阅读事件广播进既有 `on_event`：`reading.recorded`、`reading.queued`、`reading.dropped`，
  载荷仍是 `{kind, id}`（id 是笔记 ID），报告类插件据此统计阅读行为。
- 调度插件（`review.schedule` 角色）v1 只作用于卡片；阅读节奏是宿主能力（§4）。

## 11. 统计、托盘与提醒

- **热力图**：阅读会话与复习共用一份存活日志，`ReviewsPerDay` 按同一口径计数（故事 15）。
- **预测**：`stats.ForecastAll` 把卡片到期与阅读队列到期并成一条序列，统计屏的三块一起看
  全部记忆活动。复习结束的「明日到期」仍是卡片的数（它回答的是复习会话自己的问题）。
- **托盘**：标题在有待读时变成 `Remit · 12 · 3`（待复习 · 待读），没有待读时保持
  `Remit · 12`；提示条两个数都报。
- **每日提醒**：`ReminderBody(cards, reading)` 一句里报两个数，任一不为零才提醒。

## 12. 落点

| 文件 | 内容 |
|---|---|
| `internal/core/schedule/schedule.go` | `Kind`、`Event.Note/Position/Kind`、`Applier.Reading()`、`NewApplierFor`、`DefaultReadingParams` |
| `internal/core/store/log.go` | 日志行的编码 / 解码（`kind`、`note`、`position`） |
| `internal/core/store/reading.go` | `Reading`、`ReadingQueue`、`ReadingFor`、`ReadingPosition`、`AddToReading`、`RemoveFromReading`、`ReadNote` |
| `internal/core/store/index.go` | `reading` 表（schema 9） |
| `internal/core/store/library.go` | `Excerpt`、重建与尾重放、阅读事件 |
| `internal/core/queue/queue.go` | `Reading`、`ReadingCount` |
| `internal/core/stats/stats.go` | `ForecastAll`、`ReadingForecast` |
| `internal/core/excerpt.go` | `Excerpt`（块的文本形状） |
| `internal/core/links.go` | 链接锚点：`[[标题#^块]]` |
| `internal/ui/screens/reading/` | 阅读会话屏 |
| `internal/ui/reading.go` | `Deps` 的实现、摘录、`loadReading` |
| `internal/ui/view.go` | `queueView`：复习 \| 阅读 |
| `internal/ui/window.go` | 读态键位 |
| `internal/agentapi/server.go` | 三条路由、`readingJSON` |
| `internal/golden/golden.go` | `Excerpts` 段 |
| `cmd/remit/` | 托盘标题 / 菜单项、提醒 |

## 13. 用户故事与验收

| # | 故事 | 落地 | 测试 |
|---|---|---|---|
| 1 | 任意笔记加入阅读队列 | `AddToReading`；命令面板「加入阅读队列」 | `TestPaletteCommandsForReading`、`TestReadingQueueIsMembership` |
| 2 | 按记忆节奏排出下一篇 | `queue.Reading` + FSRS 重放 | `TestReadingQueueOrder`、`TestReadingQueueLeavesWhatIsNotDue` |
| 3 | 回到上次读到的块 | `ReadingFor.Position`；屏从 `Note.Position` 起 | `TestReadingRoundTrip`、`TestResumeAtTheStoredPosition`、`TestReadingResumesWhereItStopped` |
| 4 | 划选即摘录 | 读态当前块 + `Library.Excerpt` | `TestExcerptFromTheReadingScreen`、`TestExcerptLeavesTheReaderInPlace` |
| 5 | 摘录携带指向原块的引用 | `[[标题#^块]]` + 读态「打开原文」 | `TestExcerptMakesAChildNote`、`TestReferenceOpensTheSource` |
| 6 | 仅摘录 / 摘录并制卡 | 勾选框 + `Deps.Excerpt(…, withCard)` | `TestExcerptLeavesTheReaderInPlace` |
| 7 | 按类型制卡（basic / cloze） | 既有 `deriveBlock` + `CreateCards` | 金标准 `excerpts`、`TestExcerptFromTheReadingScreen` |
| 8 | 标消化难度 | 三档 → Again / Good / Easy | `TestReadingDifficultyRecordsThePosition`、`TestReadingDifficultyMovesTheDueDate` |
| 9 | 跳过不产生失败记录 | `Advance(false)`，无日志 | `TestReadingSkipAndRemove` |
| 10 | 移出阅读队列 | `dropped` 事件 | `TestReadingQueueIsMembership`、`TestReadingSkipAndRemove` |
| 11 | 复习 \| 阅读切换 | `queueView` 分段 | `TestReadingQueueOnTheReviewScreen` |
| 12 | 入口看到「N 篇待读」 | 分段行常驻计数 | `TestReadingQueueOnTheReviewScreen` |
| 13 | 一键跳到编辑态 | `Deps.EditNote` / `M` | `TestActionsReachTheApp` |
| 14 | 摘录出现在原文反链 | 链接索引（标题匹配） | `TestExcerptMakesAChildNote`、`TestExcerptFromTheReadingScreen` |
| 15 | 阅读量并入热力图与预测 | 同一份日志 + `ForecastAll` | `TestReadingJoinsTheHeatmapAndForecast`（`internal/core/stats`） |
| 16 | 托盘与提醒报两个数 | `OnCounts`、`TrayTitleReading`、`ReminderBody` | `TestReadingCountsReachAService`、`TestReminderOncePerDay`、`TestTrayMenu` |
| 17 | 摘录笔记也能进队列 | 子笔记就是笔记 | `TestExcerptFromTheReadingScreen` + 故事 1 |
| 18 | 拷库到另一台机器位置与节奏都在 | 明文日志 + 重放 | `TestReadingRoundTrip` |
| 19 | 删派生索引一切照旧 | `Rebuild()` | `TestReadingRoundTrip` |
| 20 | 720×520 走完会话 | 滚动 + 换行动作 | `TestNoteOnScreen`、`TestChineseAtTheMinimumSize` |
| 21 | 全键盘完成 | 读态键位表 | `TestReadingSessionWritesTheLog`（J 与数字键） |
| 22 | 摘录后焦点回到位置附近 | `State.Excerpt` 取回焦点 | `TestExcerptLeavesTheReaderInPlace` |
| 23 | 随语言设置显示中文 | 两份目录 | `TestChineseAtTheMinimumSize` |
| 24 | 插件收到阅读事件 | `on_event` 三个 kind | `TestReadingEventsReachTheHandler` |
| 25 | 插件不为阅读学新钩子 | 无新钩子 | §10 |
| 26 | 复用同一套 FSRS | `Step` / `Applier` | `TestReplayReading`、金标准 |
| 27 | 帧不碰 store、帧内不调插件 | `Deps` + `loadReading` | 读态屏测试（无 store） |
| 28 | 阅读与卡片事件同一条日志可分辨 | `kind` + `note` / `card` | `TestReadingRoundTrip`（明文断言） |

## 14. 测试缝

全部复用既有缝，新增为零：

1. **store 库级 round-trip**（最高缝）：`internal/core/store/reading_test.go`——写入、重开、
   删索引重建后到期与位置不丢；日志明文可读。
2. **core/queue**：`internal/core/queue/queue_test.go`——计数与顺序、限额、未到期过滤。
3. **schedule Replay 表驱动**：`internal/core/schedule/schedule_test.go`——入队 / 阅读 / 移出、
   未读条目的到期、宿主参数独立、撤销退回位置。
4. **ui.NewTester 双尺寸**：`internal/ui/screens/reading/reading_test.go`（1024×720 与 720×520，
   外加一次 zh-Hans 最小窗）与 `internal/ui/reading_screen_test.go`（装配：队列、写日志、摘录、
   托盘计数、命令面板）。
5. **internal/golden**：`internal/golden/testdata/golden.json` 的 `excerpts` 段——摘录 →
   子笔记块 → basic / cloze 卡面逐字比对。

## 15. 不在范围

- SuperMemo 式阅读优先级排序；v1 按 FSRS 到期先进先出。
- 插件自定义阅读调度策略或新增阅读钩子。
- 网页 / EPUB 抓取进阅读队列（importer 钩子 #153 的地盘）。
- agent API 的移出队列与写位置。
- 卡片 `reading` 状态的调度语义（ADR 0002 的预留位不动）。
- 专门的阅读进度统计视图（并入统计屏既有三块）。
- 跨块的字符区间选区（见 §16）。

## 16. 与 #156 冻结稿的差异

1. **选区粒度是块**（§6）：冻结稿写「划选」。块是笔记的引用单位，跨块字符区间会把引用变成
   偏移量，与 ADR 0002 冲突；v1 用块选区，读态里当前块即选区。
2. **引用链接的语法**：`[[标题#^块]]` 是本次新增的链接锚点（`core.Link.Block`）。冻结稿只说
   「指向原文块的 Reference 链接」，落地时选了与既有 wiki 链接同族、且标题仍可做反链匹配的
   一种写法。
3. **三档消化难度**去掉 `Hard`：太难 / 刚好 / 太浅 → Again / Good / Easy。四档里「困难」与
   「刚好」在阅读上没有可分辨的含义，卡片侧的四档不变。
4. **成员变更是显式事件**（`queued` / `dropped`）：冻结稿只说「加入 / 移出队列」。用事件而
   不是配置列表，是因为库要能拷走、要能重放、要能手改。
5. **位置只在会话记录时落库**：移动阅读位置本身不写日志（未读的移动不留痕），「位置记忆」
   的口径是「上次会话停下的块」。
6. **阅读参数是宿主默认值**（§4）：冻结稿说「配独立的参数默认值」，落地时不与库的卡片参数
   联动，也不读取调度插件的策略。
