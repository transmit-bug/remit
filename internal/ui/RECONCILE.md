# Reconciliation against MyGo v0.7.4

Every UI need in the prototype, put against MyGo's inventory at the version
`go.mod` compiles (`go list -m -f '{{.Version}}' github.com/egoist/mygo` →
`v0.7.4`). The right column is what the code uses; a gap that survived is one
of the two wrappers documented below, or the heatmap's recorded composition.

| Need | Widget at v0.7.4 |
|---|---|
| Two faces of one window, notes and review | App state, `ui.Row` / `ui.Column` skeletons |
| A row of controls along the window (mode switch, new note) | `ui.Toolbar` + `ui.Segmented` + `ui.Button` + `ui.Spacer` |
| Choosing notes or review as one of a few views | `ui.Segmented`, labelled "Mode" |
| A source list of notes, keyboard-driven, with card counts | `ui.Sidebar`, `ui.SidebarSection`, `ui.SidebarItem`, `ui.Badge` |
| A note as a vertical document of blocks | `ui.Scroll` + `ui.Column` (not `ui.List`: see `blockRow`) |
| Editing a block's text | `ui.TextArea`, one per block, bound to the app's draft of the note and written through `store.SaveNote` once the fields are built |
| Renaming the note | `ui.TextInput` bound to the draft's title, written through `store.SaveNote`; the store mints the file's slug once, so the path survives |
| Making a card from a block, split at the first `::` | `ui.Button` calling `core.Split` |
| Adding, splitting, moving and deleting a block | `ui.PopoverMenuButton` per block (`ui.Menu.Item`, `Disabled` at the ends, `Separator` before Delete) — a command with options, not a row of buttons |
| Splitting a block where the caret is | `Element.TextSelection` read while the field is built; the last caret is kept in app state for the menu and the palette |
| Adding a block from the keyboard | `Element.OnTextKey`, because a text area takes Enter itself; the block is added after the pass, with the text as the key saw it |
| Marking a block that a card references | `ui.Badge` ("1 card", "2 cards") |
| The review card and its reveal | `flipCard` wrapper — gap 1 |
| The four grades, each in its rating colour | `ui.ButtonBase`, styled from the rating tokens |
| How many cards are left | `ui.Badge` ("5 left") |
| Undo, suspend, open the card's note | `ui.Button` in the review footer |
| The outcome of undo and of making a card | `c.Toast` (transient, fires once) |
| A card whose reference is frozen or lost | `ui.Badge` tinted with warning/danger + `ui.Text` |
| Pointing a lost card at another block | the palette's overlay again: `ui.DialogBase` + `ui.TextInput` + `ui.ButtonBase` rows over the notes snapshot, Enter or a click choosing (a `ui.List`'s own key model would take the keys from the search field) |
| The start view before a library exists: make one, or open one | `ui.Column` + `ui.Text` + `ui.PrimaryButton` + `ui.Button`; the folder comes from `mygo.Dialog.Open` with `Directory: true` |
| Empty states: no cards left, no notes | one `ui.Text` line + one `ui.Button` |
| The command palette over the window | `ui.DialogBase` + `ui.TextInput` + `ui.ButtonBase` rows + `ui.Spacer` |
| Filtering the palette by typing | App state + `ui.TextInput` (`ui.SearchField` is not used: its Escape clears the query before the dialog sees it) |
| The palette keeping and returning the keyboard focus | `ui.DialogBase` with the field focused through a `ui.Handle` request |
| Searching every card by its text | `ui.SearchField` (its Escape clears the query, then leaves browse), focused from anywhere through a `ui.Handle` request |
| One of a few card filters | `ui.Segmented` over All, Due, New, Suspended, Lost, labelled "Filter cards" |
| A long list of matching cards, keyboard-driven | `ui.List` + `ui.ListState` (`Key` by card ID, `Selected`, `Submitted` to open, `Label`) |
| Opening the note behind a card, or the lost card in review | App handlers calling `setMode`, never the row's own click |
| The year's reviews as a calendar, weeks over weekdays | composed: `ui.Row` + `ui.Column` + tinted `ui.Box` squares + `ui.Text`; see the heatmap note below |
| The forecast, over the next 30 days | `ui/chart`: `Plot` + `NewTime` + `Rect` marks; day labels through `Time.Format` and the catalogs' `{date, date, short}` |
| Retention per predicted chance bucket | `ui/chart`: `Plot` + `NewBand` + `Rect` marks |
| Switching to browse or statistics | `ui.Segmented` (two more segments) + palette actions; statistics needs no key of its own |
| A section a statistics series has no data for | one `ui.Text` empty line per section, and no axis: a rate with no reviews is not divided by zero; the heatmap says it has no reviews yet |
| Grading advancing as a short slide and fade | `Element.Transition` + `ui.Motion` (reduce motion honoured by the framework) |
| Separating panes, prompt and answer | `ui.Divider` |
| Theme, accent and dark mode following the system | `c.Theme()` resolved into tokens; `c.SetTheme` for one density |
| Keyboard: global and review keys | `c.OnShortcut` (window scope) |
| The command palette trapping the pointer and keys | `ui.DialogBase` (`Modal`) over the dimmed window |

## Wrapper 1 — `flipCard` (`review.go`)

**The need.** One review card: the prompt always visible, the answer revealed
in place, the reference state when it is not live, and an identity the advance
transition can follow. This is the unit the queue advances and grades.

**Checked at v0.7.4.** `ui.Collapsible` and `ui.Accordion` (a section that opens
and closes under its own label, not a two-sided review card with a reference
state), `ui.Toggle` and `ui.ToggleGroup` (a pressed state, no content swap),
`Element.ContentTransition` (fades or grows replaced content; it has no sides,
no link state and no grading), `ui.Popover` and `ui.DialogBase` (overlays above
the window, not an in-page card), `ui.ButtonBase` (used for the reveal action,
a control rather than the card), `ui.Box` / `ui.Column` (the layout it
composes). The command palette uses `ui.DialogBase` directly for its own need,
which is an overlay.

**Why it is a different need.** None of these carries "a card with a prompt, an
answer and a reference to a block"; they change a value, a section's visibility
or a whole overlay. The wrapper is the composition of `ui.Column`, `ui.Text`,
`ui.Divider`, `ui.Badge`, `ui.ButtonBase` and `ui.PrimaryButton`, with no
behaviour re-implemented from a widget.

## Wrapper 2 — `blockRow` (`editor.go`)

**The need.** One block of the editor: its multiline text, the cards that
already point at it, and the actions that change it or mint another card from
it, in a vertical document where every block is editable and all are laid out.

**Checked at v0.7.4.** `ui.TextArea` (the field itself, used inside the row;
it has no per-row actions or reference mark), `ui.TextAreaBuffer` (indexed
storage for large documents; a block is a short string and the buffer's
machinery buys nothing here), `ui.EditableText` (single-line rename in place,
as a file's name in Finder, not a multiline block), `ui.Table` +
`EditableText` (a grid of short cells with in-place rename; a block is
multiline document text that the table would not measure as one), `ui.List`
rows (builds only the rows in view; the scroll guide says a document whose
text selection must span it uses a plain scroll container, and every block's
field keeps focus and state across the whole note). The block's changes are
`ui.PopoverMenuButton`, the widget for a command with options, drawn in the
window so its items are the app's own elements.

**Why it is a different need.** No widget carries "an editable block that a
card can be minted from and that can be split, moved or deleted". The row
composes `ui.Row`, `ui.TextArea`, `ui.Column`, `ui.Badge`, `ui.Button` and
`ui.PopoverMenuButton`, and delegates the text editing, focus and keyboard
entirely to `ui.TextArea`. A menu's choice is recorded in the pass and
performed after the fields are built, so no store write happens while the
view is being built.

## The review heatmap, composed rather than wrapped (`stats.go`)

**The need.** Decision #11 names a review heatmap: a calendar where weeks are
columns, weekdays are rows, and a day's colour is how many reviews it holds,
with a legend and an empty state.

**Checked at v0.7.4.** `ui/chart` has no heatmap: its marks are `Rect`,
`Point`, `RuleX` and `RuleY` over its scales, which a grid of squares could be
built from but which would carry no day, no label and no legend.
`ui.Calendar` is a month picker — one month of selectable days, not a year's
map with intensities. `ui.GridView` lays out and virtualises a scrolling grid
of items, not a fixed calendar whose cells address days. `ui.Grid` is the
layout grid; the heatmap uses `ui.Row` and `ui.Column` because the calendar's
shape is its data, not a layout choice. `ui.Meter` and `ui.Progress` show one
value, not a matrix of them. `ui.Icon` has no square glyph MyGo draws itself.

**What it is.** Seven `ui.Text` weekday names and 26 week columns of seven
`ui.Box` squares, tinted from `surfaceSunken` towards the accent in four
steps, each counted day named and tooltipped with its date and count
(`statsDayCount`), plus a five-box legend from "Less" to "More". The two
sizes it needs are named in `stats.go` (`heatCell`, `heatGap`) and nothing is
painted; the weekday names and the range come from the catalogs, so the
calendar follows the locale like every other text.

## Audit notes

- The inventory grep ran against `/Users/pony/go/pkg/mod/github.com/egoist/mygo@v0.7.4`.
- The charts use `ui/chart`'s plot, scales and `Rect` marks only: no bar or
  row table was hand-built, and no pixel is painted. `Axes.Label` and
  `Time.Format` carry the window's language, so axis labels follow the locale
  like every other text.
- Browse lists cards through `store.SearchCards`, `store.CardsWithStatus` and
  the queue's buckets, all through `loadBrowse` on the app's load path: no view
  builds rows from a store call.
- Literal geometry audit: the only fixed sizes outside `tokens.go` are
  `sidebarWidth` (260 DIP, a real fixed column), `reviewWidth` (680 DIP, the
  review column's maximum), `messageWidth` (520 DIP, the error state's line),
  `statsWidth` (760 DIP, the statistics page's measure),
  `statsChartHeight` (150 DIP, one plot's height), `relinkPanel` (360 DIP, the
  picker's height), and the heatmap's `heatCell`, `heatGap` and
  `heatWeekdayWidth` (12 DIP squares, 3 DIP between them, a 34 DIP label
  column); all are named and commented.
- Colours and spacing used by the views come from `tokens.go`; widgets keep
  their own theme-derived look.
- `ui.Mix` documented on the site is not exported at v0.7.4; the tokens blend
  with `Color.Mix`, which is.
