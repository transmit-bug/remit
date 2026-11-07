# ADR 0007 — Plugin system: host API, capabilities and the "built-in is a plugin" boundary

- **Status:** accepted — amended by ADR 0008 (the `cards.render` hook is withdrawn, the derive contract is typed and the host renders), by the v1 host API (#139/#140: the hook ids and shapes below were renamed and reshaped; the vocabulary is `internal/ext/manifest.go` and the shapes are `docs/specs/plugins/system.md` §5) and by #146 (UI is declared in `contributes`, drawn by the host)
- **Date:** 2026-11-07
- **Amends:** #9 (extension form), #11 (v1 scope), ADR 0002 (who derives a card)
- **Related:** #68, #69, #71, #72–#75, #77, ADR 0006

## Context

#9 defined extension *transport*. #69's map asked what an extension may *do*.
The direction chosen there is that the plugin system is a core part of the
product, not an add-on: card derivation, card rendering and the scheduling
policy all move behind one boundary, and the built-in behaviour becomes the
**default plugin** rather than a privileged code path.

Two constraints shape the boundary: review history is an asset that may not be
silently rewritten, and a plugin failure may not stop the review flow.

## Decision

### One hook vocabulary, declared and validated by the host

`extension.json` declares `implements: { "<hook id>": <contract version> }` over
a host-defined vocabulary — `cards.derive`, `cards.render`, `review.schedule` —
plus `activation` (when to load) and `capabilities` (what it may reach). Export
names are fixed by the host, so a plugin's claimed surface is validated without
loading it, and there is no runtime-appended hook list whose order becomes an
implicit contract.

*Amended (#139/#140):* the vocabulary is now `describe`, `deriveBlock`,
`policy`, `identity`, `summary` and `on_event`, each at contract version 1, and
`contributes` rides beside `implements`. The vocabulary is closed: a hook id
outside it is refused before a line of JavaScript runs.

### Three pure hooks, one presentation hook and one notification

```ts
deriveBlock(block)    -> [{ ordinal, kind, front, back }]
renderCard(card)      -> { frontMarkdown, backMarkdown }
scheduleGrade(input)  -> { intervalSeconds, state, previews? }
reviewSummary(input)  -> { title, lines: string[], tone: "quiet" | "celebrate" }
onEvent(event)        -> void
```

*Amended (#139/#140):* the shapes above became `deriveBlock(block,
types) -> [{ typeId, ordinal, fields, unknown? }]`, `policy() -> { family,
version, retention }` and `summary(session) -> { tone, lines }`, and `onEvent`
is `on_event`. `renderCard` is withdrawn (ADR 0008); the host renders a card
type's templates.

The first three never write. Their output is a **suggestion**: the host
validates, clamps and writes, stamping `producer { id, version }` on what it
derives. Card faces are never persisted; they are re-derived, so a live
reference still follows its block (ADR 0002).

### Capabilities are objects, granted at enable

`notes:read`, `cards:write`, `review:read`, `review:write`, `schedule:fsrs`,
`store:own`. Enabling a plugin shows its declared list and asks the user; at
runtime the host installs only the granted `remit.*` functions and the standard
library is frozen. There is no `fs`, `net` or UI, and no `node_modules`.

*Built (#142, #144):* the six names are installed when declared, the settings
page lists them at enable time and the approval is the write into the
library's `extensions`, and a call without its declaration is a
`CapabilityError` naming the plugin and the capability.

*Amended by [#146](https://github.com/transmit-bug/remit/issues/146):* a plugin
may **declare** commands, menus, pages and settings in its manifest's
`contributes`, and the host draws them with its own components and tokens
([`docs/specs/plugins/ui.md`](../specs/plugins/ui.md)). What stays forbidden is
plugin-owned DOM, CSS, canvas or drawing.

### The completion moment is content, the host owns the motion

When a session empties, the host asks the scheduler plugin's `reviewSummary`
what to say: a title, some markdown lines, and a **tone**. The host renders it
and picks the motion from its own vocabulary — a quiet fade by default, a
bounded celebration when the tone asks for one, and the same reduced-motion
rules as everything else.

This replaces #21's blanket *"no celebration"* with *"the default plugin does not
celebrate, and a plugin may ask for it"*. What does not change: the host renders
it, so a plugin still cannot inject UI, and the motion stays inside the table.

The same boundary holds for declarative UI contributions (#146): the host owns
every widget, layout, token and motion; a plugin names *what* it wants to say.

### Roles, not ordering

Each role (`cards.derive`, `cards.render`, `review.schedule`) has **one** plugin
per library, defaulting to the built-in one. The host therefore has no
plugin-ordering problem, which is the failure mode of runtime hook lists.

*Amended (ADR 0008, #144):* `cards.render` is gone; the two roles are
`cards.derive` and `review.schedule`, and the settings page chooses one plugin
for each, stored in the library's `roles`.

### Templates live inside a plugin

The render contract is markdown, and the host renders it with `ui/markdown`. An
Anki-style field/template system, if a plugin wants one, lives inside that
plugin with its settings in `store:own`. The host schema never grows note types
or templates; switching render plugins means switching templates, and removing
one falls back to the block's text.

*Withdrawn by ADR 0008:* types, fields and templates are library assets and the
host is the renderer. A plugin answers typed fields; it does not render, and it
cannot bring a type definition (`produces` is a declaration the library must
already satisfy).

### Scheduling: state and maths in the host, policy in the plugin

`go-fsrs` stays a host capability (`remit.fsrs.review`); `state` is opaque to
plugins. A scheduler plugin supplies policy — steps, caps, fuzz, when to
demote. Every review record carries `scheduler { family, flags, version }`; a
switch is recorded as a boundary and **never rewrites earlier history**. A
family-plus-flags shape is deliberate: Anki's `schedVer` is a family, and "v3"
was a flag on V2, not a third family.

### Failure falls back to the default plugin

Throw, invalid shape, 1 s, 64 MiB: count one failure, fall back to the built-in
default plugin for that hook, and after three consecutive failures disable the
plugin and say so. A load error affects only that plugin; the library still
opens.

### Built-in is a plugin, and there is no second implementation

The default derive/render/schedule plugin ships in the application, is loaded
through the same loader and checked against the same capabilities, with no
privileged entry. The Go implementation is deleted once (a) golden fixtures
captured from it pass against the JavaScript one and (b) the default plugin is
the only implementation left.

### Versioning

`apiVersion` is the version of the contract artifact `remit.d.ts`; a host loads
any plugin whose `apiVersion` is not newer than its own. Breaking changes bump
it, with a one-major deprecation window. `protocol` (the #9 wire protocol) and
`apiVersion` (the host surface) are separate.

*Built (#145):* `remit.d.ts` is generated from the host's own tables
(`go run ./cmd/remit ext types -o remit.d.ts`) and a test fails when the checked-in
file drifts from the code; `apiVersion` is `ext.HostAPIVersion`.

## Consequences

- v1 keeps the Go implementation until the plugin path passes the fixtures; the
  window where both exist is bounded by that condition, not by preference.
  *Done:* the golden set (`internal/golden/testdata/golden.json`) passed against
  the JavaScript plugins and the Go deriver and scheduler were deleted (`aecb7de`).
- The review flow cannot be locked out by a misbehaving plugin.
- Review history stays readable and explainable after a plugin is removed.
- In-process plugins make the engine a stability dependency (ADR 0006).
