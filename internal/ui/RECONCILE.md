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
| The settings page: preferences written as they change | `ui.Form` + `ui.Fieldset` + `ui.Field` (label, `Description`, `Error`) |
| An on/off preference that applies at once | `ui.Switch` in a `ui.Field`, which names it and toggles it when its label is clicked |
| A shortcut the user types | `ui.TextInput` in a `ui.Field`, with the refusal in the field's `Error` |
| Outcomes that are not a form value: where the agent listens, what an update check found | `ui.Text` beside the control, muted and small; `c.Toast` when the action is done |
| A page taller than the window | `ui.Scroll` + the centred `ui.Column` measure the statistics page uses |
| Importing an Anki package | a palette action that opens `mygo.Dialog.Open` with an `.apkg` filter, then `importanki.Import` from the handler; `c.Toast` for the report |
| The command palette trapping the pointer and keys | `ui.DialogBase` (`Modal`) over the dimmed window |
| A symbol beside a note in the sidebar | `ui.SidebarItem(c, id, icon, label)`, which takes the `*SVG` itself |
| A symbol before a button's label | `ui.Button(c, "")` + `Children(icon + text)`: the chrome, hover and press stay the widget's, the button names itself with `Label` |
| An action that is only a symbol | `ui.Button(c, "")` + `Children(ui.Icon)` + `Label` + `Tooltip`, as the re-link dialog's close button |
| A symbol on the card's link state | `ui.Badge(c, "")` + `Children(icon + text)`, tinted as before |
| A page or a panel with nothing in it | a centred `ui.Column`: `ui.Icon` at 24, one sentence, the action that fills it |
| Where the symbols come from | `design.Icon(name)` over the vendored Lucide set; `ui.Icon` draws them in the text colour |
| Symbols inside the mode switch | **gap** — see below |

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

## Wrapper 2 — the card badge in the editor's gutter (`editor.go`)

**Correction (ADR 0005).** This entry used to claim that no MyGo widget carried
"an editable block that a card can be minted from", and built that claim on
`ui.TextArea`, `ui.TextAreaBuffer`, `ui.EditableText`, `ui.Table` and `ui.List`.
It never checked `ui/richtext`, `ui/markdown` or `ui/editor`, all of which ship
at the same version. The claim was wrong: a note is a **document**, not a stack
of fields, and the document editor exists (`richtext.Editor` — headings, lists,
quotations, code, inline images, links, Markdown shortcuts, IME, accessibility,
large documents, Markdown round-trip).

**The need that survives.** In that document, a paragraph that a card points at
shows, in the editor's gutter, how many cards it feeds and their link state, and
clicking it opens the card's face for an override.

**Checked at v0.7.4.** `richtext.Editor` exposes paragraph geometry but nothing
that draws outside the text column; `ui.Badge` is an in-flow badge, not a gutter
mark; `ui.Tooltip` is a hover surface, not a persistent mark; `ui.List` builds
rows, not paragraphs of one continuous text; `ui.Overlay` draws relative to an
element, not to a paragraph inside an editor.

**Why it is a different need.** A gutter mark is not a widget in the layout — it
is a decoration pinned to a paragraph's line box inside someone else's editor.
The wrapper draws it from the paragraph rects the editor reports and keeps the
click target outside the text flow; everything else (text, focus, caret,
selection, IME) stays the editor's.

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

## Motion — which mechanism each moment takes (`internal/ui/design/motion.go`)

Every moment of the design language names its mechanism, so nothing here is a
hand-built clock:

| Moment | Mechanism | Why not another |
|---|---|---|
| The first frame, in two beats | `Keyframes` on the toolbar and on the main area, the second with `Delay: EnterStagger` | A spring (`AnimateTo`) has no curve to stagger; the preset's frames carry the light overshoot |
| A screen replacing another | `Keyframes` run by the mode's own count | `AnimateTo` has one target, so a replayed entrance would need a jump the API does not offer |
| A panel or a dialog arriving | `Keyframes` run by the panel's count, on the panel's `Opacity` and `Scale`, and the scrim's `Opacity` | `ElementTransition.Enter` needs a parent whose last frame showed the element; the palette's parent is new each time |
| Hover and press on a grade | `ElementTransition{Colors: true}` at `Hover`, and `AnimatePose` at `Press` | Poses are paint-only, so a press cannot move the layout it sits in |
| Reveal, grade, row height, counts | presets only, wired by the screens that own them (#89, #87) | the landing points are other tickets |

Reduced motion needs no code of ours: `Keyframes`, `AnimatePose` and
`ElementTransition` read `Preferences.ReduceMotion` and snap to their ends,
which `TestReducedMotionSwitchesAtOnce` asserts against the same switch running
normally.

## Gap — icons in the mode switch (`shell.go`)

**The need.** Spec #78 asks the five modes to read as an icon and a word, so
that the switch is recognised rather than read.

**Checked at v0.7.4.** `ui.Segmented` (draws its `labels ...string` and nothing
else), `ui.SegmentedBase` (lookless by design: the segments are built by the
caller, and its own doc names this as the icon path), `ui.ToggleGroup` /
`ui.Toggle` (a multi-select of pressed states, not one choice of five),
`ui.Tabs` / `ui.TabsBase` (the same base/look split, labels only), `ui.Toolbar`
(a container, not a choice), `ui.Sidebar` (a source list with its own frame).

**Why it is left as a gap.** `SegmentedBase` is the documented route to icons,
but carrying the look over means re-creating what `styleButton` does for a
segment — padding, radius, the selected face, and the hover and press faces —
inside this package. That is the same hand-built widget look ADR 0005 was
written against, and it would drift from the theme the moment MyGo's segment
styling changes. The mode switch therefore keeps its words, and the icons it
should carry wait for one of:

- a MyGo release whose `Segmented` takes an icon per label (an upgrade is the
  fix, per the version protocol — not a wrapper), or
- a decision to build the switch in the kit once #87 has a card to sit beside,
  with the duplicate look recorded here as a deliberate cost.

`ui.SidebarItem` needs none of this: the side list's rows carry their icon
through the widget's own parameter.

## Audit notes


- The inventory grep ran against `/Users/pony/go/pkg/mod/github.com/egoist/mygo@v0.7.4`.
- ADR 0005 corrects this file: the packages that decide the note editor are
  `ui/richtext`, `ui/markdown` and `ui/editor`, and they were missing from the
  first pass. An entry whose evidence skipped a package is a bug, not a scope
  call.
- The charts use `ui/chart`'s plot, scales and `Rect` marks only: no bar or
  row table was hand-built, and no pixel is painted. `Axes.Label` and
  `Time.Format` carry the window's language, so axis labels follow the locale
  like every other text.
- Browse lists cards through `store.SearchCards`, `store.CardsWithStatus` and
  the queue's buckets, all through `loadBrowse` on the app's load path: no view
  builds rows from a store call.
- The settings page needed no wrapper: `ui.Form`, `ui.Fieldset`, `ui.Field`,
  `ui.Switch`, `ui.TextInput`, `ui.Button` and `ui.Scroll` carry every control
  and every explanation, and the inline error is `Field.Error`, not a tinted
  line. Its one composed need — a status line under a control, as "Listening
  on 127.0.0.1:51234" or what an update check found — is `ui.Text` in the
  fieldset, because the value belongs to the control rather than the look.
- Literal geometry audit: the only fixed sizes outside `tokens.go` are
  `sidebarWidth` (260 DIP, a real fixed column), `reviewWidth` (680 DIP, the
  review column's maximum), `messageWidth` (520 DIP, the error state's line),
  `statsWidth` (760 DIP, the statistics page's measure),
  `settingsWidth` (560 DIP, the settings page's measure),
  `statsChartHeight` (150 DIP, one plot's height), `relinkPanel` (360 DIP, the
  picker's height), and the heatmap's `heatCell`, `heatGap` and
  `heatWeekdayWidth` (12 DIP squares, 3 DIP between them, a 34 DIP label
  column); all are named and commented.
- Colours and spacing used by the views come from `tokens.go`; widgets keep
  their own theme-derived look.
- `ui.Mix` documented on the site is not exported at v0.7.4; the tokens blend
  with `Color.Mix`, which is.
- Icons are Lucide's, vendored from `lucide-static@0.460.0` under ISC in
  `internal/ui/design/icons/` and embedded: `git grep -n '<svg' -- '*.go'` finds
  no inline drawing, and the app requests nothing for an icon. Sizes come from
  the scale (`iconSmall` 16, `iconMedium` 20, `iconLarge` 24).
