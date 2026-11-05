# 竞品 UI 解构：布局 / 层级 / 交互 / 动效证据

> 工单：[#17](https://github.com/transmit-bug/remit/issues/17)（父工单 [#14](https://github.com/transmit-bug/remit/issues/14)）。
> 服务对象：UI 方向定调票 [#15]、布局原型票 [#18]。
> 调研方式：web 调研，官方文档/手册优先，用户抱怨取自官方论坛与 reddit。
> 标注约定：【事实】= 有来源链接的直接观察；【推断】= 基于事实的推断，供 #15/#18 决策参考。
> 本文只解构**界面层**；功能层竞品基线见主地图 #5，不重复。

---

## 1. Anki（各平台复习界面）

### 1.1 复习界面布局与层级【事实】

桌面端主窗口是单窗口三屏结构（Decks → Overview → Review），官方手册截图与描述：

```
┌────────────────────────────────────────────┐
│ 顶部工具区: Decks | Add | Browse | Stats | Sync │
├────────────────────────────────────────────┤
│ Decks 屏:                                    │
│   Deck 列表（含子 deck 缩进展示）              │
│   每行: Deck名  [新卡数][学习数][待复习数] ⚙    │
│   底部: Filter Deck / Create Deck 按钮        │
├────────────────────────────────────────────┤
│ Overview 屏（点某个 deck 后）:                 │
│   Deck 名                                    │
│   今日: X new, Y learning, Z to review       │
│   [Study Now] 大按钮（+ Unbury 若有埋藏卡）     │
├────────────────────────────────────────────┤
│ Review 屏:                                   │
│   顶栏: 剩余新/学/复习计数                     │
│   中部: 卡片问题区（HTML 渲染）                 │
│   底部: [Show Answer]（或答案区）              │
│   答案显示后: 四个评分按钮一行                  │
│   左下: Edit   右下: More（flag/bury/suspend…）│
└────────────────────────────────────────────┘
```

来源：<https://docs.ankiweb.net/studying.html>（Decks 屏截图、Overview 屏截图、Answer Buttons、Editing and More 各节）。

四键评分按钮【事实】：

- **Again**（快捷键 `1`）、**Hard**（`2`）、**Good**（`3`/`Space`/`Enter`）、**Easy**（`4`）。
- 每个按钮上同时显示**选择后下次复习的间隔**（"Each answer button shows the next time a card will be reviewed again"）。
- 官方明确支持只用 Again/Good 两键模式，并给出使用频率建议（Again 5–20%，Good 80–95%）。
- 手册推荐的键盘流："answer most cards with Space and keep one finger on 1 for when they forget"。

来源：<https://docs.ankiweb.net/studying.html#answer-buttons>、<https://docs.ankiweb.net/studying.html#keyboard-shortcuts>。

### 1.2 键盘交互清单【事实】

- `Space` / `Enter`：显示答案；答案显示后 = Good。
- `1`–`4`：四档评分。
- `S`：返回 deck overview；Decks 屏上 `S` = 学习当前选中 deck。
- `/`：Study Deck 弹窗（带过滤的 deck 快速切换器，支持多词空格分隔匹配，如 "ja 1" 匹配 "Japanese::Lesson1"）。
- `T`：打开统计窗口。
- `I`：当前卡片 Card Info。
- 菜单项旁列快捷键、按钮 hover 显示 tooltip 快捷键（可发现性策略）。

来源：<https://docs.ankiweb.net/studying.html#keyboard-shortcuts>、<https://docs.ankiweb.net/stats.html>。

### 1.3 统计页结构【事实】

入口：顶部 Stats 按钮或 `T`。结构（<https://docs.ankiweb.net/stats.html>）：

```
Statistics 窗口
├── 顶部: deck 选择框 + Collection 复选框 + 搜索过滤框
├── Today 文字块（Again 数、正确率、Learn/Review/Relearn/Filtered 计数）
├── 图表区（可滚动）:
│    Future Due（预测）/ Calendar（活动日历）
│    Reviews（按日/周/月计数，堆叠着色+累积曲线）
│    Card Counts（饼图）/ Review Time / Review Intervals
│    Card Ease（FSRS 下换成 Stability/Difficulty/Retrievability）
│    Hourly Breakdown / Answer Buttons / True Retention 表
└── 底部: 时间范围选择（1月/1年/全部/deck 生命期）+ Save PDF
```

### 1.4 动效观察【事实】

- 图表切换时间范围时有过渡动画，且曾被用户报出"用了两段动画"的 bug（说明动画存在但非重点打磨对象）：<https://forums.ankiweb.net/t/in-statistics-reviews-graph-uses-wrong-range-change-animation/19435>。
- 复习界面本身基本无动效（翻卡即时替换），卡片渲染是 HTML/CSS，社区靠共享牌组 CSS 自己加动画。【推断】Anki 把视觉表现外包给用户 CSS，本体克制到"无"。

### 1.5 用户 UI 抱怨【事实】

- "Anki 的 UI/UX 是程序员写给程序员的……太多按钮、命名糟糕、不直观"：<https://www.reddit.com/r/Anki/comments/1l56142/why_do_a_lot_of_people_think_anki_has_an_outdated/>
- 2023 新 UI 改版后"界面太大、按钮加边框、字体过大"：<https://forums.ankiweb.net/t/the-interface-is-too-big-after-the-new-update/27991>
- "Really hate where Anki is going with it's new design"（老用户认为新设计降低生产力）：<https://forums.ankiweb.net/t/really-hate-where-anki-is-going-with-its-new-design/3651>
- 社区持续存在 "Brainstorming for modern UI [Anki 3.0]" 长贴：<https://forums.ankiweb.net/t/brainstorming-for-modern-ui-anki-3-0/17510>
- 编辑器杂乱、要求把 Cards.../Fields... 移入上下文菜单：<https://forums.ankiweb.net/t/make-the-editor-less-cluttered-by-moving-cards-and-fields/22442>

【推断】对 remit 的教训：Anki 的键盘流（Space=Good、1=Again、数字四键）是被验证的好设计；其抱怨集中在**默认视觉密度/字号**与**功能按钮过载**——remit 应保留键盘流、控制同屏按钮数、密度可调。

---

## 2. RemNote（笔记+复习一体）

### 2.1 布局与双态切换【事实】

桌面端是三栏 Web 应用布局：

```
┌────────┬──────────────────────────┬─────────┐
│ 左侧栏  │  中央: 大纲(bullet)编辑器   │ 右侧栏   │
│ 文档树  │  无限层级 bullet, 可折叠     │ (引用/   │
│ (文件夹 │  /-命令菜单插入任意块       │  词典等) │
│ /文档)  │                          │         │
│ +Queue │                          │         │
│ 入口+   │                          │         │
│ streak │                          │         │
│ 火焰+   │                          │         │
│ 到期数  │                          │         │
└────────┴──────────────────────────┴─────────┘
```

- 左侧栏底部/顶部有 *Flashcards* 入口，侧栏条目直接显示到期卡数和 streak 火焰：<https://help.remnote.com/en/articles/7925835-the-flashcard-home>
- 双态：**编辑态**（大纲编辑器）与**练习态**（全屏 flashcard practice 界面）是两个明确分离的界面，不混排。侧栏 Flashcards 图标 / 文档上的 Practice 按钮进入练习；`Shift+Ctrl+L`（mac `Shift+Cmd+L`）跳过主页直接开始练习。

来源：<https://help.remnote.com/en/articles/7925835-the-flashcard-home>。

- Flashcard Home（练习入口页）结构【事实】：Today's Goal（Due Reviews + Practice [X] Cards 蓝色大按钮）→ 右侧一周练习柱状图（标注 streak goal / daily goal / stretch goal 三条目标线）→ 下方按优先级排序的文档列表（每行带 Practice 按钮与 … 菜单设 Priority），考试模式下顶部出现 Exams 区与 catch-up 警告横幅。同上来源。

### 2.2 Portal 的 UI 表达【事实】

来源：<https://help.remnote.com/en/articles/6030742-portals>

- Portal = 在别处文档"开一扇窗"显示同一条 bullet；**左侧粗蓝竖线**标记 portal 内容；光标进入 portal 时**整个 portal 被蓝色方框包围**。
- Portal 头部有**面包屑**（如 "Biology → Cell Biology"）指示内容的真实位置；右上角 handle 用于选中 portal 本身（边框变色 = 选中 portal 而非内容）。
- **Hidden bullets**：新增子条目默认在原位可见、其他 portal 隐藏；portal 右上角显示 "N hidden" 按钮，点击可选择显示。
- 创建方式：输入 `((` 后搜索 bullet 回车；或粘贴 bullet 后按 `p` 转为 portal；或 `Ctrl+Shift+Enter` / `/portal` 新建空 portal。
- **选中语义双轨**：点内容内部 = 编辑原位置内容；点边框/面包屑 handle = 操作 portal 本身（删除 portal 不删内容）。这是 RemNote 用视觉（蓝框/边框变色）+ 命中区域（边框窄条）解决的"引用 vs 内容"歧义。

### 2.3 卡片在笔记中的视觉标记【事实】

- 卡片即笔记内的 bullet：用 `;;`、`::`、`>>` 等记忆符（memorization delimiters）把一行变成卡片，渲染为**高亮色**（蓝/橙等区分卡片类型：basic / descriptor / cloze）；`Ctrl+Shift+K` 把选中 bullet 整行转为卡片。
- 练习界面四键 Again/Hard/Good/Easy 与 Anki 同构，快捷键 `1-4` / Space。
- 来源：<https://help.remnote.com/en/articles/8663109-flashcard-basics>、<https://help.remnote.com/en/articles/7893440-keyboard-shortcuts>（快捷键总表）。
- 全局搜索 `Ctrl+P`，文档内 `Ctrl+F`：<https://help.remnote.com/en/articles/6030721-searching-your-knowledge-base>
- Omnibar：键盘访问几乎任意动作：<https://help.remnote.com/en/articles/7852647-using-the-omnibar>

### 2.4 动效观察

- 官方文档大量用 GIF 演示（portal 创建、hidden bullet 展开等），交互反馈以"展开/折叠/蓝色高亮"为主，无夸张动效。【事实，来源同上 portal 文档】【推断】RemNote 的动效预算花在"状态变化的即时可读"（颜色+边框）而非装饰。

### 2.5 用户 UI 抱怨【事实】

- "I like the capabilities, but I dislike the interface. The GUI is way too complicated."（离开 RemNote 的用户）：<https://www.reddit.com/r/remNote/comments/1ug4git/im_leaving_remnote/>
- 性能/卡顿是长期主题：官方专门开帖收集 Sync/Performance/Stability 反馈（大库加载 10 秒级）：<https://forum.remnote.io/t/collecting-feedback-for-sync-or-performance-or-stability-issues/9135>；reddit 持续有 lag 帖：<https://www.reddit.com/r/remNote/comments/1nzoxer/how_to_handlereduce_lag>
- 移动端编辑器"不如桌面精致"：<https://www.reddit.com/r/remNote/comments/1gbi4zm/why_the_android_version_of_remnote_is_so>
- 早期用户要求 Ctrl+Shift+P 式全命令面板（后由 Omnibar 解决）：<https://forum.remnote.io/t/keyboard-driven-workflow-and-commands/1292>

【推断】教训：笔记↔复习双态分离 + 笔记内卡片用**轻量视觉标记（颜色）**表达是 RemNote 最值得抄的两点；其复杂度失控与 Web 技术栈卡顿是反面教材——remit 用 mygo 原生 UI 天然规避后者，但要警惕功能按钮堆叠。

---

## 3. Obsidian（可定制布局与命令面板）

### 3.1 布局模型【事实】

来源：<https://help.obsidian.md/workspace>（镜像 <https://retypeapp.github.io/obsidian/workspace/>）、<https://help.obsidian.md/tabs>、<https://help.obsidian.md/sidebar>

```
┌──┬──────────────┬─────────────────────┬──────────────┬─┐
│ Rib │ 左侧栏       │  中央内容区            │ 右侧栏        │状│
│ bon │ (可折叠)     │  Tab groups           │ (可折叠)      │态│
│ 竖  │ 文件/搜索/   │  ┌────────┬────────┐  │ 反链/大纲/   │栏│
│ 排  │ 书签等 tabs  │  │tab group│tab group│  │ 标签等 tabs  │  │
│ 图  │ 可再纵向分栏  │  └────────┴────────┘  │ 可再纵向分栏  │  │
│ 标  │             │  可横/纵任意分割          │             │  │
└──┴──────────────┴─────────────────────┴──────────────┴─┘
```

- 桌面 workspace = Ribbon（左竖条图标栏）+ 左右可折叠侧栏（侧栏内是 tab 组，可纵向再分割）+ 中央 tab groups（可横纵分割成任意网格）+ 右下状态栏。
- 布局通过**拖拽**重排：tab 可拖到 tab 组内、跨组、拖出成新窗口；布局持久化到下次打开。
- 插件默认在侧栏开新 tab 而非新 tab group（约束插件不过度侵占中央区）。<https://help.obsidian.md/User+interface/Sidebar>

### 3.2 命令面板【事实】

来源：<https://help.obsidian.md/plugins/command-palette>

- `Ctrl+P`（mac `Cmd+P`）打开；输入命令名 → 方向键 → Enter。
- **模糊匹配**（"scf" 找到 Save current file）。
- 1.8.3 起最近使用的命令置顶（但被过滤时短命令优先）。
- 支持在设置里 **pin 常用命令**到顶部；更频繁的命令建议直接设 hotkey。
- 命令面板同时是**快捷键发现器**（列出命令及其现有快捷键）；Hotkeys 设置允许给任意命令绑任意组合键：<https://help.obsidian.md/hotkeys>

### 3.3 插件 UI 约束【事实+推断】

- 插件 UI 通道基本只有：侧栏 tab（`ItemView`）、ribbon 图标、命令、设置页 tab、状态栏条目、编辑器扩展（live preview 装饰）。【事实，见 <https://help.obsidian.md/sidebar>】
- 设置页是自绘 DOM，官方只给 `PluginSettingTab` 接口，样式靠约定，导致各插件设置页观感不一；开发者论坛有 "Why does PluginSettingTab not use the Component interface?" 等讨论：<https://forum.obsidian.md/t/why-does-pluginsettingtab-not-use-the-component-interface/113911>
- UI 不一致的抱怨长期存在：界面控件 CSS 类不一致（<https://forum.obsidian.md/t/inconsistent-css-classes-in-interface-controls/7749>）、社区插件设置页 UX 问题（<https://forum.obsidian.md/t/ux-issue-in-the-community-plugins-settings/62840>）、设置不是独立窗口影响多窗工作流（<https://forum.obsidian.md/t/make-the-settings-menu-a-seperate-windows/74764>）。

【推断】教训：**"核心命令全部可经命令面板+可绑键"是键盘可达性的黄金标准**；但"插件自由绘制 UI"会牺牲一致性——remit 若走单体 UI，应提供官方 widget 集（对应 mygo-components）而非让扩展自绘。

---

## 4. SuperMemo（渐进阅读的优先级队列呈现）

### 4.1 界面模型与优先级队列【事实】

来源：<https://help.supermemo.org/wiki/Priority_queue>、<https://help.supermemo.org/wiki/Read_toolbar>、<https://help.supermemo.org/wiki/Extract>

- 单一 Element Window 承载所有元素类型（topic=待读文章/extract、item=问答卡），底部 **Learnbar/Read toolbar** 按当前元素类型切换工具集（Read 工具条上有 Extract `Alt+X`、Cloze、优先级等按钮）。
- **优先级队列**：全部元素按 priority 排成一条相对队列，`Position=1` 即最高优先；也可用 0%–100% 百分比表达，**0%=最高**（官方解释：手输小数字省打字时间）。低优先元素在时间不够时被牺牲/顺延（auto-postpone），每日 auto-sort 保证先服务高优先元素。
- 呈现方式：
  - `Alt+P` 打开 Element priority 对话框：输入 position 或 percent，对话框里显示 **After / Before 字段**——即"这个优先级夹在哪两个元素之间"，用邻近元素名给数字提供语境。
  - `Shift+Ctrl+↑/↓` 直接升/降当前元素优先级。
  - View : Priority queue 以浏览器（表格）查看整条队列。
  - Priority protection 统计（Analysis : Use）显示"前 X% 的内容实际受到保护"——把溢出可视化。
- 排序准则允许设定 **topic:item 比例与随机度**，官方建议 topic 占比 ≤1:4，并用随机化对抗 "priority bias"（总觉得新文章最重要）。

### 4.2 布局文字图【事实+推断】

```
┌──────────────────────────────────────────────┐
│ 菜单栏 File/Edit/Navigation/Learn/…           │
│ Element Window:                               │
│   元素内容区（文章 HTML / 问答卡）                │
│   （extract 高亮残留、cloze 空洞内联在文中）      │
│ 底部 Learnbar（随元素类型变化）:                 │
│   [Learn][Repeat]… | Read 工具条:              │
│   Extract(Alt+X) Cloze Priority…               │
│ 状态: element 编号/优先级/间隔                  │
└──────────────────────────────────────────────┘
```
（Learnbar/Read 工具条位置与按钮为【事实】，整体排布为基于官方帮助截图的【推断】简化。）

### 4.3 用户 UI 抱怨【事实】

- "Damn, the interface looks terrible."——5 年 Anki 老用户转 SuperMemo 的第一反应，全文反复权衡"功能最强 vs 界面劝退"：<https://masterhowtolearn.wordpress.com/2018/10/28/why-i-switched-to-supermemo-after-anki-for-5-years-with-over-50000-cards-and-420k-total-reviews>
- reddit r/Anki："more complicated than I could ever imagine"：<https://www.reddit.com/r/Anki/comments/iziyaj/did_anyone_here_try_supermemo_i_bought_sm_18_and>
- Anki 论坛讨论 onboarding 时直接拿 SuperMemo 当反面教材：<https://forums.ankiweb.net/t/revisiting-onboarding-decks-and-tutorials/43692/87>

【推断】教训：优先级队列是渐进阅读的**概念核心**（做完不完都从队尾牺牲，心理负担小），但 SuperMemo 用"对话框输数字+菜单丛林"呈现它，劝退了所有人。remit 若做阅读优先级，应当用**可拖拽排序列表 / 相对位置暗示**呈现，而非数字输入框；0%=最高这类反直觉约定绝不要学。

---

## 5. 参照系：Duolingo（温暖/动效）与 iA Writer（纸感/排版）

### 5.1 Duolingo【事实】

来源：Apple Behind the Design: Duolingo <https://developer.apple.com/news/?id=jhkvppla>、Duolingo 官方博客 <https://blog.duolingo.com/shape-language-duolingos-art-style/>、<https://blog.duolingo.com/world-character-visvisemes>（正确 URL: <https://blog.duolingo.com/world-character-visemes>）

- 设计定位原话："We're not an education company. We're a fun and motivation company."（VP of design Ryan Sims）。
- **学习路径从"树"改为单一"path"**：避免同等努力走向不同结局，单一主线降低选择负担——一次彻底的产品策略重启。
- **角色动效即奖励**：答对时角色的细微动画（Lily 的慢 clap、Oscar 的戏剧化反应）本身就是正反馈；角色动画有专门技术栈（viseme 口型系统等）。
- 视觉语言：圆润形状语言、动物命名的明亮色板（Macaw blue、Fox orange 等）、白底大留白、**每屏单一焦点单一动作**。
- streak/连续打卡是核心动机机制（Dolo 猫头鹰人格化的提醒文化）。

【推断】对 remit"温暖"的可迁移点：奖励性动效放在**正确答案的微反馈**上（而非全屏庆祝）；单一主线复习流优于多入口分支；streak/目标线（RemNote 也在 Flashcard Home 画了三条目标线）是两家共同验证的动机 UI。不需要抄吉祥兽，抄"微动效=奖励"的预算分配方式即可。

### 5.2 iA Writer【事实】

来源：<https://ia.net/writer>（产品页）、<https://ia.net/topics/in-search-of-the-perfect-writing-font>（Duospace）、<https://ia.net/topics/a-typographic-christmas>（三字体）

- 产品口号级描述："No buttons, no popups, no…"——主打"手在键盘上、心在文本里"的极简。
- **Focus Mode**：聚焦当前句/词，其余文本变暗——注意力引导靠排版而非界面元素。
- **三套自研字体**：Nitti（mono）、iA Writer Duo（duospace：仅 m/M/w/W 放宽 50%）、iA Writer Quattro（proportional）。选择字体的理由是**写作节奏**：monospace 传达"草稿、未完成"，放慢写作、字母可辨、进度可感；duospace 在保留 monospace 优点的同时让文本流动。
- 设计过程（735 个变体、与 Matthew Carter 论证）体现"排版即产品"的纸感路线。

【推断】对 remit"纸感"的可迁移点：复习界面的问题/答案排版值得当成一等资产（等宽或 duospace 类字体传达"工作草稿"心态）；用 Focus 式的"当前卡片高亮、历史淡出"引导注意力；克制到"无按钮感"（Anki 的键盘流 + iA 的极简 = remit 复习屏的参照组合）。

---

## 6. 汇总：给 #15（UI 方向）与 #18（布局原型）的交叉结论

【推断，全部由上文事实支撑】

1. **复习屏四键 + 键盘流是行业收敛点**：Anki（1-4/Space）与 RemNote（同构）一致；Duolingo 的单屏单动作佐证"复习时一屏只干一件事"。remit 复习屏应以键盘为主、按钮为辅，每键显示预测间隔（Anki）并可在设置退化为两键。
2. **笔记↔复习双态分离**（RemNote）vs **阅读即复习混排**（SuperMemo IR）。remit 若做渐进阅读，参考 SuperMemo 的队列语义但用 RemNote 式的"笔记内轻量视觉标记（颜色高亮/边框）"表达卡片与 extract。
3. **Portal/引用的 UI 语法**（RemNote 蓝色左竖线 + 面包屑 + 边框命中区分"操作引用 vs 操作内容" + "N hidden" 折叠）是现成的引用可视化方案，若 remit 需要跨文档引用可直接借鉴。
4. **命令面板 + 全命令可绑键**（Obsidian）是桌面键盘可达性标配；Anki 的 `/` deck 过滤切换器是其复习场景特化。remit 应在 mygo 里做等价的 omnibar/command palette。
5. **优先级不要用数字对话框**（SuperMemo 教训）：用拖拽排序、相对位置暗示（After/Before 邻居）、保护率统计图呈现溢出。
6. **动效预算**：正确性微反馈（Duolingo 角色微动效的等价物：卡片翻面/答对高亮）+ 状态变化的颜色/边框即时可读（RemNote）；避免 Anki 式"图表双段动画"式瑕疵与全屏庆祝。
7. **排版即纸感**：iA Writer 的字体策略（mono/duospace 表达草稿心态、Focus 淡出）+ 白底大留白单焦点（Duolingo）构成"温暖而安静"的参照组合。
8. **反面教材清单**：Anki（密度/按钮过载、视觉外包给用户 CSS）、RemNote（Web 卡顿、GUI 复杂度失控）、SuperMemo（菜单丛林+0%=最高）、Obsidian（插件自绘 UI 不一致）。
