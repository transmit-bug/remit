# Remit 产品说明：3 合 1 的笔记、卡片与增量阅读

> 状态（2026-11-08）：1.0 产品定义 + post-1.0 全功能追平方向。
> 决策地图：[3 合 1 地图](https://github.com/transmit-bug/remit/issues/201)。
> 本文是稳定口径；地图里的开放决策不因本文写成已定。

## 1. 一句话

Remit 是一个本地优先的开源知识记忆工作台：完整的 Markdown 笔记、FSRS 制卡复习、
SuperMemo 式增量阅读，三者共享一个明文库、一套块模型和一条调度史。设计哲学是
**Remember it**——每个功能要么帮用户把读过的记住，要么承载这份记忆；两者都不服务的不做。

## 2. 为什么是 3 合 1

三个产品各自解决了同一件事的一段：

- **Obsidian**：写下来、连起来、留在本地。
- **Anki**：把知识变成可主动回忆的卡片，按记忆节奏复习。
- **SuperMemo**：把长文按节奏读、摘、挖空，逐步消化。

Remit 不做三个并列模块。三者在数据上共享 **Block**，在时间上共享
`reviews.jsonl`：

- **Note** = 有序 Block 的 Markdown 文档。
- **Card** = Block 的记忆角色（Card Type + Field + Template）。
- **Reading Item** = Note / Block / Range 的注意力角色（read point + priority + schedule）。
- **Excerpt** = 带源块引用的子 Note，可继续读、可制卡。
- **Capture** = 把外部文章变成普通 Note / Block / Asset 的入口。

> 注：Reading Item 是候选抽象（见地图「统一知识单元」），不是已定术语；阅读单位仍是
> Note，不是 Block / Range。**统一 Today 已落地**（混合队列 + 时间预算），口径见
> [`docs/specs/ui-redesign-v2.md`](ui-redesign-v2.md) §1.2/§5.1，实现是 B1 的 `queue.Today` 与
> W2 的 S04；
> priority 只做队列排序、不落字段（§1.3），选区摘录仍推迟（§1.4）。

## 3. 产品原则

1. **Remember it**：功能要么帮助记住（写、读、摘、制卡、复习、重访），要么承载记忆
   （明文库、插件、导出、迁移）；两者都不服务的不进核心。
2. **一套模型、一条调度史、一份明文**：不为笔记、卡片、阅读各造一套数据或交互。
3. **本地优先**：库是目录里的明文；索引可删可重建；无账号、无遥测、无默认外发。
4. **功能等价，不逐字复刻**：对 Obsidian / Anki / SuperMemo 取用户 job，用 Remit 的模型
   实现；不追求二进制兼容、专有格式或插件生态照搬。
5. **插件受限、宿主绘面**：插件只通过 manifest 声明能力，内置功能也走同一加载器。
6. **AI 在边缘**：AI 可以转换、修复、建议，但不是第二套真相，默认不联网、不直接写库。

## 4. 1.0 范围

1.0 的目标是**完整功能集**，不是三个产品的全功能追平。当前范围：

> 1.0 尚未发布；下表是 1.0 的产品范围，不是已发布版本。
>
> 这里的“完整”指产品闭环完整：写得下、读得回、摘得出、制得卡、复习得了、库能带走；
> 不是三家功能全集。全功能追平是 1.0 之后的覆盖计划。

| 层 | 1.0 范围（当前实现 + 收尾） |
|---|---|
| 笔记 | Markdown 文档、内联块 ID、双链/反链、标签、Asset、搜索、笔记模板、Markdown/Anki 导入导出 |
| 卡片 | Card Type / Field / Template、basic / bidirectional / cloze、FSRS、Review Queue、Review Scope、Anki 导入 |
| 阅读 | Reading Queue、块级阅读位置、整块摘录、三档消化、跳过/移出、统计/托盘/提醒、只读 agent API |
| 插件 | 制卡、调度、UI 贡献、importer；内置插件与第三方走同一加载器 |
| 平台 | 单窗口、本地库、`.remitlib` 归档、loopback agent API（写入需 token，不暴露删除） |

1.0 的阅读是 **v1 闭环**：note-level 阅读队列 + 块位置 + 整块摘录。它还不是
SuperMemo 的完整增量阅读；选区级摘录、就地 cloze、priority/overload、显式 read point
在 1.0 之后。

## 5. 1.0 之后：全功能追平

“追平全部功能”以 **capability parity** 为目标，不是功能克隆。做法见
[全功能 parity matrix](https://github.com/transmit-bug/remit/issues/210)：先枚举能力全集，
逐项决定 **adopt / adapt / replace / reject**，再排 horizon。

| 层 | 全功能追平方向 |
|---|---|
| 笔记 | Obsidian vault 导入/导出、frontmatter/alias/heading/path link、表格块、graph 视图、完整 Markdown 保真 |
| 卡片 | Anki 迁移保真、复杂 cloze 扁平化、制卡 ergonomics、媒体、card browser |
| 阅读 | 选区摘录、就地 cloze、read point、priority/overload、终态语义、摘录自动入队、统一 Today |
| 捕获 | HTML/SingleFile/EPUB/web 捕获、确定性转换 + AI 修复、来源元数据 |
| 平台 | 同步、移动端、多窗口按独立地图设计；保持本地优先、可选、无账号锁定 |

明确用 **adapt / replace / reject** 处理的候选：

- Anki deck → Review Scope / 保存范围，不引入 deck 容器实体。
- Anki add-on / Obsidian 插件二进制 → Remit 插件能力，不做宿主兼容层。
- SuperMemo A-Factor / 第二套调度器 → FSRS 算间隔，priority 只做排序/限额。
- SuperMemo 的破坏性 Done → Archived / Finished，不默认删除源 Note。
- 云同步/账号锁定/遥测 → 不做；本地优先的可选同步另开图。

## 6. 当前关键缺口

- **笔记**：[表格块模型](https://github.com/transmit-bug/remit/issues/218)；代码块里的
  `::` / `{{...}}` 不应被当成卡片语法；
  链接按标题还是路径解析；frontmatter/alias/heading/path link；Obsidian vault 导入/导出。
- **卡片**：复杂 cloze（同号分组、hint、嵌套）保真；Anki zstd/schema-18；
  notetype 分组；卡片创建从“手写标记”走向“选中→制卡”。
- **阅读**：选区级摘录、就地 cloze、显式 read point、priority/overload、终态语义、
  摘录自动入队；阅读队列目前也不吃 Review Scope。
- **捕获**：确定性 HTML/SingleFile MVP；URL/EPUB/AI 后置；PDF 不在 MVP。
- **队列**：卡片与阅读合成一个今日（混合队列 + 时间预算）**已落地**：B1 的 `queue.Entries`
  （返回 `queue.Today`，`internal/core/queue/today.go`）供 W2 的 S04，读 / 摘 / 忆按预算排序，
  超出预算的行带徽章；口径仍是 [`docs/specs/ui-redesign-v2.md`](ui-redesign-v2.md)
  §1.2/§5.1。本节其余缺口未变。

## 7. 明确不做

- 逐字复刻 SuperMemo 的专有算法/格式。
- 直接运行 Obsidian 插件或 Anki add-on 的二进制/API。
- 把 deck 作为新的内容容器实体。
- 三个产品各自拥有独立数据文件、调度器或同步机制。
- 遥测、默认外发、默认联网 AI。
- 以云服务/账号锁定作为产品核心。

## 8. 工作方式

- 大方向决策：wayfinder 地图（[3 合 1 地图](https://github.com/transmit-bug/remit/issues/201)）。
- 1.0 顺序与 PR 边界：[1.0 执行顺序](https://github.com/transmit-bug/remit/issues/158)。
- 五屏视觉/组件：[UI 地图](https://github.com/transmit-bug/remit/issues/128)。
- 不可逆决定：`docs/adr/`；区域契约：`docs/specs/`；回归：行为测试与 golden。
- SOP：capability universe / parity matrix → domain model / ADR → architecture / seams →
  vertical slices → dogfooding → hardening / release → parity review。

## 9. 参考

- `GLOSSARY.md`：Note / Block / Card / Card Type / Field / Template / Reading Queue /
  Reading Position / Excerpt / Review Scope。
- `docs/adr/0002-content-model.md`、`0003-storage-layout.md`、`0007-plugin-system.md`、
  `0008-card-types-templates.md`。
- `docs/specs/incremental-reading.md`、`docs/specs/plugins/`、`docs/specs/ui-redesign.md`。
- 研究分支：`research/supermemo-incremental-reading`、`research/obsidian-anki-core`、
  `research/article-capture`（不合并；结论已进地图 Decisions）。
