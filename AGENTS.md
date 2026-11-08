# Repository Guidelines

## Project Overview

Remit is a local-first, open-source knowledge-memory desktop app: plain-text
notes and FSRS-scheduled flashcards share one library. Go on MyGo
(`github.com/egoist/mygo`), AGPL-3.0, pre-1.0 — the data format may still change.

Design philosophy — **Remember it**: every feature either helps the user
remember (writing, review, revisiting, incremental digestion) or carries the
memory (the plaintext library, the plugin system); a feature serving neither is
out of scope. Notes are complete Markdown documents — the block is the unit of
reference, not the shape of the note. The plugin system is a core capability:
it extends both notes and flashcards, and the built-in behaviours are plugins.

This file holds what stays true as the code moves: what the app is, the
architecture it keeps, where things live, and how to build it. It states
direction, not implementation detail; where the code and this file disagree,
the code wins and this file follows.

## Where to read

| Material | It answers |
| --- | --- |
| `GLOSSARY.md` | the domain vocabulary — name things with its terms |
| `docs/adr/` | decisions already taken, one per file; read the one covering the area before reshaping it |
| `docs/specs/` | designs in progress for an area that is being rewritten; plugin contracts live under `docs/specs/plugins/` |

## Architecture

- **The library is plain text.** Notes, card definitions and an append-only
  review log are the truth; the index is derived and may be deleted and
  rebuilt at any time. Durability, no lock-in, replayable history.
- **Identity is minted once**, never derived from titles or paths; block IDs
  live inline in the note text so references survive editing and reordering.
- **Dependencies point one way:** `cmd/remit` → `internal/ui` → `internal/core`.
  `internal/core` stays UI-free, so it can be tested as pure inputs/outputs.
- **UI is immediate-mode over snapshot state.** State lives in app structs;
  a `View` reads it and returns. Handlers mutate state, write to the store on
  the UI thread, then retake the snapshot — no frame touches the store.
- **OS services are injected seams** (dialogs, hotkeys, tray, updates) through
  `Options` structs and callback interfaces, so the whole UI runs headless.
- **The agent API is loopback-only** and token-gated for writes; it exposes no
  deletes.
- **Plugins run under the app's limits, not its trust.** Two substrates, one
  contract: `exec` (a local process speaking newline-delimited JSON on its
  pipes) and `js` (JavaScript in this process, 1 s and 64 MiB per call, a pool
  of two runtimes, no dynamic code). A plugin reaches the host only through the
  capabilities its manifest declares; the host draws every pixel a plugin
  contributes. Built-in plugins load through the same loader with no
  privileged door. The in-process substrate is **not** process isolation — an
  engine memory-safety bug is the app's (ADR 0006).

## Key directories

| Path | Purpose |
| --- | --- |
| `cmd/remit/` | the binary: bootstrap, offline CLI, tray/hotkey/deep-link wiring |
| `internal/core/` | domain and storage, UI-free |
| `internal/ui/` | the views, the design language, the kit |
| `internal/*` (rest) | focused services — agent API, extensions, settings, update, deeplink, logging |
| `locales/` | i18n catalogs, `en` source plus `zh-Hans` |

New packages appear as boundaries emerge; nothing is exported, so there is no `pkg/`.

## Commands

```sh
go build ./...
go test ./...
CGO_ENABLED=0 go test ./...     # the shipped build is cgo-free
go run ./cmd/remit
go run ./cmd/remit ext test examples/plugins/cards-mini  # a plugin's hooks against its fixtures/
go run ./cmd/remit ext check <dir>                       # manifest and contributions only
go run ./cmd/remit ext types -o remit.d.ts               # regenerate the plugin contract
go tool mygo vet .
go tool mygo i18n check .       # catalog completeness
go tool mygo generate           # regenerate locales/
go tool mygo build .            # package
```

MyGo arrives through `go.mod`'s tool directive; never install it separately.
`remit` is not a tool directive, so the plugin commands run as
`go run ./cmd/remit ext …`, not `go tool remit ext …`.

## Conventions that outlive the code

- **Errors:** sentinel errors in core; the UI reports them (log + toast) and
  degrades — broken services are skipped, openable states become views,
  nothing panics.
- **Time is injected:** `store.Options.Now` — core reads no clock of its own.
- **i18n from day one:** every user-facing string goes through the catalog.
- **Privacy:** zero telemetry; a new outbound URL is a deliberate decision.
- **Licensing:** dependencies stay AGPL-3.0-compatible.
- **Tests earn their place:** see Preference → Tests catch regressions. A red
  job on `master` is a bug, not a known limitation.

## Preference

### Prefer MyGo's own mature widgets

### Tests catch regressions

A test does not prove the code runs; it turns red when you break it. So a test is
worth what the bug it can catch is worth: if you cannot name a bug that would
make it fail, do not write it — and delete the ones you cannot name. The
judgement, the four shapes that fail it and the cleanup rule: `docs/specs/tests.md`.

- Test behaviour, not implementation — a refactor leaves it green, a broken
  behaviour turns it red.
- A useless test is not harmless surplus: it buys false confidence and drowns a
  real regression in noise.
- One test guards one thing, and its name is that thing.
- A test is documentation: whoever reads it learns how the system should behave.

## Agent skills

### Issue tracker

GitHub Issues in `transmit-bug/remit` (via `gh` CLI). See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
