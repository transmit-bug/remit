# UI inventory

Every surface, the official widget that carries its meaning, and the MyGo version
the row was checked against. Rule and enforcement: `docs/specs/ui-composition.md`.
A row whose State is not `ok` is work, not a note.

Checked against `github.com/egoist/mygo` **v0.8.1** (`go list -m -f '{{.Dir}}'`).
Update a row in the same change that touches its surface.

## Shell

| Surface | Need | Widget | State |
|---|---|---|---|
| Window toolbar | a row of window controls | `ui.Toolbar` | ok (sparse: see Notebook) |
| Mode switch (笔记/复习/浏览/统计) | one of a few adjacent views | `ui.Segmented` | ok |
| Notebook panes | two panes the user resizes | `ui.Split` / `SplitPercent` + `SplitReset` | ok |
| Paper | one centred measure, no box | `Row(Justify Center)` + `Column(MaxWidth 760)` + `Padding` | ok |
| Document typography | body 18 / 1.85, headings 1.9em down | `richtext.Theme` + `ui.Font{Size, LineHeight}` | ok |
| Editor surface | text on paper, no field look | `richtext.EditorBase` | ok |
| Status line | words, paragraphs, cards, saved | `Row` + `Text` 11px | ok |
| Sidebar search | find a note by title or text | `ui.SearchField` | ok |
| View switch | one of three views | `ui.Segmented` in the window toolbar | ok |
| Notes column | source list with sections and counts | `ui.Sidebar` + `SidebarItem` + `Badge` | ok |
| Tag filter | a row of filters, one on | `ui.ToggleGroup` | **todo S3** — `ButtonBase` chips |
| Note panel: view switch | one of two adjacent views | `ui.Segmented` | ok |
| Note panel: outline | a tree of a note's headings, current one marked | `ui.Outline` + `OutlineState` | ok (the caret marks the current heading) |
| Note panel: backlinks | rows of any height, chosen | `ui.List` | **todo S3** — `ButtonBase` loop |
| Command palette | type to narrow, keys to choose | `ui.List` (input + keys) inside `ui.DialogBase` | ok |
| Relink picker | free text with hints | `ui.Autocomplete` | **todo S3** — hand rows |
| Toasts | transient outcome | `c.Toast` | ok |
| Dialogs / overlays | a custom dialog above the page | `ui.DialogBase` | ok |

## Notebook

| Surface | Need | Widget | State |
|---|---|---|---|
| Title | one line of text | `ui.TextInput` | ok |
| Body | attributed text with headings, lists, links | `richtext.Editor` + `richtext.ParseMarkdown` | ok |
| Save state | a short status label | `ui.Badge` / `ui.Text` | n/a — every edit is written at once, so a status label would report nothing |
| View modes (编辑/分栏/阅读) | one of a few adjacent views | `ui.Segmented` + `ui.Split` + `ui/markdown.View` | ok |
| Text size | a choice from a menu | `ui.MenuButton` + `c.SetTheme` | todo (the desktop's text scale already reaches the whole window) |
| Export | a menu of commands | `ui.MenuButton` | todo — no export exists yet to put in the menu |
| Overflow | a menu of rare commands | icon `Button` + `Element.Menu` | ok (import, make card, settings) |
| Format bar | toggles that report the selection's formats | `ui.Toolbar` + `ui.MenuButton` + `Document.Formatting()` + `Document.Toggle*` | ok (the pressed look is a `Button` in the accent tint, as MyGo's notes example does) |
| Find in note | find inside a document | `ui.FindBar` | ok |
| Paragraph handles | a menu per visible paragraph | `Document.VisibleParagraphs()` + `ui.MenuButton` | gap (not built) |
| Card badges | action per paragraph | `ui.ButtonBase` in a `ScrollHorizontal` row | ok (composition, no widget carries "a row of badges") |

## Review

| Surface | Need | Widget | State |
|---|---|---|---|
| Card face | Markdown, read-only | `ui/markdown.View` | ok |
| Grades | push buttons, main action | `ui.Button` + `kit.Keycap` | ok |
| Progress | work done | `ui.Progress` | ok |
| Session summary | a report after the queue | `kit.Card` + `kit.Stat` + `PrimaryButton` | ok |

## Browse

| Surface | Need | Widget | State |
|---|---|---|---|
| Card list | a data table, sortable | `ui.Table` + `Columns`/`Sortable` | **todo S3** — `ListState` + hand rows |
| Search | search with Escape to clear | `ui.SearchField` | ok |
| Filter | one of a few adjacent views | `ui.Segmented` | ok |

## Stats and settings

| Surface | Need | Widget | State |
|---|---|---|---|
| Retention / forecast | a trend, bars | `ui/chart` | ok |
| Review heatmap | a calendar of levels | — | gap: `ui/chart` has no heatmap at v0.8.1 |
| Settings rows | a form | `ui.Form` + `ui.Field` + `ui.Fieldset` | ok |
| Toggles | on/off setting | `ui.Switch` | ok |
| About | a dialog | `ui.DialogBase` | ok |

## Known gaps (not violations)

- **Heatmap** — `ui/chart` (v0.8.1) has no calendar heatmap; `stats.go` builds a grid of tinted boxes.
- **Split editing** — `richtext` keeps one view per `Document` (v0.8.1), so the 分栏 right side is `ui/markdown.View`, read-only.
- **`::` card derivation** — product design (ADR 0002), not a rendering gap.
- **Block handles, insert menu, export** — not built yet; `docs/specs/ui-redesign.md` P1.
