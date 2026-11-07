# ADR 0001 — Repository layout and dependency direction

- **Status:** accepted
- **Date:** 2026-11-06
- **Related:** #24, #27, #9

## Context

Remit is a desktop app built on MyGo, a Go immediate-mode UI framework. The
domain logic (content model, storage, scheduling, queue, stats) must stay
testable and reusable without a window, and the UI must not become the only
place where behaviour lives. #9 will refine package boundaries once the
extensibility architecture is decided; until then the repository needs a
minimal skeleton that does not have to be undone.

## Decision

```
cmd/remit/        process entry: MyGo app, windows, wiring
internal/core/    domain logic; imports neither MyGo nor internal/ui
internal/ui/      views; imports internal/core and mygo/ui
docs/adr/         decisions with a lasting architectural effect
```

- Dependencies point one way: `cmd` → `internal/ui` → `internal/core`.
  `internal/core` may depend on ordinary libraries (storage, FSRS) but never on
  a UI toolkit.
- `internal/` rather than `pkg/`: nothing here is a public import path. The
  product is the app; agent and extension surfaces are contracts, not Go
  packages.
- Packages are split when a boundary is needed, not in advance. #9 owns the
  final domain package boundaries and supersedes this sketch where they
  disagree.
- `main.go` lives in `cmd/remit`, so the tree has room for a second binary
  (tooling, extension host) without moving anything.

## Consequences

- Core logic is testable with plain table-driven tests and `CGO_ENABLED=0`.
- UI tests use `ui.NewTester` against `internal/ui`.
- Moving code from `internal/ui` into `internal/core` is the normal way to
  remove a dependency in the wrong direction; the reverse is a review smell.

## Amendment (2026-11-08)

The tree has grown focused services beside the three above — `internal/ext`
(loader, manifests, capabilities, the JS host), `internal/ext/contrib` (the
window contract), `internal/ext/builtin` (the shipped plugins), `internal/agentapi`,
`internal/settings`, `internal/update`, `internal/deeplink`, `internal/logging`
— and the plugin tools (#145) are subcommands of `cmd/remit`, not a second
binary. The rule this ADR exists for is unchanged: nothing in `internal/core`
imports a UI toolkit or `internal/ui`, and a screen package reaches neither the
store nor `internal/ui`.
