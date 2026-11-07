# ADR 0003 — Storage layout

- **Status:** accepted
- **Date:** 2026-11-06
- **Amended:** 2026-11-08 — `reviews.jsonl` also carries reading events (a note
  queued, read or dropped), so one replay answers cards and the reading queue
  (#156, `docs/specs/incremental-reading.md`); the file name and the format
  version stand
- **Related:** #23, #7, ADR 0002

## Context

#7 verified that ID addressing is the only reference scheme that survives a
move, and that a 100k-card library needs an index (due queue ~0.3 ms, review
write ~4 ms) that a plain file-tree walk cannot provide. Notes and cards must
stay open, text, hand-editable files; scheduling must not be spread through
the prose. A library must be movable and copyable as one directory.

## Decision

A library is one directory:

```
<library>/
  remit.json              format version, FSRS weights, desired retention
  notes/<slug>-<id>.md    plaintext note; referencable blocks end with ^block_<hex>
  cards/<card-id>.json    card definition: block reference, overrides, status
  reviews.jsonl           append-only review events — the only scheduling source
  .remit/index.sqlite     derived index: blocks, cards, FSRS state, due, stats
```

**Sources of truth are all text:** notes, card files, `reviews.jsonl`,
`remit.json`. Everything in `.remit/` is derived and may be deleted; it holds
nothing that cannot be rebuilt.

**Per-card FSRS state is not persisted separately.** It is replayed from
`reviews.jsonl` in `seq` order, and the index caches the result. One authority
for scheduling history beats two things that must be kept in step.

**Note files are named `<slug>-<id>.md`.** The slug is minted once at creation
and never renamed when the title changes, so a path is stable and still
browsable. IDs are `kind_` plus 128 random bits, written into the source and
never derived from a path or a title.

**Card files are JSON.** Overrides may be multi-line, and JSON is unambiguous
for both a person and the stdlib; reviews never rewrite a card file, only
`reviews.jsonl`.

**Undo is a log event, not a rewrite.** A review appends
`{seq, card, rating, at}`; an undo appends a void event naming the `seq` it
cancels, and replay skips voided entries. The log is never rewritten.

**Reading entries live in the same log.** A note joining, being read or leaving
the reading queue appends an event that names the note instead of a card (`note`
instead of `card`, a `kind` of `queued` / `dropped` / `reviewed`, and the block
a session stopped at). Replay derives the reading queue's due dates and
positions the way it derives a card's schedule; the index caches both.

## Consequences

- Rebuilding a library means replaying the log plus reindexing the text; the
  cost is paid at startup or after a manual rebuild, not per review.
- A copied library carries everything needed; a copied `.remit/` is discarded.
- Splitting a block mints a new ID, so the split path must record which side
  inherits the reference (ADR 0002).
- A hand-edit can orphan a card; the index reports it as lost, and the log
  keeps the history, so nothing is silently lost.
- `reviews.jsonl` grows with reviews (~50 bytes each); compaction is a later
  concern, and any compaction must preserve replay output.
- SQLite must be a pure-Go driver (`modernc.org/sqlite`, BSD-3) so the shipped
  build stays cgo-free.
