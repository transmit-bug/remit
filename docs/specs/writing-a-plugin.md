# Writing a plugin

A Remit plugin is a directory with an `extension.json` manifest and either a
JavaScript file or a program. It is loaded by `internal/ext`, gets only the
capabilities its manifest declares, and runs under the app's time and memory
budgets — a built-in plugin has no door a downloaded one lacks.

This page documents the host API **as it runs now**: the typed `deriveBlock`
contract of ADR 0008 and the hook vocabulary in `internal/ext/manifest.go`. The
example in [`examples/plugins/cards-mini`](../../examples/plugins/cards-mini)
is a whole plugin; [`internal/ext/example_test.go`](../../internal/ext/example_test.go)
loads and runs it through the same loader, so this page cannot drift from the
code. The design behind the contract is in
[`plugin-system.md`](plugin-system.md).

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
{ "format": "remit/2", "extensions": ["cards-mini"] }
```

`store.Library.EnabledExtensions` / `SetEnabledExtensions` read and write that
list. There is no page in the app that turns a plugin on yet: the list is the
door, and a plugin named there is started and sent `on_event` (§4). The card and
scheduling roles are still the built-in plugins' (§8).

## 2. The manifest

```json
{
  "id": "cards-mini",
  "name": "cards-mini",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "capabilities": [],
  "implements": { "deriveBlock": 1 },
  "activation": [],
  "produces": ["remit.basic"]
}
```

| Field | Meaning |
|---|---|
| `id` | A js extension's stable identity, required; a derived card's `producer.id` carries it. |
| `name` | Required for both runtimes; the name the library's enabled list and the app's log line use. Two plugins with one name cannot be told apart by the list. |
| `version` | Free text, for a human and for the producer stamp's `version`. |
| `runtime` | `"js"` for JavaScript in this process, or omitted/`"exec"` for a program speaking the protocol on its pipes. |
| `entry` | A `js` plugin's file, relative to its own directory. `..` and absolute paths are refused. |
| `entrypoint` | An `exec` plugin's command and arguments, as an array. |
| `protocol` | The exec wire protocol: **1**. A js manifest must not carry it — the two versionings are separate. |
| `apiVersion` | A js plugin's host API: **1**. The host loads 1 through its own (`ext.HostAPIVersion`). |
| `implements` | The hook ids the plugin carries, each with the contract version it speaks (§4). Validated without loading the code. |
| `activation` | When the plugin would like to be loaded. Stored, not yet acted on: the library's enabled list is what loads it. |
| `produces` | The card types a derive plugin answers. Declarative: the host only accepts a type the library already holds. |
| `capabilities` | What the host offers the plugin, by name (§3). |

A manifest that fails any of these is refused by `ext.Validate`, and
`ext.OpenJS`/`ext.OpenBuiltin`/`ext.Load` say why:

```
ext: <dir>: protocol 2, this app speaks 1
ext: <dir>: a js extension takes no protocol field (found 1); it is versioned by apiVersion
ext: <dir>: apiVersion 2, this app speaks 1 through 1
ext: <dir>: a js extension needs an entry
ext: <dir>: entry "../../etc/passwd" is outside the extension
ext: <dir>: implements "cards.cook": no such hook
```

The hook vocabulary is closed. `describe`, `deriveBlock`, `policy`, `identity`,
`summary` and `on_event` are the only ids a manifest may name, each at contract
version 1; anything else is refused before a line of JavaScript runs.

## 3. Capabilities

A plugin reaches the host through a `remit` object carrying one function per
capability it declared — and nothing else.

| Capability | What it gives |
|---|---|
| `remit.echo` | `remit.echo(value)` answers `value` back. For a plugin's own tests. |
| `remit.now` | `remit.now()` answers the current time as RFC 3339, which is the app's to give. |

These are the two functions the host installs today. The named grants in
[`plugin-system.md`](plugin-system.md) §6 — `notes:read`, `cards:write`,
`review:read`, `review:write`, `schedule:fsrs`, `store:own` — are the planned
set; declaring one is accepted but installs no function.

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

| Hook | Contract | Who calls it |
|---|---|---|
| `describe` | 1 | `ext.Runner.Describe` for an exec plugin; the app does not call it. |
| `deriveBlock` | 1 | The card resolver, once per paragraph text. |
| `policy` | 1 | The app's scheduler, when it schedules a review. |
| `identity` | 1 | Declared by the built-in scheduler; the app stamps from `policy` today, so no hook is called. |
| `summary` | 1 | The review screen when the queue empties. |
| `on_event` | 1 | The app's event fan-out, for a library's enabled plugins. |

### `deriveBlock(block, types)` — what a paragraph asks, in order

```js
// block: { "text": "spacing :: the gaps" }
// types: [ { "id": "remit.basic", "fields": ["Front", "Back"], "version": 1 } ]
→ [ { "typeId": "remit.basic", "ordinal": 0, "fields": { "Front": "spacing", "Back": "the gaps" } } ]
```

A card is a *type* and the field values that fill it; the host checks the type
exists in the library, logs any field the plugin reports in `unknown` (an
optional `unknown: ["Nope"]` in the answer), stamps `producer {id, version}`,
and writes. A card file stores only `type`, `ordinal`,
`producer` — and a field the user **edited**; the other values are resolved from
the paragraph when the card is rendered (ADR 0008). A paragraph that asks
nothing answers `[]`, not an error.

`ordinal` is the card's place among the paragraph's cards — each blank of a
cloze, both directions of a paragraph asked both ways. It is stored, and a card
whose ordinal the paragraph no longer asks takes the first answer of its type,
so a template can ask what it can instead of going blank.

There is no `faces` and no `render` hook. The host renders a card's two sides
from its type's templates (`{{Field}}`, `{{#Field}}`, `{{cloze:Text}}`,
`{{hint:}}`, `{{type:}}` — see [`plugin-system.md`](plugin-system.md) §3); a
plugin never draws.

### `policy()` — what to schedule with, and what to call the scheduler

```js
→ { "family": "fsrs", "version": "1", "retention": 0.9 }
```

The schedule's arithmetic stays the host's: a plugin decides retention, and the
name every review it schedules is stamped with (`"fsrs-1"`). It does not compute
intervals, and the FSRS state it never sees. `ext.Hooks` also names `identity`
for the stamp, but the app composes it from `policy` today — a plugin without
that hook is fine.

### `summary(session)` — what a finished session is worth saying

```js
// { "reviewed": 12, "dueTomorrow": 3, "accuracy": 0.9 }
→ { "tone": "quiet", "lines": [] }
```

`tone` is `"quiet"` or `"celebrated"`; `lines` are markdown the host draws under
its own wording. The built-in scheduler answers `quiet` with no lines, so the
numbers are said in the user's language; a plugin's own lines are its own words
in its own language.

### `on_event(event)` — a notification, not a veto

```js
// { "kind": "review.recorded", "id": "card_…" }
→ anything; the host ignores the answer
```

Kinds are `note.saved`, `card.created` and `review.recorded`. The app fans an
event out to every enabled plugin that carries the hook. A throwing plugin is
logged and the action goes on.

## 5. Limits, and what a failure looks like

One call runs under the app's budget, which a plugin cannot widen:

| Limit | Value | On reaching it |
|---|---|---|
| Time | `ext.DefaultJSTimeout` (1 s) | The hook is interrupted: `ext.ErrTimeout`. |
| Memory | `ext.DefaultJSMemory` (64 MiB) per runtime | The hook stops: `ext.ErrMemoryLimit`. |
| Dynamic code | off | `eval` and `new Function` throw. |

A js plugin keeps a **pool of two runtimes**. The app fills it when it opens the
card and scheduler plugins — built-in or injected — so the first call pays no
load cost and a syntax error surfaces while enabling rather than mid-review. An
extension enabled through `remit.json` is not warmed today: its first call
loads. A call takes an idle runtime, or makes one when both are busy. The pool
is the deliberate trade from ADR 0006: module-level state a call leaves behind
is still there for the next call on the same runtime, bounded by the pool's size
and cleared only by a fresh `Open`.

Calls are synchronous and bounded: the caller waits at most the budget, and the
view asks once per paragraph text — the resolver caches the answer and the
failure, so a broken plugin is not retried every frame. There is no background
dispatch yet.

The app treats a failing plugin as a degraded plugin, never as a broken library:

- A card plugin that throws, loops or answers nonsense: the card reads its
  paragraph's own words, and the review goes on.
- No card plugin at all: a paragraph makes no new card; the cards that exist read
  their paragraphs.
- A scheduler with no policy: the app schedules with its own defaults, and stamps
  nothing.
- Three consecutive failures disable the plugin: later calls answer
  `ext.ErrDisabled` until it is opened again.
- Every failure is logged with the plugin's name
  (`remit: the card plugin could not derive`, `remit: the scheduler plugin
  answered no policy`).

## 6. Running it locally

The example is a working plugin; load it the way the app does:

```sh
go test ./internal/ext -run TestTheGuidesExamplePluginRuns -v
```

The built-in plugins and the templates they feed are held to the frozen answers:

```sh
go test ./internal/ext/builtin ./internal/cards ./internal/core/template
```

To call your own plugin from a Go test, open it with the same loader and limits:

```go
runner, err := ext.OpenJS("path/to/plugin", ext.JSLimits{})
if err != nil { t.Fatal(err) }
defer runner.Close()

var cards []struct {
	TypeID  string            `json:"typeId"`
	Ordinal int               `json:"ordinal"`
	Fields  map[string]string `json:"fields"`
}
block := map[string]any{"text": "q :: a"}
types := []any{map[string]any{"id": "remit.basic", "fields": []string{"Front", "Back"}, "version": 1}}
err = runner.Call("deriveBlock", []any{block, types}, &cards)
```

`ext.JSLimits{Timeout: 5 * time.Second}` widens the budget for a test; the app
does not.

## 7. Debugging

- A plugin that does not load says why, with its manifest's path.
- A hook that throws answers `ext: <name>: …` with the JavaScript error; a hook
  the plugin forgets to export answers `ext: the extension has no such hook:
  <name>.<hook>`.
- A hook that never returns is reported as `ErrTimeout` after its budget, one
  that allocates without end as `ErrMemoryLimit`; the app carries on with the
  fallback.
- The app's log lines start with `remit: the card plugin …` or
  `remit: the scheduler plugin …`, so a plugin's failures are findable next to
  the action that hit them.

## 8. What a plugin cannot do yet

The app runs its own `cards` and `scheduler` plugins for those two roles. A
library can *enable* an extension by name, and the app starts it and sends it
`on_event`, but choosing a third-party **js** plugin for the card or scheduler
role is not wired: there is no page, and no per-role setting. The loader, the
capability rules and the limits a third-party plugin would get are the ones this
page documents — that is what `ext.OpenJS` runs today, and what the example test
exercises.

Also not built: the six named capability grants of
[`plugin-system.md`](plugin-system.md) §6 (only `remit.echo` and `remit.now`
exist), capability approval at enable time, a settings surface a plugin
contributes to, and `activation` actually gating a load. A derive plugin cannot
bring a card type definition of its own: `produces` is a declaration, and the
library must already hold the type.
