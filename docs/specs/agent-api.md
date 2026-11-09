# Agent API · 本机回环上的只读面

> 状态（2026-11-09）：在既有读/写路由上补一批**只读**路由——搜索、标签、卡类型、
> 统计、反向链接、事件流与 schema。不新增写路径，不新增删除。
> 实现在 `internal/agentapi`，与插件宿主共用一份读写声明（`agentapi.Library`）。

## 1. 口径

- **只监听回环**：`127.0.0.1` 上由系统选端口，端口写进数据目录的 `agent-address`。
  网络上的进程到不了这里。
- **读不要 token**：数据是用户自己的，本机进程本来就能直接读文件。
- **写要 bearer token**：`Authorization: Bearer <agent-token>`，token 在数据目录的
  `agent-token`（仅属主可读，`0600`）。没配 token 时写路由答 `503 writes_disabled`；
  缺 token 或 token 不对答 `401 unauthorized`。
- **没有删除**：任何集合上的 `DELETE` 都是 `405`；已知路径上的其它方法同样是 `405`，
  未知路径才是 `404`。错误一律是 `{"error": {"code": "...", "message": "..."}}`。
- **不写阅读位置**：位置只由读者自己的会话写下；API 只能把笔记加入阅读队列。

## 2. 读路由

| 路由 | 参数 | 响应 |
|---|---|---|
| `GET /v1/schema` | — | `{"api": "v1", "format": "remit/2", "routes": ["GET /v1/library", …]}` |
| `GET /v1/library` | — | 既有：`{format, notes, cards, due, reading}` |
| `GET /v1/notes` | — | 既有：笔记数组 |
| `GET /v1/notes/{id}` | — | 既有：一篇笔记及其块 |
| `GET /v1/notes/{id}/backlinks` | — | `[{note, title, block, paragraph, text}]`，按所在笔记排；未知笔记 `404` |
| `GET /v1/cards` | — | 既有：卡片数组 |
| `GET /v1/cards/{id}` | — | 既有：一张卡 |
| `GET /v1/queue` | `limit`（`0` = 全部） | 既有：现在到期的卡 |
| `GET /v1/search` | `q`（必填）、`kind=note\|card`（默认两者）、`limit`（默认 50，`0` = 全部）、`offset` | `[{id, kind, title, note, block, snippet}]` |
| `GET /v1/tags` | — | `[{tag, notes, cards}]`，按 tag 排 |
| `GET /v1/types` | — | `[{id, name, fields: […], templates: {front, back}, version}]` |
| `GET /v1/types/{id}` | — | 同上单个；未知类型 `404` |
| `GET /v1/stats/summary` | — | `{reviewed, due_tomorrow, accuracy}` |
| `GET /v1/stats/forecast` | `days`（默认 30） | `[{day: "YYYY-MM-DD", count}]`，`stats.ForecastAll` |
| `GET /v1/stats/retention` | — | `{reviews, passes, accuracy, buckets: [{low, high, reviews, passes, accuracy}]}` |
| `GET /v1/events` | — | `text/event-stream`，见 §4 |
| `GET /v1/reading` | — | 既有：阅读队列 |
| `GET /v1/reading/{id}` | — | 既有：一条条目及其事件 |

**搜索**走 store 既有的 `SearchNotes` / `SearchCards`（块文本 + 手改字段值的 `LIKE`），
不依赖任何未来的 FTS 接口。笔记结果在前（新版在前，与笔记本一致），卡片结果在后；
`offset`/`limit` 跨两类一起翻页。结果字段：

- `id`：命中的笔记或卡片 ID。
- `kind`：`note` 或 `card`。
- `title`：笔记用自身标题；卡片没有标题，用其块所在笔记的标题。
- `note`：命中所在的笔记 ID（笔记结果等于 `id`）。
- `block`：命中的块 ID；`snippet` 是该块（卡片改过字段时是字段值）里对上的文本。

搜索只读两样东西：块文本和用户手改的字段值——卡片渲染出的正面/背面不建索引。

**标签**同时给笔记数与卡片数：同一个标签落在一篇笔记的多个块上仍是一篇笔记，
一个块喂三张卡就是三张卡（两侧都是 distinct 计数）。

**统计**是 `internal/core/stats` 的三个答案：`Summarize`（今天的复习数、正确率与明日到期）、
`ForecastAll`（卡片与阅读队列并成一条序列）、`RetentionOf`（每条复习的预测对结果）。
它们不是 API 自己算的，而是同一个包交给统计屏的同一份数字。没有配置统计来源的服务器
（只有测试会这样搭）在这三条路由上答 `501 stats_unavailable`，而不是拿零冒充空库。

## 3. 写路由（既有，需 token）

| 路由 | 请求体 | 说明 |
|---|---|---|
| `POST /v1/notes` | `{title, blocks: [文本]}` | 建笔记；块 ID 由库铸造 |
| `POST /v1/cards` | `{block, fields?}` | 由卡片插件从块派生卡片 |
| `POST /v1/reading` | `{note, at?}` | 笔记加入阅读队列 |
| `POST /v1/cards/{id}/reviews` | `{rating, at?}` | 记一次复习（again/hard/good/easy） |
| `PATCH /v1/cards/{id}` | `{fields?, status?}` | 只改措辞与状态 |

## 4. 事件流

`GET /v1/events` 是一条 Server-Sent Events 流，内容就是窗口看到的库事件
（`note.saved`、`card.created`、`review.recorded`、`reading.queued`、`reading.recorded`、
`reading.dropped`）：

```
Content-Type: text/event-stream; charset=utf-8

data: {"kind":"note.saved","id":"note_…"}

: heartbeat

```

- **不回放**：订阅从流打开那一刻起生效。订阅先于响应头 flush 完成，所以客户端一旦收到
  响应头，之后发生的写一定在流里。回放由读路由负责。
- **心跳**：空闲时每 ~20s 一帧注释 `: heartbeat`，客户端忽略，代理不超时。
- **慢订阅者被丢弃**：写永远不等监听者。一个订阅落后超过库的缓冲（64 个事件）就被移出
  广播，流随之安静下来；客户端重新连接并补齐读路由即可，写不受影响。
- **结束**：请求上下文取消（客户端断开）即关闭。

实现上，库有两个互不替代的听众：窗口的 `SetEventHandler`（在写线程上同步调用，用来重取
快照）与 `Subscribe` 注册的订阅者（各自一个 goroutine，按序投递，队列满即丢弃）。两者
在一个写里同时收到同一个 `store.Event{Kind, ID}`。

## 5. 特性探测

客户端先读 `GET /v1/schema`：

- `api`：HTTP 面的形状，今天只有 `v1`；不认识的版本应当停下，不要猜。
- `format`：数据格式版本（`store.FormatVersion`，今天 `remit/2`）。
- `routes`：`"METHOD /path"` 的完整清单，含写路由——`POST`/`PATCH` 开头的需要 token。
  新路由只增不改，客户端按 `routes` 里有没某一行决定要不要用。

## 6. 不做的事

- **不删除**：卡片、笔记、复习记录都没有删除；删卡是插件（声明 `cards:write`）与窗口的。
- **不改调度**：不写 FSRS 参数、不改已写入的复习史。
- **不写阅读位置**：只有 `POST /v1/reading`（加入队列）。
- **搜索不是全文检索**：今天是 `LIKE`（大小写不敏感，按块与手改字段）；
  FTS 若要来，是 store 自己的决定，API 的形状不必变。
- **事件流不回放、不按 kind 服务端过滤**：要历史就读读路由。
- **既有路由的 JSON 形状不变**：本次只加路由、加字段（`/v1/library` 的 `format` 仍是同一
  个值），不改已有字段。
