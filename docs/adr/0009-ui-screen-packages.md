# ADR 0009 — Where the screens live, and what `internal/ui` keeps

- **Status:** accepted
- **Date:** 2026-11-07
- **Amends:** ADR 0004 (the path of a screen's package; the shape stands)
- **Related:** #128, ADR 0001, ADR 0005, ADR 0007

## Context

ADR 0004 fixed a screen's shape — `State` / `Deps` / `View` — and put the
review screen in `internal/ui/review`. The rest of the window stayed flat:
sixteen files in one package, all hanging off one `*App`. `browse.go` read
`a.editor`, `shell.go` read `a.browse`, `a.stats` and `a.palette`, and
`shell.go` alone held the toolbar, the notebook's layout, the panel, the
outline, the sidebar and the palette.

In Go a directory *is* a package, so "just move the files into folders" is not
available: every folder is an API. The flat files were therefore not a
readability problem but the absence of a boundary — no screen said what it asks
of the app, because it could ask for anything. Two consequences followed:

- A screen's test could only run by opening the whole app against a library.
- The design vocabulary the screens draw with (`tokens`: the scale, the paper,
  the semantic colours) lived in package `ui`, which a screen must not import.

## Decision

### Screens are a folder of their own

A screen that becomes its own package lives in `internal/ui/screens/<name>/`,
with ADR 0004's `State` / `Deps` / `View`. The vocabulary stays beside `ui`:

```
internal/ui/
  app.go view.go window.go sidebar.go panel.go outline.go palette.go
  library.go snapshot.go prefs.go options.go route.go i18n.go
  design/            theme, scale, tokens, motion, icons   (leaf)
  kit/               composition vocabulary                (leaf)
  maths/                                                   (leaf)
  screens/
    review/          State / Deps / View (ADR 0004)
    notes/ browse/ stats/ settings/ start/ about/          (extracted one at a time)
```

`screens/` rather than directly under `internal/ui/` because the window keeps
its own packages (`design`, `kit`) and its own files; as plugins contribute
pages (#146) the count of screens grows, and one folder must mean one thing.

### The token vocabulary belongs to the design language

`design.Tokens` (`design.NewTokens(base *ui.Theme)`) is the one description of
what a view draws with: the resolved theme, the paper's scale, the four review
ratings, elevation and the heat scale. `ui` and every screen resolve it from
the context's theme; no screen re-derives a palette and none imports `ui`.

### `internal/ui` keeps the assembly and the frame

One file per module, each owning one thing (the split that made this ADR
possible):

| File | Owns |
| --- | --- |
| `app.go` | `App`'s state, `NewApp`, the app's identity |
| `view.go` | which screen the window shows, and the switch between them |
| `window.go` | the toolbar, the key map, the window's find |
| `sidebar.go` | the notes list, its search and the tag filter |
| `panel.go` | the panel: outline and backlinks |
| `outline.go` | headings: read, list, jump to |
| `palette.go` | the command palette |
| `library.go` | open, close, reload, the derived queue |
| `snapshot.go` | `libraryData`, the store as the views read it |
| `prefs.go` | `settings.json`: read, write, remember the library |
| `options.go` | `Options`, the constructor's contract |
| `route.go` | deep links |

### Extraction order

A screen moves out when the epic touches it, one PR per screen, its tests
moving with it and the app-level tests staying to prove the assembly:

1. `about`, `start` — leaf screens, no store reach.
2. `settings`, `stats` — they read the snapshot and already have seams
   (`AgentControls`, `settings.Settings`).
3. `browse`, `relink` — rows derived by the app, written back through `Deps`.
4. `notes` — the editor, the document and the card panel; most of `a.editor`
   moves into the screen's `State`.

`internal/ui` is done when no screen state is left on `App` and every screen's
test runs without a library on disk.

## Consequences

- The dependency direction is unchanged in kind and sharper in practice:
  `cmd/remit → internal/ui → internal/ui/screens/* → internal/ui/{design,kit}`
  plus `internal/core`. A screen never imports `internal/ui` or a sibling.
- `design` grows a vocabulary every screen shares; a change to the scale or a
  rating colour reaches all screens without a second palette.
- The split already landed is behaviour-free: the files moved, nothing else.
- A screen that needs a new store answer adds a `Deps` function; reaching for
  `internal/core/store` from `screens/` is the review smell ADR 0004 names.
