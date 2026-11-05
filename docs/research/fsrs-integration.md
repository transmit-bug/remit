# FSRS 集成事实（工单 #6）

> slug: `fsrs-integration` · 分支 `research/fsrs-integration` · 调研日期 2026-11-06。
> 前提：地图已定决策调度算法 = FSRS、开源本地优先、GUI 主体 mygo、不造轮子。本文只收集集成事实，不重开选型。

## TL;DR

**结论：有合格的官方 Go 实现，直接 `go get github.com/open-spaced-repetition/go-fsrs/v4` 即可，无需移植/绑定。** 它是 FSRS 作者组织的官方仓库，MIT 许可（与 MIT/Apache-2.0/AGPL-3.0 拟定 license 全部兼容），活跃维护（最近 push 2026-11-04），v4.0.0（2026-07-25）实现 **FSRS v6**（21 参数权重），带完整 Go API（`Repeat`/`Next`/`MemoryState`/`Reschedule`/`Rollback`）。**唯一缺口是 optimizer（每用户参数训练）尚未发布**（README 标注 in development，`optimizer` 分支尚无 optimizer 包代码）；短期内 optimizer 路径建议：等 go-fsrs/optimizer 落地，或可选地将复习日志喂给 fsrs-rs（Rust）/ py-fsrs（CLI/子进程）训练，再把 21 个权重写回本地调度器。

## 1. Go 实现候选对比

| | [go-fsrs](https://github.com/open-spaced-repetition/go-fsrs)（官方） | 自行移植 / cgo 绑定 fsrs-rs | 调用 py-fsrs / fsrs-rs CLI |
|---|---|---|---|
| 来源 | open-spaced-repetition 官方组织（FSRS 作者 ecosystem） | — | 官方 |
| 语言/形态 | 纯 Go module，`v4` 语义版本 | 纯 Go 重写或 cgo | 子进程 + 序列化 |
| FSRS 版本 | **v6**（21 权重；v4.0.0 起）[[README]](https://github.com/open-spaced-repetition/go-fsrs#readme) | 取决于移植目标 | v6 |
| 版本/发布 | v4.0.0（2026-07-25），v4.1.0 tracking 中（issue #89）；历史 v0.0.1→v4 共 16 个 tag，有 rc 流程 | 自维护 | fsrs-rs v6.6.2（2026-08-28）[[releases]](https://github.com/open-spaced-repetition/fsrs-rs/releases) |
| 维护活跃度 | 最近 push 2026-11-04；2026-10/11 仍有 fix/test 提交（[#92](https://github.com/open-spaced-repetition/go-fsrs/pull/92) 等） | 自维护 | fsrs-rs pushed 2026-11-05，非常活跃 |
| 成熟度信号 | 157 ★；[awesome-fsrs](https://github.com/open-spaced-repetition/awesome-fsrs) 收录（Go 栏目）；真实消费者：[siyuan-note/riff](https://github.com/siyuan-note/riff)（思源笔记调度后端）、WordVault（见 awesome-fsrs README） | 重复造轮子，违反地图工程原则 | 需打包运行时，桌面分发负担 |
| Optimizer | ❌ `go-fsrs/optimizer` 标注 **in development**（`optimizer` 分支截至 2026-07-09 尚无 optimizer 包，仅与 main 同构的骨架） | — | ✅ fsrs-rs `compute_parameters`、py-fsrs `Optimizer`（`pip install "fsrs[optimizer]"`） |
| 许可证 | **MIT** | BSD-3-Clause（fsrs-rs） | BSD-3-Clause（fsrs-rs）/ MIT（py-fsrs） |

**判定**：go-fsrs 满足「成熟/活跃/官方/纯 Go」，是唯一合理选择；其余路径仅作为 optimizer 的补充手段（见 §4）。

### go-fsrs v4 能力清单（API 事实）

来源：[README](https://github.com/open-spaced-repetition/go-fsrs#readme) 与源码（克隆 main @ 2026-11-04）。

- 高层 `*FSRS`：`Repeat(card, now)`（四档全预览）、`Next(card, now, grade)`（应用一个评级→新卡 + ReviewLog）、`Retrievability`、`Reschedule`（重放历史重建状态/到期）、`Forget`、`Rollback`、`MemoryState(history, start)`、`HistoricalMemoryStates`。
- 低层 `Parameters`（纯算法，无状态机/无 fuzz）：`NextState(state, retention, days, grade)`、`NextStates`、`ForgettingCurve`、`ApplyFuzz`。
- `Card` 持久化字段：`Due / Stability / Difficulty / ScheduledDays / Reps / Lapses / State / LastReview / RemainingSteps`（[models.go](https://github.com/open-spaced-repetition/go-fsrs/blob/main/models.go)），全部 JSON 可序列化。
- Rating：`Again=1, Hard=2, Good=3, Easy=4`；State：`New/Learning/Review/Relearning`。
- 权重迁移助手：`ConvertV5Weights`（19）、`ConvertV45Weights`（17）、`MigrateWeights`（自动检测 17/19/21）。
- 结构化错误 `*fsrs.Error`（`errors.Is/As` 友好）；fuzz 用 per-card 种子可复现（README「Weights」「Errors」节）。

## 2. 许可证兼容性矩阵

事实：go-fsrs = **MIT**（[LICENSE](https://github.com/open-spaced-repetition/go-fsrs/blob/main/LICENSE)，GitHub API `license.spdx_id` 确认）。备用相关库：py-fsrs = MIT；fsrs-rs / fsrs-optimizer = BSD-3-Clause（GitHub API）。

依赖方向是「Remit（拟开源）依赖 go-fsrs（MIT）」，弱 copyleft 依赖宽松许可永远单向兼容：

| Remit 拟定 license \ 依赖 | MIT（go-fsrs/py-fsrs） | BSD-3-Clause（fsrs-rs） |
|---|---|---|
| MIT | ✅ | ✅（保留 BSD 声明即可） |
| Apache-2.0 | ✅（保留 MIT 声明） | ✅（BSD-3 与 Apache-2.0 兼容，附 NOTICE） |
| AGPL-3.0 | ✅ | ✅（BSD-3 可并入 AGPL） |

**结论：无论 Remit 最终选 MIT/Apache/AGPL，集成 go-fsrs 无许可证障碍**（只需在分发物中保留上游版权与许可声明）。思源（AGPL 应用）依赖 MIT 的 go-fsrs/riff 即为现成先例。

## 3. 与内容模型解耦的最小调度接口

go-fsrs 本身已把「调度」与「内容」完全解耦——它只认识 Card 数值状态与评级。Remit 侧据此可定义最小接口（映射自 go-fsrs 实际 API，非臆造）：

```go
// 评级 + 历史进，下次时间 + 记忆状态出。
type Scheduler interface {
    // 一次性预览四档（Repeat），或应用一个评级（Next）。
    Next(card fsrs.Card, now time.Time, grade fsrs.Rating) (fsrs.Card, fsrs.ReviewLog, error)
    // 由纯历史（评级 + 距上次间隔天数）重建记忆状态，无需持久化卡。
    MemoryState(history fsrs.ReviewEntries, start ...time.Time) (*fsrs.MemoryState, error)
    Retrievability(card fsrs.Card, now time.Time) (float64, error)
}
```

关键事实：

- **输入**：一次复习 = `(rating ∈ {1..4}, 时间戳)`。历史的最小表示是 `ReviewEntry{Rating, DeltaT}`（自上次复习的间隔天数，[models.go](https://github.com/open-spaced-repetition/go-fsrs/blob/main/models.go)）；完整重放用 `ReviewHistory`（含每步 State/Stability/Difficulty/Due）。
- **输出**：下次到期 `Card.Due` + 记忆状态 `MemoryState{Stability, Difficulty}` + 可回滚的 `ReviewLog`。`Retrievability` 给当前回忆概率。
- **每用户参数**：FSRS v6 = **21 个 float 权重**（`Weights [21]float64`，`DefaultWeights()` 为基线）+ 每用户 `desired_retention`（默认 0.9）+ 开关（`EnableFuzz`、learning steps）。这些都是标量数组，存储/同步成本可忽略。
- **Optimizer 需要什么数据**：仅**该用户的完整复习日志序列**（每次复习的评级 + 时间戳/间隔；Anki FAQ 明确 FSRS 只用间隔与评级，不用答题耗时——耗时仅用于计算最优保留率，[FAQ Q5](https://docs.ankiweb.net/faqs/frequently-asked-questions-about-fsrs)）。数量门槛：现代优化器**任意数量复习即可跑**（Anki 24.06.3 起；旧版要求 1000 条；[FAQ Q6](https://docs.ankiweb.net/faqs/frequently-asked-questions-about-fsrs)）；重训频率：每月一次或复习量翻倍时（[FAQ Q4](https://docs.ankiweb.net/faqs/frequently-asked-questions-about-fsrs)）。参照 fsrs-rs 的 `compute_parameters(FSRSItem/FSRSReview)` 与 py-fsrs 的 `Optimizer(review_logs).compute_optimal_parameters()`（两者 README）。

**对存储层的含义**（供 StorageFormats 票参考，非本文档决定）：Card 上只需保存 go-fsrs 的 9 个数值字段；另需一张 append-only 复习日志表（card_id、rating、reviewed_at），它同时服务 optimizer 与 Reschedule/Rollback。

## 4. Optimizer 缺口的路径（若 go-fsrs/optimizer 迟迟不落地）

go-fsrs 调度本体无缺口，此节仅针对「每用户参数训练」：

1. **首选：等官方 `go-fsrs/optimizer`**（README 已列入路线图，状态 in development）。先以 `DefaultWeights()` + desired_retention 运行，参数模型预留 21 权重字段。
2. **可选增强（事实）**：把复习日志导出为 `FSRSItem` 格式，经 [fsrs-rs](https://github.com/open-spaced-repetition/fsrs-rs)（Rust，`compute_parameters`，BSD-3）做 cgo 绑定，或经 [py-fsrs](https://github.com/open-spaced-repetition/py-fsrs) Optimizer（MIT）以 CLI/子进程调用，训练出的 21 权重写回本地。两者都是官方实现，输出的权重格式同族（fsrs-rs 由 Anki 内置优化器抽取而来，其 README/示例即以 21 权重为目标）。
3. **不推荐**：纯 Go 重写 optimizer（违反「不造轮子」，训练代码含数值优化，维护成本高）；社区亦无第三方 Go optimizer 的成熟实现（[awesome-fsrs](https://github.com/open-spaced-repetition/awesome-fsrs) Go 栏目只有调度器条目）。

## 与地图决策的冲突

**无。** 「调度=FSRS、不造轮子」被事实进一步支持：官方纯 Go 实现 + MIT 许可直接可用。

## 证据索引

- go-fsrs 仓库/README/源码：https://github.com/open-spaced-repetition/go-fsrs（克隆 main 与 `optimizer` 分支核对于 2026-11-06）
- go-fsrs releases/tags（v4.0.0 = 2026-07-25；共 16 tag）：https://github.com/open-spaced-repetition/go-fsrs/releases
- awesome-fsrs（实现矩阵 + 采用者 WordVault/SiYuan）：https://github.com/open-spaced-repetition/awesome-fsrs
- siyuan-note/riff（Go + FSRS，AGPL，真实消费者）：https://github.com/siyuan-note/riff
- fsrs-rs（Rust 实现 + optimizer，BSD-3）：https://github.com/open-spaced-repetition/fsrs-rs
- py-fsrs（Python 实现 + Optimizer，MIT）：https://github.com/open-spaced-repetition/py-fsrs
- Anki 官方 FSRS FAQ（optimizer 数据需求/频率）：https://docs.ankiweb.net/faqs/frequently-asked-questions-about-fsrs
- 许可证数据来自 GitHub API（各仓库 `license.spdx_id`）
