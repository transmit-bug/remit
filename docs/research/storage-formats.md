# 数据格式与存储选型：事实调研（工单 #7）

- 工单：[#7 开放数据格式与存储选型事实](https://github.com/transmit-bug/remit/issues/7)（父工单：Wayfinder 地图 #1）
- 分支：`research/storage-formats`（本分支，永不合并）
- 日期：2026-11-06；调研者：storage-formats 子代理
- 方法：只采信一手来源（官方文档、源码/Schema、官方公告、官方 FAQ），社区讨论仅作补充并标注。本地实测在本机执行（macOS 25.6.0, arm64, APFS），标注【实测】。
- 标记：**【事实】**=来源可查；**【推断】**=由事实推导，非直接来源；**【实测】**=本机复现。

---

## 0. TL;DR

三种候选都有成熟的、跑在生产上的先例：**纯文件树**（Obsidian、Logseq OG）、**纯 SQLite 单库**（Anki、Joplin、Logseq DB/2.0）、**混合：明文源文件 + 派生索引**（SiYuan：`.sy` JSON 文件为权威数据，SQLite 只做可重建索引）。事实最支持的两个并列候选：

- **候选 A（混合，SiYuan 模式）**：明文（markdown/JSON）文件为唯一权威数据，文件名/块内嵌 ID 为稳定标识；SQLite 索引放 temp 类目录、可随时重建、不进 Git。人类与 agent 直接读写明文，Git diff/merge 有意义，10 万级查询靠索引满足。抗移动/重命名靠「ID 不随路径变化」。
- **候选 B（SQLite 单库，Anki/Logseq 2.0 模式）**：查询与调度性能、增量同步（usn/mod 时间戳）事实最强；代价是 Git 不可 diff、人类不可直接读、同步冲突面集中为整库文件（Anki 用专门协议+服务器解决，RemNote/Logseq 用云端/专用同步解决）。

Logseq 从文件树迁移到 SQLite 的官方动机清单（性能、持久 ID、时间戳、同步丢数据）几乎逐条对应本工单的五个 Question，是对两条路线最权威的「对照实验」【事实】。与地图既定决策（开源、本地优先、agent 可编程、同步后议）无冲突事实。

---

## 1. 参照系产品怎么存（一手来源）

| 产品 | 存储形态 | 权威数据落点 | 索引/缓存 | 来源 |
|---|---|---|---|---|
| Obsidian | markdown 文件树 | vault 目录下的 `.md` 明文 | 内部缓存（不落 vault；移动端曾因 IndexedDB 不落盘拖慢大库启动） | [How Obsidian stores data](https://obsidian.md/help/data-storage)、[forum#88470](https://forum.obsidian.md/t/unpractical-vault-load-time-for-large-vaults-on-mobile-indexeddb-transactions-are-not-flushed-to-disk/88470) |
| Logseq OG（文件版） | markdown/org 文件树 | pages/journals 下的 `.md`，块 ID 写进 `id::` 属性 | 启动时在内存中重建 DB | [logseq/docs Block Reference](https://github.com/logseq/docs/blob/master/pages/Block%20Reference.md)、[迁移公告](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744) |
| Logseq DB / 2.0 | SQLite 单库（本地） | `db.sqlite` 为 canonical store；2026-05 起支持 DB↔markdown 往返导出 | 同库 | [Why the database version](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744)、[DB Changelog](https://discuss.logseq.com/t/logseq-db-changelog/30013/37) |
| Anki | SQLite 单库 | 单个 `collection.anki2`：cards/notes/revlog/col/graves 五张核心表 | 同库（SQLite 索引） | [AnkiDroid Database-Structure wiki](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure) |
| Joplin | SQLite 单库 | `database.sqlite`（全部笔记/标签/同步状态/E2E 密钥），附件在 `resources/` | 同库 | [Joplin user profile spec](https://github.com/laurent22/joplin/blob/dev/readme/dev/spec/user_profile.md) |
| SiYuan 思源 | 混合 | `data/` 下 `.sy` JSON 文件是权威内容；**文件名即块 ID**，目录结构即文档层级 | `temp/` 下 SQLite（siyuan.db、blocktree.db 等）为**可重建索引**，删了不影响源数据 | [SiYuan WORKSPACE.md](https://github.com/siyuan-note/siyuan/blob/master/docs/WORKSPACE.md) |
| RemNote | 云端优先（非本地文件） | 服务器存储（可导出 RemNote Complete/OPML/Anki apkg/HTML/Markdown/Text；本地 KB 存在但媒体仍在服务器） | — | [Exporting Notes](https://help.remnote.com/en/articles/7898019-exporting-and-printing-notes)、[Importing from Anki](https://help.remnote.com/en/articles/6751471-importing-from-anki) |

关键原文引述：

- SiYuan（混合形态的定义性描述）【事实】：「The file system contains the authoritative source content, while SQLite databases under `temp/` are rebuildable indexes and caches.」「The filename is the ID; the directory structure is the document hierarchy.」（[WORKSPACE.md §0](https://github.com/siyuan-note/siyuan/blob/master/docs/WORKSPACE.md)）
- Obsidian（文件树形态的定义性描述）【事实】：「Obsidian stores your notes as Markdown-formatted plain text files in a vault. A vault is a folder on your local file system… you can use other text editors and file managers… Obsidian automatically refreshes your vault to keep up with any external changes. Obsidian syncs with Obsidian Sync, Dropbox, iCloud, OneDrive, Git…」（[data-storage](https://obsidian.md/help/data-storage)）
- Logseq（为什么放弃纯文件树作为主存储）【事实】：markdown 文件上做实时协作/大库的三个痛点——「Creating a new block requires rewriting the entire Markdown file」「Renaming a page updates all files that reference it」「structure data support is limited compared to a database, lacking features like persistent IDs, timestamps」；旧版问题包括「Data loss when using Logseq sync with multiple clients」「Poor performance with large graphs」；DB 版目标是「effortlessly supporting up to 50,000 pages」，且长期目标为 DB 与 markdown 文件双向同步（[官方公告 2024-04](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744)）。

---

## 2. 事实矩阵

✅=好 ❌=差 ⚠️=有代价/依赖做法。每格依据见 §3 对应小节。

| 维度 | A. markdown 文件树 | B. SQLite 单库 | C. 混合（明文源 + 派生索引） |
|---|---|---|---|
| 人类可读 | ✅ 明文，任何编辑器 | ❌ 需 sqlite3/工具，BLOB/转义字段难读 | ✅ 源文件明文（SiYuan 用 JSON；若用 markdown 更贴 Obsidian 生态） |
| 人类可写（不经 app） | ✅ Obsidian 官方支持外部编辑并自动刷新 | ⚠️ 可写（SQL），但绕过应用约束易破坏一致性（Logseq 团队在公告下的态度） | ✅ 源文件可外部写，但须走 app/API 让索引跟进（SiYuan 明言：变更优先走 HTTP API/MCP/CLI，直接改文件只用于批量离线迁移） |
| Agent 可编程 | ✅ cat/sed/grep/任意 LLM 工具链 | ⚠️ sqlite3 CLI 处处可用；MCP sqlite server 曾官方提供（现归档）；难点同上 | ✅ 读明文 + 变更走 CLI/API；SiYuan 官方把 MCP 列为推荐变更通道 |
| Git 友好 | ✅ 行级 diff/merge；obsidian-git 插件成熟 | ❌ 二进制：diff 无意义、文件级冲突不可合并；需 dump-filter 类辅助 | ✅ 源文件 diff/merge 有意义；索引库 .gitignore 掉即可（可重建） |
| 同步：增量 | ✅ 文件级（Syncthing 按块传输、重命名不重传） | ✅ 行级（Anki 按 usn 增量上下行） | ✅ 源文件粒度同 A；索引不参与同步 |
| 同步：时间戳 | ⚠️ 文件系统 mtime（Syncthing 同步 mtime；语义弱） | ✅ 每行 mod/usn（Anki） | ⚠️ 明文内嵌 frontmatter 时间戳可做，须自定 |
| 同步：冲突面 | ⚠️ 单文件冲突可文本合并（Obsidian Sync 用 diff-match-patch；Syncthing 保留 `.sync-conflict` 副本）；通用文件同步无合并 | ⚠️ 整库文件=最大冲突面（不可文本合并）；Anki 用专用协议把冲突拆到对象级+服务器仲裁；SQLite 放网络盘会损坏 | ⚠️ 同 A（明文可合并）；索引冲突无意义（重建即消） |
| apkg 导入/导出 | ⚠️ 需解析 apkg 内 SQLite 再写入文件 | ✅ apkg 本身就是 SQLite+ZIP，Anki schema 公开，转换近无损 | ✅ 同 B（索引侧做），或经源文件 |
| RemNote 导入/导出 | ⚠️ 走 RemNote 的 markdown/OPML 导入；RemNote 官方对 Anki 单向支持最好 | ⚠️ 同左，apkg 通道最完整 | ✅ 同上（双通道都保留可能） |
| markdown 导入/导出 | ✅ 即本身 | ⚠️ 需自研序列化（Anki 文本导出仅 TSV+HTML；Joplin/Logseq DB 已证明可行） | ✅ 源即明文（或定义导出器） |
| 10 万卡片查询/调度 | ❌ 纯文件树无索引（grep 实测 1 万文件 0.16s，10 万≈1.6s 级、且无 due 队列语义）；Logseq OG 大库卡顿为证 | ✅ 实测：13.3 万卡片 due 队列查询 0.3ms、500 行复习更新 4ms；Anki >10 万卡浏览器检索 <1s | ✅ 索引承担查询（同 B 的数字量级），源文件承担人类读写 |
| 启动（大库） | ⚠️ 全量索引重建：实测 1 万文件 0.14s，线性外推 10 万≈1.4s【实测+推断】；Obsidian 13 万笔记 vault 可用（graph view 挂），5 万笔记有卡顿报告；Logseq OG 300MB 图启动失败 | ✅ 打开即查：实测冷启动+count 0.6ms | ✅ 同 B（索引在手）/A（需重建时同左列） |
| 「卡片引用笔记内容」表示 | 文本内嵌：Obsidian `[[note#^blockid]]`（ID 落在目标块行尾）/Logseq `((uuid))`（ID 落在目标块 `id::` 属性） | 外键/ID 列：Anki card.nid→note（仅卡↔本笔记字段，无跨笔记引用）；RemNote 引用=rem ID 双向链接 | 源文件内嵌 ID（同 A 的语义）+ 索引里反查表（同 B 的性能） |
| 引用抗移动/重命名 | ⚠️ 按名字/路径寻址（Obsidian wikilink）：app 内改名会全库改写链接【事实】，app 外改名链接断裂【事实】；按块 ID 寻址（Logseq/`^blockid`）则天然抗改名，但 Logseq 指出「页面改名要改写所有引用它的文件」 | ✅ ID 寻址与路径无关（RemNote：目标文本变了引用自动更新；Anki nid 不受 deck 移动影响） | ✅ SiYuan：文件名=ID，移动文档只改 HPath（派生），ID 不变；引用按 ID |

---

## 3. 分维度事实与来源

### 3.1 人类/Agent 可读写性

- **【事实】** Obsidian：明文 markdown + 官方明确支持外部工具编辑、自动刷新外部变更（[data-storage](https://obsidian.md/help/data-storage)）。这是「agent 用任意编辑器直接改库」的产品级先例。
- **【事实】** Anki：全部数据在单个 SQLite，`notes.flds` 字段以 `\x1f` 分隔符拼在一个 TEXT 列里，模板/牌组配置为 col 表 JSON 列（[AnkiDroid schema](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure)）。人类裸读可行但体验差；程序化读写=普通 SQL。
- **【事实】** SiYuan：源数据 `.sy`（JSON）+ 明确的「变更优先走 API/MCP/CLI，直接改文件仅限批量离线迁移」的官方边界（[WORKSPACE.md §11](https://github.com/siyuan-note/siyuan/blob/master/docs/WORKSPACE.md)）——即混合形态下 agent 的正确姿势是「读明文、写走 CLI」。
- **【事实】** MCP 官方参考实现曾含 sqlite server（现移入 archived，[servers-archived/src/sqlite](https://github.com/modelcontextprotocols/servers-archived/tree/main/src/sqlite)）——SQLite 对 agent 并非不可达，只是比明文多一层工具依赖。

### 3.2 Git 友好度

- **【事实】** 文件树：Obsidian 官方文档直接给出 vault 用 Git 的建议（`.gitignore` 掉 `workspace.json`），并列举 Git 为可用同步方式之一（[data-storage](https://obsidian.md/help/data-storage)）；社区 obsidian-git 插件长期维护（[Vinzent03/obsidian-git](https://github.com/Vinzent03/obsidian-git)）。
- **【事实】** SQLite 单文件是二进制：行级变更在 git diff 中不可读、不可三方合并；SQLite 官方对「边开事务边拷贝」明确警告会产生损坏的备份（[How To Corrupt §1.2](https://www.sqlite.org/howtocorrupt.html)）——对「Git 定时自动提交」类流水线是真实风险（须用 `VACUUM INTO`/backup API/sqlite3_rsync 产出一致性快照，同源文档）。
- **【推断】** 混合形态把上述二者取并集：源文件走 Git，索引库忽略；索引与源的偏差通过重建消除（SiYuan 语义，WORKSPACE.md §0）。

### 3.3 同步友好性（只要事实，CRDT 留给同步票）

- **【事实】** Anki（SQLite 路线的同步先例）：每对象 `usn`（update sequence number，-1=待上传）+ `mod` 时间戳 + `graves` 删除表，构成增量 diff 的全部机制（[schema](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure)）；官方文档描述冲突语义：复习与笔记编辑可合并；同一张卡两设备各复习一次→两条 revlog 都保留、卡片取最近作答状态；schema 级变更（加字段/删模板）不可合并→强制单向同步二选一（[docs.ankiweb.net Syncing](https://docs.ankiweb.net/syncing.html)）。
- **【事实】** AnkiWeb 硬限额：压缩后 100MB / 未压缩 250MB；官方口径「25,000 张平均卡片+数年复习史 ≈ 25MB」（[faqs.ankiweb.net（经论坛原文引用）](https://forums.ankiweb.net/t/can-anki-handle-more-cards/57864)）。**【推断】** 10 万张平均卡片 ≈ 100MB，逼近云端同步上限——单库路线的同步代价随库体积线性上升。
- **【事实】** 文件树同步（通用文件同步器）：Syncthing 按块传输、重命名不重传、同步文件 mtime（[FAQ](https://docs.syncthing.net/users/faq.html)）；冲突处理=把较旧一方改名为 `*.sync-conflict-<date>-<time>-<device>.*` 保留两份（[syncing 文档](https://docs.syncthing.net/users/syncing.html)）。
- **【事实】** 文件树同步（厂商私有）：Obsidian Sync 对 `.md` 用 Google diff-match-patch 做行级自动合并，非文本文件 last-modified-wins，设置类 JSON 按键合并；1.9.7 起可改为保留冲突副本；并承认「两设备几乎同时新建同名文件时可能不合并、保留远端」（[Troubleshoot Obsidian Sync](https://help.obsidian.md/help/Obsidian+Sync/Troubleshoot+Obsidian+Sync)）。
- **【事实】** SQLite 放在网络文件系统（NFS/网盘目录）上直接多端打开有锁损坏风险（[How To Corrupt §2.1](https://www.sqlite.org/howtocorrupt.html)）——「单库+Dropbox 式同步」不可行，必须走专门协议（Anki 模式）或导出-导入。
- **【事实】** Logseq 官方承认文件版自研同步「多客户端下丢数据」，DB 版将 RTC（实时协作）纳入设计前提（[公告](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744)）。

### 3.4 导入导出（apkg / RemNote / markdown）

- **【事实】** apkg 结构：ZIP 内含 `collection.anki2`（SQLite）+ media 清单；新版格式（默认）用 zstd 压缩+protobuf，legacy 版兼容旧客户端；`.colpkg`=整库快照（导入即整库替换），`.apkg`=牌组包（按 note guid 匹配、`mod` 新者胜出合并）（[docs.ankiweb.net Exporting](https://docs.ankiweb.net/exporting.html)、[AnkiDroid wiki](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure)、[anki-apkg-export deck-format.md（对照 Anki 26.8.1 实测整理）](https://github.com/shbernal/anki-apkg-export/blob/master/docs/reference/deck-format.md)）。
- **【事实】** 生态库：Python 端 [genanki](https://github.com/kerrickstaley/genanki)（2.8k star，MIT）可完整生成 apkg，文档明确 guid 语义（同 guid 再导入=更新旧卡）；JS 端 [anki-apkg-export](https://github.com/shbernal/anki-apkg-export)；即「程序化产出 apkg」是被验证过的成熟路径。
- **【事实】** RemNote：官方支持导入 apkg（含调度信息与复习史），并明示限制——不导 CSS/JS 模板、部分 TTS 不可用、导入器标签选择上限 2000 个（[Importing from Anki](https://help.remnote.com/en/articles/6751471-importing-from-anki)）；导出格式含 RemNote(Complete)/OPML/Anki apkg/HTML/Markdown/Text，且 Complete 格式「不含图片/PDF（媒体在 RemNote 服务器）」（[Exporting Notes](https://help.remnote.com/en/articles/7898019-exporting-and-printing-notes)）。
- **【事实】** Anki 的 markdown 能力：原生只有「Notes in Plain Text」TSV（内嵌 HTML）导出/回导；markdown→Anki 靠社区件（如 [Obsidian_to_Anki](https://github.com/Pseudonium/Obsidian_to_Anki)）（[Exporting](https://docs.ankiweb.net/exporting.html)）。
- **【推断】** 对我们的含义：若数据源是 markdown/明文，进 Anki 生态（apkg）与进 RemNote（apkg/markdown）都有现成格式与参考实现；若数据源是 SQLite，同样可行（apkg 本就是 SQLite），只是导出器要自己写或借 genanki 类库。**导入导出难度对 A/B/C 三候选差异不大，差异在「源格式到中间表示的映射工作量」。**

### 3.5 性能（10 万卡片级；实测优先）

**本机实测【实测】**（sqlite3 via Python，100,000 notes / 133,334 cards，74MB 库，APFS/arm64；脚本逻辑：随机英文词组模拟卡片内容，schema 仿 Anki：notes(fields 以 \x1f 拼接) + cards(due/state/reps) + 调度索引）：

| 操作 | 耗时 |
|---|---|
| 全量写入 10 万 notes+13.3 万 cards（含建索引） | 1.27 s |
| 冷启动 open + `count(*)` | 0.6 ms |
| `count(*) WHERE state=2 AND due<=?`（今日队列计数） | 0.3 ms |
| `LIKE '%词%'` 前缀扫描（LIMIT 200 命中即停） | 0.1 ms |
| FTS5 全文 MATCH（建好索引后） | 0.1 ms |
| 一次复习会话：更新 500 行 cards（reps/mod） | 4.0 ms |

对照文件树【实测】：10,000 个 ~1KB markdown 文件，冷全量读取（=索引重建）0.14 s，`grep -rl` 全库词搜索 0.16 s；**【推断】**线性外推 10 万文件 ≈1.4 s / 1.6 s——纯文件树在大库下「每次冷启动重建」成本在秒级，且无 due 队列/状态查询语义。

外部佐证：

- **【事实】** Anki：用户报告 >10 万卡库中浏览器检索 <1 秒（[forum#17863](https://forums.ankiweb.net/t/default-browser-search-to-current-card-note-only/17863)）；官方口径 5 万卡库的卡顿通常与库体积无关（[forum#57864](https://forums.ankiweb.net/t/can-anki-handle-more-cards/57864)）；AnkiWeb 25MB≈2.5 万平均卡（见 §3.3）——单库瓶颈在体积配额而非查询。
- **【事实】** SQLite 官方：小 blob 从 SQLite 读取比文件系统快 ~35%（[35% Faster Than The Filesystem](https://www.sqlite.org/fasterthanfs.html)）——「把每个块存成独立文件」在性能上劣于入库（或入索引）。
- **【事实】** Logseq OG（文件树+内存 DB）：>1000 页搜索变慢、编辑大页卡顿（[discuss#4317](https://discuss.logseq.com/t/logseq-need-to-focus-on-performance-and-stability-rather-than-new-features/4317)、[issue#8137](https://github.com/logseq/logseq/issues/8137)），>300MB 图启动失败（[issue#11236](https://github.com/logseq/logseq/issues/11236)）；DB 版（SQLite）被开发者描述为「performance is hilarious great」，官方目标 5 万页毫无压力（[issue#8137 评论](https://github.com/logseq/logseq/issues/8137)、[公告](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744)）。
- **【事实】** Obsidian（文件树+自建缓存索引）：13 万笔记开源 vault（[nixpkgs-vault](https://github.com/osbm/nixpkgs-vault)）日常可用但 graph view 崩（[forum#106287](https://forum.obsidian.md/t/obsidian-graph-view-doesnt-work-for-a-large-vault/106287)）；5 万笔记有导入后卡顿报告（[forum#82241](https://forum.obsidian.md/t/help-obsidian-lags-with-many-notes/82241)）；移动端大库启动慢曾为 IndexedDB 刷盘问题（[forum#88470](https://forum.obsidian.md/t/unpractical-vault-load-time-for-large-vaults-on-mobile-indexeddb-transactions-are-not-flushed-to-disk/88470)）。
- **【推断】** 结论：10 万卡级的「查询/队列/复习写」必须走带索引的存储（B 或 C 的索引侧），纯文件树（A 裸奔）只能靠重建后内存查询且语义要自建——Obsidian/Logseq OG 都为此长期维护一套自研缓存索引，即事实上演化为 C 的变体，只是索引不落明文。

### 3.6 「卡片引用笔记内容」的表示与抗移动/重命名

产品做法对照：

- **Anki（单库，外键式）**【事实】：card.nid → note.id，卡片渲染 note 的字段；**schema 中不存在跨笔记引用表**（[schema](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure)）；跨卡互链长期是社区需求、由 add-on 补足（[feature request](https://forums.ankiweb.net/t/feature-request-linking-cards-with-each-others/6793)、[add-on 讨论](https://forums.ankiweb.net/t/link-relate-a-card-with-another-card-or-set-of-cards/13766)、[card-to-card 提议](https://forums.ankiweb.net/t/card-to-card-linking-feature-request/53009)）。移动卡片到别的牌组只改 did，nid 引用不受影响——**ID 寻址天然抗移动**。
- **Obsidian（文件树，文本内嵌式）**【事实】：链接=名字/路径寻址（`[[note]]`、`[[folder/note]]`，[Internal links](https://obsidian.md/help/links)）；块级=行尾 `^blockid` + `[[note#^blockid]]` 引用，且官方注明「块引用是 Obsidian 特有语法，非标准 markdown」；**app 内重命名会自动改写全库链接**（设置项「Automatically update internal links」，同上来源），**app 外改名/移动则链接断裂**（[forum#80455](https://forum.obsidian.md/t/links-do-not-update-when-renaming-a-file/80455)）；链接仅在 vault 内有效（[data-storage](https://obsidian.md/help/data-storage)）。
- **Logseq（文件树→DB 两代对照）**【事实】：块引用=`((uuid))`，uuid 即目标块的 `id::` 属性（[Block Reference](https://github.com/logseq/docs/blob/master/pages/Block%20Reference.md)）；DB 版的 markdown 镜像文件以 `id:: <uuid>` 开头固化页面身份（[logseq-markdown-syntax.md](https://github.com/logseq/logseq/blob/master/docs/logseq-markdown-syntax.md)）——即：**引用按 UUID 而非文件名/路径解析**，这是它抗改名的设计；代价是 Logseq 官方把「页面改名需改写所有引用文件」列为 markdown 的结构性弱点（[公告](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744)）。
- **RemNote（云端单库，rem 树）**【事实】：一切皆 rem，引用=`[[`/`++`/`@` 插入的双向 ID 链接，「改变目标文本，所有引用自动更新」，backlink 面板原生（[References](https://help.remnote.com/en/articles/6030714-rem-references)）；卡片本身就是 rem（树节点），天然挂在笔记树上。
- **SiYuan（混合，文件名=ID）**【事实】：块 ID 即文件名（`YYYYMMDDHHMMSS-xxxxxxx`，时间可排序）；移动/改名=移动 `.sy` 文件+同名目录，**HPath（人类可读路径）是从目录链派生的非权威数据**，移动只改 HPath 不改 ID（[WORKSPACE.md §4/§9/§10](https://github.com/siyuan-note/siyuan/blob/master/docs/WORKSPACE.md)）；引用按 ID 解析（同文档 §5 的 refCreateSave* 配置）。

**对 remit 的含义（GLOSSARY 的 Card/Reference 语义）**【推断】：

- 文件树形态下，Reference 的抗移动方案只有一种被反复验证：**块级稳定 ID 内嵌在明文里**（Logseq `id::`/Obsidian `^blockid`），卡片记录 ID 而非路径/标题；app 承担「改名时改写引用文本」（Obsidian/Logseq 都这么做）或干脆按 ID 解析（Logseq）。按名字寻址（纯 wikilink）已被 Logseq 官方文档点名是弱点。
- 单库形态下，Reference=外键/ID 列，移动/重命名零成本（Anki did、RemNote rem 树为证），但「卡片指向笔记块」需要自己建块表（Anki 没做，所以跨笔记引用缺失；RemNote 做了，因为块就是一等公民）。
- 混合形态可同时拿到：明文里的 ID（人/agent 可读、Git 可 diff）+ 索引里的反查表（引用计数、backlinks、due 队列 join）。

---

## 4. 候选方案（可并列）

### 候选 A：混合——明文源文件为权威 + 派生 SQLite 索引（SiYuan 模式）

- 明文侧：笔记内容 markdown（或 JSON）文件树；**块 ID 内嵌在明文中**（frontmatter 或行尾标记），文件名可含 ID；卡片=引用块 ID 的独立实体，调度数据（reps/due/FSRS 状态）属派生可重建数据的边界需明确（复习历史应属权威数据——SiYuan 把 history 放 `history/`、快照放 `repo/`，同为文件形态【事实】）。
- 索引侧：SQLite 全量可重建（SiYuan `temp/` 语义），不进 Git、不参与同步。
- 依据：SiYuan 全套先例（§1/§3.6）；Obsidian/Logseq OG 事实上也维护缓存索引，只是不落明文（§3.5）；本机实测索引查询 0.3ms 级满足 10 万卡（§3.5）。
- 风险/代价【推断】：索引重建协议（启动 1.4s 级/10 万文件，实测外推）；「外部直接改源文件后索引滞后」需要 reconcile 流程（SiYuan 用启动扫描+文件监控）；双写一致性问题=SiYuan 用「变更走内核 API」约束（§3.1）。

### 候选 B：SQLite 单库（Anki/Logseq 2.0 模式）

- 单库承载内容+调度+引用（块表），行级 mod/usn 时间戳（Anki 同步增量先例），`VACUUM INTO` 做备份/Git 快照边界（SQLite 官方推荐，§3.2）。
- 依据：Anki 十亿级装机验证 schema 与同步协议（§3.3）；Logseq 2.0 迁移 SQLite 的官方动机与性能目标（§1）；10 万卡实测毫秒级（§3.5）。
- 风险/代价【推断】：Git diff 无意义→版本管理只能靠应用级快照；agent 直写库绕过应用约束（Logseq 公告讨论中的争议点）；人类裸读体验差（`flds` \x1f 拼接为证）；同步必须自研协议（文件网盘路线被 SQLite 官方警告封死，§3.3）。

### 不推荐但记录：纯文件树（Obsidian 裸模式）

- 人类/agent/Git 三项最好，但 10 万卡级的 due 队列/引用反查只能靠「每次全量重建内存索引」（实测外推秒级）且要自研查询语义；Obsidian（闭源内核）与 Logseq OG（性能/数据丢失被官方列为迁移动机）都证明此路线在复习调度场景缺腿。**【推断】**若未来内容模型极简（无块引用、无跨卡查询）可重新评估。

### 与地图决策的对照

- 本地优先：A、B、C 均可本地（RemNote 式云端不在候选）✅
- Agent 可编程：A/C 直接明文读写+CLI 变更通道（SiYuan 把 MCP 列为官方通道）；B 需 sqlite3/MCP 工具链 ✅（程度差异）
- 同步后议：两候选都不封死——A 的文件粒度天然适配通用文件同步（Syncthing 实测语义：冲突保留副本）；B 有 Anki usn 协议完整先例；Logseq DB 走 RTC+离线（CRDT 曾评估未采用，见公告）✅
- 无与「调度=FSRS/开源/mygo GUI」决策冲突的事实；无。

---

## 5. 留给后续票的输入

1. 内容模型票：Reference 的「活引用 vs 快照」语义决定索引侧反查表设计（Obsidian 块引用=活引用渲染；Anki 字段=快照拷贝——两个极端都有产品验证）。
2. 同步票：A 路线继承文件同步生态（Syncthing/Obsidian Sync 的冲突事实在 §3.3）；B 路线需 Anki 式对象级增量（usn/mod/graves 三件套为最小完备集，§3.3）。
3. FSRS 票：调度状态（difficulty/stability 等）无论 A/B 都应视为权威数据随库走，不放可重建索引（Anki 把 FSRS 参数放 cards/revlog 行内为证，[schema](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure)）。

## 6. 来源清单（全部于 2026-11-06 访问）

官方文档/规格：[Obsidian data-storage](https://obsidian.md/help/data-storage)·[Internal links](https://obsidian.md/help/links)·[Obsidian Sync Troubleshoot](https://help.obsidian.md/help/Obsidian+Sync/Troubleshoot+Obsidian+Sync)·[AnkiDroid DB wiki](https://github.com/ankidroid/Anki-Android/wiki/Database-Structure)·[Anki Syncing](https://docs.ankiweb.net/syncing.html)·[Anki Exporting](https://docs.ankiweb.net/exporting.html)·[AnkiWeb size FAQ（经 #57864 全文）](https://forums.ankiweb.net/t/can-anki-handle-more-cards/57864)·[Joplin profile spec](https://github.com/laurent22/joplin/blob/dev/readme/dev/spec/user_profile.md)·[SiYuan WORKSPACE.md](https://github.com/siyuan-note/siyuan/blob/master/docs/WORKSPACE.md)·[SQLite How To Corrupt](https://www.sqlite.org/howtocorrupt.html)·[SQLite Faster Than FS](https://www.sqlite.org/fasterthanfs.html)·[Syncthing FAQ](https://docs.syncthing.net/users/faq.html)·[Syncthing Syncing](https://docs.syncthing.net/users/syncing.html)·[RemNote Exporting](https://help.remnote.com/en/articles/7898019-exporting-and-printing-notes)·[RemNote Importing from Anki](https://help.remnote.com/en/articles/6751471-importing-from-anki)·[RemNote References](https://help.remnote.com/en/articles/6030714-rem-references)·[Logseq Block Reference](https://github.com/logseq/docs/blob/master/pages/Block%20Reference.md)·[Logseq markdown mirror 语法](https://github.com/logseq/logseq/blob/master/docs/logseq-markdown-syntax.md)
官方公告/讨论：[Logseq「Why the database version」](https://discuss.logseq.com/t/why-the-database-version-and-how-its-going/26744)·[Logseq DB Changelog #37](https://discuss.logseq.com/t/logseq-db-changelog/30013/37)
社区证据（辅助）：Anki 论坛 [#17863](https://forums.ankiweb.net/t/default-browser-search-to-current-card-note-only/17863)/[#57864](https://forums.ankiweb.net/t/can-anki-handle-more-cards/57864)/[#6793](https://forums.ankiweb.net/t/feature-request-linking-cards-with-each-others/6793)/[#13766](https://forums.ankiweb.net/t/link-relate-a-card-with-another-card-or-set-of-cards/13766)/[#53009](https://forums.ankiweb.net/t/card-to-card-linking-feature-request/53009)；Obsidian 论坛 [#80455](https://forum.obsidian.md/t/links-do-not-update-when-renaming-a-file/80455)/[#82241](https://forum.obsidian.md/t/help-obsidian-lags-with-many-notes/82241)/[#88470](https://forum.obsidian.md/t/unpractical-vault-load-time-for-large-vaults-on-mobile-indexeddb-transactions-are-not-flushed-to-disk/88470)/[#106287](https://forum.obsidian.md/t/obsidian-graph-view-doesnt-work-for-a-large-vault/106287)；Logseq [#8137](https://github.com/logseq/logseq/issues/8137)/[#11236](https://github.com/logseq/logseq/issues/11236)/[discuss#4317](https://discuss.logseq.com/t/logseq-need-to-focus-on-performance-and-stability-rather-than-features/4317)
生态库：[genanki](https://github.com/kerrickstaley/genanki)·[anki-apkg-export](https://github.com/shbernal/anki-apkg-export)·[Obsidian_to_Anki](https://github.com/Pseudonium/Obsidian_to_Anki)·[obsidian-git](https://github.com/Vinzent03/obsidian-git)·[MCP sqlite server（archived）](https://github.com/modelcontextprotocol/servers-archived/tree/main/src/sqlite)·[nixpkgs-vault(13万笔记)](https://github.com/osbm/nixpkgs-vault)
