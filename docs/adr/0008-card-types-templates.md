# ADR 0008 — Card types, fields and host-rendered templates

- **Status:** accepted
- **Date:** 2026-11-07
- **Amends:** ADR 0002 (who derives a card and what a card stores), ADR 0003
  (storage layout: `types/`, card file v2), ADR 0005 (the typed derive contract
  replaces `{ordinal, kind, front, back}`), ADR 0007 (the `cards.render` hook is
  withdrawn; the host is the renderer)
- **Related:** #138, #139, #140, #142, #143, #67, #69

## Context

ADR 0007 froze the derive contract as two strings — front and back. That
contract cannot express a card with more than two faces, and it made every
change of presentation a plugin's job: a user who wanted their cloze cards to
read differently had nothing to edit. The spec revision (`docs/specs/
plugin-system.md` §2–§3) reopened the contract for an Anki-grade model:
fields the plugin fills, templates the user edits, types as library data.

## Decision

### Types are library assets

A type is an ordered field list plus a front and back template and a version,
stored as readable JSON at `types/<id>.json`, ID minted once, shipped with the
archive. The built-in three (`remit.basic`, `remit.bidirectional`,
`remit.cloze`) are seeded at creation and back-filled on open; they have no
privileged path — a user may edit their templates. Deleting a type referenced
by a card is refused; renaming a field rewrites every card's stored field
keys; deleting a field with a non-empty stored value is refused.

### The host is the renderer

Rendering is a pure data dialect the host evaluates in Go on the review path:
`{{Field}}`, `{{#Field}}`/`{{^Field}}` sections, `{{cloze:Text}}` (ordinal
0-based; the front masks the asked blank, the back reveals all; an ordinal
outside the value's blanks asks none of them — every blank shows, which is
what the card plugin's faces hook did),
`{{hint:Field}}` and `{{type:Field}}` (captured for the review view, not
inlined), and a reserved `{{Field|filter}}` pipe whose filter registry is
host-built and may stay empty. No script, no HTML, no CSS: templates are user
data, so they get no sandbox because they need none. A template that fails
renders the field list, never a blank card. The `cards.render` hook of
ADR 0007 is withdrawn — Anki has no render plugin either; the template *is*
the renderer. Plugins keep derivation, scheduling and summary.

### Fields are resolved, not persisted

This refines the spec's "card = type + fields": what a card file stores is
`type`, `ordinal`, `producer` — and a field map only when the user has
**edited** it (`edited: true`). At render time the derive plugin answers the
fields for the card's source (the block for a live reference, the snapshot
for a frozen one), the card's stored fields overlay it field by field, and
the type's templates render the two sides. This keeps the behaviour
one-for-one — a live card follows its block, an edit sticks, a frozen card
reads its snapshot — without a whole-library derive and without the store
depending on the extension loader. The producer stamp
stays on the card, so a derived card remains explainable after its plugin is
gone.

### Derive answers types

`deriveBlock(block, types)` answers `[{typeId, ordinal, fields}]`; the host
checks the type exists and the field names are in it, clamps, stamps
`producer {id, version}`, and writes. A paragraph may also speak a type
directly: a block that opens with `::: type <typeId>` and carries
`Field: value` lines is derived by the built-in plugin into that type — form
entry stays out of the product; the note is the form.

## Consequences

- There is no migration path and no older format to read: `remit/2` is the
  first released format, and a library declaring another is refused on open.
  Pre-1.0, no released libraries exist to keep (the migration layer built on
  this branch was deleted rather than shipped).
- The agent API's card shape is `type` + `fields` + rendered `front`/`back`;
  a breaking change, allowed pre-1.0.
- Golden faces stay byte-identical for basic and bidirectional cards; the
  cloze back now reveals the whole text (Anki's behaviour) where it used to
  show the blank alone — the only meant answer change, recorded in the
  frozen file.
- The template engine is a stability dependency of the review path; it is
  pure, stdlib-only and table-tested, deliberately not moejs.
