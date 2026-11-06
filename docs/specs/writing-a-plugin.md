# Writing a plugin

A Remit plugin is a directory with an `extension.json` manifest and either a
JavaScript file or a program. It is loaded by `internal/ext`, gets only the
capabilities its manifest declares, and runs under the app's time and memory
budgets — a built-in plugin has no door a downloaded one lacks.

The example in [`examples/plugins/cards-mini`](../../examples/plugins/cards-mini)
is a whole plugin; [`internal/ext/example_test.go`](../../internal/ext/example_test.go)
loads and runs it through the same loader, so this page cannot drift from the
code.

## 1. Where a plugin lives

```
<user data>/extensions/cards-mini/
├── extension.json
└── main.js
```

`<user data>` is the app's data directory (`mygo.App.Path(mygo.PathUserData)`).
`ext.Discover` reads one directory per plugin; a directory without a readable
manifest is reported and skipped, so one broken plugin never takes the others
down.

A library runs the plugins its own config names, in `remit.json`:

```json
{ "format": "remit/1", "extensions": ["cards-mini"] }
```

`store.Library.EnabledExtensions` / `SetEnabledExtensions` read and write that
list. There is no page in the app that turns a plugin on yet: the list is the
door.

## 2. The manifest

```json
{
  "name": "cards-mini",
  "version": "0.1.0",
  "protocol": 1,
  "runtime": "js",
  "entry": "main.js",
  "capabilities": []
}
```

| Field | Meaning |
|---|---|
| `name` | What the plugin is called; unique among a library's plugins. |
| `version` | Free text, for a human and for the scheduler's record. |
| `protocol` | The wire protocol this app speaks: **1**. Anything else is refused. |
| `runtime` | `"js"` for JavaScript in this process, or omitted/`"exec"` for a program speaking the protocol on its pipes. |
| `entry` | A `js` plugin's file, relative to its own directory. `..` and absolute paths are refused. |
| `entrypoint` | An `exec` plugin's command and arguments. |
| `capabilities` | What the host offers the plugin, by name (§3). |

A manifest that fails any of these is refused by `ext.Validate`, and
`ext.OpenJS`/`ext.OpenBuiltin`/`ext.Load` say why:

```
ext: <dir>: protocol 2, this app speaks 1
ext: <dir>: a js extension needs an entry
ext: <dir>: entry "../../etc/passwd" is outside the extension
```

## 3. Capabilities

A plugin reaches the host through a `remit` object carrying one function per
capability it declared — and nothing else.

| Capability | What it gives |
|---|---|
| `remit.echo` | `remit.echo(value)` answers `value` back. For a plugin's own tests. |
| `remit.now` | `remit.now()` answers the current time as RFC 3339, which is the app's to give. |

Undeclared is not an error at load time: the function is simply not there, and a
hook that calls it throws.

```js
export function bad() { return remit.echo("x"); }   // with no capabilities
```

```
ext: cards-mini: TypeError: remit.echo is not a function
```

Nothing in the host reads the library, the disk or the network on a plugin's
behalf. The plugin answers from what it is given.

## 4. Hooks

A hook is a function exported from the plugin's top level. The host asks for one
by name and hands it JSON; the hook answers JSON. A hook the plugin does not
export is `ext.ErrNoHook`.

### Card plugins

**`derive(block)`** — what a paragraph asks, in order.

```js
{ "id": "block_…", "text": "spacing :: the gaps" }
→ [ { "kind": "basic", "ordinal": 0, "front": "spacing", "back": "the gaps" } ]
```

`kind` is `"basic"`, `"cloze"` or `"bidirectional"`; `ordinal` is the card's
place among the paragraph's cards (each blank of a cloze, both directions of a
paragraph asked both ways). A paragraph that asks nothing answers `[]`, not an
error.

**`faces(card)`** — one card's two sides, for a card that already exists.

```js
{ "text": "spacing :: the gaps", "kind": "basic", "ordinal": 0 }
→ { "front": "spacing", "back": "the gaps" }
```

**`render(card)`** — how a card reads, as markdown the host draws.

```js
{ "front": "spacing", "back": "the gaps" }
→ { "front": "spacing", "back": "the gaps" }
```

The host applies the card's own overrides after `render`: a card may replace
either side, and that wins.

### Scheduler plugins

**`policy()`** — what to schedule with, and what to call the scheduler.

```js
→ { "family": "fsrs", "version": "1", "retention": 0.9 }
```

The schedule's arithmetic stays the host's: a plugin decides retention, and the
name every review it schedules is stamped with (`"fsrs-1"`). It does not compute
intervals.

**`summary(session)`** — what a finished session is worth saying.

```js
{ "reviewed": 12, "dueTomorrow": 3, "accuracy": 0.9 }
→ { "tone": "quiet", "lines": [] }
```

`tone` is `"quiet"` or `"celebrated"`; `lines` are markdown the host draws under
its own wording. The built-in scheduler answers `quiet` with no lines, so the
numbers are said in the user's language; a plugin's own lines are its own words
in its own language.

## 5. Limits, and what a failure looks like

One call runs under the app's budget, which a plugin cannot widen:

| Limit | Value | On reaching it |
|---|---|---|
| Time | `ext.DefaultJSTimeout` (1 s) | The hook is interrupted: `ext.ErrTimeout`. |
| Memory | `ext.DefaultJSMemory` (32 MiB) | The hook stops: `ext.ErrMemoryLimit`. |
| Dynamic code | off | `eval` and `new Function` throw. |

Every call runs in a fresh runtime, so nothing a call leaves behind — a global, a
runaway allocation — reaches the next one.

The app treats a failing plugin as a degraded plugin, never as a broken library:

- A card plugin that throws, loops or answers nonsense: the card reads its
  paragraph's own words, and the review goes on.
- No card plugin at all: a paragraph makes no new card; the cards that exist read
  their paragraphs.
- A scheduler with no policy: the app schedules with its own defaults, and stamps
  nothing.
- Every failure is logged with the plugin's name
  (`remit: the card plugin could not render`), and the view asks once per card
  rather than once a frame.

## 6. Running it locally

The example is a working plugin; load it the way the app does:

```sh
go test ./internal/ext -run TestTheGuidesExamplePluginRuns -v
```

To call your own plugin from a Go test, open it with the same loader and limits:

```go
runner, err := ext.OpenJS("path/to/plugin", ext.JSLimits{})
if err != nil { t.Fatal(err) }
defer runner.Close()

var cards []struct{ Kind string `json:"kind"`; Ordinal int `json:"ordinal"` }
err = runner.Call("derive", map[string]any{"text": "q :: a"}, &cards)
```

`ext.JSLimits{Timeout: 5 * time.Second}` widens the budget for a test; the app
does not.

## 7. Debugging

- A plugin that does not load says why, with its manifest's path.
- A hook that throws answers `ext: <name>: …` with the JavaScript error; a hook
  the plugin forgets to export answers `ext: the extension has no such hook`.
- A hook that never returns is reported as `ErrTimeout` after its budget, and the
  app carries on with the fallback.
- The app's log lines start with `remit: the card plugin …` or
  `remit: the scheduler plugin …`, so a plugin's failures are findable next to
  the action that hit them.

## 8. What a plugin cannot do yet

The app runs its own `cards` and `scheduler` plugins for those two roles. A
library can *enable* an extension by name, and the app starts it for events
(`on_event`), but choosing a third-party **js** plugin for the card or scheduler
role is not wired: there is no page, and no per-role setting. The loader, the
capability rules and the limits a third-party plugin would get are the ones this
page documents — that is what `ext.OpenJS` runs today, and what the example test
exercises.
