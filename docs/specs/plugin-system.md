# 插件系统 · 宿主 API v1

一张地图（#69）收口用的设计稿。每条选择都标了出处——参照主流做法，不发明新机制。

## 1. 五条原则

| 原则 | 出处 | 我们怎么落 |
|---|---|---|
| **静态声明 + 惰性激活** | VS Code：`contributes`（静态）+ `activationEvents`（何时加载） | manifest 用**宿主定义的钩子词汇表**声明 `implements` 与 capabilities；宿主不启动插件即可展示、校验与索引它的全部表面 |
| **能力即对象，不是权限弹窗** | Obsidian：没有沙箱，API 面（Vault / MetadataCache）本身就是边界 | JS 全局只有 `remit`，标准库冻结；拿不到 `fs` / `net` / 定时器以外的宿主能力 |
| **安装时授权，运行时不再问** | VS Code：扩展信任在安装时建立 | 启用插件时展示它声明的 capabilities 并让用户批准；运行期只安装已批准的 `remit.*` |
| **插件失败不得阻塞主流程** | Anki：add-on 出错不影响复习 | 抛错/超时/内存超限 → 回退**内置默认插件**，计数，三次停用 |
| **内置也是插件** | VS Code：built-in extensions 走同一 API，无特权 | 默认制卡/渲染/调度随应用打包，同样走 manifest 与 capability |

**不要 legacy 双实现**：插件路径跑通、金标准 fixtures 全绿之后，删除 core 里的 Go 实现（#77）。

## 2. 包与 manifest

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
  "capabilities": ["notes:read", "cards:write", "review:read", "schedule:fsrs"],
  "implements": { "cards.derive": 1, "cards.render": 1, "review.schedule": 1 },
  "activation": ["onReview", "onNoteSaved"],
  "contributes": { "settings": [] }
}
```

- `implements` 的键是**宿主词汇表里的钩子 ID**，值是**该钩子的契约版本**。导出函数名由宿主固定（`deriveBlock` / `renderCard` / `scheduleGrade`），插件不能自定义——这样宿主可以在不加载插件的情况下校验它「声称实现了什么」，也避免 Anki 那种「运行时追加的全局钩子列表」退化成的隐式顺序契约（研究 #70）。
- `activation` 只决定**何时加载**，不决定加载谁——角色（derive/render/schedule）决定谁被选中。
- `contributes.settings` 声明**由宿主渲染**的设置项（插件不做 UI，见 #9）。v1 允许为空数组。

## 3. Hooks 与数据契约

三个纯函数 + 一个事件回调。纯函数不得写库，写入只能通过 capability，由宿主在 hook 返回后应用（与仓库现有的 immediate-mode 纪律一致）。

```ts
type Block = { id: string; noteId: string; text: string; title: string }
type Grade = 1 | 2 | 3 | 4                 // again | hard | good | easy

// derive：一块文本 → 若干卡片（ADR 0002：`::` 前提示、后答案；空则整块为提示）
export function deriveBlock(block: Block): Array<{
  ordinal: number;              // 同一块派生的第几张卡（cloze 的每个空、双向的两个方向）
  kind: string;                 // 插件自己的字符串，宿主只存不解释
  front: string; back: string;
}>

// render：卡片 → 两段 markdown（宿主用 ui/markdown 渲染，插件拿不到渲染器）
export function renderCard(card: { id: string; blockId: string; front: string; back: string }):
  { frontMarkdown: string; backMarkdown: string }

// summary：队列空了 → 说什么（内容归插件，动效归宿主）
export function reviewSummary(input: {
  graded: number; again: number; hard: number; good: number; easy: number;
  dueTomorrow: number; streakDays: number; durationSeconds: number;
}): { title: string; lines: string[]; tone: "quiet" | "celebrate" }

// schedule：评分 → 间隔与预览。FSRS 数学在宿主，插件只给策略
export function scheduleGrade(input: {
  cardId: string; grade: Grade; now: string;
  state: unknown;                           // 宿主拥有、插件不可解释，原样传回
}): { intervalSeconds: number; state: unknown; previews?: Record<Grade, number> }
```

**为什么 state 不透明**：FSRS 在宿主（`remit.fsrs.*`），插件能做的是策略（步长、上限、fuzz、降级）。state 一旦让插件解释，就等于把调度数学又复制一份。

## 3.5 产出即建议

- 四个钩子的返回值都是**建议**：宿主校验结构、夹取边界（间隔上限、非负）、然后由宿主自己写库。插件永远不直接写。
- 每条派生数据都带**生产者标识与版本**（`producer: {id, version}`），卸载插件后数据仍可读、可解释（研究 #70 的第三条"该抄"）。
- 渲染钩子的产出**不落库**：每次从卡片数据重新渲染；渲染插件缺失时回退到块的原文，而不是空白。

## 4. Capabilities

| 能力 | 授予的宿主面 | 拒绝时 |
|---|---|---|
| `notes:read` | `remit.notes.list/get/search` | hook 抛 `CapabilityError` |
| `cards:write` | `remit.cards.create/update/delete` | 同上 |
| `review:read` | `remit.review.history/dueCount` | 同上 |
| `review:write` | `remit.review.record` | 同上 |
| `schedule:fsrs` | `remit.fsrs.review(state, grade, now)` | 同上 |
| `store:own` | `remit.store.get/set`（命名空间绑定到插件 id） | 同上 |

**没有** `fs` / `net` / `ui` / 任意 SQL。需要联网的插件是 v2 议题（那时加 `net:fetch` + allowlist）。

## 4.5 爆炸半径（与 #9 的差异，必须写进 ADR）

`#9` 用「独立进程 + 只能经协议改状态」把插件出错的爆炸半径压住；进程内 JS 换了这份隔离。补偿是三件事：能力面（拿不到 `fs`/`net`/`ui`）、`Interrupt`（19–25 µs 打断）、`MemoryLimit`（真实拦停）。**但不能声称等价**：引擎自身的内存安全 bug 仍会带崩 app。这一点由 #68 的 ADR 0006 记录为已知风险，不靠"沙箱"两个字带过（研究 #70 的"不该抄"第一条）。

## 5. 错误、超时、内存、停用

| 情况 | 宿主行为 |
|---|---|
| 抛异常 | 记一次失败；该 hook 回退内置默认插件；错误写进插件日志 |
| 返回结构非法 | 同上，错误信息标 `invalid result` |
| 超过 1s | `rt.Interrupt`；记一次失败 |
| 超 64 MiB | `ErrMemoryLimit`；记一次失败 |
| 连续 3 次失败 | 停用该插件，通知用户，复习继续走内置默认插件 |
| 加载失败（语法错） | 只影响该插件；库照常打开 |
| 渲染插件缺失 | 卡片面回退到块的原文（`::` 切分后的两段），不显示空白 |

`#67` 实测：打断死循环 19–25 µs，host panic 返回 `*InternalError` 且进程存活。

## 6. 版本

- `apiVersion`：整数，是**契约产物 `remit.d.ts` 的版本**（与 VS Code 的 `vscode.d.ts` 同位）。宿主按**最低版本**语义接受：插件声明的 `apiVersion` ≤ 宿主即可加载，不要求精确匹配（`#9` 的 `protocol: 1` 精确匹配只能硬失败，这是要改掉的地方——研究 #70）。
- 破坏性变更 +1；旧版插件继续可用一个大版本（deprecation 窗口），之后拒绝加载。
- `protocol`（#9 的 exec 线协议）与 `apiVersion`（JS 宿主面）分开：前者是传输，后者是数据契约。
- 复习记录里写 `scheduler: { family, flags, version }`：换调度插件后历史仍可读，只是不再可比；切换点作为一条边界记录写入。家族+开关的形态是刻意的（Anki 的 `schedVer` 是家族，"v3"只是 V2 上的 flag）。

## 7. 生命周期

```
discovered → enabled(能力已批准) → loaded(惰性) → failed(n) → disabled
                    ↑                                  │
                    └──────── 用户手动重新启用 ←────────┘
```

- 启用是**按库**的显式动作（沿用 #9），批准发生在启用时。
- 选中关系：每个角色（derive/render/schedule）每库选定**一个**插件，默认是内置默认插件。多个插件声明同一角色时不自动仲裁，由设置页选择。

## 8. 内置默认插件与"No legacy"

- 位置：`resources/plugins/remit.cards.standard/`（`//go:embed`），走与三方插件相同的加载路径、相同的 capability 检查，**没有特权入口**。
- 删除条件（写进 ADR 0007）：金标准 fixtures 全绿 + 内置默认插件成为唯一实现。
- 回退 = 回退到它，不是回退到第二份 Go 代码。

## 9. 怎么测一个插件

`go tool remit ext test <dir>`：用 fixtures（笔记 + 复习历史）跑全部 hook，逐字比对输出。同一套 fixtures 也是 #77 删除 Go 实现时的凭据。

## 10. v1 不做

插件市场 / 签名 / 自动更新 · 插件注入 UI · `net:fetch` · 插件间依赖 · 移动端。
