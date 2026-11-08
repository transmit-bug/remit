# 插件系统 · 宿主 API v1（类型与模板修订稿）

一张地图收口用的设计稿。本稿在 2026-11-07 接受稿的基础上**重开卡片契约**：卡片类型、
字段与模板进入库模型（Anki 式完成度），渲染归宿主模板引擎，`cards.render` 钩子角色取消。
配套 ADR 0008 落地时同步修订 0002（内容模型）、0003（存储）、0005（笔记编辑器）、
0007（插件面）。`apiVersion` 仍为 1：旧契约从未对第三方发布，不存在兼容承诺。

> **落地口径（2026-11-07 之后）**：本稿按 `0d87ffd`（类型/模板与宿主 API v1 落地）与
> `70e7b8f`（无迁移）校正。钩子与能力以 `internal/ext/manifest.go`、`internal/ext/js.go`
> 为准，卡片面以 `internal/cards` + `internal/core/template` 为准。标 **计划** 的是尚未建
> 的部分，其余按今天的行为写；v1 的收尾按票列在 §14。

## 1. 原则

| 原则 | 出处 | 我们怎么落 |
|---|---|---|
| **静态声明 + 惰性激活** | VS Code：`contributes`（静态）+ `activationEvents`（何时加载） | manifest 用**宿主定义的钩子词汇表**声明 `implements` 与 capabilities；宿主不启动插件即可展示、校验与索引它的全部表面。`activation` 今天只被记录，启用列表才是加载时机（惰性加载：**计划**） |
| **能力即对象，不是权限弹窗** | Obsidian：没有沙箱，API 面（Vault / MetadataCache）本身就是边界 | JS 全局只有 `remit`，标准库冻结；拿不到 `fs` / `net` / 定时器以外的宿主能力 |
| **安装时授权，运行时不再问** | VS Code：扩展信任在安装时建立 | 设置页列出发现到的插件与它声明的 capabilities，批准后才把名字写进库的 `extensions`；宿主只装声明过的 `remit.*` 函数，未声明的调用是 `CapabilityError`（§6） |
| **插件失败不得阻塞主流程** | Anki：add-on 出错不影响复习 | 抛错/超时/内存超限 → 记数并降级：卡片回退段落原文、调度回退宿主默认，三次停用 |
| **内置也是插件** | VS Code：built-in extensions 走同一 API，无特权 | 默认制卡/调度随应用打包，同样走 manifest 与 capability |
| **卡片是数据，模板是库资产** | Anki：note type 存在 collection 里，模板即渲染 | 类型（字段表+模板）是库的明文数据；渲染是宿主模板引擎，没有"渲染插件"这个角色 |

**已执行**：插件路径跑通、金标准 fixtures 全绿之后，core 里 Go 的制卡切分已删除
（`aecb7de`）；金标准现在对着内置插件跑（§12）。

## 2. 卡片类型体系

### 2.1 类型与卡片

- **类型（type）**：一份命名的卡片 schema = 有序**字段**表（字段名 + 值为标量/字符串）+
  一组**模板**（front/back 两段模板文本）+ 版本。类型随库走：`types/<id>.json` 明文，
  ID 一次性铸造；内置三件随库种入，缺文件时补齐。今天改类型仍是手改这些文件（类型编辑
  界面归 #144）。卡片引用类型。
- **卡片**：`type + ordinal + producer`，外加**只在用户手工编辑过时**才有的字段覆盖
  （`fields` + `edited`）。字段值本身不落库：渲染时按卡片的来源现算（live = 块文本，
  frozen = 快照），卡片自己的覆盖逐字段压在上面（ADR 0008）。同一 Block 派生多张卡
  （cloze 的每个空、双向的两个方向）靠 ordinal 区分。
- **卡片面永不落库**：渲染时由模板引擎从字段现算；模板缺失/渲染出错时回退到
  **字段原文列表**，不显示空白。
- 每条派生数据带 **producer {id, version}**：卸载插件后卡片仍可读、可解释。

### 2.2 内置类型与通用字段块

- 内置类型三件：`remit.basic`（`::` 前提示、后答案，字段 Front/Back）、
  `remit.bidirectional`（`:::`，字段 Front/Back，ordinal 记方向）、`remit.cloze`
  （`{{ }}`，字段 Text，每空一张卡）。
- **通用字段块**让任何类型都能被笔记直接使用：

  ```
  ::: type <typeId>
  Front: 提示文本
  Back: 答案文本
  Extra: 任意字段
  :::
  ```

  `::: type <id>` 带内容，与单独成行的 `:::`（双向卡标记）可区分。字段块由内置
  derive 插件解析：`字段名: 值` 每行一条。类型没有的字段由插件列进 `unknown`，宿主记日志
  并丢弃，卡片仍按能对上的那部分生成；今天没有面向用户的报告。
- **schema 的来源**：内置三类型（随库种入）+ 设置页管理的 `types/<id>.json`（类型列表、
  字段编辑、模板编辑与预览，#144）。
  derive 插件的 `produces` 只是声明：宿主只接受库内已有的类型，插件不能凭空带来类型定义。
  表单式"单独建卡"不做——产品是笔记优先，字段块即录入。

### 2.3 类型编辑的安全边界

类型编辑界面按上表实现：设置页保存类型时由 store 的写路径执行改名的全库重写与删除守卫
（#144）。因为字段只在
**编辑过的卡片**上持久化，"全库重写"的范围是那些 `fields` 映射，不是每一张卡。

| 操作 | 行为 |
|---|---|
| 字段改名 | 改 `types/<id>.json` 的 `fields`，并把 `edited` 卡片的 `fields` 键一并重写；笔记里 `::: type` 字段块仍用旧键的行变成 `unknown`（丢弃 + 日志），所以编辑器要先搜再改 |
| 字段删除 | 有 `edited` 卡片存着非空值即拒绝；否则旧字段块的行按 `unknown` 处理 |
| 类型删除 | 被任何卡片引用即拒绝；内置三件不可删（缺文件时下次打开会重新种入）。手删非内置类型的文件会让引用它的卡片回退字段列表 |
| 模板编辑 | 立即生效，卡片面随下次渲染更新（本就不落库）；模板编译失败 → 回退字段列表 |

### 2.4 没有存量迁移

**不读旧格式**：remit/2 是第一个发布格式，打开时声明其它 format 的库直接拒绝
（错误指向声明的值与期望值）。pre-1.0 没有已发布的库要保。搜索覆盖块文本与卡片上
**编辑过的字段值**（#149）：块正文里的字段行本就在块文本里，不另存。

## 3. 模板方言（宿主引擎）

纯数据替换，无脚本、无 HTML/CSS、无第三方函数——模板是**用户可编辑的库数据**，
由宿主（Go，复习关键路径）解释，输出 markdown，样式走主题 token。

| 构造 | 语义 |
|---|---|
| `{{Field}}` | 字段替换（markdown 内联渲染） |
| `{{#Field}}…{{/Field}}` / `{{^Field}}…{{/Field}}` | 字段非空/为空的条件段 |
| `{{cloze:Text}}` | 按 ordinal（0-based）渲染当前空：front 只罩 ordinal 空（`[…]`）、其余揭示；back 全揭示去标记；ordinal 超界=两侧全揭示（"asks what it can"） |
| `{{hint:Field}}` | 折叠提示：宿主把值捕获进渲染结果，复习屏尚未显示（**计划**） |
| `{{type:Field}}` | 键入作答：宿主捕获答案值，复习屏尚未出输入框与比对（**计划**） |
| `{{Field\|filter}}`（预留） | 过滤器管道；过滤器是**宿主内建注册表**，v1 允许为空，插件不可注册；今天引用过滤器 = 模板编译失败 → 回退字段列表 |

未知字段名、未知构造、渲染错误 → 该卡回退字段原文列表。方言语法一次性定全（含保留字）；
重复的 `{{hint:}}` / `{{type:}}` 标签在编译期报错。

## 4. 包与 manifest

```
<App data>/extensions/<id>/
├── extension.json
└── main.js          # 单文件 ES module；没有模块解析器，import 不被支持（多文件：计划）
```

```json
{
  "id": "remit.cards.standard",
  "name": "cards",
  "version": "1.0.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "capabilities": [],
  "implements": { "describe": 1, "deriveBlock": 1 },
  "activation": [],
  "produces": ["remit.basic", "remit.bidirectional", "remit.cloze"]
}
```

- `implements` 的键是宿主词汇表里的钩子 ID（`describe` / `deriveBlock` / `policy` /
  `identity` / `summary` / `on_event`），值是该钩子的契约版本；导出函数名由宿主固定。
  宿主不加载插件即可校验它声称实现了什么，词汇表外的键直接拒绝。
- `produces` 声明该 derive 插件会产出的卡片类型。它是声明，不是授权：宿主只接受库内
  已有的类型。
- `activation` 只决定何时加载，不决定谁被选中；角色决定选中。宿主今天记录它，不据此
  加载（启用列表决定加载）。
- `contributes` 是静态的 UI 声明：命令 / 菜单 / 页面 / 设置 / 文案（[`ui.md`](ui.md)）。
  契约与宿主绘制在 #146 落地；`remit ext check` 与 `ext test` 读同一份解析而不启动插件；
  单个贡献非法只丢那一条，插件照常启用。
- `runtime: "exec"` 仍被接受（ADR 0006）：需要原生库或长计算的扩展走进程协议；js
  manifest 不带 `protocol`，exec manifest 必须带 `protocol: 1`。类型体系相关钩子今天
  只有 JS 插件在用，宿主未禁止 exec 声明它们，但没有 exec 插件能跑通这些钩子。

## 5. Hooks 与数据契约

derive 一个 + 调度/展示 + 一个事件回调。纯函数不直接写库：写入只能通过 capability（
`cards:write` / `review:write` / `store:own`），由宿主在 hook 返回后应用（与 immediate-mode
纪律一致）。

```ts
// 宿主交给插件的块视图：今天只有 text（id 尚未传入）
type Block = { id?: string; text: string }
type Type  = { id: string; fields: string[]; version: number }   // 库内类型的宿主视图

// derive：一块文本 → 若干张卡（选定类型、填字段值；宿主校验后落库并盖 producer 戳）
export function deriveBlock(block: Block, types: Type[]): Array<{
  typeId: string
  ordinal: number
  fields: Record<string, string>
  unknown?: string[]   // 插件自报：类型没有的字段名，可选
}>

// 调度：插件只给策略，不给间隔。FSRS 数学与 state 都在宿主（ADR 0007）
export function policy(): { family: string; version: string; retention: number }

// 身份钩子在词汇表里，但宿主今天从 policy 拼 `family-version`，不调用它
export function identity(): string

// summary：队列空了 → 补什么话（数字由宿主用用户的语言说）
export function summary(input: {
  reviewed: number; dueTomorrow: number; accuracy: number
}): { tone: "quiet" | "celebrated"; lines: string[] }

// 通知，不是否决
export function on_event(event: { kind: string; id: string }): void

// 导入（#153）：一份文件 → 建议。文件字节以 base64 到达，名称随行；
// 插件不落库，宿主校验每条建议后写。任何一条失败丢那一条，文件其余照收。
export function importFile(input: {
  name: string;              // 文件名，含扩展名
  data: string;              // 文件字节，base64
  format: string;            // 命中该插件的哪条 contributes.importers
}): {
  notes: Array<{ title: string; blocks: string[] }>;
  cards: Array<{ note: number; block: number; typeId: string; ordinal: number;
                 fields: Record<string, string> }>;   // note/block 按本文件建议数组下标
  warnings?: string[];
}
```

**产出即建议**：宿主校验结构（类型存在；`unknown` 里自报的多余字段记日志；ordinal 照存；
字段值不落库，渲染时按类型字段取用），然后由宿主写库。渲染不再经过任何插件：
`cards.render` 已删除，模板引擎即渲染器。调用是同步且有界的：`Call` 跑在调用方 goroutine，
超 1s 打断；derive 按文本缓存（失败也缓存），所以同一段文本在一帧里只问一次。

## 6. Capabilities

**已落地**（#142）：八个名字都在宿主词汇表里，插件声明哪个就装哪个；未声明的调用是
`ext.CapabilityError`，错误里带插件 id 与能力名。

| 能力 | 授予的宿主面 |
|---|---|
| `notes:read` | `remit.notes.list/get/search` |
| `cards:write` | `remit.cards.create/update/delete` |
| `review:read` | `remit.review.history/dueCount` |
| `review:write` | `remit.review.record` |
| `schedule:fsrs` | `remit.fsrs.review(state, grade, now)` |
| `store:own` | `remit.store.get/set`，命名空间绑定插件 id |
| `remit.echo` | `remit.echo(v)` 原样返回，给插件自测 |
| `remit.now` | `remit.now()` 当前时间，RFC 3339 |

宿主面与 loopback agent API 复用同一读写层（`internal/agentapi.Library` 是现成缝），
插件调用不走 HTTP 回环。

`store:own` 的存储是库内明文 `plugins-data/<id>.json`，随 `.remitlib` 归档走（启用按库，
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
| 抛异常 / 返回结构非法 | 记一次失败；卡片回退段落原文（不重试内置），调度回退宿主默认；错误写进插件日志 |
| 超过 1s | `Interrupt`；记一次失败 |
| 超过 64 MiB（仅 js） | `ErrMemoryLimit`；记一次失败 |
| 连续 3 次失败 | 停用该插件，记日志；流程继续（卡片读原文，调度用宿主默认） |
| 加载失败（语法错） | 只影响该插件；库照常打开 |
| 模板渲染出错 | 回退字段原文列表，不显示空白 |

每插件一个 **2-runtime 池**，**激活时预热**（首次使用不付加载成本，语法错在激活期暴露）。
今天只有卡片 / 调度两个角色插件被预热（内置或测试注入）；`remit.json` 启用的插件第一次调用
才加载。调用是同步且有界的：插件代码跑在调用方的 goroutine 上，最多占用 1s，没有后台调度；
derive 按文本缓存（含失败），所以 View 不会每帧重问。

## 9. 版本

- `apiVersion`：整数，契约产物 `remit.d.ts` 的版本。宿主按**最低版本**语义接受：
  插件声明的 `apiVersion` ≤ 宿主即可加载。今天宿主只讲 1（`ext.HostAPIVersion`）。破坏性
  变更 +1，旧版一个大版本内继续可用（窗口策略随 2 落地再写）。
- `protocol`（exec 线协议）与 `apiVersion`（JS 宿主面）分开；js manifest 带 `protocol`
  会被拒绝。
- 复习记录写 `scheduler: "<family>-<version>"`（如 `"fsrs-1"`），不是对象：换调度插件后
  历史仍可读，只是不再可比。把切换点写成边界记录是**计划**，今天日志里没有这个事件。

## 10. 生命周期

```
discovered → enabled(remit.json 里有名字) → loaded(Open；角色插件的 js 池已预热) → failed(n) → disabled
                    ↑                                  │
                    └──────── 重开 = 重新启用 ←────────┘
```

- 启用是按库的显式动作：设置页列出已发现插件声明的 capabilities，批准后才把名字写进
  `remit.json` 的 `extensions`（#144）。写入立即生效，插件在下一次开库时启动——与今天的
  启用行为一致，运行中的复习不会换插件。
- 角色两个（卡片、调度）按库各选一个插件，默认内置，写在 `remit.json` 的 `roles`（#144）；
  角色名与插件名都在 store 里校验，被选中的插件必须已启用。选择同样在下一次开库生效：
  调度策略要在索引重放之前确定。
- 完成文案归调度插件：已落地为 `summary` 钩子（§5）。

## 11. 内置默认插件

- 制卡（`remit.cards.standard`）：`describe` + `deriveBlock`，`::` / `:::` / `{{ }}` 三个内置类型 + 通用字段块解析。
- 调度（`remit.scheduler.default`）：`policy` / `identity` / `summary` 三钩。FSRS 策略、调度器名与完成文案；内置的 `summary` 答 `quiet` 且无自定文案，数字由宿主说。
- 随应用打包（`//go:embed`），走与三方插件相同的加载路径与 capability 检查，无特权入口。
- 没有第二份实现：插件失败时卡片读段落原文、调度用宿主默认，而不是切到一份 Go 代码。

## 12. 怎么测一个插件

内置插件用金标准 fixtures：`internal/ext/builtin` 的测试逐项比对
`internal/golden/testdata/golden.json`（兼作删除 Go 实现的凭据），`internal/cards` 测字段
解析与回退，`internal/core/template` 测方言。内置插件随二进制打包、没有插件目录，所以
它走这条金标准路径，`ext test` 不另造第二份金标准。

第三方插件自带 `fixtures/`：`remit ext test <dir>` 载入插件、用与窗口同一份解析校验
manifest 与 `contributes`，再把 `implements` 里的每个钩子按 fixture 调用一遍，answer 以
规范 JSON 逐字比对；`fixtures/host.json` 给出插件能读到的库（notes / pluginData / now），
fixture host 只答读、拒绝写。`remit ext check <dir>` 只看 manifest 与贡献，不启动插件；
`remit ext types -o remit.d.ts` 生成契约产物，`go test ./internal/ext/ts` 在它漂移时变红。
写法见 [`writing-a-plugin.md`](writing-a-plugin.md) §7。

## 13. v1 不做

插件市场 / 签名 / 自动更新 · `net:fetch`（v2 议题，届时加 allowlist）· 插件间依赖 ·
移动端 · 插件注册模板函数或过滤器 · 排序字段（sort field） · 表单式建卡 ·
机器级（跨库）插件存储层 · exec substrate 的新能力（仅保留现状）。

类型管理界面、按角色选择插件、能力审批、`activation` 生效与插件自带 fixtures 的 harness
**不在这一列**：它们是 v1 的收尾，按 §14 的票推进。

## 14. 还没建的部分（按票）

| 票 | 内容 | 今天的状态 |
|---|---|---|
| [#141](https://github.com/transmit-bug/remit/issues/141) | 键入作答与提示的复习面 | 模板把 `{{type:}}` / `{{hint:}}` 捕获进 `template.Output`，经 `cards.Answer` 交给 UI；复习屏只渲染卡面，没有输入框、逐字比对或折叠提示 |
| [#142](https://github.com/transmit-bug/remit/issues/142) | 六个 capability 的宿主函数 | 已建：六个命名空间按声明安装，未声明的调用是 `CapabilityError`；`remit.store` 读写 `plugins-data/<id>.json` |
| [#144](https://github.com/transmit-bug/remit/issues/144) | 设置 UI：启用批准、角色单选、类型管理 | 已建：插件批准与 capabilities、两个角色单选、类型列表 / 字段 / 模板与实时预览；need→widget 见 [`ui-inventory.md`](../ui-inventory.md) |
| [#145](https://github.com/transmit-bug/remit/issues/145) | `ext test` / `ext check`、`remit.d.ts`、文档清扫 | 已建：`remit ext test/check/types`、`internal/ext/exttest` 的 fixture harness、`remit.d.ts` 与漂移测试、示例插件 fixtures、GLOSSARY 词条与 ADR 清扫 |
| [#146](https://github.com/transmit-bug/remit/issues/146) | 插件 UI 贡献（命令 / 菜单 / 页面 / 设置） | 已建：`contributes`、`internal/ext/contrib`、命令面板与菜单槽位、插件页面屏、设置页插件表单；校验走 `remit ext check`（#145） |
| [#153](https://github.com/transmit-bug/remit/issues/153) | importer 钩子（插件声明可导入格式） | 已定契约：`contributes.importers`（format / extensions / label）+ `importFile` 钩子；纯函数产出建议，宿主校验落库，能力复用六个名字 |
