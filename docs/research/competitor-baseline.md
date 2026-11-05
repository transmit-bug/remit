# 竞品事实基线：Anki / RemNote / Obsidian / Logseq / SuperMemo

> 工单：transmit-bug/remit#5（父工单 #1 Wayfinder 地图）。本文为内容模型与架构票提供事实基线，重点是「笔记↔卡片关系、数据格式开放度、扩展机制、用户主要抱怨」，结论服务于「取两者之长」的具体化。
> 调研日期：2026-11-06。标注【事实】的均有来源链接；标注【推断】的是基于事实的推断。

---

## 1. Anki

### 1.1 数据模型（note / card / deck / notetype）

【事实】核心三层抽象：

- **Note（笔记）**：一条信息记录，由若干**字段（fields）**组成，字段内容是 HTML。
- **Note type（笔记模板）**：定义字段集合 + 卡片模板（正反面模板 + 样式 CSS），一个 notetype 可以按「条件模板」生成多张卡（如正反两向卡、cloze 挖空自动生成多卡）。
- **Card（卡片）**：note 经 notetype 渲染出的可复习单元，携带独立的调度状态（due、interval、ease、lapses 等）。**卡片是调度对象，笔记是内容对象，二者严格分离**。
- **Deck（牌组）**：卡片的层级容器；v3 调度器下还有 notetype 级别的预设选项组（deck presets）。

数据落盘为 SQLite（collection.anki2 / 新版 anki_bazi 引擎），表结构以 `col`/`notes`/`cards`/`revlog` 为主，社区有较完整的表结构文档（AnkiDroid wiki：https://github.com/ankidroid/Anki-Android/wiki/Database-Structure）。

交换格式 **.apkg**：本质是 zip，内含 `collection.anki2` SQLite 库 + 媒体文件清单（历史上有 Legacy 1 / Legacy 2 / 最新带 protobuf 的 `.anki_bazi` 三代，见 https://eikowagenknecht.com/posts/understanding-the-anki-apkg-format/）。官方手册导出页：https://docs.ankiweb.net/exporting.html、https://docs.ankiweb.net/importing/packaged-decks.html。

【推断】.anki2/.apkg「开放」是事实层面的（格式公开、SQLite 可直接读），但**不是为人读写设计的**：调度数据与内容混在一个二进制库里，卡片正反面靠 notetype 模板动态渲染而非显式存储——这是 Anki 数据可移植性优于闭源竞品、但不如纯文本方案直观的原因。

### 1.2 调度器（v2 → v3 → FSRS）

- 【事实】现行唯一调度器是 **V3（2021 scheduler）**，v2 已移除（官方 FAQ：https://faqs.ankiweb.net/the-2021-scheduler.html；dev 文档 `anki.scheduler.v3`：https://dev-docs.ankiweb.net/en/latest/autoapi/anki/scheduler/v3/index.html）。
- 【事实】**FSRS 已内置**：Anki 23.10 起原生集成 FSRS，可选择 FSRS 或 SM-2 类旧算法（官方 FAQ：https://faqs.ankiweb.net/what-spaced-repetition-algorithm；23.10 release notes：https://docs.ankiweb.net/releases/changes/changes/23.10）。FSRS 原为 L.M.Sherlock 的 FSRS4Anki 插件（https://forums.ankiweb.net/t/new-features-of-fsrs4anki-from-v3-0-0-to-v3-6-0/23915），后收编进核心。
- 【事实】v2→v3 迁移曾造成用户 interval 混乱（论坛求助帖：https://forums.ankiweb.net/t/intervals-changed-after-updating-from-v2-to-v3/65934）。
- 【推断】这印证地图既定决策「调度算法=FSRS」是与上游主流一致的：Anki 官方已经把 FSRS 变成默认路径，RemNote/SR 插件社区也在跟进 FSRS。自研调度没有生态意义。

### 1.3 插件机制与痛点

- 【事实】插件（add-ons）是 Python 包，通过 hook/monkey-patch 深入 aqt（Qt GUI 层）和 anki（后端库），官方文档：https://docs.ankiweb.net/addons.html、https://addon-docs.ankiweb.net/。GUI 层 API 非稳定契约，官方明说 add-on 可能随版本破坏。
- 【事实】主要痛点即「升级即碎」：Reddit 上有高赞帖「有任何插件就别今天就更新 Anki」（https://www.reddit.com/r/Anki/comments/nmcryv/），以及每次大版本后社区流传的「修复损坏插件」帖子（https://www.reddit.com/r/Anki/comments/1llgydv/）；论坛上插件更新/兼容问题持续不断（例：https://forums.ankiweb.net/t/anki-add-on-updates-are-crashing-via-flatpak/70189）。
- 【推断】根因是插件与内部 Python/Qt 结构强耦合、无稳定 ABI/声明式扩展点。这是 remit 扩展机制设计上最该避开的坑：**扩展应面向稳定的数据/协议层，而非 GUI 内部结构**。

### 1.4 渲染栈为何「重」

- 【事实】Anki 桌面 = Python + Qt，卡片与大量界面通过 **Qt WebEngine（内嵌 Chromium）** webview 渲染（dev 文档 `aqt.webview`：https://dev-docs.ankiweb.net/en/latest/autoapi/aqt/webview/index.html；作者 dae 论及用 web 技术渲染整个界面的方向：https://forums.ankiweb.net/t/drop-multiple-qtwebengines-in-a-single-window/129）。WebEngine 排除旧机/远程桌面、软件渲染下编辑卡明显卡顿（https://forums.ankiweb.net/t/slow-card-editing-on-software-display-backend-on-windows/66754），也有 WebEngine 网络怪癖（https://github.com/ankitects/anki/issues/4230）。
- 【推断】「重」来自：Python 解释器 + Qt + 内嵌 Chromium 三层叠加；卡片模板又是任意 HTML/CSS/JS，导致渲染面完全动态化。这是为了模板自由度付出的架构代价。

### 1.5 Anki 小结（对 remit 的「长」）

- 卡片/笔记分离 + notetype 模板系统的表达力；FSRS 原生调度；开放 SQLite/apkg。
- 反面教训：二进制单一 collection 库（非本地优先友好）、插件 ABI 脆弱、渲染栈重。

---

## 2. RemNote

### 2.1 概念模型：文档/块/卡片/portal 统一

- 【事实】RemNote 的最小单元是 **Rem（块）**；一切内容（文档、文件夹、知识）都是 Rem 树上的节点（官方总览：https://help.remnote.com/en/articles/6044066-remnote-in-5-minutes；术语：https://help.remnote.com/en/articles/8196578-outlines-and-terminology）。
- 【事实】**卡片即笔记块的内嵌属性**：在块内打 `::`（Concept/Descriptor）或 `;;`（双向卡）即把该块的「术语→定义」关系变成卡片，卡片不是独立实体而是块的衍生视图（创建卡片：https://help.remnote.com/en/articles/6025481-creating-flashcards、https://help.remnote.com/en/articles/6751778-creating-concept-descriptor-flashcards）。块可加 cloze。练习时按「文档包含哪些卡片」聚合（https://help.remnote.com/en/articles/8892109-how-does-remnote-decide-what-flashcards-are-part-of-a-document）。
- 【事实】**Portal**：把某 Rem 的子树引用投影到另一处显示，flashcard 也会跟随 portal 生效（portal 折叠会影响卡片收录）：https://help.remnote.com/en/articles/6030742-portals、https://help.remnote.com/en/articles/6634227-what-s-the-difference-between-references-tags-and-portals；还有按查询聚合的 Search Portal（https://help.remnote.com/en/articles/7231624-search-portals）。

### 2.2 数据形态

- 【事实】**云端为主**：知识库默认存在 RemNote 云端并同步；桌面可建本地 KB 存任意路径，但移动端无法打开本地 KB（官方反馈帖：https://feedback.remnote.com/p/offline-kb-on-mobile）；每日自动备份本地+云端各一份（https://help.remnote.com/en/articles/6301627-remnote-backups）。
- 【事实】导出：单文档可导出 Markdown/纯文本等，带预览（https://help.remnote.com/en/articles/7898019-exporting-notes）；完整备份是 RemNote 私有格式文件，仅供导入回 RemNote。
- 【事实】用户抱怨导出损失大：Reddit「Awful Exporting…(List of unsupported things to export)」列举 portal、嵌套结构等导出不完整（https://www.reddit.com/r/remNote/comments/1iytzoe/）；同步/性能/稳定性有官方长期收集帖，大型库加载慢至秒级（https://forum.remnote.io/t/collecting-feedback-for-sync-or-performance-or-stability-issues/9135）；1.8 版同步事故（https://www.reddit.com/r/remNote/comments/vxw9kn/）。

【推断】RemNote 证明了「笔记=卡片同源、块级粒度」的模型可行且体验好；但云端私有格式 + 导出不完整是它最大的信任赤字。remit 取其长（块级概念模型、`::` 式卡片语法）而以本地开放格式落地，正面击中其弱点。

---

## 3. Obsidian（.md 生态 + SR 插件）

- 【事实】Obsidian 本体只做纯 Markdown 文件夹（本地优先的事实标准之一），SR 由社区插件补齐。主流插件 **obsidian-spaced-repetition**（st3v3nmw，2.6k+ stars）：支持 FSRS/SM-2，卡片语法是 Markdown 内 `问题::答案` 单行卡、`?` 多行卡、`==高亮==` cloze，整笔记复习；来源：https://github.com/st3v3nmw/obsidian-spaced-repetition。
- 【事实】**调度数据存储**：卡片调度写在卡片下方 HTML 注释（`<!--SR:!2024-08-16,51,230-->`），笔记级复习数据写在 YAML frontmatter（`sr-due/sr-interval/sr-ease`）；把调度数据移出笔记、集中到插件数据文件的 PR 直到 2025 年左右才合并（data-storage 文档：https://github.com/st3v3nmw/obsidian-spaced-repetition/blob/main/docs/docs/en/data-storage.md；PR #1471：https://github.com/st3v3nmw/obsidian-spaced-repetition/pull/1471）。
- 【事实】局限：卡片必须靠分隔符约定在笔记内手工标出，无 notetype 级模板抽象；早期「一文件多卡」就是长期 issue（https://github.com/st3v3nmw/obsidian-spaced-repetition/issues/20）；调度注释污染正文、多卡共享调度状态等结构问题引发大量讨论（例 #741：https://github.com/st3v3nmw/obsidian-spaced-repetition/issues/741）。
- 【推断】Obsidian 路线的本质矛盾：**把调度状态塞进内容文件**（frontmatter/HTML 注释）在「数据全在 .md」与「调度数据干净分离」之间只能二选一，两边都不干净。remit 若内容用开放文本格式，应一开始就把调度状态放独立存储（如 SQLite sidecar），而不是重蹈注释式存储。这与 Anki 的教训（内容与调度同库但至少分表）可以折中。

---

## 4. Logseq

- 【事实】块级大纲 + 本地 Markdown/Org 文件（outliner 模型，每块是 bullet）。**卡片 = 带有 `#card` 或 `[[card]]` 标签的块**，答案是子块；复习状态以块属性（`card-last-interval`、`card-ease-factor`、`card-next-schedule` 等）直接存在块的 properties 里。官方文档（源码）：https://github.com/logseq/docs/blob/master/pages/Flashcards.md。
- 【事实】调度算法为 SM-2 变体（文档附加链接直接指向 SM5/SM-2 相关资料，社区普遍反馈其调度不可换、功能简陋）；复习中「忘了」不会像 Anki 一样当场重现，社区求助帖：https://discuss.logseq.com/t/flashcards-learning/14691；卡片无法按块分组复习等长期 FR：https://discuss.logseq.com/t/card-groups-or-cards-grouped-by-block/5249。
- 【推断】Logseq 的启示是双面的：块即卡片（与 RemNote 同源的洞察）+ 数据在本地开放文本文件——这正是 remit 想要的组合；但 Logseq 把调度属性内联进 .md 块属性，与 Obsidian SR 插件同病（污染内容、diff 噪音、工具间互扰）。此外 Logseq 卡片功能长期半维护状态，说明 outliner 数据模型本身不自动解决调度质量问题。

---

## 5. SuperMemo（渐进阅读）

- 【事实】**渐进阅读（Incremental Reading, IR）**：把大量电子文章/材料导入知识库，按优先级与记忆调度「切碎地、增量地」阅读——每次只读一小段，摘录（extract）关键句、再逐步把摘录改写为 cloze/问答卡；阅读优先级和卡片复习由同一调度系统统一排队。官方定义与步骤：https://supermemo.guru/wiki/Incremental_reading、https://www.supermemo.guru/wiki/Incremental_reading_step_by_step、最小定义：https://www.supermemo.guru/wiki/Minimum_definition_of_incremental_reading。
- 【事实】IR 的核心价值主张：阅读→摘录→制卡的**连续谱系**，材料随时间自动降优先级、知识逐步「蒸馏」成卡片（20 rules of knowledge formulation：https://www.supermemo.guru/wiki/20_rules_of_knowledge_formulation）。
- 【事实】用户主要抱怨：UI 老旧、Windows-only、学习曲线陡峭、贵（Reddit r/Anki 讨论串：https://www.reddit.com/r/Anki/comments/iziyaj/；HN 讨论：https://news.ycombinator.com/item?id=24737821；深度评测与劝退文：https://masterhowtolearn.wordpress.com/2018/10/28/why-i-switched-to-supermemo-after-using-anki-for-5-years-with-over-50000-cards-and-420000-total-reviews/）。社区甚至出现了「我们自己的 SuperMemo」诉求文（https://pleasurable-learning.com/view/A_SuperMemo_of_our_own）。Obsidian 生态已有模仿 IR 的插件（incremental-reading-obsidian，FSRS+优先级队列：https://github.com/gpanakkal/incremental-reading-obsidian）。
- 【推断】IR 是 SuperMemo 独有而 Anki 系普遍缺失的能力，且社区存在明确需求缺口。对 remit：**笔记↔卡片之间的中间态（阅读队列、摘录、优先级）值得在内容模型里预留位置**，但这是产品层决策，本文只提供事实：需求与竞品缺口都存在。

---

## 6. 综合结论：取两者之长的具体化

| 维度 | 最佳事实来源（长） | 具体化建议（【推断】，供 #8 内容模型票采纳） |
|---|---|---|
| 笔记↔卡片关系 | RemNote：卡片是块的衍生属性（`::`），非独立实体 | 内容（块）为唯一真源；卡片 = 块上声明的可复习投影（含 cloze），避免 Anki「note 经模板生成 card」的双层间接性带来的模板地狱，也避免 Obsidian「分隔符约定」的脆弱性 |
| 数据格式 | Anki 的开放 SQLite/apkg；Obsidian/Logseq 的开放 .md；RemNote 的封闭云 | 内容用开放文本（.md 系）；**调度状态独立存储**（Anki 式专用库/sidecar），不走 Logseq/SR 插件的内联属性老路 |
| 调度 | Anki 官方已内置 FSRS（23.10+） | FSRS 为唯一/默认调度，与地图决策一致；无冲突事实 |
| 扩展机制 | Anki add-on 因耦合 Qt/内部 API 而碎 | 面向稳定数据层/协议扩展，不暴露 GUI 内部结构 |
| 渲染 | Anki 重在 Python+Qt+内嵌 Chromium | mygo 原生渲染 + 受控卡片模板（限定 HTML/CSS 子集）可避开此重栈 |
| 渐进阅读 | SuperMemo 独有能力，社区有明确缺口 | 内容模型预留「阅读项/摘录/优先级」中间态的位置 |

与地图既定决策的冲突：**无**（FSRS、本地优先、mygo GUI 均被竞品事实正向支持）。

## 主要来源索引

- Anki 手册/FAQ/Dev docs：docs.ankiweb.net、faqs.ankiweb.net、dev-docs.ankiweb.net；AnkiDroid DB wiki；apkg 格式解析（eikowagenknecht.com）
- Anki 论坛/Reddit：forums.ankiweb.net（v2→v3、FSRS4Anki、WebEngine、插件问题）、r/Anki（插件碎裂 PSA、SuperMemo 对比）
- RemNote：help.remnote.com（概念/portal/导出/备份）、feedback.remnote.io、forum.remnote.io、r/remNote（导出/同步抱怨）
- Obsidian：github.com/st3v3nmw/obsidian-spaced-repetition（README、data-storage、issue #20/#741、PR #1471）、incremental-reading-obsidian
- Logseq：github.com/logseq/docs（Flashcards.md）、discuss.logseq.com
- SuperMemo：supermemo.guru（IR 定义/步骤/最小定义/20 rules）、HN #24737821、Reddit 劝退串、masterhowtolearn 评测
