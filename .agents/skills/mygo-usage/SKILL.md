---
name: mygo-usage
description: How a native MyGo app is put together, and how a change to one is built and verified. Use when wiring a window or a view, deciding where state, identity and threads belong, writing or fixing a ui.NewTester headless test, or checking a MyGo UI change before handoff. Framework-level - the project keeps its own package layout, harness names, sizes and commands.
---

# Using MyGo

A MyGo app is a runtime, a window, and a view function MyGo calls to rebuild the
interface from state. In immediate-mode terms (GPUI, Dear ImGui), the view
function is the frame's build pass: elements expire with it, and identity — a
stable `Key` — is what carries state across frames; the engine underneath is
retained and repaints only what changed. Everything below follows from that one
arrangement; the sections are independent, so reach for the one the change
touches.

## Where the rest is

| Material | Reached for |
|---|---|
| `../mygo-components/SKILL.md` | which widget carries the meaning, layout, feedback, the official inventory |
| `../mygo-desktop/SKILL.md` | dialogs, notifications, menus, tray, global shortcuts, secrets, watching files, dark mode, power, network, lifecycle beyond `Run` |
| `../mygo-web/SKILL.md` | web pages: bindings, the generated client, plugins, trust, custom protocols |
| `../mygo-motion/SKILL.md` | making a change move: transitions, springs, keyframes, timelines, gestures, animation testing |
| `../mygo-drawing/SKILL.md` | custom painting: `Draw`/`Painter`, canvases, shaders, shaped text, paint cost |
| the app's `AGENTS.md`, and the docs it names | its package layout, its environment/harness types, its sizes, its commands |
| `$MYGO/docs/windows.md` and `$MYGO/docs/ui/windows.md`, where `MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)` | windows beyond the first: `NewWindow`, `WindowOptions` (`StateKey`, `Parent`, `Modal`, `Hidden`), remembered bounds, flyouts, panels |
| `$MYGO/docs/ui/` (same `MYGO`) | the UI prose: `views.md`, `layout.md`, `testing.md`, `windows.md`, `browsers.md` (the same native UI as a web page), `accessibility.md`, `performance.md`, catalogue `README.md` |
| `$MYGO/docs/` (same `MYGO`) | the non-UI prose: `native.md`, `menus.md`, `app.md`, `updates.md` (auto-update via `mygo.Updater`), `configuration.md`, `cli.md`, `distribution.md` |

## How a MyGo app is put together

- `mygo.App` owns the process: it is told what to do `WhenReady`, and `Run`
  drives it until the last window closes. Windows are created there, on the main
  thread, once — not rebuilt per frame.
- Windows beyond the first are `mygo.NewWindow(mygo.WindowOptions{…})`, once
  in `WhenReady` like the first. `Parent` keeps a child above its owner and
  closes it with the owner; `Modal` also blocks the owner; `StateKey`
  remembers bounds and maximized state across runs.
- A window's content is `ui.View(f)` with `f func(*ui.Context)`. MyGo calls `f`
  for every frame, so the view is **rebuilt from state**, never mutated. Frames
  happen for input, for timing, and for `Invalidate`.
- State therefore lives outside the view: the app's own structs, widget state
  structs, or `ui.Local`. The view reads it; handlers change it.
- Elements and the parts a `*Base` constructor returns expire before the next
  build pass. Store state and `ui.Handle`, never an element.
- `*ui.Context` is the frame's handle on the runtime, not app state: keep it out
  of long-lived fields and worker goroutines, and take `c.Services()` when a
  callback must outlive the pass.

## What this version has

The version protocol every mygo skill points here for: `go list -m -f
'{{.Version}}' github.com/egoist/mygo` is what compiles; the official site
documents the newest release, and the app may **lag** it. A gap between
them is an upgrade — `go get github.com/egoist/mygo@latest`, `go tool mygo
migrate-ui .` (its guide is `$MYGO/docs/ui/migration.md`) — not code
written against a version `go.mod` does not have. A widget the site shows
and the module lacks is that same signal, not a reason to wrap it.

## Choosing a widget

Reconcile each need against `../mygo-components/SKILL.md` before writing a box.
A view that hand-rolls a status dot, a tab row or a confirm dialog is a bug: it
costs keyboard, focus, disabled and screen-reader behaviour at once, and it is
the behaviour, not the pixels, that was being bought.

A gap that survives that reconciliation becomes one wrapper in whatever package
the app keeps for it — not a copy in the page, and not a second copy elsewhere.

## State, identity, input

In framework terms, the rules that bite:

- Identity is significant: a control whose state must survive a rebuild needs a
  stable key, given before its children and before local state.
- Persistent identity is `.Bind(&handle)`: `handle.Focused(c)` and
  `FocusWithin(c)` read committed focus, `handle.Focus(c)` and
  `CancelFocus(c)` move it, `FocusBind(&a.pane, values)` points focus at an
  app field. `handle.Bounds(c)` is committed geometry for input callbacks;
  `c.Resolve(handle)` is only the current build's element — do not capture
  an element to query it later.
- Elements and the parts a `*Base` constructor returns expire at the end of
  their build pass — another pass in the same frame included. Stale use
  panics in development and the tester, and silently no-ops in production:
  fix the ownership; `Valid()` is a quiet check, not a fix.
- Handlers (`Clicked()`, `Changed()`, `Submitted()`) run after construction and
  after bound inputs update; a bound value's notice arrives in the following
  pass, so write derived values back where they are read, not where they are
  built.
- Prefer binding to persistent model fields. A value recomputed each pass loses
  the edit before its notice.

## Writing a view function

- The pass is for building: one event may run it several times, so it reads
  state and makes no change of its own. Effects belong in handlers, or in a
  guarded tail after the children are built — out of the loops.
- To change a collection while building it, record now, act after:

```go
removed := -1
ui.Column(c).Children(func() {
	for _, it := range app.items {
		if ui.Button(c, "Remove").Clicked() {
			removed = it.ID // after the loop
		}
	}
})
if removed >= 0 {
	app.remove(removed)
}
```

- Interactivity is a query: `if b.Clicked() {…}` handles a local action beside
  the widget; `OnClick`/`OnShortcut` when the action must outlive the pass or
  routes a command.
- Frame-local state of one view is `ui.Local`; state shared across views or
  windows belongs to the app's own structs.

## Threads and frames

- The UI thread builds the view and runs handlers. Slow I/O and long computation
  belong on a goroutine of the app's own.
- `Window.Update(fn)` is the way back to the UI thread; `Invalidate()` only asks
  for a frame. Both are safe from any goroutine.
- `Update` is asynchronous. Cancel obsolete work and drop stale results rather
  than letting a late callback overwrite a newer screen.

## Testing a view headless

`ui.NewTester(view, w, h)` renders the view in memory and drives it without a
window, as fast as a unit test; `tt.Frame()` draws one frame. Build the smallest
view that carries the risk:

| Layer | What goes under the tester | What it catches |
|---|---|---|
| One widget | the widget in a small view | pixels, via `tt.Image()` — drawing that parses but never renders |
| A page or panel | the page's view, with the app's I/O replaced by the fakes it already has | text, boxes, focus, handlers |
| The whole window | the view that holds the whole window, at the size the app opens at | overlaps, scrolling, dispatch between sections |
| Startup | whatever builds the window, plus its config | wiring: settings, logging, defaults |

What the tester can do — the calls whose behaviour is not guessable; the
full surface is `$MYGO/docs/ui/testing.md`:

| Need | Calls |
|---|---|
| Pointer | `tt.ClickAt`, `tt.ClickWith(ui.Shift, …)`, `tt.Scroll` (touchpad, immediate) vs `tt.Wheel` (one notch, glides as the clock advances) |
| Keyboard | `tt.Compose`, `tt.Command("copy")` |
| Menus | `tt.RightClick`, `tt.ChooseMenuItem("Move to", "Archive")` |
| The window | `tt.SetScale`, `tt.SetDark`, `tt.SetPreferences`, `tt.SetLocale`, `tt.SetDirection` |
| What it shows | `tt.Find` (a box), `tt.Image()` (pixels), `tt.Clipboard`, `tt.Announcements`, `tt.OpenedURLs` |
| Frames and time | `tt.Frame`, `tt.FramePending`, `tt.Advance(d)`, `tt.Now()` |

Two clocks, and mixing them up is the usual flake:

- **View time is virtual.** Animations, tooltips, toasts, double clicks and
  `c.After` follow `tt.Advance`, so a test reads the same on a busy machine as on
  an idle one. `tt.Advance` runs only the frames the view asks for: after
  changing state from outside the view, call `tt.Frame()` first.
- **Goroutine time is real.** A job on its own goroutine is not driven by the
  virtual clock: wait for it through whatever the app already has — a done
  channel, a queue it drains — instead of sleeping to let a frame happen.

Signals worth asserting, because they catch layout bugs that nothing else does:

- The window at its **minimum** size as well as its default. A control laid out
  under a fixed bar is invisible and unclickable, and only a `tt.Find` box
  comparison notices.
- `tt.FramePending()`, for a view that never settles: an animation that restarts
  itself, a job that never reports.
- Text and boxes, over renders; `tt.Image()` only where the risk is drawing.

Done when: the test fails before the fix and passes after, it drives the real
interaction rather than a helper, and `tt.FramePending()` is false wherever the
view should settle.

## Verifying a change

```bash
go build ./... && go vet ./...
go test ./...
CGO_ENABLED=0 go test ./...   # the shipped build must not need cgo
go tool mygo vet .            # where go.mod has the tool directive
go run .                      # the real frame, GPU and all
go tool mygo dev -web         # the same UI in a browser tab; docs/ui/browsers.md
```

A headless test proves text, boxes, focus, and that a click reached a handler.
It does not prove GPU rendering on the target, a native dialog round trip, or
anything about hardware the app talks to. Report those as unverified, and say
which of them still needs a human.
