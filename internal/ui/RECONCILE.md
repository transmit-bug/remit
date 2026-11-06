# Reconciliation against MyGo v0.7.4

Every UI need in the prototype, put against MyGo's inventory at the version
`go.mod` compiles (`go list -m -f '{{.Version}}' github.com/egoist/mygo` →
`v0.7.4`). The right column is what the code uses; a gap that survived is one
of the two wrappers documented below.

| Need | Widget at v0.7.4 |
|---|---|
| Two faces of one window, notes and review | App state, `ui.Row` / `ui.Column` skeletons |
| A row of controls along the window (mode switch, new note) | `ui.Toolbar` + `ui.Segmented` + `ui.Button` + `ui.Spacer` |
| Choosing notes or review as one of a few views | `ui.Segmented`, labelled "Mode" |
| A source list of notes, keyboard-driven, with card counts | `ui.Sidebar`, `ui.SidebarSection`, `ui.SidebarItem`, `ui.Badge` |
| A note as a vertical document of blocks | `ui.Scroll` + `ui.Column` (not `ui.List`: see `blockRow`) |
| Editing a block's text | `ui.TextArea`, one per block, bound to the stored block |
| Making a card from a block, split at the first `::` | `ui.Button` calling `core.Split` |
| Marking a block that a card references | `ui.Badge` ("1 card", "2 cards") |
| The review card and its reveal | `flipCard` wrapper — gap 1 |
| The four grades, each in its rating colour | `ui.ButtonBase`, styled from the rating tokens |
| How many cards are left | `ui.Badge` ("5 left") |
| Undo, suspend, open the card's note | `ui.Button` in the review footer |
| The outcome of undo and of making a card | `c.Toast` (transient, fires once) |
| A card whose reference is frozen or lost | `ui.Badge` tinted with warning/danger + `ui.Text` |
| Empty states: no cards left, no notes | one `ui.Text` line + one `ui.Button` |
| The command palette over the window | `ui.DialogBase` + `ui.TextInput` + `ui.ButtonBase` rows + `ui.Spacer` |
| Filtering the palette by typing | App state + `ui.TextInput` (`ui.SearchField` is not used: its Escape clears the query before the dialog sees it) |
| The palette keeping and returning the keyboard focus | `ui.DialogBase` with the field focused through a `ui.Handle` request |
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
already point at it, and the action that mints another card from it, in a
vertical document where every block is editable and all are laid out.

**Checked at v0.7.4.** `ui.TextArea` (the field itself, used inside the row;
it has no per-row actions or reference mark), `ui.TextAreaBuffer` (indexed
storage for large documents; a block is a short string and the buffer's
machinery buys nothing here), `ui.EditableText` (single-line rename in place,
as a file's name in Finder, not a multiline block), `ui.Table` +
`EditableText` (a grid of short cells with in-place rename; a block is
multiline document text that the table would not measure as one), `ui.List`
rows (builds only the rows in view; the scroll guide says a document whose
text selection must span it uses a plain scroll container, and every block's
field keeps focus and state across the whole note).

**Why it is a different need.** No widget carries "an editable block that a
card can be minted from". The row composes `ui.Row`, `ui.TextArea`,
`ui.Column`, `ui.Badge` and `ui.Button`, and delegates the text editing,
focus and keyboard entirely to `ui.TextArea`.

## Audit notes

- The inventory grep ran against `/Users/pony/go/pkg/mod/github.com/egoist/mygo@v0.7.4`.
- Literal geometry audit: the only fixed sizes outside `tokens.go` are
  `sidebarWidth` (260 DIP, a real fixed column) and `reviewWidth` (680 DIP,
  the review column's maximum); both are named and commented.
- Colours and spacing used by the views come from `tokens.go`; widgets keep
  their own theme-derived look.
- `ui.Mix` documented on the site is not exported at v0.7.4; the tokens blend
  with `Color.Mix`, which is.
