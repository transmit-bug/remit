# ADR 0011 — 库格式 v3：终态事件、推后与预设

- **Status:** accepted
- **Date:** 2026-11-10
- **Related:** #213, #216, #207, #226, `docs/specs/ui-redesign-v2.md` §1.9/§1.12/§5.2/§5.3, `docs/specs/incremental-reading.md` §3, ADR 0002, ADR 0003
- **Amends:** ADR 0003（事件种类、配置版本、索引版本）；笔记与卡片明文不变

## 背景

#213 要回答 3 合 1 的模型决定如何落进明文而不破坏「明文库、可重放、可迁移」。三件事逼出一个新
格式值：阅读需要三种终态与推后（#216），每日上限与阈值需要一个随库走的位置（#207 的
`presets`），而库配置必须保留未知键、更新的库必须被拒（#226）。

今天的起点：`FormatVersion = "remit/2"`（`internal/core/store/library.go:58`）、索引
`schemaVersion = "10"`（`internal/core/store/index.go:24`）、日志的 `Kind` 只有
`Reviewed` / `Queued` / `Dropped`（`internal/core/schedule/schedule.go`）。

## 决定

### 库配置升到 `remit/3`，新增 `presets`

```json
{
  "format": "remit/3",
  "fsrs": { "weights": ["… 21 floats …"], "desired_retention": 0.9 },
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

- `retention: null` 等于跟随调度插件的策略（今天的行为）；非 null 是用户覆盖，写进
  `store.Options.Params.DesiredRetention`（§5.3）。
- 预设**随库走**：它回答「这个库的节奏」，不是这台机器的偏好。外观、纸面宽度、最近库、保存的
  筛选留在 `internal/settings` 的 `settings.json`，不得写进库配置（§5.3「作用域分界」）。
  预设里的每日上限与难卡阈值只是**限额与筛选**：间隔仍由 FSRS 给（`schedule.Step` /
  `Applier`），priority 只是队列排序、不落字段，也没有第二套调度器（§1.3）。
- 落点：`store.Presets`、`Library.Presets()`、`Library.SetPresets()`、`DefaultPresets()` 与
  `store.Options.Presets`（§5.3）。
- **迁移**：打开 `remit/2` 时读回 `fsrs` / `extensions` / `roles`，`presets` 取默认值；先把原文件
  复制到 `remit.json.bak`，再写 `remit/3`。未知顶层键原样保留在 `"unknown"` 里再写回
  （#226 的要求）。迁移失败不改文件、只报错：库仍打开，预设用默认值，记一条日志并在设置页留
  一行提示。
- **回滚**：v2 读者按格式值拒绝 v3 库并报错，不会改写文件；回滚是把 `remit.json.bak` 放回
  （§1.9、§5.3）。

### 日志加三种事件：`archived` / `dismissed` / `postponed`

`reviews.jsonl` 的行形状不变，`kind` 多三个值（§5.2）：

| `kind` | 含义 | 载荷 | 重放效果 |
|---|---|---|---|
| `archived` | 归档：收起、不进队列 | `note` | `archived=true`，`queued=false` |
| `dismissed` | 不再出现：明确不再排进今日 | `note` | `dismissed=true`，`queued=false` |
| `postponed` | 推后：成员不变，到期顺延 | `note` + `until` | `due=until`，队列成员与评分史不变 |

```jsonl
{"seq":412,"note":"note_…","kind":"archived","at":"2026-09-18T09:31:07Z"}
{"seq":413,"note":"note_…","kind":"dismissed","at":"2026-09-18T09:32:00Z"}
{"seq":414,"note":"note_…","kind":"postponed","until":"2026-09-19T00:00:00Z","at":"2026-09-18T09:33:10Z"}
```

- `kind` 缺省仍是评分；非评分事件必须带 `note`、不带 `card`、不带 `rating`。
- `until` 只出现在 `postponed` 上，RFC 3339；缺失或早于 `at` 视为坏行，打开失败。
- `logRecord`（`internal/core/store/log.go`）加一个 `Until` 字段（JSON 名 `until`，空值省略），
  编码与解码各加一条分支。
- 日志只手写可改；坏行让打开失败，末行缺换行视为写了一半——`streamLog`
  （`internal/core/store/log.go`）已有的规则不变（ADR 0003）。

**撤销与恢复（冻结）**：`void` 仍只撤销一条**评分**事件，不撤销成员变更与终态。终态的恢复是写
**反向事件**：`archived` / `dismissed` → `queued`；`postponed` → 再写一条 `postponed`（`until` 为
今日）或 `queued`。阅读会话的「撤销」走新增的 `Library.UndoReading(noteID, at)`——对一条笔记
void 掉最新一条存活的评分事件并重放该笔记的 due 与 position；复习会话继续用既有的
`UndoLast`（§5.2）。

### 索引 schema 11，只加派生

`internal/core/store/index.go` 的 `schemaVersion` 从 `10` 升到 `11`：`reading` 表加
`archived` / `dismissed` / `until`，`card_state` 加 `lapses`；`store.Reading` 加 `Archived`、
`Dismissed` 字段（§5.2、§5.5）。索引仍是派生：删掉重建后，终态、到期、位置与难卡判定从日志与
明文重算，答案一致（`docs/specs/ui-redesign-v2.md` §0.2）。

### 笔记与卡片明文不变

`notes/*.md` 与 `cards/*.json` 的形态不动：块 ID 仍是唯一引用单位，一张卡仍引用一个块
（ADR 0002），字段、类型与模板仍按 ADR 0008 存。字符区间引用不在本格式里（§1.4、§10.1 第 1
条）；卡片的 `reading` 预留状态仍不启用（§1.1）。`.remitlib` 的进出继续复用既有
`internal/core/transfer`，不随格式值改字段（§10.1 第 9 条）。

### 破坏性 Done 被拒绝

读完不删正文。#216 与 `three-in-one.md` §5 已把「Done 清理源 Note」判为 reject：归档与不再出现
只是日志事件，源笔记仍可搜索、仍可被卡片引用、仍可制卡；源块被删是既有的 lost 语义，与终态
无关（§1.12）。

## 后果

- `store.FormatVersion` 改值，v3 库的每一次打开都要过格式检查；迁移是显式一步，且有备份。
- `schedule.Kind` 的 `String` / `ParseKind` 各加三条；重放、`queue.Reading` 与 agent API 的
  `GET /v1/reading` 同步（每项加 `archived` / `dismissed` / `due`，`GET /v1/library` 的计数加
  `today` / `archived` / `unexcerpted`，§5.2、§5.8）。
- 索引 schema 变动只影响派生表：重建是安全的，无需迁移数据文件。
- 一条日志仍是唯一的调度真相：`archived` / `dismissed` / `postponed` 是事件，不是第二份状态
  （ADR 0003）。
- 预设进库后，跨机器拷库带走节奏；跨机器的界面偏好不随之搬家。
