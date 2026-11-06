# v1 verification report

Run on `master` at the merge of the accessibility slice. Every command below was
run; the output is what it printed.

## Automatic checks

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | ok |
| Vet | `go vet ./...` | ok |
| Tests | `go test ./...` | 15 packages ok |
| cgo-free tests | `CGO_ENABLED=0 go test ./...` | ok |
| MyGo vet | `go tool mygo vet .` | ok |
| Formatting | `gofmt -l .` | empty |
| Catalogs | `go tool mygo i18n check .` | 144 messages in en, zh-Hans |
| 100k budget | `REMIT_PERF_CARDS=100000 go test ./internal/core/store -run TestPerformanceBudget` | due read 3.5 ms, review write 20 ms, open 387 ms, rebuild 4.1 s (ceilings 30/40 ms) |
| Telemetry grep | `grep -RIn --include='*.go' 'https\?://'` minus the allowlist | nothing unexpected |
| Packaging | `go tool mygo build .` | `remit.app` 15.4 MB, `remit-0.1.0-arm64.dmg` 5.0 MB |
| Bundle metadata | `PlistBuddy` on `Info.plist` | version 0.1.0, scheme `remit`, `.remitlib` document type, `AppIcon.icns` |
| CI | PR runs | five jobs green: test, privacy, three platform builds |

## Long-term spec (#13)

- **AGPL-3.0** — `LICENSE` is the FSF text verbatim; dependencies are MIT
  (go-fsrs), BSD-3 (modernc sqlite), Apache/MIT (MyGo). Recorded per PR.
- **FSRS** — `internal/core/schedule` replays `reviews.jsonl` through
  `go-fsrs/v4`; weights and desired retention live in `remit.json`.
- **Zero telemetry** — no analytics, no crash reporting; the CI privacy job
  fails on an unlisted URL; the update check is off by default and gated.
- **Go idioms, one-way `ui → core`** — `internal/core` imports no UI toolkit;
  `go list` and review enforce it.
- **Selection principle** — MyGo, go-fsrs, modernc sqlite, `archive/zip`,
  `crypto/ed25519`; nothing re-implemented that a maintained library provides.
- **i18n first day** — `locales/en.json` + `zh-Hans.json`, generated accessors
  committed, views read the frame's locale.

## Engineering baseline (#27)

- **Repository structure** — `cmd/remit`, `internal/core{,/store,/schedule,/queue,/stats,/importanki,/transfer}`, `internal/ui`, `internal/{agentapi,ext,deeplink,logging,settings,update}`; ADR 0001.
- **CI gates** — vet, tests, cgo-free tests, catalog parity and staleness, privacy grep, three-platform binaries as artifacts, tag-triggered release workflow.
- **Tests** — table-driven for core; `ui.NewTester` for views at 1024×720 and
  720×520, with `tt.FramePending()` asserted where the view settles.
- **Platform quartet** — `remit://` routes, `.remitlib` association with
  single-instance forwarding, tray with the remaining count, global hotkey with
  conflict reporting.
- **Release** — one stable channel, GitHub Releases through `go tool mygo
  build -upload`, Ed25519 update signatures independent of OS signing;
  `docs/release/`.
- **Privacy** — logs at `mygo.App.Path(PathLogs)` only, 0600; update check
  explicit and switchable; no server unless the user enables the agent API.
- **Brand** — geometric R, About page, packaging wiring; official design is a
  later effort.
- **Accessibility** — review changes are announced, the flow is keyboard-only,
  and zh-Hans is asserted inside the smallest window.

## Product behaviour (after the audit pass)

- A library is plaintext: notes with inline block IDs, one JSON file per card,
  an append-only review log, and a derived SQLite index that rebuilds.
- Cards reference one block; live by default, freezable, and a lost card keeps
  its schedule.
- Reviewing writes the log, replays the card's state and updates the queue;
  undo appends a void event.
- The agent API is loopback-only, reads open, writes token-gated, and has no
  delete route.
- Extensions run as separate processes, enabled per library, notified of
  `note.saved`, `card.created`, `review.recorded`.
- `.apkg` imports into notes, cards and review history, from the command line
  (`-import-apkg`) and from the palette's **Import Anki package…**; `.remitlib`
  exports and re-imports.
- The window opens on today's queue; a first run offers **Create library** or
  **Open library…** and remembers the choice.
- The editor edits a note's title, adds, splits, moves and deletes blocks, and
  makes a card from a block; splitting keeps a card's reference on the side
  that held its text.
- A lost card can be re-linked to another block from review or browse, keeping
  its history.
- Settings (palette or `Cmd/Ctrl+,`) carry the agent-API switch with token
  rotation, the update check with **Check now**, the global hotkey with conflict
  reporting, and notifications; every change persists.
- The tray shows the remaining count, starts a review, pauses reminders, and
  offers an update when one exists.
- The statistics page is a calendar heatmap plus retention and forecast.

## Known limitations (not hidden as passing)

1. **Concurrency**: writes made by the agent API do not push a repaint to the
   window; the UI reloads on its own actions. Acceptable for one local agent,
   not for a fan-out.
2. **Update trust**: `updates.publicKey` and the signing key are the one manual
   step in `docs/release/releasing.md`; verification code is tested with a
   generated key, the release identity is not yet chosen.
3. **Rich text**: blocks are plain text; Markdown rendering is not in v1.
4. **Density**: one comfortable density ships; a compact mode is not in v1.

## Needs a person

[docs/verification/handoff.md](handoff.md) lists what a headless test cannot
prove: GPU rendering, live theme switching, the real tray and hotkey, native
dialogs, notarized first launch.
