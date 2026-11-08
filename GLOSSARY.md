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

**Field**（字段）:
卡片类型里的一个具名值位，值是标量。字段值默认不落库：渲染时按卡片来源（live 读块 / frozen 读快照）由制卡插件现算，只有用户手工编辑过的字段存为覆盖。见 [ADR 0008](docs/adr/0008-card-types-templates.md)。
_Avoid_: 属性（attribute 是存储细节）, 列（column 是表格的词）

**Template**（模板）:
卡片类型自带的正、反两段文本，宿主用纯数据方言（`{{Field}}`、`{{cloze:Text}}`、`{{hint:}}`、`{{type:}}`）在 Go 里渲染成卡片面。模板是类型的一部分、是用户可编辑的库数据，不经插件；模板坏掉时回退字段原文列表，不出白卡。见 [ADR 0008](docs/adr/0008-card-types-templates.md)。
_Avoid_: 卡面（face 是渲染结果，不是模板）, 布局（layout 是窗口的事）, Note Template（笔记模板是另一概念，见下）

**Note Template**（笔记模板）:
库里带 `#template` 标签的**普通笔记**：新建笔记从它起，复制正文、字段块与卡片类型，块 ID 重新铸造。模板因此复用编辑器、索引、搜索与阅读队列，只在笔记列表与搜索里被滤掉；模板本体是宿主功能，不经插件。见 #154。
_Avoid_: Template（那是卡片类型的正反面文本，见上）, 原型（prototype 是开发物）, 前缀/目录约定（不是独立存储位置，模板就在 `notes/` 里）
_2026-11-08 实现（PR #192）补充的口径_：**`#template` 是笔记的角色，不是内容**。副本剥掉标记（含只装标记的块整块不复制）——保留它的副本自身也成模板、永远从笔记本消失，与「新建从模板起」相悖。模板本体保留标记，所以一个模板可以无限次开工。

**Field Block**（字段块）:
笔记里以单独一行 `::: type <typeId>` 开启、`Field: value` 每行一条、`:::` 结束的一段明文，让库里的任何类型都能直接从笔记录入。由内置制卡插件解析；类型没有的字段作为 `unknown` 丢弃并记日志，卡片仍按对得上的部分生成。
_Avoid_: 表单（form 指宿主 UI）, front matter（不是笔记元数据，而是块内容）

**Reference**（引用）:
卡片与它所指块之间的联系，三态：live（默认，跟随块）/ frozen（保留快照、不再跟随）/ lost（块已消失且无快照，但调度史保留）。见 [ADR 0002](docs/adr/0002-content-model.md)。摘录的块用同一种形式指回它来自的块：`[[笔记标题#^块 ID]]`。
_Avoid_: 链接（link 指笔记间连接，若进入模型属另一概念）

**Asset**（资产）:
笔记里带的非文本文件（图片），按内容命名存于库内 `assets/<sha256>.<ext>`，笔记以相对路径引用；同图重复落入只存一份，随库目录与 `.remitlib` 归档走，与笔记同属真相。见 [ADR 0003](docs/adr/0003-storage-layout.md)。
_Avoid_: 附件（attachment 暗示外挂）, media（定名时弃用）, 缓存（资产从不重建）
