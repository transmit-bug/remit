---
name: mygo-components
description: Pick MyGo's own widget before composing layout by hand. Use when writing or changing native UI that imports github.com/egoist/mygo/ui — a new screen, table, form, navigation, feedback, status, empty state or split pane, a styling/theme pass, or any moment a screen "seems to need a widget MyGo lacks". Reconciles each UI need against the official MyGo inventory at the version go.mod compiles against, and records the verdict.
---

# Reconcile against MyGo's inventory

A UI need is answered by the widget that already carries its meaning.
`Row`, `Column` and `Text` compose what no widget covers; choosing them is a
recorded decision, not the default.

Scope: widget choice, layout skeletons, visual consistency, accessibility that
comes from using the real widget; product design is the project's. Making a
change move is `../mygo-motion`; painting beyond widgets is `../mygo-drawing`.

The whole skill is one move: **reconcile**. Name the need, put it against the
official inventory, and record which side wins.

## Reconcile, before the first line of layout

1. **Resolve the source, newest first.** Version protocol at `../mygo-usage`:
   the copy `go list -m -f '{{.Dir}}'` names is what compiles and may **lag**
   the site; a widget the site has and the module lacks is an upgrade, not a
   wrapper — upgrade the module, then re-read these tables. Read the module's
   copy for what compiles — the catalogue is `$MYGO/docs/ui/README.md`, the
   constructors are
   `grep -rhE '^func [A-Z][A-Za-z0-9_]*[\[(]' --include='*.go' --exclude='*_test.go' $MYGO/ui | sed -E 's/func ([A-Za-z0-9_]+)[\[(].*/\1/' | sort -u`
   (every package-level constructor: generics and hand-written ones, tests left out).
   Read the docs' prose, not only the signature: the same page states the
   keyboard, focus and assistive-technology behaviour you would otherwise
   rebuild.
   Done when: every widget you plan to use was opened in the copy that has it,
   and you know from `go list -m -versions` whether a newer release is ahead of
   this app.

2. **Name the need, not the shape.** "Report that the save worked" and "two
   panes the user can resize" are needs; "a row with two texts" is a shape.
   Restate shapes as needs, then find the row in the map below.
   Done when: every UI need in the change has a row, or a recorded gap.

3. **Read the widget's whole capability set** before building around it:
   `go doc github.com/egoist/mygo/ui.TableColumn`, or
   `sed -n '/type ListState struct/,/^}/p' $MYGO/ui/list.go`. The usual failure
   is using a widget for its pixels and then hand-building what it already does
   — sorting, resizing, reordering, in-place rename, menus, keyboard, focus,
   disabled reasons, screen-reader roles.
   Done when: for every stateful widget in the change, each behaviour you plan
   to hand-build has been checked against its methods and fields.

4. **Extend only on a real gap.** No widget carrying that meaning, after step 3,
   means one small wrapper in the project's widget package — never inline in a
   page, never a second copy elsewhere (`docs/ui/custom-widgets.md` covers the
   lookless base widgets a wrapper composes: `Box`, the `*Base` variants, and
   the parts they return). Its doc comment names the widgets you
   checked, why each is a different meaning, and the MyGo version checked.
   A stale "MyGo has no X" comment is a bug: re-check it against the source
   go.mod resolves to whenever you touch the file.

5. **Verify** (commands at the end). Done when the audit output is fixed or
   explained, and every touched interaction has a `ui.NewTester` test.

## Need → widget

The third column is the tell: composition in the codebase that means this row.

This map tracks the official catalogue, which documents the newest release. A
name here that does not resolve in the module go.mod requires is newer than this
app: upgrade there (step 1), rather than wrapping what MyGo is about to ship.

### Actions

| Need | Use | The tell it retires |
|---|---|---|
| A row of controls along a window | `ui.Toolbar` | `Row` of `Button`s: loses one tab stop, arrow keys, overflow menu |
| A command with options | `ui.MenuButton`; a menu of choices with their current one ticked `ui.PopoverMenuButton` | `Button` + `Popover` wired by hand |
| Right-click on an element, row, cell or header | `Element.ContextMenu`, `ListState.CellMenu`, `ListState.HeaderMenu` | `Popover` placed at the pointer |
| A button that stays pressed | `ui.Toggle`, `ui.ToggleGroup` | `int` field plus accent tint |
| One of a few adjacent views | `ui.Segmented` | Row of buttons with an index |
| Push button, main action | `ui.Button`, `ui.PrimaryButton` | `Box` + `OnClick` |
| Icon-only button | `ui.Button(c, "").Children(Icon).Label(...).Tooltip(...)` | clickable icon with no name |
| A URL or a router page | `ui.Link` | `Text` + `OnClick` |
| Back and forward | `ui.BackButton`, `ui.ForwardButton` | custom history buttons |

### Choices

| Need | Use | The tell it retires |
|---|---|---|
| On/off setting | `ui.Switch` | checkbox labelled like a setting |
| Checks, one or a group | `ui.Checkbox`, `ui.CheckboxGroup` | `Box` + tick glyph |
| One of a few, all visible | `ui.RadioGroup` | buttons with a selected style |
| One of many | `ui.Select`; long and typable `ui.Combobox`; free text with hints `ui.Autocomplete` | `TextInput` + validation later |
| Value in a range, or two | `ui.Slider`, `ui.RangeSlider`, `ui.StepSlider` | number box the user must compute |
| Value stepped by arrows | `ui.Stepper`; typed as well `ui.NumberInput` | `TextInput` + parse on submit |
| Stars, a level out of a few | `ui.Rating` | `Row` of star glyphs and an `int` |

### Text

| Need | Use | The tell it retires |
|---|---|---|
| One line, many lines | `ui.TextInput`, `ui.TextArea` | — |
| One text whose spans differ in style | `ui.RichText(c, spans…)` | a `Row` of styled `Text` fragments |
| Search with Escape to clear | `ui.SearchField` | `TextInput` + a clear button |
| Rename in place | `ui.EditableText` | a dialog to change a name |
| Tags, recipients | `ui.TokenField` | comma-split string |
| Find inside a document | `ui.FindBar` | ad-hoc search field plus counts |
| Code or JSON, read-only | `github.com/egoist/mygo/ui/editor` | `TextArea` with monospace |
| Code with carets, folding, completions, diagnostics | `editor.View` + `editor.Buffer` | `TextArea` plus a gutter built by hand |
| The changes between two texts | `editor.DiffView` + `editor.DiffState` (`DiffSplit`/`DiffUnified`) | two text areas and a column of markers |
| Attributed text with formats, links, headings, lists, images | `richtext.NewDocument`, `richtext.ParseMarkdown`/`ParseHTML`/`ParseRTF`, `richtext.Editor` | `TextArea` behind a row of format toggles |
| Markdown, read-only | `ui/markdown.View` | pre-split paragraphs of `Text` |
| Text the app owns: native composition, retained geometry | `ui.TextInputBuffer`, `ui.TextAreaBuffer`, `ui.TextInputClient` | a `Box` wired to `HandleTextInput` by hand |

### Dates and colours

| Need | Use | The tell it retires |
|---|---|---|
| A day of a month | `ui.Calendar` | a `GridView` of numbers and a `time.Time` |
| A date, or a time of day, in a field | `ui.DateInput`, `ui.TimeInput` | three `TextInput`s and a parse |
| A colour, as a swatch or a picker | `ui.ColorWell`, `ui.ColorPicker` | a hex `TextInput` and a preview `Box` |

### Collections

| Need | Use | The tell it retires |
|---|---|---|
| A data table | `ui.Table` + `Sortable`, `Fixed`, `RowHeader`, `CellSelect`, `Columns`, `CellMenu`, `Reorder`, `EditableText`, `Key` | header `Row` with literal widths, rows built in a loop |
| Rows of any height, appending | `ui.List` + `Key`, `FollowEnd`, `Header`, `Reorder`, `DragSelect` | `Scroll` + `Column` + index loops |
| A tree | `ui.Tree`/`TreeItem`; large or columned `ui.Outline`, `ui.OutlineTable` | indented rows with a bool map |
| A grid of items | `ui.GridView` (builds what shows); layout grid `ui.Grid` | `Column` of `Row`s |
| A value dragged within the window, or files dropped on a target | `Element.Draggable`, `ui.DragOver[T]`/`ui.Drop[T]`, `ui.DataDragOver`/`ui.DropData`, `Element.DroppedFiles` | pointer tracking on `InputEvent` plus a bool |

### Containers

| Need | Use | The tell it retires |
|---|---|---|
| Two panes the user resizes | `ui.Split`, `ui.SplitVertical`, `ui.SplitPercent`, `ui.SplitVerticalPercent` | `Width(220)`, `HeightPercent`, a bool drag flag |
| A section that opens and closes | `ui.Collapsible`; several `ui.Accordion`, `ui.AccordionItem` | bool + chevron glyph + conditional children |
| A form | `ui.Form` + `ui.Field` + `.Description`/`.Error` + `Fieldset.Disabled` | fields stacked as `Row`s, error text tinted by hand |
| A scrolling region | `ui.Scroll`, `ui.ScrollBoth`, `ui.ScrollHorizontal` | clipping by fixed height |
| A web page inside the window | `ui.WebView` + `mygo.Window.NewWebView` | a second window that is only a browser |

### Navigation

| Need | Use | The tell it retires |
|---|---|---|
| A few pages as a row of tabs | `ui.Tabs`; parts with `ui.TabsBase` | `Row` of `Button`s and an `int` |
| Source list, with sections and counts | `ui.Sidebar`, `ui.SidebarItem`, `ui.SidebarSection` + `ui.Badge` | two rows of tabs, one per level |
| Pages with history | `ui.Router` + sidebar or tab paths | an `int` section field, no back |
| A path back | `ui.Breadcrumbs` | breadcrumb texts with clicks |

### Dialogs and messages

| Need | Use | The tell it retires |
|---|---|---|
| A question before a rare, irreversible action | `ui.AlertDialog` | acting at once; or a whole page to confirm |
| A custom dialog | `ui.DialogBase`, `ui.Modal`, `ui.Overlay` | absolutely positioned `Box` above everything |
| A panel under an element | `ui.Popover` | absolutely positioned `Box` |
| A tip on rest or focus | `.Tooltip(…)` on the element; **disabled controls show none**, so a reason lives in visible text or `Field.Description` | tooltip as the only explanation |
| Transient outcome of an action | `c.Toast`, `c.ToastAction`, `c.AddToast` | the outcome only in a log line |

### Status, charts, images

| Need | Use | The tell it retires |
|---|---|---|
| Work done, or of unknown length | `ui.Progress`, `ui.Spinner` | nothing, or a stale "running" label |
| A value inside a range, coloured by level | `ui.Meter` + `ui.MeterLevels` | a bare number |
| A short count or label | `ui.Badge` | `Box` + `Text` + radius |
| Several stages on one playhead | `Element.Timeline(key, ui.TimelineOptions)`; a single value `Element.AnimateTo`/`AnimateWith` | percentages recomputed each frame |
| A trend, bars, an area | `ui/chart`: `Plot`, `Axes`, `NewLinear`/`NewBand`/`NewTime`, `Rect`/`Line`/`Area` marks | showing only the current sample |
| An icon | `ui.Icon` over `ui.MustParseSVG`/`ui.ParseSVG` (MyGo draws icons, supplies none) | text glyphs as icons |
| A picture, a person | `ui.Image`, `ui.Avatar` | — |
| A clip that plays, or loops behind content | `ui.Video` + `ui.NewVideoPlayer` (`ui.NewGIFDecoder`) | a timer replacing an `Image` each frame |

### Platform

| Need | Use |
|---|---|
| Theme, accent, density, text scale | `c.SetTheme(&t)` with `Spacing`, `FontSize`, `Radius`, `Accent`; size your own elements with `t.Space(n)`, `t.Rem(n)` |
| The desktop's settings | `c.Preferences()`: accent, high contrast, text scale — the default theme already follows them |
| Motion, enter/exit | `Element.Transition(ui.ElementTransition{Enter: &ui.Motion{…}})`, keyframes, `Element.Spring`; reduced motion is honoured by the widgets |
| A visual move, scale or rotation | `Element.Translate`, `Rotate`, `Scale`, `Skew`, `TransformOrigin` | an absolutely positioned copy that ignores the pointer |
| Pixels painted of your own | `ui.NewCanvas`; painters and shaped text in `docs/ui/drawing.md` |
| The window's options, or a title bar the view draws | `mygo.WindowOptions` (`StateKey`, min size, `TitleBarStyle`), `c.TitleBar()` | fixed window chrome drawn inside the page |
| A layout from the right | `Element.Direction`, `MarginStart`/`MarginEnd`, `PaddingStart`/`PaddingEnd` and the other logical edges | mirrored `Left`/`Right` arithmetic |
| The window's language and regional formats | `c.Locale`, `c.Region`, `Element.Lang`; the widgets' own strings follow them | hardcoded labels and `1/2/2006` |
| Names and roles | `.Label()`, `.Role()`, `.Description()`; announce async results with `c.Announce` |
| See why a box is empty or off by 4px | the window's developer tools (`F12`), `docs/ui/inspector.md` |

The guides behind these rows: `docs/ui/accessibility.md`,
`docs/ui/right-to-left.md`, `docs/ui/styling.md`.

## Lay it out without literal geometry

- Size with the theme: `t.Space(n)`, `t.Rem(n)`, `t.FontSize`, `t.Radius`. A
  literal width is a column that must stay fixed; name it and say why.
- Give a container a face: `Background(t.Surface).Radius(t.Radius).Border(1, t.Border)`.
  A table on the window background reads as unfinished, not minimal.
- A pane the user will want bigger is `Split`, not a fixed size.
- Separate sections with `ui.Divider`, and hold space open with `ui.Spacer`; a fixed-height empty `Box` is the hand-rolled copy.
- Empty state: centred icon, one sentence, and the action that fills it. A
  muted "no data" line over `Grow(1)` reads as a bug.
- Density is a theme decision (`Spacing` 3/4/5), not a per-widget padding.

## Feedback lands where the action is

- The outcome appears beside the control: `c.Toast` for transient, a persistent
  notice block while the condition holds, `AlertDialog` only for a question.
- A log is a record, not a report. Logging and showing nothing on screen is the
  "I clicked it and nothing happened" bug report.
- Success is a message too: silent success and silent failure look the same.

## Verify

Build and test as `../mygo-usage` says, with a `ui.NewTester` test for the
interactions you touched. This skill adds its own checks:

```bash
MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)
grep -rhE '^func [A-Z][A-Za-z0-9_]*[\[(]' --include='*.go' --exclude='*_test.go' "$MYGO/ui" | sed -E 's/func ([A-Za-z0-9_]+)[\[(].*/\1/' | sort -u
```

Drift check: every entry under Actions → Images in `"$MYGO"/docs/ui/README.md`
has a row here or a recorded gap. Then ask what the newest release has, since
this map follows the site and the app may lag it:

```bash
names() { grep -rhE '^func [A-Z][A-Za-z0-9_]*[\[(]' --include='*.go' --exclude='*_test.go' "$1/ui" | sed -E 's/func ([A-Za-z0-9_]+)[\[(].*/\1/' | sort -u; }
V=$(go list -m -versions github.com/egoist/mygo | tr ' ' '\n' | tail -1)
go mod download github.com/egoist/mygo@$V && NEW=$(go env GOMODCACHE)/github.com/egoist/mygo@$V
diff <(ls "$MYGO"/docs/ui) <(ls "$NEW"/docs/ui)   # catalogue pages added since
diff <(names "$MYGO") <(names "$NEW")              # constructors added since
```

A name there that the module go.mod requires lacks is an upgrade, not a
wrapper — step 1 says how. The tester's full surface is
`"$MYGO"/docs/ui/testing.md`, wider than the calls a change usually needs:
`tt.SetLocale`, `tt.SetRegion`, `tt.SetDirection`, `tt.SetDark`, `tt.Advance`
for anything timed.

Audit, pointed at the view packages (`UI_DIR`):

```bash
UI=${UI_DIR:-./internal/ui}
# Literal geometry: each hit is either a real fixed column or an unmigrated default.
grep -rnE '\.(Width|Height|FontSize|Padding|Gap)\([0-9]' "$UI" --include='*.go'
# Capabilities you may be rebuilding by hand.
grep -rnE 'Sortable:|Columns:|CellMenu|HeaderMenu|Reorder:|EditableText|FollowEnd' "$UI" --include='*.go'
# Feedback: toasts present, and how many outcomes only reach a log.
grep -rn 'Toast' "$UI" --include='*.go'
```

Done when: the inventory grep ran against the module version go.mod resolves to;
every UI need has an official widget or a recorded gap; each audit hit is fixed
or explained in the change description; touched interactions have a tester test.
