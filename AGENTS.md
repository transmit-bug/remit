# remit

## Agent skills

### Issue tracker

GitHub Issues in `transmit-bug/remit` (via `gh` CLI). See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### MyGo implementation skills (mandatory)

Every implementation session and every subagent it spawns reads, before writing
code:

1. `.agents/skills/mygo-usage/SKILL.md` — app structure, state and identity,
   threads and frames, `ui.NewTester` headless tests, verification commands.
2. `.agents/skills/mygo-components/SKILL.md` before any UI work — reconcile each
   need against the official widget inventory before composing a layout by hand.

Reach for `mygo-motion`, `mygo-drawing`, `mygo-desktop`, or `mygo-maintenance`
when the change is about motion, custom painting, OS services, or fixing an
existing MyGo feature. The MyGo version that matters is the one `go.mod` pins.

### Engineering conventions

`docs/engineering/conventions.md` — layout and dependency direction, test
rules, CI gates, branch/PR flow, licensing of dependencies.
