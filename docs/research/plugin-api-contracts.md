# Plugin contract layers: what five mature systems actually define

Reference research for the next revision of remit's host API.
Read-only investigation; no repo files were edited.

**Sources.** Only first-party artifacts: the projects' own published docs, and
their own source/schema files as read at access time (2026-11, `master`/`main`).
No blog posts or third-party write-ups are used as evidence.

**Legend**

- `[doc]` — verified in the project's official documentation.
- `[src]` — verified in the project's own source / schema / typings file.
- `(inferred)` — my conclusion drawn from the above; not stated by the project.

---

## 0. Baseline: remit already ships an extension contract (`internal/ext`)

The five systems below are being read against something that exists, so start
with what is verified in `internal/ext/ext.go`:

- **Isolation by construction**: the host `exec.Command`s `manifest.entrypoint`
  with cwd = the extension directory. `[src]`
- **Wire protocol**: newline-delimited JSON, one request/response at a time,
  serialized behind a mutex, response `id` checked against the request `id`.
  `[src]`
- **Manifest** `extension.json` = `{name, version, protocol, entrypoint}`;
  `Protocol = 1` and any mismatch is a hard load failure
  (`"protocol %d, this app speaks %d"`). `[src]`
- **Declared trust posture**: the package doc states the extension "is trusted as
  a local process — the app does not sandbox it, and the protocol cannot change
  the app's state except through what the app asks it to do (decision #9)".
  `[src]`
- **Failure handling**: per-call `Timeout = 1s`; on timeout or I/O error the
  process is `kill()`ed and the runner marked dead, so later calls fail fast;
  `Discover` accumulates per-directory problems so "a broken extension must not
  take the others down". `[src]`
- **API surface today**: exactly one method, `describe`. `[src]`

The gaps, stated as the question each section below answers:

| Gap | Question answered by |
| --- | --- |
| Nothing is declared about *what* an extension does, only how to start it | §1.1 `contributes` vs registration |
| No capability vocabulary, no read/write separation | §1.2, §3.2, §4.1, §5.2 |
| `protocol` is an exact-match integer: no floor, no deprecation window | §1.4, §3.3 |
| No link between data an extension produced and the extension+version that produced it | §2.3, §4.1, §5.3 |
| Failures surface as an error string, not as a per-extension health/log surface | §1.5, §2.1, §4.2 |

---

## 1. VS Code (primary reference)

### 1.1 Declared (`contributes`) vs registered at runtime

The manifest `package.json` is the declaration surface. It carries `contributes`
(38 documented contribution points, e.g. `contributes.commands`,
`contributes.menus`, `contributes.languages`, `contributes.keybindings`,
`contributes.views`), `activationEvents`, `engines`, `main`/`browser`,
`extensionDependencies`, `extensionKind`, and `capabilities`.
[doc: manifest](https://code.visualstudio.com/api/references/extension-manifest),
[doc: contribution points](https://code.visualstudio.com/api/references/contribution-points)

What the declarations mean, concretely:

- **Declared data is usable with zero plugin code.** `contributes.languages`,
  `contributes.grammars`, `contributes.snippets`, `contributes.themes` are
  consumed by the host directly; the docs' own example merges a language
  definition, a grammar and snippets with no `main` at all.
  [doc: manifest](https://code.visualstudio.com/api/references/extension-manifest)
- **Declared behaviour is only an *address*; the implementation is registered at
  activation.** `contributes.commands` puts an id + title + category in the
  palette, but the id is inert until
  `vscode.commands.registerCommand(id, handler)` runs inside `activate()`.
  [doc: command guide](https://code.visualstudio.com/api/extension-guides/command#registering-a-command)
- **Activation is the gate between the two halves.** `activationEvents`
  (`onLanguage:python`, `onCommand:…`, `onView:…`, `onStartupFinished`, `*`)
  decide *when* the runtime half is allowed to exist.
  [doc: activation events](https://code.visualstudio.com/api/references/activation-events)
- **The declared list has been shrinking as the host got smarter.** Since 1.74,
  languages / commands / views / custom editors / authentication providers
  contributed in `package.json` no longer need a matching `onLanguage` /
  `onCommand` / `onView` / `onCustomEditor` / `onAuthenticationRequest` entry;
  since 1.76 the same for tasks. The host derives activation events from
  `contributes`. The old explicit list is kept only for compatibility.
  [doc: activation events](https://code.visualstudio.com/api/references/activation-events)

Design lesson that is verifiable here: the split is not "static vs dynamic" for
its own sake — it exists so that (a) the host can render, index and search the
extension's surface without executing it, and (b) **activation can be lazy**.
The docs say the lazy loading is the point: "the Markdown extension should only
be loaded when a user opens a Markdown file."
[doc: extension host](https://code.visualstudio.com/api/advanced-topics/extension-host)

### 1.2 Permission model: why there are no runtime prompts

There is no per-extension capability prompt anywhere in the docs, and the reason
is stated flatly:

> "The extension host has the same permissions as VS Code itself. This means that
> any action that VS Code can perform, an extension can also perform through the
> extension host. For example, an extension can read and write files on your
> machine, make network requests, run external processes, and modify workspace
> settings."
> [doc: extension runtime security](https://code.visualstudio.com/docs/configure/extensions/extension-runtime-security)

What replaces the prompt is a stack of four different mechanisms, each answering
a different question:

| Question | Mechanism | Source |
| --- | --- | --- |
| Do I trust this *publisher*? | First install from a third-party publisher shows a trust dialog (1.97+); trusting a pack trusts its dependencies; managed by *Extensions: Manage Trusted Extensions Publishers* | [doc](https://code.visualstudio.com/docs/configure/extensions/extension-runtime-security) |
| Is this *package* what it claims? | Marketplace signature verification, malware scan, clean-room dynamic detection, block list with forced uninstall | [doc](https://code.visualstudio.com/docs/configure/extensions/extension-runtime-security) |
| Is this *workspace* safe to run code from? | Workspace Trust / Restricted Mode, with a declarative per-extension capability statement | [doc: workspace trust](https://code.visualstudio.com/api/extension-guides/workspace-trust) |
| Which *process* does it run in? | `extensionKind` (`ui` / `workspace`), local / web / remote extension hosts | [doc: extension host](https://code.visualstudio.com/api/advanced-topics/extension-host) |

The only thing an extension *declares* about its needs is a boolean-ish
trust-dependency, not a capability list:

```ts
capabilities:
  untrustedWorkspaces:
    { supported: true }
  | { supported: false, description: string }
  | { supported: 'limited', description: string, restrictedConfigurations?: string[] }
```

`restrictedConfigurations` is the sharpest part: listed setting ids return only
the user-level value in Restricted Mode, so a malicious workspace cannot inject
values into a trust-sensitive setting. The runtime half is
`workspace.isTrusted` + `onDidGrantWorkspaceTrust`, plus the
`isWorkspaceTrusted` context key for `when` clauses.
[doc: workspace trust](https://code.visualstudio.com/api/extension-guides/workspace-trust)

Note what is *absent* from the declaration: filesystem, network, process spawn,
and which other extensions it talks to. The host does not model those at all.

### 1.3 `ExtensionContext` — the host-mediated identity and state boundary

The extension never gets a handle to the editor's storage. It gets a context
object whose fields are all host-owned namespaces: `subscriptions`,
`workspaceState`, `globalState`, `secrets`,
`environmentVariableCollection`, `extensionUri`, `storageUri`,
`globalStorageUri`, `logUri`, `extensionMode`.
[src: vscode.d.ts](https://github.com/microsoft/vscode/blob/main/src/vscode-dts/vscode.d.ts)

Two properties matter for the contract:

- **Lifetime is expressed as a list.** `subscriptions` is "an array to which
  disposables can be added. When this extension is deactivated the disposables
  will be disposed." Registration and teardown are the same act.
  [src: vscode.d.ts](https://github.com/microsoft/vscode/blob/main/src/vscode-dts/vscode.d.ts)
- **State is per-extension-identity, not a shared table.** `globalState` /
  `workspaceState` / `secrets` / `storageUri` are all scoped to the extension by
  the host; there is no API to write "the database".

### 1.4 API versioning: `engines.vscode`, `@types/vscode`, `vscode.d.ts`

- `engines.vscode` is required and **cannot be `*`**. A bare `1.8.0` means exactly
  that version; `^1.8.0` means "1.8.0 and onwards". Its stated purpose is
  install-time filtering: "ensure the extension only gets installed for clients
  that contain the API you depend on."
  [doc: publishing / compatibility](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#visual-studio-code-compatibility)
- The contract artifact is a single hand-written `.d.ts`: the API reference page
  says it is "compiled from the [vscode.d.ts] file from the VS Code repository."
  [doc: API reference](https://code.visualstudio.com/api/references/vscode-api)
  `@types/vscode` is that artifact republished for the compiler. The docs pin the
  mechanism only for proposed APIs (use `@types/vscode@<version>` together with
  `npx @vscode/dts dev <version>` to get a matching proposal file);
  `(inferred)` keeping `@types/vscode` at or below the `engines.vscode` floor is
  the author's discipline, not something the host checks.
  [doc: proposed API](https://code.visualstudio.com/api/advanced-topics/using-proposed-api)
- Breaking changes are handled by **never breaking the stable surface, and paying
  the cost explicitly**: "we take Extension API compatibility seriously … once we
  introduce an API, we cannot easily change it anymore." The escape valve is the
  proposed API channel: `vscode.proposed.<name>.d.ts`, Insiders-only, "subject to
  change", not allowed in published extensions, promoted into the stable surface
  when it settles.
  [doc: proposed API](https://code.visualstudio.com/api/advanced-topics/using-proposed-api)
- Version drift is resolved at *install* time by the engine range, not at runtime
  by feature detection. `(inferred)` This is why VS Code can add API freely but
  essentially cannot remove or reshape it — the whole cost lands on the host team.

### 1.5 Isolation, crash and hang

- The extension host is a **separate process** (Node.js local, Node.js remote, or
  a browser WebWorker for web extensions). Which host runs a given extension is a
  deterministic function of the available hosts, the extension's `main`/`browser`
  entry and its `extensionKind` preference order.
  [doc: extension host](https://code.visualstudio.com/api/advanced-topics/extension-host)
- The stated guarantee is a *negative* one: "VS Code aims to deliver a stable and
  high performance editor to users, and misbehaving extensions should not impact
  the user experience. The Extension Host in VS Code prevents extensions from:
  impacting startup performance; slowing down UI operations; modifying the UI."
  [doc: extension host](https://code.visualstudio.com/api/advanced-topics/extension-host)
  `(inferred)` Note the exact shape of the claim: the extension cannot touch the
  UI thread or the startup path, but a hang inside the extension host still
  stalls that bare host's own providers — the guarantee is about the *editor*,
  not about each extension's responsiveness.
- The unit of recovery is the host, and it is user-visible as a verb: uninstall,
  disable and update all prompt the user to **"Restart Extensions"** (= restart
  the extension host).
  [doc: extension marketplace](https://code.visualstudio.com/docs/configure/extensions/extension-marketplace)
- Attribution is a product feature, not a log line: **Extension Bisect** is
  offered in the Extensions view menu "to isolate problematic extension
  behaviour".
  [doc: extension marketplace](https://code.visualstudio.com/docs/configure/extensions/extension-marketplace)
  `(inferred)` Bisect is what you build when you cannot attribute a failure to a
  plugin automatically: binary-search the *set* by disabling halves.

---

## 2. Anki (add-ons + scheduler)

### 2.1 Hooks: which points exist, how they register, how failure surfaces

Two hook kinds, defined by who keeps the final value:

- **Regular hooks**: called for side effects, may mutate the objects passed in.
- **Filters**: take a value as the first argument and must return it (possibly
  changed) — necessary because Python strings are immutable.
  [doc: hooks and filters](https://docs.ankiweb.net/addons/hooks-and-filters)

Registration is a plain list mutation on an imported module, since Anki 2.1.20:

```python
from aqt import gui_hooks
gui_hooks.reviewer_did_show_question.append(myfunc)
gui_hooks.reviewer_did_show_question.remove(myfunc)
```

"Multiple add-ons can register for the same hook or filter — they will all be
called in turn", and mutating a hook from inside its own callback is explicitly
called out as breaking.
[doc: hooks and filters](https://docs.ankiweb.net/addons/hooks-and-filters)

Legacy API is `addHook(name, fn)` / `runHook(name, …)` / `runFilter(name, first,
…)`; new-style hooks mostly still call the legacy ones, "so old add-ons will
continue to work for now".
[doc: hooks and filters](https://docs.ankiweb.net/addons/hooks-and-filters)

The hook *list itself is a curated, generated artifact*, not an open extension
point:

- Definitions live in `pylib/tools/genhooks.py` and `qt/tools/genhooks_gui.py`;
  the build writes the hook modules from those definitions.
  [doc: hooks and filters](https://docs.ankiweb.net/addons/hooks-and-filters),
  [src: qt/tools/genhooks_gui.py](https://github.com/ankitects/anki/blob/main/qt/tools/genhooks_gui.py)
- The reference page is generated from those files, so the docs cannot drift from
  the contract. [doc: hooks reference](https://docs.ankiweb.net/addons/hooks-reference)
- Adding a hook requires a PR with a use case; "hooks that are general in nature
  will typically be approved; hooks that target a very specific use case may need
  to be refactored."
  [doc: hooks and filters](https://docs.ankiweb.net/addons/hooks-and-filters)

Representative points (from the generated reference): backend
`note_will_be_added`, `notes_will_be_deleted`, `schema_will_change(proceed) ->
bool`; GUI `reviewer_did_show_question`; the rendering filter
`card_will_show(html, card, context)`; the webview pair
`webview_will_set_content` / `webview_did_receive_js_message`.
[doc: hooks reference](https://docs.ankiweb.net/addons/hooks-reference),
[doc: reviewer javascript](https://docs.ankiweb.net/addons/reviewer-javascript)

`card_will_show` is worth studying as a *typed extension point*: the third
argument is a closed enum of contexts — `reviewQuestion`, `reviewAnswer`,
`clayoutQuestion`, `clayoutAnswer`, `previewQuestion`, `previewAnswer` — so the
renderer can say exactly where a plugin is allowed to inject, and the docs even
state the cross-context rule ("Javascript you append on the back side … should
not depend on Javascript that is only added on the front").
[doc: reviewer javascript](https://docs.ankiweb.net/addons/reviewer-javascript)

**Failure surfacing.** There is no per-hook isolation: hooks are ordinary Python
lists, so a raising callback propagates. What Anki does build is *attribution*:
the global error dialog scans the traceback for `addons21/<dir>/` paths, maps
them to add-on names, and prepends "the following add-ons were possibly
involved", plus `addon_debug_info()` which lists every enabled add-on with
folder, install time, version and whether its config was modified.
[src: qt/aqt/errors.py](https://github.com/ankitects/anki/blob/main/qt/aqt/errors.py)

`(inferred)` This is a deliberate trade: cheap and transparent for add-on
authors (no sandbox, no IPC), expensive for the user (one bad hook takes down the
operation, and the culprit is a *regex over a traceback*).

### 2.2 Note type model: fields + templates, and where generation happens

Storage model (`NotetypeSchema11`): a notetype has `flds` (ordered fields) and
`tmpls` (`CardTemplateSchema11`: `name`, `ord`, `qfmt`, `afmt`, `bqfmt`,
`bafmt`, optional deck override), plus `sortf` (which field sorts the browser)
and `type` (regular vs cloze). A card is identified by notetype + template
ordinal, not by content.
[src: rslib/src/notetype/schema11.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/schema11.rs),
[src: rslib/src/storage/schema11.sql](https://github.com/ankitects/anki/blob/main/rslib/src/storage/schema11.sql)

Template syntax is a small closed language, not a scripting language:
`{{Field}}`, `{{#Field}}…{{/Field}}` / `{{^Field}}…{{/Field}}` conditionals,
special fields (`{{FrontSide}}`, `{{Tags}}`, `{{Deck}}`, `{{Card}}`, `{{Type}}`),
and field filters (`cloze:`, `type:`, `hint:`, TTS, `furigana:`, `kana:`,
`kanji:`).
[doc: templates/fields](https://github.com/ankitects/anki/blob/main/docs-site/manual/templates/fields.mdx),
[doc: templates/generation](https://github.com/ankitects/anki/blob/main/docs-site/manual/templates/generation.mdx)

The card-generation rule is the contract's sharpest semantics, and it is a rule
about *rendering*, not about the field list:

- A card exists iff its **front side renders non-empty**; "Anki does not consider
  special fields or non-field text for the purposes of card generation".
- Therefore conditional replacement on the front is the mechanism for
  "create this card only when X" — and wrapping only part of the front does not
  work as expected.
- Cloze notetypes generate one card per distinct cloze number.
- Editing a note creates newly-valid cards but never deletes now-empty ones
  (Tools → Empty Cards is the explicit deletion path).
- `{{FrontSide}}` is only meaningful in the back template.
  [doc: templates/generation](https://github.com/ankitects/anki/blob/main/docs-site/manual/templates/generation.mdx)

Generation runs in Rust: `rslib/src/notetype/cardgen.rs` consumes
`ParsedTemplate`s and decides, per template ordinal, whether a card should exist
and in which deck. `(inferred)` So the *authoring* surface (templates) is
declarative data in the DB, but the *interpreter* is host code that no add-on can
replace — only hook into.
[src: rslib/src/notetype/cardgen.rs](https://github.com/ankitects/anki/blob/main/rslib/src/notetype/cardgen.rs)

### 2.3 Scheduler: how FSRS replaced v1/v2/v3, and what is recorded

The scheduler choice was historically **two axes**, not one:

- `schedVer` in collection config, deserialized into
  `SchedulerVersion::{V1, V2}` — the collection-level "which scheduler family".
  [src: rslib/src/config/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/config/mod.rs)
- A separate boolean `SCHED_2021` ("v3") that only exists on top of V2. The
  Python layer says it outright: "For backwards compatibility, the v3 scheduler
  currently returns 2. Use the separate `v3_scheduler()` method", with
  `_supported_scheduler_versions = (1, 2)` and `upgrade_to_v2_scheduler()`
  calling a backend `upgrade_scheduler()`.
  [src: pylib/anki/collection.py](https://github.com/ankitects/anki/blob/main/pylib/anki/collection.py)

So "v3" was never a third `schedVer` value — it was a flag on V2. That matters
for anyone designing a "scheduler version" field: the historical design allowed a
migration path (v1 → v2 requires an explicit upgrade op; v2 ↔ v3 is a flag flip)
without a third enum member.

Then the algorithm axis was added *orthogonally*:

- v3 became the only scheduler: "As of Anki/AnkiMobile 23.10, and AnkiDroid 2.17,
  the v3 scheduler is the default and only option." v2 and v3 are declared
  compatible — switchable without a full sync, no scheduling breakage — but "a v2
  and v3 client may show a different number of due cards on a given day, and may
  show them in a different order."
  [doc: the 2021 scheduler](https://docs.ankiweb.net/faqs/the-2021-scheduler)
- FSRS is **not** a fourth scheduler version. It is a collection-level boolean
  (`BoolKey::Fsrs`) that switches the interval computation inside the same v3
  pipeline, with per-preset parameters:
  "FSRS can only be enabled globally; you cannot enable it for some presets and
  disable it for others."
  [src: rslib/src/scheduler/answering/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/scheduler/answering/mod.rs),
  [doc: deck options §FSRS](https://docs.ankiweb.net/manual/deck-options#fsrs)

**Is history comparable across schedulers?** The system never records *which*
engine produced a given review, so comparability is asserted by compatibility
window and reconstructed by replay:

- `revlog` has no scheduler-version column: `id, cid, usn, ease, ivl, lastIvl,
  factor, time, type`.
  [src: rslib/src/storage/schema11.sql](https://github.com/ankitects/anki/blob/main/rslib/src/storage/schema11.sql)
- Engine differences are instead encoded *in band*, with documented sentinels:
  `ease` "in the V1 scheduler, 3 represents easy in the learning case" while
  "0 represents manual rescheduling"; `factor` (ease factor) stores FSRS
  difficulty normalized to 100–1100 "so a 0 difficulty can be distinguished from
  SM-2 learning"; `type` is a closed enum including `Manual`, `Rescheduled`,
  `Filtered`; and two derived predicates (`is_reset`, `is_cramming`) rely on the
  `ease_factor == 0` sentinel.
  [src: rslib/src/revlog/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/revlog/mod.rs)
- The FSRS *parameter version* is recorded by **field-name versioning** in the
  deck config schema: `fsrs_params_4 -> "fsrsWeights"` (the legacy FSRS-4 slot),
  plus `"fsrsParams5"` and `"fsrsParams6"` as distinct slots, migrated forward
  rather than overwritten.
  [src: rslib/src/deckconfig/schema11.rs](https://github.com/ankitects/anki/blob/main/rslib/src/deckconfig/schema11.rs)
- Derived state is persisted per card (`memory_state`, `decay`) **and** is
  recomputable: when a card has no memory state (moved/imported into an FSRS
  deck), the scheduler replays its revlog through
  `fsrs_item_for_memory_state(…)` with `config.historical_retention` to
  synthesize the initial state.
  [src: rslib/src/scheduler/answering/mod.rs](https://github.com/ankitects/anki/blob/main/rslib/src/scheduler/answering/mod.rs)
- The docs are honest about the cost of mixing algorithms rather than hiding it:
  an add-on that "affects intervals and scheduling in some way… shouldn't be used
  with FSRS"; if any client in the sync set lacks FSRS, "things will not work
  correctly"; and FSRS has a knob for "some of your review history is missing"
  that assumes 90% historical retention.
  [doc: deck options §FSRS](https://docs.ankiweb.net/manual/deck-options#fsrs)

`(inferred)` Anki's answer to "is history comparable" is: *within* a declared
compatibility window yes (v2↔v3), *across* algorithm changes only
approximately — the history is reused as a fitting/replay input, never as a
like-for-like measurement. Since Anki also exposes a *custom scheduling* JS field
that can rewrite the next states (a global option, "applies to every preset",
operating on the `states` object), the combination "plugin changes the answer +
algorithm changed + no per-row engine stamp" is exactly why the docs tell you not
to mix them.
[doc: deck options §Custom Scheduling](https://docs.ankiweb.net/manual/deck-options#custom-scheduling)

---

## 3. Obsidian

### 3.1 API surface and its boundaries

The whole contract is one `.d.ts`. Top-level entry is `App`:
`workspace`, `vault`, `metadataCache`, `fileManager`, `keymap`, `scope`,
`lastEvent`, `renderContext`, `secretStorage`, `isDarkMode()`,
`loadLocalStorage()/saveLocalStorage()`.
[src: obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)

- **Vault** is 28 methods over a single object: path lookups
  (`getFileByPath`, `getAbstractFileByPath`), reads (`read`, `cachedRead`,
  `readBinary`), writes (`create`, `createBinary`, `append`, `appendBinary`,
  `modify`, `modifyBinary`, `delete`, `trash`, `rename`), a raw `adapter`
  (`DataAdapter`), and an atomic read-modify-write `process(file, fn)`.
  Reads and writes are *the same handle* — there is no scope object.
  [src: obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)
- **metadataCache** is the only supported path to parsed note structure
  (`CachedMetadata`: headings, links, embeds, blocks, frontmatter); **workspace**
  owns leaves/views and the layout; **fileManager** owns the semantic operations
  on links (rename-rewrite).
- **Capability-as-API rather than capability-as-grant**: `requestUrl()` exists so
  plugins can make CORS-free HTTP requests *through the host*; `registerMarkdownPostProcessor`
  is the rendering interception point; `registerEditorExtension` is CodeMirror.
  [src: obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)
- **Every member carries provenance**: `@public` plus `@since 0.9.7` (etc.).
  `(inferred)` That annotation is the actual versioning mechanism: it is what lets
  `minAppVersion` be a *derived* number instead of a promise.
  [src: obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)
- **Lifecycle is disposal-based**: `Component.register(cb)`,
  `registerEvent(EventRef)`, `registerInterval(id)` — everything registered is
  torn down by `unload()`; `Plugin` extends `Component` and adds `onload`,
  `onUserEnable`, `onExternalSettingsChange`, `addCommand`, `registerView`,
  `registerExtensions`, `registerMarkdownPostProcessor`,
  `registerMarkdownCodeBlockProcessor`, `registerBasesView`,
  `registerEditorExtension`, `registerEditorSuggest`, `registerObsidianProtocolHandler`,
  `registerCliHandler`, `loadData`/`saveData`.
  [src: obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)

### 3.2 `manifest.json` fields — and why there is no sandbox

Required for all: `author`, `minAppVersion`, `name`, `version` (semver x.y.z).
Optional: `authorUrl`, `fundingUrl`. Plugin-only: `description`, `id` (lowercase
+ hyphens; may not end in `plugin`, may not contain `obsidian`), `isDesktopOnly`.
[doc: manifest](https://docs.obsidian.md/Reference/Manifest)

There is **no** permissions field, no dependency field, no contribution-point
section. Everything a plugin *does* is registered at runtime in `onload`, and
everything it *needs* is implicit.

The stated reason, in the vendor's own words:

> "Due to technical limitations, Obsidian cannot reliably restrict plugins to
> specific permissions or access levels. This means that plugins will inherit
> Obsidian's access levels. As a result… Community plugins can access files on
> your computer. Community plugins can connect to internet. Community plugins can
> install additional programs."
> [doc: plugin security](https://obsidian.md/help/plugin-security)

Stated consequences, all three of them policy rather than mechanism:

1. **One binary switch, on by default the safe way.** Restricted Mode is on by
   default; only disabling it allows third-party code. Installed plugins remain in
   the vault but "are ignored by Obsidian" while Restricted Mode is on.
   [doc: plugin security](https://obsidian.md/help/plugin-security)
2. **Disclosure substitutes for authorization.** Developer policies require README
   disclosure for network use, for "accessing files outside of Obsidian vaults",
   for accounts/payment, and for server-side telemetry; and *forbid* plugins from
   installing or updating themselves or their dependencies.
   [doc: developer policies](https://docs.obsidian.md/Developer+policies)
3. **The directory review is the trust boundary.** Automated scanning for
   vulnerabilities/quality/malware with a published scorecard, manual review for
   popular/featured/flagged plugins, and "consider an independent security audit"
   as the user's own recourse.
   [doc: plugin security](https://obsidian.md/help/plugin-security)

`(inferred)` The consequence for a host API designer: "no sandbox" is not a
missing feature, it is a **choice of where the trust boundary lives** — at the
directory/review layer, before code ever runs. It only works because a human
reads the disclosure and a scanner reads the package; neither is available to a
local-first app that lets users sideload.

### 3.3 Versioning: `versions.json`

Unusually, the *app* version selects the *plugin* version at install time:
`manifest.json.minAppVersion` is compared to the running app; if the app is older,
Obsidian looks up `versions.json` (a map `pluginVersion -> minAppVersion`) at the
repo root and installs the newest plugin release compatible with that app.
[doc: versions](https://docs.obsidian.md/Reference/Versions)

`(inferred)` This is a *release-channel* answer to the version-matrix problem: no
feature detection, no capability flags — just "we will ship you an older plugin".
It requires the plugin author to keep old builds installable, which is fine for a
directory and impossible for anything with on-disk data migrations.

### 3.4 Plugin inter-dependency

- No manifest field exists for dependencies (see the full field list above).
- The public typings expose no plugin registry: `App` has no `plugins` member,
  and there is no `getPlugin(id)` in `obsidian.d.ts`.
  [src: obsidian.d.ts](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts)
- The policies forbid a plugin from installing or updating "themselves or their
  dependencies".
  [doc: developer policies](https://docs.obsidian.md/Developer+policies)

`(inferred)` So inter-plugin dependency is *unmanaged by design*: it happens by
convention (documented install order, shared files in the vault, DOM/event
coupling, or reaching into undocumented runtime globals), and the host carries no
contract for load order, version negotiation or failure. This is the clearest
negative example in this report.

---

## 4. SiYuan and Logseq — how the plugin API relates to the kernel

### 4.1 SiYuan: plugin as an HTTP client of a Go kernel

Two-tier: the **kernel** (Go) owns data and exposes an HTTP API
(`/api/file/*`, `/api/filetree/*`, …); the **frontend plugin** (TS, loaded into
the webview) talks to it, and there is an explicit rule against bypassing it:

> "If plugins or external extensions require direct reading or writing of files
> under the `data` directory, please use the kernel API to achieve this. **Do not
> call `fs` or other electron or nodejs APIs directly**, as it may result in data
> loss during synchronization and cause damage to cloud data."
> [doc: plugin-sample README](https://github.com/siyuan-note/plugin-sample)

`plugin.json` declares *where it can run* and *what the publish surface may see* —
capability as platform/environment declaration plus field-level publish scopes:

| Field | Meaning |
| --- | --- |
| `backends` | `windows`/`linux`/`darwin`/`docker`/`android`/`ios`/`harmony`/`all` |
| `frontends` | `desktop`/`desktop-window`/`mobile`/`browser-desktop`/`browser-mobile`/`all` |
| `kernels` | same vocabulary; if absent, the plugin's `kernel.js` is never started |
| `disabledInPublish` | whether the plugin is disabled in the publish service |
| `publish.resources` | additional frontend files visitors may fetch (exact paths or `dir/`) |
| `publish.data` | **public scalar field names**; "access requires a separate administrator grant" |
| `minAppVersion`, `version`, `settingsWindow` | version floor, semver, separate settings-window instance |

[doc: plugin-sample README](https://github.com/siyuan-note/plugin-sample),
[src: plugin.json](https://github.com/siyuan-note/plugin-sample/blob/main/plugin.json)

**Can plugins change rendering or scheduling?** Rendering yes, via a typed,
data-addressed extension point; scheduling no such contract is documented.

- Rendering: `customBlockRenders` registers a block type; the *persisted* markdown
  names the producer: `;;;plugin-sample/counter\n0\n;;;`. The contract around it is
  the interesting part — the renderer may only modify the given `element` mount;
  `content` is the persisted raw content; `setContent` returns `false` in
  read-only mode; a renderer may return a cleanup function; and "SiYuan displays
  the raw content as a fallback when the plugin is unavailable or the block type
  is not registered. Rendered DOM is transient; persisted data belongs in
  `content`, block attributes, or plugin-owned storage."
  [doc: plugin-sample README](https://github.com/siyuan-note/plugin-sample)
- The only permission model with real teeth is scoped to the **publish service**,
  and it separates three acts that are usually conflated: allowing the plugin in
  the service, *authorizing specific data fields*, and *generating* the snapshot.
  "Authorization does not generate data or copy private settings." Grants are
  exact-set ("Enabling a grant requires the exact current field set, preventing
  approval of a changed declaration from an outdated dialog"), revocation clears
  the snapshot, and the state lives in `conf/plugin-publish/<name>.json`, separate
  from plugin storage. HTTP codes are part of the contract: `403` no grant,
  `404` grant but no snapshot.
  [doc: PLUGIN-PUBLISH.md](https://github.com/siyuan-note/siyuan/blob/master/docs/PLUGIN-PUBLISH.md)
- Lifecycle is where it is most concrete about failure: hooks run strictly
  serially and awaited; disabling/reloading/uninstalling starts **one shared
  five-second removal budget**; after it expires "SiYuan stops waiting… a timed-out
  hook may continue during or after teardown. The five-second budget limits only
  how long SiYuan waits for Promises; it cannot interrupt synchronous
  JavaScript." Plugins are told to make teardown idempotent, use `AbortController`,
  and persist eagerly instead of relying on teardown.
  [doc: plugin-sample README](https://github.com/siyuan-note/plugin-sample)

`(inferred)` SiYuan is the best available reference for "plugins are clients of a
service, not co-tenants of the process": the API boundary is HTTP, so the same
contract serves desktop, mobile, browser and Docker; the cost is that the host
cannot enforce "don't call `fs`" — it is a documented rule, not a guarantee.

### 4.2 Logseq: plugin runs out-of-realm behind a message bus

Logseq's manifest is the `logseq` key inside `package.json`, typed as
`LSPluginPkgConfig`: `id`, `main` (alias `entry`), `title`,
**`mode: 'shadow' | 'iframe'`**, `themes`, `icon`, `devEntry`.
Runtime info (`LSPluginBaseInfo`) adds `settings.disabled` and `effect`.
[src: libs/src/LSPlugin.ts](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.ts),
[src: libs/src/LSPlugin.core.ts](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.core.ts)
(`const logseq: Partial<LSPluginPkgConfig> = pkg.logseq || {}`)

- **Isolation is a declared plugin choice.** The plugin picks `shadow` (a
  `QSandbox` container with `attachShadow({mode:'open'})`, class
  `lsp-shadow-sandbox`) or `iframe` for its UI mount.
  [src: libs/src/LSPlugin.shadow.ts](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.shadow.ts)
- **The API is an RPC allowlist, not an object graph.** `LSPluginCaller` layers a
  `Postmate` parent/child channel with a tagged protocol
  (`#lspmsg#`, `#lspmsg#ready#`, `#lspmsg#settings#`, `#lspmsg#error#`,
  `#lspmsg#beforeunload#`, `#await#response#`), and the host side is a small,
  explicit set of callables — the file `callable.apis.ts` says: "These APIs run in
  the host and are callable by users, similar to `logseq.apis.cljs` but for
  JS/TS."
  [src: libs/src/LSPlugin.caller.ts](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.caller.ts),
  [src: libs/src/callable.apis.ts](https://github.com/logseq/logseq/blob/master/libs/src/callable.apis.ts)
- **Capability expression**: there is no permission list; what exists is the
  *narrowness of the proxied surface* plus the declared UI-isolation mode. The
  proxy namespaces visible in the typings (`IAppProxy`, `IDBProxy`,
  `IEditorProxy`, `IUIProxy`, `ICommandsProxy`, `IAssetsProxy`, `IGitProxy`) are
  the de-facto capability groups.
  [src: libs/src/LSPlugin.user.ts](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.user.ts)
- **Failure visibility is per-plugin and first-class**: `PluginLogger` keeps
  entries per plugin and the host ships a log viewer panel for a single plugin
  ("Viewer panel for a single plugin's logs powered by PluginLogger", with a
  copy-all action).
  [src: src/main/frontend/components/plugin_logs.cljs](https://github.com/logseq/logseq/blob/master/src/main/frontend/components/plugin_logs.cljs)
- **Rendering**: plugins can mount UI in slots/paths and register hooks/filters
  (`IUserHook`, `IUserSlotHook`, `IUserConditionSlotHook`, `RendererKey`).
  Scheduling: no documented scheduling contract.
  [src: libs/src/LSPlugin.ts](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.ts)

`(inferred)` Logseq is the cheapest viable "realm + message bus" design: it gets
crash/lifetime separation and a compile-time-visible capability surface
(`callable.apis.ts`) without an OS sandbox. The cost is that every API call is
async and serializable — which is precisely what makes the allowlist auditable.

---

## 5. Cross-cutting

### 5.1 Declarative capability lists vs runtime-registered permissions

Two different questions are usually conflated here, and the mature systems
separate them: *what can a human review before install* (declared) versus *what
can the host hold accountable while running* (registered).

VS Code declares almost nothing about capability and registers everything.
`contributes` is a UI/data declaration (38 contribution points), `capabilities.untrustedWorkspaces`
is a single trust-dependency fact, and the actual authority — files, network,
processes, settings — is neither declared nor granted: "the extension host has the
same permissions as VS Code itself."
[doc](https://code.visualstudio.com/docs/configure/extensions/extension-runtime-security)
The cost is exactly what you would predict: there is no user-facing capability
inventory to review, and no way to run an extension with reduced authority. What
pays for it is a completely different layer — publisher trust, package signing,
and a review pipeline — none of which is part of the API contract.

SiYuan goes the other way for the one surface where it matters. `plugin.json`
declares `publish.data: ["theme", "showAuthor"]`, and the administrator grants
exactly that set at runtime; a changed declaration requires a *new* grant, and
granting/revoking clears the snapshot so stale data cannot outlive the scope.
[doc](https://github.com/siyuan-note/siyuan/blob/master/docs/PLUGIN-PUBLISH.md)
That is a real capability system, but note its boundary: it governs the
**published read path only**. Ordinary plugin code still talks to the kernel with
the user's full authority, and "don't call `fs`" is a README rule
[doc](https://github.com/siyuan-note/plugin-sample) — the declaration is precise
where enforcement is cheap and advisory where it is not.

Anki declares nothing and grants nothing, and instead makes the *API surface* the
capability: the hook list is a curated, generated whitelist
(`genhooks.py` → hook modules → generated reference docs), and adding a hook is a
reviewed PR.
[doc](https://docs.ankiweb.net/addons/hooks-and-filters) Obsidian is the limiting
case of the declarative style: capability appears only as a *disclosure
obligation* (network use, files outside the vault), which is a human-readable
declaration with no runtime counterpart at all.
[doc](https://docs.obsidian.md/Developer+policies)

`(inferred)` The pattern across all four: **declarations are cheap to review and
expensive to enforce; registrations are cheap to enforce and impossible to
review.** A system that wants both must declare the *scope* (field lists, hook
names, activation triggers) and register the *authority* (the actual call), and
must make the declared scope fail closed — as SiYuan does by requiring the exact
current field set at grant time.

### 5.2 Separating "read" from "write" authorization

Almost nobody does this as an authorization axis, and the ones that appear to are
actually separating something else.

Obsidian has no read/write split at all: `Vault` hands out `read`, `cachedRead`,
`modify`, `append`, `delete`, `rename` on the same object, behind the same
(unenforceable) authority.
[src](https://github.com/obsidianmd/obsidian-api/blob/master/obsidian.d.ts) What
it does provide instead is a *concurrency* contract — `process(file, fn)` as an
atomic read-modify-write — and a *host mode* (read-only) that is a property of the
session, not of the caller. SiYuan's read-only mode is likewise a mode: the
custom-block `setContent` "returns `false` in read-only mode".
[doc](https://github.com/siyuan-note/plugin-sample)

VS Code's nearest equivalent is not read vs write but **trusted vs untrusted
workspace**, and its one genuinely asymmetric mechanism is on *configuration*
reads: `restrictedConfigurations` makes listed settings return only the
user-level value in Restricted Mode, so a workspace cannot inject a value that
changes what code the extension executes.
[doc](https://code.visualstudio.com/api/extension-guides/workspace-trust)

SiYuan is the only system here that separates read from write *per data item*:
`loadData`/`saveData` are private plugin storage (read+write, admin context),
`loadPublishData`/`savePublishData` are the public snapshot — visitors may read
it, only an administrator may write it, and the grant is read-only by
construction. "Saving requires administrator access and replaces the entire
snapshot"; "Reading requires an installed, enabled, publish-enabled plugin and a
valid grant."
[doc](https://github.com/siyuan-note/siyuan/blob/master/docs/PLUGIN-PUBLISH.md)

Anki separates on a different axis worth stealing: **who owns the final value,
not who may read it.** A hook is "run for its side effects"; a filter "takes
[the value] and must return it, after maybe changing it." `schema_will_change`
is a filter over a `proceed: bool`, i.e. an explicit, auditable veto point rather
than an implicit mutation.
[doc](https://docs.ankiweb.net/addons/hooks-and-filters)

`(inferred)` If remit wants real read/write separation, the cheapest honest
version is not a permission dialog but **two different surfaces for two different
audiences** (SiYuan's private vs published split), plus **filters instead of
mutation hooks** wherever a plugin would otherwise write into host-owned state.

### 5.3 Replayability of plugin output

Which systems stamp produced data with an engine/plugin version, and why?

**Anki stamps semantics, not versions — and pays for it.** Nothing in `revlog`
says which scheduler produced the row (`id, cid, usn, ease, ivl, lastIvl, factor,
time, type`). The engine is instead recoverable only from in-band conventions:
`ease = 3` means "easy in the learning case" under V1 and `0` means manual
rescheduling; `factor` holds FSRS difficulty shifted into 100–1100 "so a 0
difficulty can be distinguished from SM-2 learning"; `type` is a closed enum, and
two predicates (`is_reset`, `is_cramming`) *depend on* the `ease_factor == 0`
sentinel.
[src](https://github.com/ankitects/anki/blob/main/rslib/src/revlog/mod.rs) The
algorithm's parameters are versioned by field name (`fsrsWeights`, `fsrsParams5`,
`fsrsParams6`), migrated forward rather than in place
[src](https://github.com/ankitects/anki/blob/main/rslib/src/deckconfig/schema11.rs),
and derived state is persisted per card (`memory_state`, `decay`) *plus*
recomputable: a card that lost its memory state gets it back by replaying its
revlog through `fsrs_item_for_memory_state(…)` with a `historical_retention`
assumption.
[src](https://github.com/ankitects/anki/blob/main/rslib/src/scheduler/answering/mod.rs)
That is the strongest form of replayability in this set: the history *is* the
source of truth, and the engine is a pure function of it — which is exactly why
mixing algorithms is documented as unsafe rather than supported
("if an add-on affects intervals and scheduling in some way, it shouldn't be used
with FSRS").
[doc](https://docs.ankiweb.net/manual/deck-options#fsrs)

**SiYuan stamps identity and guarantees a fallback.** A custom block's persisted
content is `;;;plugin-name/block-type`, i.e. the producer is part of the data
format, not metadata about it; the rendered DOM is explicitly transient, and the
host renders the raw content when the plugin or block type is missing.
[doc](https://github.com/siyuan-note/plugin-sample) Note the granularity: plugin
*identity*, never plugin *version* — so "which plugin made this" is answerable and
"which version of it" is not.

**The rest do not stamp anything.** VS Code versions the *contract artifact*
(`vscode.d.ts` + `engines.vscode` range) and namespaces extension state per
identity, but nothing records "which extension version produced this workspace
state". Obsidian's `versions.json` solves app↔plugin compatibility at *install*
time, not producer↔data compatibility at *read* time.
[doc](https://docs.obsidian.md/Reference/Versions) Logseq's `LSPluginBaseInfo`
carries internal bookkeeping (`effect`, `iir`, `lsr`) but no producer stamp on
consumer-facing data.
[src](https://github.com/logseq/logseq/blob/master/libs/src/LSPlugin.ts)

`(inferred)` For a spaced-repetition app the ordering of value is: (1) the review
history must be replayable by the host alone, (2) any persisted artifact a plugin
produced must name the producer, (3) the producer's *semantics* version should be
recorded or derivable from a versioned field — Anki's `fsrsWeights`/`fsrsParams5`/`fsrsParams6`
slot pattern is the concrete trick for (3) without a migration framework, and its
in-band `ease_factor` range trick is what you get when you skip it.

### 5.4 Summary table (synthesis)

| Contract decision | VS Code | Anki | Obsidian | SiYuan | Logseq |
| --- | --- | --- | --- | --- | --- |
| Declaration artifact | `package.json` (`contributes`, `engines`, `capabilities`) | none (hook whitelist in repo) | `manifest.json` + `versions.json` | `plugin.json` | `package.json` → `logseq` |
| Where behaviour lives | runtime `registerCommand` etc. | `gui_hooks.x.append` | `Plugin.onload` `register*` | `onload` + kernel HTTP | `onload` over RPC |
| Capability declared | trust-dependency only | none | none (README disclosure) | platform + `publish.data` fields | UI isolation `mode` |
| Capability granted at runtime | workspace trust | none | Restricted Mode (global) | per-field admin grant | none |
| Version floor | `engines.vscode` (must not be `*`) | prose in release notes ("added in 2.1.20", "Anki 2.1.45+") | `minAppVersion` + `versions.json` | `minAppVersion` | not in `LSPluginPkgConfig` |
| Breaking-change policy | never break stable; proposed API channel | hook whitelist grows; legacy hooks kept alive | `@since` annotations + old builds installable | `minAppVersion` | typings only |
| Isolation unit | extension host process (local/web/remote) | same process, in-proc Python | same process | kernel HTTP + webview | iframe realm + message bus |
| Failure surface | Restart Extensions, Extension Bisect | error dialog naming add-ons + debug info | none specified | shared 5 s teardown budget | per-plugin log viewer |
| Producer stamp on data | none | in-band only (`ease`/`factor`/`type`) | none | plugin name in block fence | none |

---

## 三条我们应该抄

1. **契约产物 + 必填的提供方版本区间（VS Code `engines.vscode` 不可为 `*`，`vscode.d.ts` 是唯一契约）** —— 因为 `protocol: 1` 这种精确匹配只能硬失败，改成「契约产物 + 最低宿主版本」才能让旧扩展在新宿主上体面降级。
2. **声明与注册分离，且声明本身要能被宿主读懂（`contributes` 渲染/索引/激活，Anki 的 hook 白名单由 `genhooks` 生成并自动出文档）** —— 因为宿主可以在不启动子进程的前提下展示、校验并懒启动扩展的全部表面。
3. **派生输出不落库，落库必带生产者标识与可回退原文（SiYuan 的 `;;;plugin/type` + 缺插件回退原文；Anki 用 revlog 重放重建 `memory_state`）** —— 因为扩展被卸载后，数据库里的东西必须仍然可读、可解释。

## 三条我们不该抄

1. **「零权限、全信任」的扩展宿主（VS Code 明说宿主继承全部权限，Obsidian 明说无法限制插件）** —— 我们已经用「独立进程 + 只能经协议改状态」（decision #9）把爆炸半径压住了，退回进程内共享状态就是自毁这个优势。
2. **允许插件改调度结果与答案（Anki 的 custom scheduling 与 reviewer 钩子能改 `states`/`ease`，文档自己承认与 FSRS 不兼容）** —— 复习历史是不可重建的资产，扩展只能产出「建议」，写库必须由宿主完成并署名。
3. **把「运行时追加的全局钩子列表」当作唯一扩展点（`gui_hooks.x.append` 顺序取决于加载顺序、无排序键、异常直穿到全局错误框）** —— 扩展一多，调用顺序就变成隐式契约；要的是带名字、带排序、带数据归属的显式注册点（Obsidian `sortOrder`、VS Code contribution 的做法）。
