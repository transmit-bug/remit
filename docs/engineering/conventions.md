# Engineering conventions

The one-page contract every PR is reviewed against. Closing decisions live in
the issue tracker; this file says how code is written, gated and merged.

## Layout and dependencies

- `cmd/remit` — process entry. `internal/ui` — views. `internal/core` — domain
  logic. See [ADR 0001](adr/0001-module-layout.md).
- Dependencies point one way: `cmd` → `ui` → `core`. `internal/core` never
  imports `mygo/ui`; if it needs to, the code belongs in `internal/ui`.
- Split a package when a boundary is needed. Do not create packages in
  anticipation of one.

## Tests

- `internal/core` is covered by table-driven tests: inputs and outputs, no
  window, no clock, no filesystem outside `t.TempDir()`.
- UI behaviour is driven through `ui.NewTester` against the real view, at the
  default **and** the minimum window size. Assert text and boxes; use
  `tt.Image()` only where the risk is painting.
- Tests assert external behaviour. A test that reaches into a private field or
  a widget's internals is a design signal, not a test.
- Time is injected, never read inside logic: no `time.Now()` in `internal/core`
  code paths that tests exercise.
- Every change must pass `CGO_ENABLED=0 go test ./...`; the shipped build does
  not need cgo.

## CI gates

A PR is mergeable only when every job is green:

1. `go vet ./...` and `go test ./...` on Linux.
2. `CGO_ENABLED=0 go test ./...`.
3. Cross-compiled binaries for `darwin/arm64`, `linux/amd64`, `windows/amd64`,
   uploaded as artifacts — platform breakage must surface before merge, not at
   release.
4. `go tool mygo vet .` where the tool directive resolves.

## Branch and PR flow

- Short branches off `master`, one concern each, merge by PR (squash).
- Branch names: `feat/…`, `fix/…`, `docs/…`, `chore/…`.
- Commit messages follow Conventional Commits: `type(scope): summary`, where
  type is one of `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`,
  `ci`, `chore`. The body explains *why*, not *what*.
- A PR body states the issue it closes, what was verified, and what remains
  unverified (native dialogs, GPU output, hardware).
- One issue per working session: claim it before the first commit, comment the
  resolution, close it.

## Licensing and dependencies

- The project is AGPL-3.0. Every dependency must be AGPL-3.0 compatible
  (MIT/BSD/Apache-2.0 are fine; GPL-2-only, proprietary or unclear licenses are
  not). Check the license before adding a dependency and note it in the PR.
- Do not copy code from a repository with an incompatible license. MyGo's
  `examples/notes` (MIT) is the preferred reference; MixMusic (GPL) is not.
- Prefer a mature, maintained library over a hand-rolled one; hand-roll only
  the product's own differentiators.

## MyGo discipline

- Before writing UI, reconcile the need against the MyGo component inventory
  (`mygo-components` skill) at the version in `go.mod`. A hand-rolled status
  dot, tab row, dialog or menu is a bug.
- A surviving gap becomes exactly one wrapper, with a note recording what was
  checked, why it is not the same need, and the MyGo version.
- State lives in the app's structs, never in the view; persistent identity is a
  stable key or `Bind`. Work from goroutines returns through `Window.Update`.
- Never write code against a MyGo version `go.mod` does not have. Upgrade with
  `go get` plus `go tool mygo migrate-ui .`, not by guessing APIs.
