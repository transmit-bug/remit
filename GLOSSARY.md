# Remit

Remit 是开源、本地优先的桌面知识记忆应用：完整的 Markdown 笔记与 FSRS 闪卡共用一个明文库。设计哲学是 **Remember it**——一切功能为了把读过的记住。块是笔记的引用单位，不是笔记的形态。

## Language

**Review Queue**（复习队列）:
今日待复习卡片的序列，应用的默认首屏。
_Avoid_: 今日列表, study list, deck（已决定不引入：复习范围跟随笔记结构，见 Review Scope）

**Reading Queue**（阅读队列）:
今日待读的笔记序列，与复习队列并列在同一入口；成员是笔记本身，顺序按 FSRS 到期先进先出，节奏是宿主能力、不开插件面。见 `docs/specs/incremental-reading.md`。
_Avoid_: 待读列表, to-read, deck

**Reading Position**（阅读位置）:
一次阅读会话停下的块；打开待读笔记时回到这里。它是明文调度史的一部分，不是配置。
_Avoid_: 滚动位置, 阅读进度

**Excerpt**（摘录）:
从某个块取出的文本：成为新笔记的首块，并在块内携带指向原块的 Reference 链接；可立即按卡片类型制卡，也可以只是一条笔记。
_Avoid_: 高亮, 剪藏, clip

**Review Scope**（复习范围）:
一次复习会话的取卡范围，三源：全库 / 某篇笔记 / 某个标签；会话内选择、不落库，组织由笔记与标签承担。
_Avoid_: deck（Anki 容器概念，不进模型）, 子集（不说明来源）

**Dual-mode**（双态）:
同一数据仓库的两个界面形态：编辑态（写笔记）与复习态（过队列）。
_Avoid_: 双应用, 前后台

**Block**（块）:
笔记的最小可寻址单位；ID 一次性铸造、内联在明文源里，移动与重排不改变引用。
_Avoid_: 段落（paragraph 暗示排版而非寻址）, 行（line 不稳定）

**Card**（卡片）:
引用**一个块**的独立复习实体；调度历史的归属者。形状由卡片类型给出；字段值默认不落库，渲染时按来源现算（live 跟随块 / frozen 读快照），仅用户编辑过的字段存为覆盖。`::` 前提示后答案是 basic 类型的约定，不是卡片的定义。见 [ADR 0008](docs/adr/0008-card-types-templates.md)。
_Avoid_: flashcard, block（block 是笔记侧概念）

**Card Type**（卡片类型）:
库资产：一份有序字段表加正/反两面模板，明文存于库内、随库归档；卡片引用类型。内置三件（basic / bidirectional / cloze）随库种入，无特权路径。见 [ADR 0008](docs/adr/0008-card-types-templates.md)。
_Avoid_: note type（Anki 词汇，模型不同）, 模板（template 是类型的组成部分，不是独立概念）

**Reference**（引用）:
卡片与它所指块之间的联系，三态：live（默认，跟随块）/ frozen（保留快照、不再跟随）/ lost（块已消失且无快照，但调度史保留）。见 [ADR 0002](docs/adr/0002-content-model.md)。摘录的块用同一种形式指回它来自的块：`[[笔记标题#^块 ID]]`。
_Avoid_: 链接（link 指笔记间连接，若进入模型属另一概念）
