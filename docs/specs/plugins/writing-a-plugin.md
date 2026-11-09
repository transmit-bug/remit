# Writing a plugin

A Remit plugin is a directory with an `extension.json` manifest and either a
JavaScript file or a program. `internal/ext` loads it, gives it only the
capabilities its manifest declares, and runs it under the app's time and memory
budgets. The plugins the app ships are loaded by the same code and get no
privileged door.

This page is the contract **as it runs now**: the hook vocabulary in
`internal/ext/manifest.go`, the host API the JavaScript runtime installs in
`internal/ext/js.go`, and the window a plugin may declare in
`internal/ext/contrib`. The contract artifact is
[`remit.d.ts`](../../../remit.d.ts) at the repository root, generated from those
same tables and versioned by `apiVersion`. Two whole plugins live in
[`examples/plugins/`](../../../examples/plugins): `cards-mini` derives cards,
`leeches` contributes a page and commands. The suite and CI run both through
`remit ext test`'s harness (`internal/ext/exttest`), so this page cannot drift
from the code. The design behind the
contract is in [`system.md`](system.md); the window contract is in
[`ui.md`](ui.md).

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
{
  "format": "remit/2",
  "extensions": ["cards-mini"],
  "roles": { "cards.derive": "cards-mini" }
}
```

- **Enabling is per library and explicit.** The settings page lists what
  discovery found, with the capabilities each plugin declares; approving one
  writes its name into `extensions`. A write takes effect at the next library
  open, never mid-review.
- **Roles choose one plugin each.** `cards.derive` and `review.schedule` are
  the two roles; a role the config leaves out is played by the built-in
  (`cards`, `scheduler`). The chosen plugin must be enabled, and only a `js`
  plugin can play a role today.
- `activation` is recorded, not acted on: the enabled list decides when a
  plugin loads.

## 2. The manifest

```json
{
  "id": "example.cards.mini",
  "name": "cards-mini",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "capabilities": [],
  "implements": { "describe": 1, "deriveBlock": 1 },
  "produces": ["remit.basic"]
}
```

The example's manifest, verbatim: [`examples/plugins/cards-mini/extension.json`](../../../examples/plugins/cards-mini/extension.json).

| Field | Meaning |
|---|---|
| `id` | A js plugin's stable identity, required; a derived card's `producer.id` carries it. |
| `name` | Required for both runtimes; the name the library's enabled list and the app's log line use. |
| `version` | Free text, for a human and for the producer stamp's `version`. |
| `runtime` | `"js"` for JavaScript in this process, or omitted/`"exec"` for a program speaking the protocol on its pipes. |
| `entry` | A js plugin's file, relative to its own directory. `..` and absolute paths are refused. |
| `entrypoint` | An exec plugin's command and arguments, as an array. |
| `protocol` | The exec wire protocol: `1`. A js manifest must not carry it — the two versionings are separate. |
| `apiVersion` | A js plugin's host API: `1`. The host loads 1 through its own (`ext.HostAPIVersion`); see [`remit.d.ts`](../../../remit.d.ts). |
| `implements` | The hook ids the plugin carries, each with the contract version it speaks (§4). Validated without loading the code. |
| `activation` | When the plugin would like to be loaded. Recorded, not acted on. |
| `produces` | The card types a derive plugin answers. Declarative: the host only accepts a type the library already holds. |
| `capabilities` | What the host offers the plugin, by name (§3). |
| `contributes` | What the plugin adds to the window: commands, menus, pages, settings, strings (§5). |

A manifest this build cannot run is refused by `ext.Validate`, and every caller
(`ext.Load`, `ext.OpenJS`, `ext.OpenBuiltin`, `remit ext check`) says why:

```
ext: <dir>: protocol 2, this app speaks 1
ext: <dir>: a js extension takes no protocol field (found 1); it is versioned by apiVersion
ext: <dir>: apiVersion 2, this app speaks 1 through 1
ext: <dir>: a js extension needs an entry
ext: <dir>: entry "../../etc/passwd" is outside the extension
ext: <dir>: implements "cards.cook": no such hook
ext: <dir>: capability "net:fetch": no such capability
```

The vocabulary is closed and checked as a whole: every wrong hook in
`implements` and every unknown capability is listed in one refusal, so an author
sees the whole picture at once. `describe`, `deriveBlock`, `policy`, `identity`,
`summary` and `on_event` are the hook ids a manifest may name; anything else is
refused before a line of JavaScript runs.

## 3. Capabilities

A capability is a door, not a permission prompt. A plugin gets only the ones it
declared: the `remit` object carries one namespace per capability, every
function checks the declaration first, and a call without it throws
`ext.CapabilityError` naming the plugin and the capability.

| Capability | `remit.` functions |
|---|---|
| `notes:read` | `notes.list()`, `notes.get(id)`, `notes.search(text)` |
| `cards:write` | `cards.create(blockId)`, `cards.update(cardId, patch)`, `cards.delete(cardId)` |
| `review:read` | `review.history(cardId)`, `review.dueCount()` |
| `review:write` | `review.record(cardId, rating, at?)` |
| `schedule:fsrs` | `fsrs.review(state, rating, at?)` |
| `store:own` | `store.get()`, `store.set(value)` — the plugin's own value |
| `remit.echo` | `echo(value)` — answers the value back, for the plugin's own tests |
| `remit.now` | `now()` — the time as RFC 3339, which is the app's to give |

The shapes, as JSON:

```js
remit.notes.list()        // [{ id, title, blocks: [{ id, text }] }]
remit.notes.get(id)       // one note, or null
remit.notes.search(text)  // notes whose title or a block contains text
remit.cards.create(blockId)                    // derives the block and answers the card
remit.cards.update(cardId, { fields, status }) // fields replace the card's own edits
remit.cards.delete(cardId)
remit.review.history(cardId)                   // [{ seq, card, rating, at, scheduler? }]
remit.review.dueCount()
remit.review.record(cardId, rating, at?)       // rating: again | hard | good | easy
remit.fsrs.review(state, rating, at?)          // the next state; the maths stays the host's
remit.store.get()                              // the string last stored, or null
remit.store.set(value)
```

- Writes go through the same layer the loopback agent API serves
  (`internal/agentapi.Library`), not through HTTP and not through a second
  read/write path.
- `store:own` is the plugin's namespace and no other: `plugins-data/<id>.json`
  in the library, carried in the `.remitlib` archive. Settings a plugin
  contributes (§5) land in the same document, under `settings`.
- The FSRS state a plugin receives is the host's shape. A plugin carries it
  back unchanged; it does not compute intervals.
- There is no `fs`, no `net`, no UI, no arbitrary SQL. A plugin that needs the
  network is a v2 conversation.

## 4. Hooks

A hook is a function exported from the plugin's top level. The host asks for
one by name and hands it JSON; the hook answers JSON. A hook the plugin does not
export is `ext.ErrNoHook`.

| Hook | Contract | Who asks it |
|---|---|---|
| `describe` | 1 | An exec client's `ext.Runner.Describe`; the app does not call it. |
| `deriveBlock` | 1 | The card resolver, once per block text (the answer and its failure are cached). |
| `policy` | 1 | The scheduler role, when a library opens. |
| `identity` | 1 | In the vocabulary; the app composes the stamp from `policy` (`family-version`) instead, so no call is made. |
| `summary` | 1 | The review screen when the queue empties. |
| `on_event` | 1 | The event fan-out, for a library's enabled plugins. |
| `command` | — | A contributed command and a page's data. It rides with `contributes`, not `implements` (§5). |

### `deriveBlock(block, types)` — what a block asks

```js
// block: { "id": "block_…", "text": "spacing :: the gaps" }
// types: [ { "id": "remit.basic", "fields": ["Front", "Back"], "version": 1 } ]
→ [ { "typeId": "remit.basic", "ordinal": 0, "fields": { "Front": "spacing", "Back": "the gaps" } } ]
```

A card is a *type* and the field values that fill it. The host checks the type
exists in the library, logs any field the plugin reports in `unknown` (an
optional `unknown: ["Nope"]` in the answer), stamps `producer { id, version }`
and writes. A card file stores only `type`, `ordinal`, `producer` — and a field
the user **edited**; the rest is resolved from the block when the card renders
(ADR 0008). A block that asks nothing answers `[]`, not an error.

`ordinal` is the card's place among the block's cards — each blank of a cloze,
both directions of a block asked both ways. It is stored, and a card whose
ordinal the block no longer asks takes the first answer of its type.

There is no `faces` and no `render` hook. The host renders a card's sides from
its type's templates (`{{Field}}`, `{{#Field}}`, `{{cloze:Text}}`,
`{{hint:}}`, `{{type:}}` — see [`system.md`](system.md) §3); a plugin never
draws.

### `policy()` — what to schedule with, and what to call the scheduler

```js
→ { "family": "fsrs", "version": "1", "retention": 0.9 }
```

The schedule's arithmetic stays the host's: a plugin decides retention and the
name every review it schedules is stamped with (`"fsrs-1"`). It does not compute
intervals, and it never sees the FSRS state.

### `summary(session)` — what a finished session is worth saying

```js
// { "reviewed": 12, "dueTomorrow": 3, "accuracy": 0.9 }
→ { "tone": "quiet", "lines": [] }
```

`tone` is `"quiet"` or `"celebrated"`; `lines` are markdown the host draws under
its own wording. The built-in scheduler answers `quiet` with no lines, so the
numbers are said in the user's language; a plugin's own lines are its own words.

### `on_event(event)` — a notification, not a veto

```js
// { "kind": "review.recorded", "id": "card_…" }
→ anything; the host ignores the answer
```

Kinds are `note.saved`, `card.created` and `review.recorded`. The app fans an
event out to every enabled plugin that carries the hook. A throwing plugin is
logged and the action goes on.

### `command(input)` — a contributed command, and a page's data

```js
// { "name": "leechesData", "context": { "mode": "review", "library.open": true } }
→ { "kind": "page", "children": [ … ] }      // a page's data
→ { "result": "navigate", "page": "leeches" } // a command's answer
```

One hook answers both: `name` is what the contribution declared after
`command:`. A command's answer is `navigate` (open a page by id), `refresh` (ask
the page on screen again), `toast` (a `%key%` or a literal) or `none`. A page's
data answers the node tree of [`ui.md`](ui.md) §5. The host draws every node
with its own components and tokens; a plugin names structure, never a widget,
a colour or a size.

## 5. What a plugin adds to the window

`contributes` is static: the host reads and validates it from the manifest
without starting the plugin. One bad entry costs only itself — it is dropped
with a reason, and the plugin's other entries stand.

```json
"contributes": {
  "commands": [
    {
      "id": "example.leeches.show",
      "title": "%command.show%",
      "category": "%category.review%",
      "icon": "chart-column",
      "keybinding": "mod+shift+l",
      "when": "library.open && (mode == review || mode == browse)",
      "action": "command:showLeeches"
    }
  ],
  "menus": {
    "review.card": [
      { "command": "example.leeches.pause", "group": "plugin", "order": 100, "when": "card.present" }
    ]
  },
  "pages": [
    {
      "id": "leeches",
      "title": "%page.title%",
      "icon": "chart-column",
      "nav": "plugins",
      "when": "library.open",
      "data": "command:leechesData"
    }
  ],
  "settings": [
    { "key": "threshold", "type": "number", "title": "%setting.threshold%", "default": 8 }
  ],
  "locales": {
    "en": { "command.show": "Leech report" },
    "zh-Hans": { "command.show": "水蛭报告" }
  }
}
```

The rules, in one place:

- A command `id` is globally unique and the `remit.` prefix is the host's. Two
  plugins claiming one id, or one keybinding, is reported and the later plugin
  loses that entry.
- `title` and `category` are `%key%` of the plugin's own catalog, never a
  literal. A missing key shows the key, so a gap is visible rather than blank.
- `icon` is a name in the host's icon set; an unknown name drops the entry.
- `action` is a host action from `contrib.HostActions` (`remit.command.openSettings`
  and friends) or a `command:<name>` callback. A plugin cannot add a host
  action.
- `menus` puts a command in one of the host's slots — `toolbar.overflow`,
  `note.overflow`, `note.block`, `editor.slash`, `review.card`, `browse.row`,
  `sidebar.note`, `page.header`, `today.widget`, `stats.block` — and the `when`
  it is shown under reads `area` (`today` / `write` / `read` / `recall` / `stats` /
  `settings` / `plugin`) and `sub` (a sub-page's stable id) as well as `mode`.
  `status.item` is reserved: a manifest declaring it validates, and nothing draws
  it. A known slot the host has no receiver for yet is accepted and not shown.
- `pages` adds a page in the host's plugin group (`"nav": "plugins"`) or one a
  command opens (`"hidden"`).
- `settings` are edited by the host's form and written to `store:own` under
  `settings`; the plugin is never called to store them.
- `locales` may also live in `locales/<lang>.json` beside the manifest. A
  message is looked up in the exact locale, its base language, then `en`; a key
  no catalog holds shows the key itself, so a gap is visible rather than blank.

The whole vocabulary — context keys, slots, page nodes, limits — is
[`ui.md`](ui.md) §2–§6, and the types are in
[`remit.d.ts`](../../../remit.d.ts).

## 6. Limits, and what a failure looks like

One call runs under the app's budget, which a plugin cannot widen:

| Limit | Value | On reaching it |
|---|---|---|
| Time | `ext.DefaultJSTimeout` (1 s) | The hook is interrupted: `ext.ErrTimeout`. |
| Memory | `ext.DefaultJSMemory` (64 MiB) per runtime | The hook stops: `ext.ErrMemoryLimit`. |
| Dynamic code | off | `eval` and `new Function` throw. |
| Runtimes | a pool of two per plugin | A call beyond the pool runs on a runtime of its own; the pool drops it after. |

A js plugin keeps a **pool of two runtimes**, and a role's plugin is warmed when
the library opens — so a syntax error surfaces while enabling, not mid-review.
Module-level state a call leaves behind is still there for the next call on the
same runtime, bounded by the pool's size and cleared only by a fresh `Open`
(ADR 0006). Calls are synchronous and bounded: the caller waits at most the
budget. A derived answer is cached by block text, failure included, so a
paragraph is asked once however many of its cards are on screen.

The app treats a failing plugin as a degraded plugin, never as a broken library:

- A card plugin that throws, loops or answers nonsense: the card reads its
  block's own words, and the review goes on.
- No card plugin at all: a block makes no new card; the cards that exist read
  their blocks.
- A scheduler with no policy: the app schedules with its own defaults and
  stamps nothing.
- Three consecutive failures disable the plugin: later calls answer
  `ext.ErrDisabled` until it is opened again.
- A contributed command or page that fails three times is hidden on its own;
  the plugin's other contributions stand.
- Every failure is logged with the plugin's name (`remit: the card plugin could
  not derive`, `remit: a plugin page failed`).

## 7. Testing a plugin

A plugin brings its own fixtures. The layout:

```
cards-mini/
├── extension.json
├── main.js
└── fixtures/
    ├── host.json                    the library the hooks may reach (optional)
    └── deriveBlock/
        ├── basic.in.json            the arguments (optional)
        └── basic.out.json           the answer the hook must give
```

`remit ext test <dir>` loads the plugin, validates what it declares with the
same parse the window runs, and calls every hook `implements` names with its
fixture's arguments, comparing the answer with the fixture **as JSON** — both
sides are read and written back with sorted keys and two-space indents, so a
fixture's whitespace is not part of the answer. Every declared hook needs at
least one case; a plugin that contributes a command callback also needs
`fixtures/command/` cases. A hook without a case, a fixture for a hook the
plugin does not carry, and a mismatch are all problems, each naming the file.

`fixtures/host.json` is what the plugin may reach while its hooks run:

```json
{
  "now": "2026-11-08T09:00:00Z",
  "notes": [
    { "id": "note_1", "title": "Memory", "blocks": [{ "id": "block_1", "text": "spacing :: the gaps" }] }
  ],
  "pluginData": "{\"settings\":{\"threshold\":2}}"
}
```

The fixture host answers reads from that file and **refuses writes**: a case
compares an answer, not a library change, so a case that writes cannot be
replayed. Every field is optional; without `host.json` the host holds no notes,
no plugin data, and the fixed clock `2026-11-08T09:00:00Z`. `remit.echo` and
`remit.now` are the host's live functions, so a fixture whose hook reads the
wall clock is not reproducible — take the time from the input instead.

The commands, run from a checkout:

```sh
go run ./cmd/remit ext test examples/plugins/cards-mini
go run ./cmd/remit ext check examples/plugins/leeches
go run ./cmd/remit ext types -o remit.d.ts
```

`remit` is not a tool directive of this module, so `go tool remit ext …` does
not resolve; `go run ./cmd/remit ext …` is the invocation the repository
supports, and an installed binary answers the same commands.

`ext check` reads the manifest and contributions alone, without starting the
plugin — the door for a plugin that has no fixtures yet. `ext types` writes the
contract artifact; [`remit.d.ts`](../../../remit.d.ts) is checked in, and
`go test ./internal/ext/ts` fails when it drifts from the code.

The built-in plugins ship inside the binary and have no plugin directory of
their own. Their frozen answers are `internal/golden/testdata/golden.json`,
compared by `go test ./internal/ext/builtin ./internal/golden`; `ext test` is
the harness for a plugin directory, not a second golden set.

A Go test can drive a plugin through the same loader and limits:

```go
runner, err := ext.OpenJS("path/to/plugin", nil, ext.JSLimits{})
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

## 8. Debugging

- A plugin that does not load says why, with its manifest's path.
- A hook that throws answers `ext: <name>: …` with the JavaScript error; a hook
  the plugin forgets to export answers `ext: the extension has no such hook:
  <name>.<hook>`.
- A call to a capability the manifest did not declare answers
  `ext: <id> did not declare capability <name>`.
- A hook that never returns is reported as `ErrTimeout` after its budget, one
  that allocates without end as `ErrMemoryLimit`; the app carries on with the
  fallback.
- The app's log lines start with `remit: the card plugin …`, `remit: a plugin
  page failed` or `remit: a plugin called a capability it did not declare`, so
  a plugin's failures are findable next to the action that hit them.

## 9. What a plugin cannot do yet

- `activation` is recorded, not acted on: enabling is what loads a plugin.
- Not every menu slot has a host place yet; the one list is
  [`ui.md`](ui.md) §4.
- `identity` is in the vocabulary but the app composes the scheduler's stamp
  from `policy`, so the hook is never called.
- A plugin cannot bring a card type definition of its own: `produces` is a
  declaration, and the library must already hold the type.
- No `fs`, no `net`, no UI toolkit, no plugin-owned templates, DOM, CSS or
  drawing. Types, fields and templates are library data (ADR 0008); the host
  renders them.
- Roles run `js` plugins only; `exec` stays for an extension that needs a native
  library or a long computation, and its hooks are the JSON protocol's.
