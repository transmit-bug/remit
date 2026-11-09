# Remit — A local-first, open-source knowledge base where notes and flashcards share one home: write it once, remember it forever.

## Status

Early development. Remit is pre-1.0 and not yet usable for real libraries; the
data format and the agent interface may still change between minor versions
(see `docs/engineering/conventions.md`).

## What it is

Remit is a local-first, open-source **3-in-1 knowledge-memory workbench**:

- **A complete Markdown notebook.** Notes are plain text on disk; blocks are
  addressable, links and backlinks are derived, and assets live in the
  library. This is the Obsidian-like layer.
- **An Anki-grade card system.** Card types, fields and templates turn a block
  into a reviewable card; FSRS schedules it through a Review Queue. This is
  the Anki-like layer.
- **A SuperMemo-style reading loop.** A Reading Queue brings long notes back
  by FSRS cadence, remembers the block where reading stopped, and turns the
  current block into a referenced excerpt that can become a card. This is the
  incremental-reading layer.
- **One model, one library, one history.** Notes, cards and reading share the
  same plaintext library and the same append-only review log; the index is
  derived and rebuildable.
- **Local-first, open, agent-friendly.** No account, no server, no telemetry.
  A loopback agent API reads the library and writes cards with a token; it
  exposes no deletes.

The full 3-in-1 product definition, including what 1.0 ships and what full
parity with Obsidian, Anki and SuperMemo means after 1.0, is in
[`docs/specs/three-in-one.md`](docs/specs/three-in-one.md).

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

## Plugins

Card derivation, scheduling policy and the window's commands, menus and pages
are plugins. The two the app ships are plugins too, loaded by the same code.
A plugin is a directory with an `extension.json` manifest and a JavaScript file;
it reaches the host only through the capabilities its manifest declares.

```sh
go run ./cmd/remit ext test examples/plugins/cards-mini  # hooks against its fixtures
go run ./cmd/remit ext check <dir>                       # manifest and contributions
go run ./cmd/remit ext types -o remit.d.ts               # the contract artifact
```

The contract and the guide are [`remit.d.ts`](remit.d.ts) and
[`docs/specs/plugins/writing-a-plugin.md`](docs/specs/plugins/writing-a-plugin.md).

## License

Remit is licensed under the GNU Affero General Public License, version 3. See
[LICENSE](LICENSE).
