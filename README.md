# Remit — A local-first, open-source knowledge base where notes and flashcards share one home: write it once, remember it forever.

## Status

Early development. Remit is pre-1.0 and not yet usable for real libraries; the
data format and the agent interface may still change between minor versions
(see `docs/engineering/conventions.md`).

## What it is

- **Review is the core.** Cards are scheduled with FSRS and surfaced through a
  Review Queue, which is the app's first screen.
- **Notes are first-class.** Notes are plain text on disk; cards reference
  them instead of copying them, so editing a note updates every card that
  points at it.
- **Local-first and open.** No account, no server, no telemetry. Your library
  is a directory of readable files plus a rebuildable index.
- **Agent-friendly.** A local interface lets an external agent read the
  library and write cards, without Remit shipping an AI of its own.

## Building and running

```sh
go build ./...
go test ./...
go run ./cmd/remit
```

Packaging and release builds go through MyGo's CLI:

```sh
go tool mygo build .
```

## Your library

A library is one directory of plain text; the app's derived index lives beside
it and may be deleted at any time.

```
<library>/
  remit.json              format version, FSRS weights, desired retention
  notes/<slug>-<id>.md    notes; a referencable block ends with ^block_<hex>
  cards/<id>.json         card definitions: block reference, overrides, status
  reviews.jsonl           append-only review log — the only scheduling source
  .remit/index.sqlite     derived index, rebuilt from the files above
```

`notes/`, `cards/`, `reviews.jsonl` and `remit.json` are the truth. Deleting
`.remit/` loses nothing: the app replays the log and reindexes the text.

## Moving a library

A `.remitlib` file is a zip of those plaintext sources — never the index — so
it can be inspected, versioned or handed to someone else.

```sh
go run ./cmd/remit -export ~/Desktop/library.remitlib
go run ./cmd/remit -import ~/Desktop/library.remitlib   # into a new library
go run ./cmd/remit -library /path/to/library            # name the library
```

Unpacking refuses an archive that escapes its target directory or one that
would overwrite a library. Writing and reading live in
`internal/core/transfer`.

## License

Remit is licensed under the GNU Affero General Public License, version 3. See
[LICENSE](LICENSE).
