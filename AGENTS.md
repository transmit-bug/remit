# Repository Guidelines

## Project Overview

Remit is a local-first, open-source knowledge-memory desktop app: block-level
plain-text notes and FSRS-scheduled flashcards share one library. The Review
Queue is the first screen; notes are first-class; no account, server, or
telemetry. A loopback HTTP API lets external agents read the library and
write cards. Built in Go on MyGo (`github.com/egoist/mygo`), AGPL-3.0,
pre-1.0 — the data format and agent interface may change between minor
versions.

This is a fast-moving project. This file describes direction and purpose,
not pinned implementation detail; where the code and this file disagree, the
code wins and this file gets updated.

## Agent skills

### Issue tracker

GitHub Issues in `transmit-bug/remit` (via `gh` CLI). See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### MyGo implementation skills (mandatory)

Every implementation session and every subagent it spawns reads, before writing
code:

1. `.agents/skills/mygo-usage/SKILL.md` — app structure, state and identity,
   threads and frames, `ui.NewTester` headless tests, verification commands.
2. `.agents/skills/mygo-components/SKILL.md` before any UI work — reconcile each
   need against the official widget inventory before composing a layout by hand.

Reach for `mygo-motion`, `mygo-drawing`, `mygo-desktop`, or `mygo-maintenance`
when the change is about motion, custom painting, OS services, or fixing an
existing MyGo feature. The MyGo version that matters is the one `go.mod` pins.

### Engineering conventions

`docs/engineering/conventions.md` — layout and dependency direction, test
rules, CI gates, branch/PR flow, licensing of dependencies.

## Architecture & Data Flow

- **Dependency direction:** `cmd/remit` (process + OS services) →
  `internal/ui` (views) → `internal/core` (domain + storage). One-way only:
  core stays free of UI imports so it remains testable as pure
  inputs/outputs.
- **Truth lives in plain text.** A library is a directory of readable files —
  notes, card definitions, and an append-only review log that is the sole
  scheduling source. Everything derived (the SQLite index) is a cache that
  can be deleted and rebuilt at any time. The goals: durability, no lock-in,
  replayable history.
- **Stable minted identity.** IDs are minted once, never derived from titles
  or paths; block IDs live inline in the note text so references survive
  editing and reordering.
- **UI is immediate-mode over snapshot state.** State lives in app structs,
  never in views: `View` reads state and returns; handlers mutate it, write
  to the store on the UI thread, then retake a snapshot — no frame touches
  the store.
- **Threading rule:** UI writes happen on the UI thread. Long or external
  work (agent API server, update checks, extension IO) runs on goroutines
  and returns results through `Window.Update` callbacks — never by touching
  UI state directly.
- **OS services are injected seams.** Dialogs, hotkeys, tray, and update
  checks reach `internal/ui` only through `Options` structs and callback
  interfaces, so the whole UI runs headless in tests.
- **Safety posture for external input:** the agent API is loopback-only,
  token-gated for writes, and add-only (no deletes); extensions run as
  external processes with strict timeouts; archive import refuses path
  escape and refuses to overwrite an existing library.

## Key Directories

| Path | Purpose |
| --- | --- |
| `cmd/remit/` | The binary: bootstrap, offline CLI (`-library`, `-export`, `-import`), tray/hotkey/deep-link/file-association wiring |
| `internal/core/` | Domain and storage, UI-free: content model, scheduling, queue, stats, library store, transfer, Anki import |
| `internal/ui/` | All views in one package: app state, root `View`, per-mode views, theme tokens |
| `internal/*` (rest) | Focused service packages — settings, agent API, update, deeplink, extensions, logging — split out only when a boundary is needed |
| `locales/` | i18n catalogs (`en` source, `zh-Hans`); generated catalog code is committed |
| `docs/` | Engineering contract, agent/tracker policy, release and verification docs |
| `.agents/skills/` | Vendored MyGo skills (pinned by `skills-lock.json`) — the mandatory pre-reads above |

The layout is not frozen: expect new packages under `internal/` as boundaries
emerge. Nothing is exported, so there is no `pkg/`.

## Development Commands

```sh
go build ./...                      # build
go vet ./...                        # lint (part 1)
go test ./...                       # full suite
CGO_ENABLED=0 go test ./...         # REQUIRED pass: shipped builds are cgo-free
go run ./cmd/remit                  # run the app
go tool mygo vet .                  # lint (part 2)
go tool mygo i18n check .           # catalog completeness
go tool mygo generate               # regenerate locales/; commit the result
go tool mygo dev -web               # dev loop
go tool mygo build .                # package

# Perf budget (CI runs at 100k cards; skips under -short)
REMIT_PERF_CARDS=100000 go test ./internal/core/store/ -run TestPerformanceBudget -v -count=1 -timeout 20m
```

CI gates every PR: vet, tests, cgo-free tests, i18n catalogs in sync, MyGo
vet, perf budget, a zero-telemetry privacy scan, and cross-compilation for
darwin/linux/windows. Releases are tag-driven via `go tool mygo build …
-upload`; runbook in `docs/release/`.

## Code Conventions & Common Patterns

- Go standard formatting; the lint surface is `go vet` + `go tool mygo vet` —
  no golangci-lint, no Makefile or scripts layer; stock `go` commands plus
  `go tool mygo`.
- **Errors:** sentinel errors in core; the UI reports them (log + toast) and
  degrades — never panic. Unrecoverable-but-openable states become views;
  broken optional services are logged and skipped.
- **State & time:** clocks are injected (`Options.Now`); no `time.Now()` in
  core code paths that tests exercise.
- **i18n from day one:** every user-facing string goes through the catalog;
  the `zh-Hans` translation lands in the same change.
- **MyGo wrappers:** at most one thin wrapper per real gap in the widget
  inventory, each recorded with a note. Upgrade MyGo with `go get` +
  `go tool mygo migrate-ui .`.
- **Privacy:** zero telemetry. New outbound URLs are deliberate — the CI
  privacy job fails on any URL outside its allowlist.
- **Licensing:** dependencies must be AGPL-3.0-compatible (MIT/BSD/Apache-2.0
  fine; GPL-2-only, proprietary, or unclear — not). Check before adding,
  note it in the PR, and never copy code from incompatible repos.
- **Branch/PR flow:** short branches off `master` (`feat|fix|docs|chore/…`),
  one concern each, squash-merged; Conventional Commits. The PR body states
  what was verified and what remains unverified.
- **Vocabulary:** use `GLOSSARY.md` terms exactly and avoid its listed
  synonyms — the glossary is the tie-breaker for naming anything.

## Important Files

| File | Why it matters |
| --- | --- |
| `GLOSSARY.md` | Canonical domain vocabulary; name things with these terms |
| `docs/engineering/conventions.md` | The contract every PR is reviewed against |
| `mygo.json` | Packaging config; its `version` is kept equal to `internal/ui.Version` by a test |
| `go.mod` | Pins Go, MyGo, and dependencies; the tool directive powers `go tool mygo` |
| `README.md` | Library format, CLI flags, build/run basics |

## Runtime/Tooling Preferences

- Pure Go, cgo-free: everything must build and test with `CGO_ENABLED=0`.
  No Node/Bun/JS toolchain of any kind.
- Only CLIs: `go` and `go tool mygo` (pinned by `go.mod`'s tool directive —
  never install MyGo separately) plus `gh` for issues and releases.
- Versions are pinned in-repo (`go.mod`, `skills-lock.json`); don't drift
  from those pins.
- No Makefile, scripts directory, or custom build layer; the commands above
  are the entire surface.

## Testing & QA

- Stdlib `testing` only — no assert libraries, no mocks; fakes implement the
  injected seams. Table-driven tests with `t.Run`; helpers marked
  `t.Helper()`.
- **Core:** tested as pure inputs/outputs from external `_test` packages —
  no window, no clock, no filesystem beyond `t.TempDir()`; fixtures are
  built in code, not golden files.
- **UI:** headless via `ui.NewTester` driving the real `View`. Every mode
  must pass at both the default and the minimum window size (see
  `internal/ui/app_test.go`); Chinese must fit at the minimum size; assert
  text and boxes rather than pixels (`tt.Image()` only when the risk is
  paint); drive real input (keys, clicks, scroll) and the virtual clock.
- **Persistence:** assert against the on-disk plaintext, not internals.
- **Gates:** no coverage number — the bar is the full suite, the cgo-free
  run, the perf budget, and the CI privacy/i18n/platform jobs. What headless
  tests cannot prove is listed in `docs/verification/`.
