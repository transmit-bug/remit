# ADR 0002 — Content model

- **Status:** accepted — amended by ADR 0005 (block identity and the editing surface), ADR 0007 (a plugin derives a card), ADR 0008 (the derive contract is typed; the host renders a type's templates) and ADR 0011 (a note's terminal reading state is a log event; reading it never deletes the note)
- **Date:** 2026-11-06
- **Related:** #8, #7, ADR 0001

## Context

Remit is note-first and review-centred. Cards must stay attached to what a
note says without turning the note into a card database, and a card's review
history must survive every edit a user makes to the text it came from. The
storage shape (#23) is not decided yet, so this ADR fixes semantics only.

## Decision

**Blocks are the unit of reference.** A note is ordered plaintext blocks, each
with a stable ID minted once and written inline in the source. Moving,
reordering, splitting or merging text keeps the ID, so a reference never
follows position.

**A card references exactly one block.** The card renders its two sides from
that block: the text before the first `::` is the prompt, the rest is the
answer; a block without `::` is all prompt. A card may override either side,
and the override wins. (ADR 0008 keeps the reference and the one-block rule,
and withdraws the idea that the two sides are the model: a card is a type plus
field values, and `::` is the *basic* type's convention, not the card's shape.
The card's stored overrides remain the user's edited fields.)

**A reference is live by default and freezable.** A frozen reference keeps a
snapshot (text plus the moment it was taken) and stops following the block; it
can be thawed back to live. A live reference whose block is gone resolves to
*lost* rather than to empty content that looks real.

**Losing a source never loses scheduling.** A card's status, FSRS memory state
and rating history belong to the card, not to the block. A lost card keeps all
of it and can be re-pointed at another block.

**Card lifecycle status is separate from FSRS.** `new`, `learning`, `review`
and `suspended` are the v1 set; `reading` is stored and filtered on as a
reserved value for incremental reading, which v1 does not schedule. Adding the
reading queue later therefore needs no sidecar migration.

## Rationale

- Inline block IDs were the only scheme the storage research verified as
  surviving moves; the cost is a small marker in the text.
- One-block references keep the lost, frozen and highlighted states
  unambiguous. Ranges can be added later as a new kind of reference.
- Overrides rather than stored copies keep the default behaviour live while
  still letting a user sharpen the wording.

## Consequences

- Splitting a block creates a new block ID; the edit path must carry the
  reference to whichever side kept the content, and say so in the UI.
- The plaintext source carries ID markers, so a hand-edited note can orphan a
  card — the lost state is a normal, tested outcome, not corruption.
- `internal/core` owns these rules; storage and UI only move values around.

## Amended 2026-11-10

**归档不删除源笔记。** 读完的终态（归档 / 不再出现 / 推后）是 `reviews.jsonl` 里的事件，不是文件
操作：`notes/*.md` 不因「读完」被删、被截断或被移出库。归档的笔记仍可搜索、仍可被卡片引用、
仍可制卡；源块被删时走既有的 lost 语义，与归档无关。所以「一个块、一条活引用」的规则不变——
终态只决定阅读队列的成员资格（#216，ADR 0011，`docs/specs/ui-redesign-v2.md` §1.12/§5.2）。
