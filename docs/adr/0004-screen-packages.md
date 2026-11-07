# ADR 0004 — Screen packages, and what a screen may reach

- **Status:** accepted
- **Date:** 2026-11-07
- **Amended by:** ADR 0009 (a screen's package is `internal/ui/screens/<name>/`;
  the shape below stands)
- **Related:** #82, #88, ADR 0001, ADR 0005

## Context

ADR 0001 said packages are split when a boundary is needed, "not in advance",
and named no boundary inside `internal/ui`. In practice `internal/ui` had grown
into one package holding the app's state, every screen, the store's snapshot,
the agent-facing handlers and the design vocabulary. A review-screen change
touches a file that also owns the notebook's editor state, so it can only be
tested by starting the whole app against a real library on disk.

The review screen is the first screen rewritten (#82). It is also the one with
the sharpest contract: a queue the store derives, one card at a time, four
grades, and a session order the store does not own. That shape belongs to a
package that can be driven with fakes.

## Decision

A screen that is rewritten becomes its own package under `internal/ui/`, with
the shape `State` / `Deps` / `View`:

```
internal/ui/review/
  State   the session: what the screen remembers across frames
  Deps    what the screen asks the app for — functions, not the store
  View    the frame: reads State and Deps, draws, schedules nothing
```

- `State` holds the session only. What the store owns — the cards, the queue's
  membership, the review log — stays out of it; the app passes the derived
  answer in (`Sync(due)`) and reads it back.
- `Deps` is the whole of the screen's reach: one function per thing the user
  can do, plus the lookup that resolves a card to what is drawn. The screen
  imports `internal/ui/design` and `internal/ui/kit`; it never imports the
  store, `internal/core/store`, or `internal/ui`.
- `internal/ui` keeps the assembly: it owns the library snapshot, loads the
  queue, implements every `Deps` function, and hands the screen a `Deps` built
  per frame. Handlers that write stay there, on the UI thread.
- The screen's tests live in the package and use fakes for `Deps`. The app-level
  tests keep exercising the assembled window.
- `kit` is where a screen's vocabulary is composed (ADR 0005's note editor will
  reuse it); the screen does not invent its own faces.

Dependency direction, therefore:

```
cmd/remit → internal/ui → internal/ui/{review,notes,…} → internal/ui/{design,kit}
                                  ↘ internal/core            ↙
```

`design` and `kit` are leaves: they import the framework and nothing of the app.
A screen never imports a sibling screen.

## Consequences

- The review screen is tested without a library, a clock or a disk, at both the
  sizes the window opens at. A regression in grading, undo or the queue's order
  no longer needs a seeded store to show itself.
- `internal/ui`'s app tests keep their value: they prove the assembly — that
  the screen is handed the right card and that its actions write.
- This amends ADR 0001's "not in advance": the split is still not speculative —
  it happens when a screen is rewritten — but it is now expected for every
  screen the epic touches, and the shape is fixed (`State`/`Deps`/`View`).
- A new screen that needs the store adds a `Deps` function; reaching for
  `internal/core/store` from a screen is the review smell this ADR exists to
  prevent.
