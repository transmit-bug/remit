# Verification report

Run on `master` at `905f0be` on 2026-11-10. Every command below was run; the
output is what it printed. The tree is the 3-in-1 rebuild: the six-area window
`docs/specs/ui-redesign-v2.md` defines, its 52-face acceptance record
(`docs/verification/ui-acceptance.md`), and the prototype it follows
(`ref/prototype/`, whose 40 screens are re-captured by
`ref/prototype/tools/capture.mjs`). That commit carries the code and the
prototype; the commit after it is this report, which changes documentation only.

## Automatic checks

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | ok |
| Vet | `go vet ./...` | ok |
| Tests | `go test -count=1 ./...` | 43 packages, 1856 tests |
| cgo-free tests | `CGO_ENABLED=0 go test ./...` | ok |
| MyGo vet | `go tool mygo vet .` | ok |
| Formatting | `gofmt -l .` | empty |
| Catalogs | `go tool mygo i18n check .` | 995 messages in en, zh-Hans |
| 100k budget | `REMIT_PERF_CARDS=100000 go test ./internal/core/store -run TestPerformanceBudget` | due read 4.8 ms and review write 10.0 ms (first 25 ms) against 30/40 ms ceilings; queue 19 ms, counts 14 ms, notebook page 1.2 ms, card page 1.3 ms, excerpt tree 9.4 ms, one-note search 1.5 ms (331 ms when every row matches); open with the index current 387 ms, rebuilt 13.5 s |
| Telemetry grep | `grep -RIn --include='*.go' -E 'https?://'` minus the CI allowlist | nothing unexpected |
| Packaging | `go tool mygo build .` | `remit.app` 25.8 MB, `remit-0.1.0-arm64.dmg` 8.0 MB |
| Bundle metadata | `PlistBuddy` on `Info.plist` | version 0.1.0, scheme `remit`, `.remitlib` document type, `AppIcon.icns` |
| Prototype screens | `cd ref/prototype/tools && node capture.mjs` | 40 shots, 40 distinct hashes, each asserted for its own text |
| CI | run `42827244762` on `905f0be` | green: `test`, `test-default`, `privacy`, `perf`, and `build` for darwin, linux and windows |

## The 3-in-1 window

- **Six areas** — Today, Write, Read, Recall, Stats, Settings, plus a plugin
  group in the rail; `⌘1`–`⌘6` choose one, and a deep link names an area and a
  page.
- **Today** — one mixed queue (reading first, then recall) under a daily budget,
  the three counts, notes read but not yet excerpted, and tomorrow's load.
- **Write** — notes list, search, tags, the excerpt tree, assets and templates;
  the document is a 760 paper with a 48 gutter, an outline/backlinks panel, a
  block menu and inspector, a selection bar with in-place cloze, and the card
  editor; below 760 the gutter becomes a row above the paragraph.
- **Read** — the queue with each note's position and progress, a session driven
  by `J/K/1/2/3/E/S/A/D/P/M`, and an archive page. Archived, dismissed and
  postponed are events in `reviews.jsonl`, so they replay and survive an index
  rebuild.
- **Recall** — the queue grouped by note, a card table with sorting, filters,
  the `is:` search syntax and bulk actions, card types, the `remit/3` scheduling
  presets, today's history, and re-linking a lost reference.
- **Stats** — overview, 26-week heatmap, 14-day forecast, the production figures
  (characters, notes, excerpts, cards per excerpt) and the time share, all from
  the same review log.
- **Settings** — appearance, review, budget, keys, plugins, agent, update, about;
  each item writes and takes effect at once.
- **Overlays** — the command palette, the shortcut sheet, the import wizard (a
  source, its counts, then an activity bar while it writes), the export dialog
  and the whiteboard plugin page.
- **Plugins** — the ten drawn slots receive items, and a contribution reaches the
  host only through the capabilities its manifest declares.

The per-face record, the three suspended faces and every deviation live in
`docs/verification/ui-acceptance.md`; `missing` is 0, and the open items are
disclosed there rather than counted as passing.

## Long-term spec (#13)

- **AGPL-3.0** — `LICENSE` is the FSF text verbatim; dependencies are MIT
  (go-fsrs), BSD-3 (modernc sqlite), Apache/MIT (MyGo). Recorded per PR.
- **FSRS** — `internal/core/schedule` replays `reviews.jsonl` through
  `go-fsrs/v4`; weights and desired retention live in `remit.json`, and the
  `remit/3` presets override them per library.
- **Zero telemetry** — no analytics, no crash reporting; the CI privacy job
  fails on an unlisted URL; the update check is off by default and gated.
- **Go idioms, one-way `ui → core`** — `internal/core` imports no UI toolkit;
  `go list` and review enforce it.
- **Selection principle** — MyGo, go-fsrs, modernc sqlite, `archive/zip`,
  `crypto/ed25519`; nothing re-implemented that a maintained library provides.
- **i18n first day** — `locales/en.json` + `zh-Hans.json`, generated accessors
  committed, views read the frame's locale.

## Engineering baseline (#27)

- **Repository structure** — `cmd/remit`; `internal/core` (with `store`,
  `schedule`, `queue`, `stats`, `importmd`, `importanki`, `transfer`); `internal/ui`
  (the frame, the kit, the design tokens, `screens/*`); `internal/{agentapi,ext,
  deeplink,logging,settings,update}`; 43 packages; ADR 0001.
- **CI gates** — vet, the cgo-free suite, catalog parity and staleness, the
  example plugins, MyGo vet, the privacy grep, the 100k performance budget, and
  three-platform builds as artifacts; a tag-triggered release workflow.
- **Tests** — table-driven for core; `ui.NewTester` for views at 1024×720 and
  720×520, with `tt.FramePending()` asserted where the view settles.
- **Platform quartet** — `remit://` routes, `.remitlib` association with
  single-instance forwarding, tray with the remaining count, global hotkey with
  conflict reporting.
- **Release** — one stable channel, GitHub Releases through
  `go tool mygo build -upload`, Ed25519 update signatures independent of OS
  signing; `docs/release/`.
- **Privacy** — logs at `mygo.App.Path(PathLogs)` only, 0600; the update check
  explicit and switchable; no server unless the user enables the agent API.
- **Accessibility** — grading and the remaining count are announced, the flow is
  keyboard-only, and zh-Hans is asserted inside the smallest window.

## Product behaviour

- A library is plaintext: notes with inline block IDs, one JSON file per card,
  an append-only review log, and a derived SQLite index that rebuilds.
- Cards reference one block; live by default, freezable, and a lost card keeps
  its schedule.
- Reviewing writes the log, replays the card's state and updates the queue;
  undo appends a void event.
- The agent API is loopback-only, reads open, writes token-gated, and has no
  delete route.
- Extensions speak one contract through two substrates — a local process, or
  JavaScript in a 1 s / 64 MiB sandbox — and are enabled per library.
- `.apkg` imports into notes, cards and review history, from the command line
  (`-import-apkg`) and from the import wizard; `.remitlib` exports and re-imports.
  A wizard run leaves the frame, so a long package draws an activity bar rather
  than a frozen window.
- The window opens on Today's queue; a first run offers **Create library** or
  **Open library…** and remembers the choice.
- Settings carry the agent-API switch with token rotation, the update check with
  **Check now**, the global hotkey with conflict reporting, the daily budget and
  the scheduling presets; every change persists.

## Known limitations (not hidden as passing)

1. **Agent-API repaint**: a write through the agent API does not ask the window
   for a frame; the change appears on the next repaint the user causes.
2. **Update trust**: `updates.publicKey` and the signing key are the one manual
   step in `docs/release/releasing.md`; verification is tested with a generated
   key, the release identity is not yet chosen.
3. **Export has no busy state**: the S03 spinner is still open (acceptance gap
   #15); an export writes inside the click.
4. **Esc during an import** closes the wizard while the write continues; the
   result is applied but its report is not shown.
5. **Density**: one comfortable density ships; a compact mode is not in v1.

## Needs a person

[docs/verification/handoff.md](handoff.md) lists what a headless test cannot
prove: GPU rendering, live theme switching, the real tray and hotkey, native
dialogs, notarized first launch — and the checks the redesign and its audit
rework added on top.
