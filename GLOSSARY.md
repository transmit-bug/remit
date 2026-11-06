# Remit

Remit 是开源、本地优先的桌面知识记忆应用：块级笔记为一等公民，复习是核心价值。

## Language

**Review Queue**（复习队列）:
今日待复习卡片的序列，应用的默认首屏。
_Avoid_: 今日列表, study list, deck（deck 是 Anki 的组织容器概念，是否进入模型待内容模型票决定）

**Dual-mode**（双态）:
同一数据仓库的两个界面形态：编辑态（写笔记）与复习态（过队列）。
_Avoid_: 双应用, 前后台

**Block**（块）:
笔记的最小可寻址单位；ID 一次性铸造、内联在明文源里，移动与重排不改变引用。
_Avoid_: 段落（paragraph 暗示排版而非寻址）, 行（line 不稳定）

**Card**（卡片）:
引用**一个块**的独立复习实体；调度历史的归属者。内容默认由块渲染（`::` 前为提示、后为答案），卡片可覆盖任一侧文案。
_Avoid_: flashcard, block（block 是笔记侧概念）

**Reference**（引用）:
卡片与它所指块之间的联系，三态：live（默认，跟随块）/ frozen（保留快照、不再跟随）/ lost（块已消失且无快照，但调度史保留）。见 [ADR 0002](docs/adr/0002-content-model.md)。
_Avoid_: 链接（link 指笔记间连接，若进入模型属另一概念）
