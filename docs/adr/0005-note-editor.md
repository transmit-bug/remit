# ADR 0005 — The note editor: a document whose paragraphs are addressable

- **Status:** accepted — amended by ADR 0008 (the derive contract is typed: `deriveBlock` answers `{typeId, ordinal, fields}` and the host stores `type`, not `kind` and two faces)
- **Date:** 2026-11-07
- **Amends:** ADR 0002 (block identity and who derives a card)
- **Related:** #59, #60 (research), #61, #62, #65

## Context

ADR 0002 made the block the unit of **reference**: a card points at one block,
whose text before the first `::` is the prompt. The interface implemented that
decision as if the block were also the unit of **editing**: one text field and
one action row per block. The result was a form builder — no headings, lists,
images or links, no continuous text, and a note that existed only to mint cards.

The framework ships the widget the note needed. At v0.7.4 `ui/richtext` holds a
document editor (headings, bullet/number/task lists, quotations, fenced code,
inline images, links, Markdown shortcuts, IME, rich copy and paste, accessibility
on three platforms, large documents, Markdown round-trip through
`Text.Markdown`/`ParseMarkdown`), and `ui/markdown` renders GitHub-flavoured
Markdown with tables, footnotes and maths. The earlier reconciliation had never
looked at either package, and rejected a wrapper on evidence from
`TextArea`/`TextAreaBuffer`/`EditableText`/`Table`/`List` only.

## Decision

**A note is a document whose paragraphs are addressable.** Editing happens in one
`richtext.Editor` per note; the reading and review surfaces render through
`ui/markdown`. The block survives as the reference unit, not as a layout unit.

**Identity is the plaintext marker.** A paragraph that a card points at carries
`^id` at its end, in the file. The editor hides it; Markdown keeps it; pasting a
paragraph re-mints it; splitting a paragraph leaves the marker with the text that
held it. `ParagraphStyle.ID` is used as an in-session mirror, recovered from the
markers on open, because it does not survive Markdown.

**Card creation is a gesture.** Typing `::` in a paragraph is what makes it a
card; the paragraph's gutter shows a badge with its cards and their link state,
and the per-block "make card" button disappears.

**A block may feed several cards.** The derive plugin returns
`{ ordinal, kind, front, back }` per card, and the host stores
`{ ref, ordinal, kind, producer }`. Faces are re-derived rather than stored, so
the live reference of ADR 0002 keeps working, including for cloze and
bidirectional cards. (ADR 0008 replaced the two strings with a typed answer —
`{ typeId, ordinal, fields }`, stored as `type`, `ordinal` and `producer` plus
the user's edited fields; re-derivation and the live reference are unchanged.)

**Maths is source in the editor.** `richtext` keeps maths as written and does not
render it; `ui/markdown` renders it. The editor shows the source, review shows
the formula.

## Consequences

- The reconciliation entry that claimed "no widget carries an editable block" is
  void and is rewritten to name what was checked.
- The marker is an application convention inside a document editor: the framework
  preserves it but does not interpret it, and `[^id]` is forbidden because it is
  GFM's footnote syntax.
- Splitting a note into blocks is now a modelling act (a paragraph boundary), not
  a UI affordance; ADR 0002's rule that a split creates a new block still holds
  for the card's reference.
