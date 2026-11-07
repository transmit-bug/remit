# 插件系统 · 宿主 API v1（类型与模板修订稿）

一张地图收口用的设计稿。本稿在 2026-11-07 接受稿的基础上**重开卡片契约**：卡片类型、
字段与模板进入库模型（Anki 式完成度），渲染归宿主模板引擎，`cards.render` 钩子角色取消。
配套 ADR 0008 落地时同步修订 0002（内容模型）、0003（存储）、0005（笔记编辑器）、
0007（插件面）。`apiVersion` 仍为 1：旧契约从未对第三方发布，不存在兼容承诺。

## 1. 原则

| 原则 | 出处 | 我们怎么落 |
|---|---|---|
| **静态声明 + 惰性激活** | VS Code：`contributes`（静态）+ `activationEvents`（何时加载） | manifest 用**宿主定义的钩子词汇表**声明 `implements` 与 capabilities；宿主不启动插件即可展示、校验与索引它的全部表面 |
| **能力即对象，不是权限弹窗** | Obsidian：没有沙箱，API 面（Vault / MetadataCache）本身就是边界 | JS 全局只有 `remit`，标准库冻结；拿不到 `fs` / `net` / 定时器以外的宿主能力 |
| **安装时授权，运行时不再问** | VS Code：扩展信任在安装时建立 | 启用插件时展示它声明的 capabilities 并让用户批准；运行期只安装已批准的 `remit.*` |
| **插件失败不得阻塞主流程** | Anki：add-on 出错不影响复习 | 抛错/超时/内存超限 → 回退内置默认插件，计数，三次停用 |
| **内置也是插件** | VS Code：built-in extensions 走同一 API，无特权 | 默认制卡/调度随应用打包，同样走 manifest 与 capability |
| **卡片是数据，模板是库资产** | Anki：note type 存在 collection 里，模板即渲染 | 类型（字段表+模板）是库的明文数据；渲染是宿主模板引擎，没有"渲染插件"这个角色 |

**不要 legacy 双实现**：插件路径跑通、金标准 fixtures 全绿之后，删除 core 里的 Go 实现。

## 2. 卡片类型体系

### 2.1 类型与卡片

- **类型（type）**：一份命名的卡片 schema = 有序**字段**表（字段名 + 值为标量/字符串）+
  一组**模板**（front/back 两段模板文本）+ 版本。类型随库走：`types/<id>.json` 明文，
  ID 一次性铸造。卡片引用类型。
- **卡片**：`type + fields（字段值表）+ ordinal + producer`。同一 Block 派生多张卡
  （cloze 的每个空、双向的两个方向）靠 ordinal 区分。
- **卡片面永不落库**：渲染时由模板引擎从字段现算；模板缺失/渲染出错时回退到
  **字段原文列表**，不显示空白。
- 每条派生数据带 **producer {id, version}**：卸载插件后卡片仍可读、可解释。
### 2.2 内置类型与通用字段块

- 内置类型三件：basic（`::` 前提示、后答案）、bidirectional（`:::`）、cloze（`{{ }}`）。
- **通用字段块**让任何类型都能被笔记直接使用：

  ```
  ::: type <typeId>
  Front: 提示文本
  Back: 答案文本
  Extra: 任意字段
  :::
  ```

  `::: type <id>` 带内容，与单独成行的 `:::`（双向卡标记）可区分。字段块由内置
  derive 插件解析：`字段名: 值` 每行一条，类型不匹配（字段不存在）在保存时报告。
- **schema 的来源**：内置三类型 + 用户在类型管理界面创建（字段表 + 模板 + 实时预览）
  + derive 插件声明产出的 typeId。表单式"单独建卡"不做——产品是笔记优先，字段块即录入。

### 2.3 类型编辑的安全边界

| 操作 | 行为 |
|---|---|
| 字段改名 | 全库卡片键名一次性重写 |
| 字段删除 | 任一卡片该字段非空即拒绝 |
| 类型删除 | 被任何卡片引用即拒绝 |
| 模板编辑 | 立即生效，卡片面随下次渲染更新（本就不落库） |

### 2.4 没有存量迁移

**不读旧格式**：remit/2 是第一个发布格式，打开时声明其它 format 的库直接拒绝
（错误指向声明的值与期望值）。pre-1.0 没有已发布的库要保。搜索仍覆盖块文本；把
字段文本纳入索引搜索属后续票。

## 3. 模板方言（宿主引擎）

纯数据替换，无脚本、无 HTML/CSS、无第三方函数——模板是**用户可编辑的库数据**，
由宿主（Go，复习关键路径）解释，输出 markdown，样式走主题 token。

| 构造 | 语义 |
|---|---|
| `{{Field}}` | 字段替换（markdown 内联渲染） |
| `{{#Field}}…{{/Field}}` / `{{^Field}}…{{/Field}}` | 字段非空/为空的条件段 |
| `{{cloze:Text}}` | 按 ordinal（0-based）渲染当前空：front 只罩 ordinal 空（`[…]`）、其余揭示；back 全揭示去标记；ordinal 超界=两侧全揭示（"asks what it can"） |
| `{{hint:Field}}` | 折叠提示，展开动效归宿主 |
| `{{type:Field}}` | 键入作答：复习界面出输入框与比对视图 |
| `{{Field\|filter}}`（预留） | 过滤器管道；过滤器是**宿主内建注册表**，v1 允许为空，插件不可注册 |

未知字段名、未知构造、渲染错误 → 该卡回退字段原文列表。方言语法一次性定全（含保留字）。

## 4. 包与 manifest

```
<App data>/extensions/<id>/
├── extension.json
└── main.js          # 单文件 ES module；多文件走宿主解析的模块图，不读磁盘其它位置
```

```json
{
  "id": "remit.cards.standard",
  "name": "Standard cards",
  "version": "1.0.0",
  "apiVersion": 1,
  "runtime": "js",
  "entrypoint": "main.js",
  "capabilities": ["notes:read", "review:read", "schedule:fsrs"],
  "implements": { "cards.derive": 1 },
  "activation": ["onReview", "onNoteSaved"],
  "contributes": { "settings": [] },
  "produces": ["remit.basic", "remit.cloze"]
}
```

- `implements` 的键是宿主词汇表里的钩子 ID，值是该钩子的契约版本；导出函数名由宿主固定。
  宿主不加载插件即可校验它声称实现了什么。
- `produces` 声明该 derive 插件会产出的卡片类型（可引用库内已有类型或自带类型定义）。
- `activation` 只决定何时加载，不决定谁被选中；角色决定选中。
- `contributes.settings` 声明由宿主渲染的设置项（插件不做 UI）。v1 允许为空数组。
- `runtime: "exec"` 仍被接受（ADR 0006）：需要原生库或长计算的扩展走进程协议；
  类型体系相关钩子仅 JS 运行时可 `implements`。

## 5. Hooks 与数据契约

derive 一个 + 调度/展示 + 一个事件回调。纯函数不得写库，写入只能通过 capability，
由宿主在 hook 返回后应用（与 immediate-mode 纪律一致）。

```ts
type Block = { id: string; noteId: string; text: string; title: string }
type Type  = { id: string; fields: string[]; version: number }   // 库内类型的宿主视图

// derive：一块文本 → 若干张卡（选定类型、填字段值；宿主校验后落库并盖 producer 戳）
export function deriveBlock(block: Block, types: Type[]): Array<{
  typeId: string
  ordinal: number
  fields: Record<string, string>
}>

// schedule：评分 → 间隔与预览。FSRS 数学在宿主，插件只给策略；state 对插件不透明
export function scheduleGrade(input: {
  cardId: string; grade: 1 | 2 | 3 | 4; now: string; state: unknown
}): { intervalSeconds: number; state: unknown; previews?: Record<number, number> }

// summary：队列空了 → 说什么（内容归插件，动效归宿主）
export function reviewSummary(input: {
  graded: number; again: number; hard: number; good: number; easy: number
  dueTomorrow: number; streakDays: number; durationSeconds: number
}): { title: string; lines: string[]; tone: "quiet" | "celebrate" }

// 通知，不是否决
export function onEvent(event: { kind: string; id: string }): void
```

**产出即建议**：宿主校验结构（类型存在、字段名在类型里、夹取边界），然后由宿主写库。
渲染不再经过任何插件：`cards.render` 角色删除，模板引擎即渲染器。

## 6. Capabilities

| 能力 | 授予的宿主面 | 拒绝时 |
|---|---|---|
| `notes:read` | `remit.notes.list/get/search` | hook 抛 `CapabilityError` |
| `cards:write` | `remit.cards.create/update/delete` | 同上 |
| `review:read` | `remit.review.history/dueCount` | 同上 |
| `review:write` | `remit.review.record` | 同上 |
| `schedule:fsrs` | `remit.fsrs.review(state, grade, now)` | 同上 |
| `store:own` | `remit.store.get/set`，命名空间绑定插件 id | 同上 |

`store:own` 落在**库内**明文 `plugins-data/<id>.json`，随 `.remitlib` 归档走（启用按库，
数据按库——Obsidian 的 per-vault 模型；VS Code 式机器级层等真实需求出现再加）。

**没有** `fs` / `net` / `ui` / 任意 SQL。需要联网的插件是 v2 议题（那时加 `net:fetch` +
allowlist）。

## 7. 爆炸半径

进程内 JS 的隔离 = 能力面 + `Interrupt` + `MemoryLimit`，**不等价**进程隔离：引擎自身的
内存安全 bug 仍会带崩 app（ADR 0006 已知风险）。exec 运行时保留，作为需要真进程隔离
或原生库的出口。

## 8. 错误、超时、内存、停用

| 情况 | 宿主行为 |
|---|---|
| 抛异常 / 返回结构非法 | 记一次失败；该 hook 回退内置默认插件；错误写进插件日志 |
| 超过 1s | `Interrupt`；记一次失败 |
| 超过 64 MiB | `ErrMemoryLimit`；记一次失败 |
| 连续 3 次失败 | 停用该插件，通知用户，流程继续走内置默认 |
| 加载失败（语法错） | 只影响该插件；库照常打开 |
| 模板渲染出错 | 回退字段原文列表，不显示空白 |

每插件一个 **2-runtime 池**，**激活时预热**（首次使用不付加载成本，语法错在激活期暴露）。
插件调用永不占用 UI 线程；结果经回调回填。

## 9. 版本

- `apiVersion`：整数，契约产物 `remit.d.ts` 的版本。宿主按**最低版本**语义接受：
  插件声明的 `apiVersion` ≤ 宿主即可加载。破坏性变更 +1，旧版一个大版本内继续可用。
- `protocol`（exec 线协议）与 `apiVersion`（JS 宿主面）分开。
- 复习记录写 `scheduler: { family, flags, version }`：换调度插件后历史仍可读，只是不再
  可比；切换点作为边界记录写入。

## 10. 生命周期

```
discovered → enabled(能力已批准) → loaded(惰性, 池已预热) → failed(n) → disabled
                    ↑                                  │
                    └──────── 用户手动重新启用 ←────────┘
```

- 启用是按库的显式动作，批准发生在启用时。
- 角色两个：`cards.derive`、`review.schedule`，每库各选**一个**插件，默认内置。
  多个插件声明同一角色时由设置页选择，宿主不做自动仲裁（`reviewSummary` 归调度插件）。

## 11. 内置默认插件

- 制卡（`remit.cards.standard`）：`::` / `:::` / `{{ }}` 三个内置类型 + 通用字段块解析。
- 调度（`remit.scheduler.default`）：FSRS 策略与队列清空文案。
- 随应用打包（`//go:embed`），走与三方插件相同的加载路径与 capability 检查，无特权入口。
- 回退 = 回退到它们，不是第二份 Go 代码。

## 12. 怎么测一个插件

`go tool remit ext test <dir>`：载入插件，用 fixtures（笔记 + 复习历史）跑全部 hook，
逐字比对输出。内置插件用金标准 fixtures（兼作删除 Go 实现的凭据）；三方插件自带
`fixtures/` 目录（输入 + 期望输出），宿主只提供 harness 与比对。

## 13. v1 不做

插件市场 / 签名 / 自动更新 · `net:fetch` · 插件间依赖 · 移动端 · 插件注册模板函数 ·
排序字段（sort field） · 表单式建卡 · 机器级插件存储层。
