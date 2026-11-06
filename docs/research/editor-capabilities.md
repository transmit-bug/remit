# MyGo editor capabilities for `remit`

Module: `github.com/egoist/mygo@v0.7.4` at
`/Users/pony/go/pkg/mod/github.com/egoist/mygo@v0.7.4` (pinned in `go.mod:8`).

Citation notation: `pkg/file.go:NN` is relative to the module root.
`V` = verified in source (or by a compiled probe against the module).
`D` = documented only (prose in `docs/`, no source check).

Probes were run out-of-repo in `/tmp/rtprobe` (`require github.com/egoist/mygo
v0.7.4`); nothing in `remit` was modified.

---

## 1. `ui/richtext` contract

### Conclusion

- **`Text`** (`ui/richtext/text.go:28`) — value type: two unexported fields,
  `text *rope.Node` and `paras *seq[richPara]`. Immutable; every method returns
  a new `Text` sharing unchanged structure. `V`
- **`Paragraph`** (`text.go:188`): `Style ParagraphStyle`, `Runs []Run`. No
  newline; the newline's attributes belong to the paragraph. `V`
- **`Run`** (`text.go:181`): `Text string`, `Attributes Attributes`. `V`
- **`Attributes`** (`text.go:37`): `Font string`, `Size float32`, `Weight int`
  (100–900), `Italic`, `Underline`, `Strikethrough bool`, `Color`,
  `Background ui.Color`, `Code bool`, `Link string`, `Tag string`,
  `Attachment *Attachment`. `V`
  - Caveat `V`: the type doc says "They are comparable", but `Attachment` is a
    pointer, so `Text.Equal` returns **false** for two independently parsed
    copies of the same image (probe: `image no url` → `stable=true
    equalText=false`). Do not use `Equal` for dirty-checking documents that
    hold attachments.
- **`ParagraphStyle`** (`text.go:152`): `Heading int` (1–6, 0 body),
  `List ListKind`, `Checked bool`, `Indent int`, `Align ui.Align`,
  `Quote bool`, `CodeBlock bool`, `ID string` (`text.go:177`). `V`
  - `ListKind`: `NoList`, `Bullets`, `Numbers`, `Tasks` (`text.go:139-149`).
    Lists are **flat**: nesting is `Indent`, not a tree. `V`
  - `Indent` is clamped to `maxIndent` and `Heading` to 0–6 by
    `ParagraphStyle.normalized()` (`text.go:114`). `V`
- **`Document`** (`document.go:22`) — all fields unexported (`mu`, `text`,
  `anchor`, `caret`, `typing`, `version`, `selVersion`, `undo`, `redo`,
  `lastEdit`, `coalesce`, `log`, `newID`, `shownIn`, `read`, `boxes`). The API
  is the methods. `V`

### Editor construction

```go
func Editor(c *ui.Context, doc *Document, options ...EditorOptions) ui.Element      // view.go:67
func EditorBase(c *ui.Context, doc *Document, options ...EditorOptions) ui.Element  // view.go:79
```

- `Editor` adds padding/background/border/`MinHeight`; `EditorBase` is the bare
  surface for an app's own frame (`view.go:67-84`). `V`
- The key is set on the `*ui.Context`, not in options:
  `richtext.Editor(c.Key(n.ID), doc, opts)` — `examples/notes/main.go:118`. `V`
- `EditorOptions` (`view.go:18-47`): `Label`, `Placeholder`, `Font ui.Font`,
  `ReadOnly bool`, `NoMarkdown bool`, `OnLink func(string)`,
  `OnLinkCommand func()`, `TagStyle func(tag string, s *ui.Span)`,
  `Theme *Theme`. `V`
- The editor is **not** a callback with text; it writes through the `Document`.

### Getting the text back

```go
doc := richtext.NewDocument(richtext.NewText("Hello"))          // document.go:64
if richtext.Editor(c, doc).Fill().Changed() { app.save(doc.Text()) }  // view.go:57-64
```

- `doc.Text() Text` (`document.go:77`) — the current immutable text. `V`
- `Element.Changed()` — reports **user** edits only, not `doc.*` calls
  (`view.go:63-65`, `examples/notes/main.go:117-121`). `V`
- `doc.Version() uint64` (`document.go:84`) — cheaper change detector. `V`
- `doc.SetText` replaces text and clears history (`document.go:92`);
  `doc.Edit(fn)` (`document.go:372`) for programmatic edits as one undo step. `V`
- Persistence: `Text.MarshalJSON`/`UnmarshalJSON` (`clipboard.go:98,101`) —
  this is the format `examples/notes` saves, and it **keeps `ParagraphStyle.ID`**. `V`

**Verdict: use directly.** No wrapper needed for Q1.

---

## 2. Markdown shortcuts

Default **on**; off with `EditorOptions.NoMarkdown` (`view.go:29-31`). `V`

Block shortcuts fire when a **space** is typed and the paragraph prefix matches
(`blockShortcut`, `edit.go:310-337`): `V`

| Prefix | Result |
|---|---|
| `# ` … `###### ` (1–6 `#`, nothing else) | heading of that level, list cleared |
| `- `, `* `, `+ ` | `Bullets` |
| `[] `, `[ ] `, `-[ ] `, `- [ ] ` | `Tasks`, unchecked |
| `[x] `, `- [x] `, `[X] ` | `Tasks`, checked |
| `> ` | `Quote` |
| `<digits>. ` or `<digits>) ` | `Numbers` |

Inline shortcuts fire on the **closing** delimiter being typed
(`inlineShortcut`, `edit.go:340-405`): `**bold**`, `__bold__`, `*italic*`,
`_italic_`, `` `code` ``, `~~strikethrough~~`. `V` Underscore inside a word
(`snake_case`) is excluded (`edit.go:365-368`). `V`

Enter at the end of a paragraph containing only ` ``` ` starts a code block —
also gated by `NoMarkdown` (`edit.go:243-255`). `V`

`NoMarkdown` does **exactly** two things: it returns early from `typed()`
(`edit.go:286`, the only caller of both shortcut functions, reached from
`input.go:31` and `ime.go:118`) and disables the ` ``` ` code-block conversion
(`edit.go:243`). `V`

`NoMarkdown` does **not** disable: `V`

- list continuation on Enter — a new paragraph inherits the list style through
  `Text.Replace` style rules, and `enter()` (`edit.go:215-283`) never checks
  `NoMarkdown` for that;
- Backspace-unstyling at paragraph start (`edit.go:229-240`);
- pasting Markdown text — `paste` reads `transfer.Markdown` unconditionally
  (`clipboard.go:222`).

**Verdict: use directly.**

---

## 3. Round-trip fidelity (`Text.Markdown` ↔ `ParseMarkdown`)

`Text.Markdown()` (`markdown.go:17`), `ParseMarkdown()` (`markdown_parse.go:17`,
parser `GFM | ExtFootnotes`). Every case below was run through
`parse → Markdown → parse → Markdown`; "stable" means write₂ == write₁. `V`

### Survives a write→parse→write cycle

| Construct | Result | Cite |
|---|---|---|
| Multi-paragraph body | stable, ids by blank line | probe; `markdown.go:22-38` |
| Nested lists (4-space indent per level) | stable | probe `- a\n    - a1\n        - a2` |
| Fenced code, incl. content holding backticks | stable; fence widened to `` ````` `` when needed | `markdown.go:60-72`; probe |
| Inline image (`![alt](url)`) | stable, `Attachment{Name,URL}` | `markdown.go:239-245`; probe |
| Inline link | stable (title dropped) | probe `[t](https://e.com "ti")` → `[t](https://e.com)` |
| Bold/italic/strike as `**`/`*`/`~~`; underline as `<u>` | stable | `markdown.go:255-268`; probe |
| Task lists, quotes, headings, thematic-break-free prose | stable | probe |
| Hard break | stable *as two paragraphs*, not as a break | probe `line one  \nline two` → `line one\n\nline two` |
| Empty paragraphs inside a code block | stable | probe `code block w/ blank line` |

### Documented losses

`Text.Markdown` doc (`markdown.go:16-22`): "alignment, fonts and colors have no
Markdown and are left out". `V`/`D` — silently dropped, no marker left behind.

### Verified losses (probe) — these are the real hazards

- **Tables**: `| a | b |` becomes a paragraph of tab-separated runs, header
  bolded (`markdown_parse.go:78-92`); writes back as `**a\tb**\n\n1\t2`. The
  table is gone. `V` Documented as a parse behaviour only
  (`markdown_parse.go:9-16`), not flagged as a loss.
- **Footnotes**: `[^1]` ref → text `[1]`; `[^1]: the note` → paragraph
  `[1] the note` (`markdown_parse.go:94-101`); writes back escaped as
  `\[1\].` / `\[1\] the note`. Link and definition are unrecoverable. `V`
- **Math**: `$$y^2$$` and `$x_i$` survive as literal text but get escaped
  (`$x\_i$`); `\(z\)` degrades to `(z)` — the delimiters are read as Markdown
  escapes. `V` Documented that math "stays as written"
  (`markdown_parse.go:12-14`) — that claim holds only for `$$…$$`/`$…$`, not
  `\(…\)`.
- **Raw HTML blocks**: `<details><summary>s</summary>body</details>` becomes the
  text `s body` (`markdown_parse.go:56-62` → `ParseHTML`); tags lost
  irreversibly. Only inline `<u> <s> <b> <i> <br>` are honoured
  (`markdown_parse.go:216-258`). `V`
- **Thematic breaks** (`---`): no `ParagraphStyle` for them, so they vanish
  entirely. `V` (probe `thematic break` → `a\n\nb`)
- **Empty paragraphs**: `Markdown` writes them as blank lines, but the parser
  collapses runs of blank lines. `TextOf(a, {}, b)` is 3 paragraphs and
  reparses to 2. `V` (probe `empty in middle`, `two empty`, `empty at end`)
- **A list starting at `Indent > 0`**: written with leading spaces, reread as an
  indented code block. `TextOf({List:Bullets, Indent:2} "deep", {List:Bullets}
  "shallow")` → `        - deep\n- shallow` → first paragraph becomes
  `List: NoList, text "    - deep"`. `V`
- **Paragraph IDs are not in Markdown at all** — by design
  (`text.go:170-176`: "the clipboard, Markdown, HTML and RTF leave it out").
  `V` (probe: `ID:"blk-1"` → `"body"` → id `""`).
- **Ordered-list start number** is renumbered from 1 (`markdown.go:97-100`;
  probe `3. three` → `1. three`). `D`/`V`
- Trailing spaces, CRLF, `## Title ##` closing hashes, setext → ATX, indented
  code → fenced, autolinks → `[url](url)`: normalised but text-stable. `V`

**Verdict: thin wrapper.** The write path is trustworthy for text MyGo itself
produced; the read path silently loses tables, footnotes, HTML blocks, math
delimiters, thematic breaks, empty paragraphs and paragraph IDs. If `remit`
must round-trip third-party Markdown, keep the original source alongside the
`Text` and treat `ParseMarkdown` as a one-way import.

---

## 4. BLOCK ID TEXT SAFETY — a `^abc123` marker at the end of a paragraph

The short answer: **a `^`-prefixed plaintext marker is safe on every path, but
you should not need it** — `richtext` already carries per-paragraph identity in
`ParagraphStyle.ID`.

### (a) Typing — SAFE `V`

Typing goes `input.go:31 → view.replace → Document.Edit → Edit.ReplaceString →
Text.ReplaceString → Text.Replace` (`edit.go:42-73`, `document.go:265`),
which only splices runs (`text.go:445-540`). Marker chars are never inspected.
Probe: `"First paragraph text ^abc123"` → type `X` at offset 0 → marker intact;
insert `" more"` at end → `"... ^abc123 more"`, marker intact.

Splitting / merging — the marker travels with its text: `V` (probe)

- Enter at end of paragraph → marker stays in the **first** half;
- Enter at start → marker stays in the **second** half;
- Enter mid-paragraph → marker stays in whichever half holds it;
- delete the newline between two paragraphs → both markers end up in **one**
  paragraph (two markers in one block — an app problem, not a framework one).

### (b) Markdown shortcuts — SAFE for `^` `V`

- `blockShortcut` only deletes the prefix the user just typed
  (`edit.go:330-336`: `e.ReplaceString(ps, ps+n, "", ...)`, `n = len(prefix)+1`)
  and only when the prefix matches a table entry. `^abc123` matches none
  (`edit.go:311-329`). `V`
- `inlineShortcut` scans backwards for a matching `*`/`_`/`` ` ``/`~~` pair and
  no-ops when none is found (`edit.go:355-372`); a trailing `^abc123` contains
  no marks. `V`
- A marker at the *start* of a paragraph is only at risk if the user types a
  shortcut prefix **before** it — which is the intended behaviour.

### (c) Auto-list continuation — SAFE `V`

`enter()` (`edit.go:215-283`) inserts `\n` and then only calls
`FormatParagraphs`, which touches `ParagraphStyle` and never text
(`text.go:640-660`). The ` ``` ` branch does `e.ReplaceString(ps, pe, "",
...)` but only when the *whole* paragraph line is a lone fence
(`edit.go:243-255`). The empty-item "exit the list" branch requires an **empty**
paragraph (`edit.go:228`), so a marker-bearing paragraph never hits it.

### (d) Paste — SAFE for the marker, but two caveats `V`

`paste` (`clipboard.go:208-259`) picks the richest available format: own
`richtext.Format` JSON → HTML → RTF → `transfer.Markdown` → plain text. Plain
text goes through `v.insert(s)` (`clipboard.go:231,238`), i.e. `ReplaceString`
with typing attributes — marker bytes preserved. Rich paste goes through
`e.Replace(a, z, t)` (`clipboard.go:247`), which splices the target paragraph's
runs around the pasted `Text` (`text.go:445`). Marker preserved.
Caveats: (1) pasting **Markdown** text re-parses it, so a paste containing
`[^abc123]` would be rewritten; (2) copy-then-paste inside the document
duplicates the marker — two blocks with the same marker in one paragraph.

### (e) `ParseMarkdown` — SAFE for `^`, with escapes `V`

Probe results:

```
in="para ends with ^abc123\n"        out="para ends with ^abc123"        same=true
in="^abc123 is the whole paragraph"  out="^abc123 is the whole paragraph" same=true
in="para ends with [^abc123]"        out="para ends with \[^abc123\]"    same=true
in="a ^x1 ^y_2 ^z-3 ^q.4"           out="a ^x1 ^y\_2 ^z-3 ^q.4"         same=true
```

`escapeMarkdown` escapes `\ * _ ` [ ] < ~` and `&` (`markdown.go:381-398`);
it does **not** escape `^`, `$`, `>`, `#`, `|`. So `^abc123` is written
verbatim and read back verbatim. `escapeBlockStart` (`markdown.go:400-417`)
covers `# > - + =` and digit+`.`/`)` — not `^`. `V`

Do **not** bracket the marker: `[^abc123]` is the GFM footnote-reference syntax
(`parser/document.go:23`), and although the text survives it comes back
backslash-escaped and is semantically a footnote reference.

### The better tool: `ParagraphStyle.ID` `V`

`text.go:167-177` + doc `docs/ui/rich-text-editor.md` ("A paragraph's `ID` is
the app's name for it"). Probe on a paragraph with `ID:"b1"`:

| Operation | Resulting IDs |
|---|---|
| split at end | `["b1",""]` |
| split at start | `["","b1"]` |
| split in middle | `["b1",""]` |
| join a/b (`b1`,`b2`) | `["b1"]` |
| format bold / type inside | `["b1"]` |
| `MarshalJSON`→`UnmarshalJSON` | `b1` preserved |
| `Markdown`→`ParseMarkdown` | `""` (lost) |

Two gaps to plan around: new paragraphs get **no** ID unless you call
`doc.IdentifyParagraphs(func() string {...})` (`document.go:408-430`), and IDs
do not survive Markdown. `IdentifyParagraphs` runs `newID` with the document
locked and must not call document methods (`document.go:412-415`).

**Verdict: use directly** — with `ParagraphStyle.ID` + `IdentifyParagraphs`, not
an inline text marker. Keep the marker only as a fallback for text pasted in
from outside (and prefer `^`, never `[^…]`, never `_`).

---

## 5. Gutter / per-paragraph decoration

- `richtext.Theme` (`theme.go:14-68`) has **no** gutter, margin, badge or
  per-paragraph decorate hook. Its only paragraph-level knobs are
  `FirstLineIndent`, `ParagraphSpacing`, `Indent`, `Marker`, `QuoteBar`,
  `Headings`, code-block colours. `V`
- `EditorOptions` (`view.go:18-47`) has no draw/decorate callback. The only
  app-owned painter in the package is `Attachment.Draw func(p *ui.Painter, r
  ui.Rect)` (`text.go:96-98`), which paints **inside** the line for a U+FFFC
  character — an inline chip, not a gutter. `V`
- There is a first-class geometry API: **`Document.VisibleParagraphs()
  []ParagraphBox`** (`view.go:844`), `ParagraphBox{Index, Start int; Top,
  Height float32}` (`view.go:822-831`). `Top`/`Height` are DIPs from the top of
  the editor element, scroll applied, with heading margins and paragraph
  spacing **excluded** so a mark at `Top` aligns with the first line
  (`view.go:826-830`). It returns `nil` until an editor has shown the document
  (`view.go:838-842`). When the boxes move, the editor invalidates itself so
  the app rebuilds and re-reads (`view.go:872-889`). `V`
- The prescribed pattern is exactly what the guide and its test show: a
  `ui.Row` with a fixed-width `ui.Box().Clip().Draw(...)` gutter beside
  `EditorBase`, marking `r.Y + b.Top` with height `b.Height`
  (`docs/ui/rich-text-editor.md:281-311`, test `ui/richtext/paragraph_test.go:548-590`). `D`/`V`
- Mapping a paragraph to a block: `Text.ParagraphRange(rune)` (`text.go:380`),
  `Text.ParagraphStyleAt(rune)` (`text.go:407`), or
  `Document.Selection()` → `paraAt`-equivalent via `ParagraphRange`
  (`document.go:106`). To scroll to a paragraph, `Document.SetSelection(start,
  start)` — the editor scrolls the caret into view (`document.go:112-113`). `V`

**Verdict: thin wrapper.** A gutter is a sibling column that reads
`VisibleParagraphs()` once per frame and paints badges; it is not inside the
editor, so a badge can never overlap the text, and the app must render the
editor's border/background itself (`EditorBase`) to keep the gutter visually
inside the card.

---

## 6. `ui/markdown`

`func View(c *ui.Context, src string, o *Options) ui.Element` (`view.go:98`) and
`func Stream(c *ui.Context, src string, o *Options) ui.Element` (`view.go:115`).
Both take the **source string** each frame and diff it against the last
(`view.go:96-114`). `V`

### What they render `V` / `D`

| Feature | Supported | Cite |
|---|---|---|
| Tables | yes (GFM) | `parser/document.go:15,35`; `ui/markdown/table.go` |
| Task lists | yes | `document.go:17` |
| Strikethrough | yes | `document.go:19` |
| Autolinks | yes | `document.go:21` |
| Footnotes | yes (`ExtFootnotes`, in `Default`) | `document.go:23,38` |
| Math | yes, `$$…$$`, `\[…\]`, `\(…\)` (`ExtMath`, in `Default`); `$…$` only with `ExtDollarMath` | `document.go:25-32,38` |
| Images | yes, as descriptions until `Options.LoadImage` supplies them; `FileImages(dir)` | `view.go:40-43`; `image.go` |
| Links | yes: http/https/mailto via `ui.SafeToOpen`, scheme-less paths to the router, `#slug` scrolls to a heading, other schemes inert; `Options.OnLink` overrides | `view.go:20-28` |
| Raw HTML blocks | shown as text unless `Options.HTML`; inline `<b> <i> <u> <s> <code> <kbd> <mark> <sub> <sup> <br>` apply | `docs/ui/markdown.md:222-224` `D` |
| Code | highlighted with language + Copy button, `Options.CodeBlock` / `Code` / `WrapCode` | `view.go:45-52` |
| `Options.Block` | asked first for **each** block; builds it when it returns true | `view.go:68` |

### Anchors and highlighting

- Anchors come **only from headings**, derived by `Slug(text)`
  (`view.go:526-538`: lowercase, spaces→hyphens, punctuation dropped). `V`
- Scrolling is `Options.Anchor string` + `Options.AnchorVersion uint64`
  (`view.go:30-37`), resolved through `st.headingNamed`
  (`view.go:191-194, 517-524`), which matches `parser.Heading` blocks only. `V`
- **No way to attach an ID to a non-heading block**, and **no built-in
  highlight**. `V` — verified: `headingNamed` filters on
  `b.Kind == parser.Heading`.
- Closest supported approach for "scroll to and highlight a paragraph": use
  `Options.Block` (`view.go:68`) and intercept the target block yourself. You
  can locate it because `parser.Block` exposes `Start, End int` byte offsets in
  the source (`parser/document.go:106-114`), so the app can map its own block
  ranges onto parser blocks. There is **no exported "default block"** to
  delegate to (only `DefaultCodeBlock`, `code.go:87`), so a custom `Block` must
  rebuild that block's element by hand. `V`
- Alternative, no interception: normalise the target paragraph to a heading in
  the rendered copy so `Anchor` works — but that mutates the document.

### Theme

- `Options.Theme *Theme` (`view.go:19-20`); nil means `DefaultTheme` derived
  from the **window theme**, and zero fields inherit from it, so dark mode is
  followed automatically (`docs/ui/markdown.md:226-244`). `D`
- No separate theme is needed. `V`/`D`

**Verdict: use directly for rendering; thin wrapper for anchor + highlight.**
A wrapper that owns block ranges (paragraph → byte span in the source it feeds
to `View`) plus an `Options.Block` interceptor is required for per-block
navigation and highlighting.

---

## 7. Math

- `richtext` does **not** render math. `ParseMarkdown`'s doc:
  "Math, which the editor does not show, stays as written"
  (`markdown_parse.go:12-14`); `docs/ui/rich-text-editor.md:411`. `ParagraphStyle`
  (`text.go:152-178`) has no math field; `Attributes` (`text.go:37-75`) has
  none either. Math arrives as ordinary runs of text. `V`
- `ui/markdown` supports `$$…$$`, `\[…\]`, `\(…\)` by default, `$…$` with
  `parser.ExtDollarMath` (`parser/document.go:25-32`, `Default` at `:38`). `V`
  Rendering: TeX in the code font by default
  (`docs/ui/markdown.md:194-200`); `Options.Math` for display blocks and
  `Options.InlineMath` returning an `InlineObject{Width, Height, Depth, Draw}`
  for real typesetting (`view.go:47-61`). `V`

### Degradation options for an app

1. **TeX source in prose** (default everywhere): display `$…$`/`$$…$$` as
   literal text in richtext, styled code. Zero work, no fidelity risk, no
   typesetting. `V`
2. **Inline chip in the editor**: represent a formula as an `Attachment`
   (U+FFFC) with `Draw func(p *ui.Painter, r ui.Rect)` (`text.go:81-108`) — the
   one hook that lets the app paint arbitrary content inside a line, and it
   survives copy/delete-as-one-character. It does **not** survive Markdown
   (written as `![Name](URL)`, `markdown.go:239-245`). `V`
3. **Read-only render via `ui/markdown`**: show a formula block by running the
   paragraph's text through `markdown.View` with `Options.Math`/
   `InlineMath` set, keeping `richtext` for editing. Two sources of truth to
   reconcile. `V` for the APIs; the composition is the app's.
4. **Strip delimiters** at import and store TeX in `ParagraphStyle.ID`-adjacent
   metadata (or in the paragraph's own JSON), accepting that `\(…\)` is already
   mangled by `ParseMarkdown` to `(…)`. `V`
5. **Do not round-trip `\(…\)`** through `ParseMarkdown` — verified loss
   (§3). `V`

**Verdict: not available** in `richtext` (no math rendering at all);
**use directly** in `ui/markdown` for read-only display.

---

## 8. Reusable widgets for the surrounding page

| Need | Widget | Signature | Fits? |
|---|---|---|---|
| Outline panel | `ui.Outline` | `Outline[K comparable](c *Context, s *OutlineState[K], roots []K, children func(K) []K, row func(item K)) Element` (`api_gen.go:1479`); `OutlineTable` (`:1503`); state `OutlineState[K]{List ListState; Open Selection[K]; Indent float32}` (`outline.go:19-31`) | **Yes.** Virtualised (builds only rows in view, `outline.go:104-108`); `Rows()/Item()/Depth()` read back the visible model (`outline.go:52-58`); keyboard Right/Left with Option/Shift for open-all (`outline.go:99-103`). Note `UI` rows are a **tree of documents/sections**, and it keys on `K`, so the app owns the hierarchy. |
| In-note search | `ui.FindBar` | `FindBar(c *Context, open *bool, query *string, matches int, current *int) Element` (`api_gen.go:1223`) | **Display only.** It shows the count and "3 of 12" and handles Enter/⇧Enter/⌘G/Escape (`feedback.go:231-243`, `docs/ui/find-bar.md:18-28`). The app computes matches and decides what "current" highlights. `richtext` has **no** find API (grep: no `Find`/`Search` in `ui/richtext`) — you must scan `Text.String()` yourself and map offsets back with `ParagraphRange`. |
| Quick filter for a list | `ui.SearchField` | `SearchField(c *Context, query *string) Element` (`api_gen.go:1803`) | **Yes**, for filtering the outline or a backlinks list; has a clear button and `Submitted()`/`Changed()`. |
| Backlinks panel | `ui.List` | `List(c *Context, s *ListState, n int, row ...func(int)) Element` (`list_builder.go:22`); `ListState` (`list.go:38`): `Key func(row int) any`, `KeepOffset bool`, `FollowEnd bool`, `FollowAnimation *Animation`, `Selected *int`, `Selection Selector`, `MenuKeepsSelection bool`, `Label func(int) string`, `Header func(int) bool` | **Yes.** Virtualised; `Key` keeps row identity while the list reorders; variable row heights measured lazily (`docs/ui/list.md:4-10`). |
| Draggable split | `ui.Split` | `Split(c *Context, size *float32, first func(), second func()) Element` (`api_gen.go:2029`); `SplitPercent(c, p *float32, ...)` (`:2045`) | **Yes.** Divider is user-draggable and arrow-movable, keeps 40 DIPs at each edge, restores the chosen size (`docs/ui/split.md:4-16`); `Changed()` reports a move. RTL aware. |
| Draggable split (stacked) | `ui.SplitVertical` | `SplitVertical(c *Context, size *float32, first func(), second func()) Element` (`api_gen.go:2057`); `SplitVerticalPercent` (`:2069`) | **Yes**, for a bottom panel above/below the note. |

**Verdict: use directly** for outline, backlinks, quick filter and both splits.
**Thin wrapper** for find: `FindBar` is the chrome, and the app supplies the
match engine over `richtext`.

---

## 9. Performance — "Large documents"

`docs/ui/rich-text-editor.md:387-395` is the whole section. It states, `D`
(corroborated by `V` reading of the layout code):

- The text is "a rope of chunks with a persistent sequence of paragraphs beside
  it", so an edit copies what it touches and the path to it: **O(log n)** per
  edit.
- The editor "lays out only the paragraphs in view, keeps the layouts of those
  near it, and estimates the heights of the others" — confirmed in code:
  `forget()` drops layouts beyond `maxLaid/4` from the viewport
  (`view.go:892-903`), `VisibleParagraphs` lays out only what is in view
  (`view.go:838-841`).
- The one measured claim: "Typing a character in a document of **100,000
  formatted paragraphs**, and drawing the frame on the CPU, takes about as long
  as in one of 1,000" (`BenchmarkTypeFrame` in `ui/richtext`). `D`

### Verified by running the benchmark `V`

`ui/richtext/bench_test.go:40` is real and parameterised at
`{1000, 10000, 100000}` paragraphs (`bench_test.go:35`), each paragraph
carrying bold/italic/link runs, every 50th a heading and every 7th a bullet
(`bench_test.go:12-33`). Run read-only in the module cache on an Apple M4
(`CGO_ENABLED=0 GOMAXPROCS=1`, headless `ui.NewTester` 1000x800, one typed
character plus its frame):

| Paragraphs | ns/op | B/op | allocs/op |
|---:|---:|---:|---:|
| 1,000 | 633,777 | 275,320 | 1,662 |
| 10,000 | 644,263 | 298,714 | 1,724 |
| 100,000 | 623,954 | 298,131 | 1,778 |

Flat across a 100x size range, confirming the documented claim (~0.62 ms and
~0.3 MB per typed frame, essentially all of it frame drawing rather than
document work). Note the machine is an M4, not the doc's M5, and the absolute
figure is a headless CPU frame, not a windowed GPU frame.

**No thresholds, no memory numbers, no prescribed strategy beyond keeping the
document size.** The section prescribes nothing like "paginate above N KB" and
gives no per-frame budget. Contrast `docs/ui/markdown.md:315-330`, which *does*
prescribe one: "a document much longer than a chat's reply is best shown a
section at a time" (a 50 KB reply costs 1.3 ms and 350 allocations per frame
when shown whole, 0.3 ms to parse).

Related context: `docs/ui/performance.md:8-18` tabulates steady-frame costs
(1,000 labels 397 µs; visible rows of a million-row list 14.5 µs; 50 KB
Markdown 1.3 ms) and `:24-26` gives ~2.50 MB retained Go heap for 1,000 labels.
`BenchmarkTypeFrame` itself is **not** in the published table — the 100,000-
paragraph claim is documented only.

**Verdict: use directly.** The editor is built for large documents; no batching
or virtualisation layer is needed. But note the documented 100k-paragraph result
is a single unreproduced claim, and the only *prescribed* strategy for huge
documents lives in the Markdown doc (show a slice at a time), not the richtext
doc.

---

## Verdict table

| # | Question | Verdict |
|---|---|---|
| 1 | `richtext` contract, editor construction, reading text back | **use directly** |
| 2 | Markdown shortcuts / `NoMarkdown` | **use directly** |
| 3 | Round-trip fidelity | **thin wrapper** (keep source alongside `Text`) |
| 4 | Block ID text safety | **use directly** — via `ParagraphStyle.ID`, not a text marker |
| 5 | Gutter / per-paragraph decoration | **thin wrapper** (`VisibleParagraphs` + sibling gutter) |
| 6 | `ui/markdown` render / anchors / theme | **use directly** (render), **thin wrapper** (anchor + highlight) |
| 7 | Math | **not available** in `richtext`; **use directly** in `ui/markdown` (read-only) |
| 8 | Outline / FindBar / SearchField / List / Split | **use directly**; **thin wrapper** for find matching |
| 9 | Performance | **use directly** (measured flat to 100k paragraphs) |

---

## Unanswered

1. ~~Exact per-frame cost at 100,000 paragraphs~~ — resolved: benchmark run,
   see §9. What remains open is why `BenchmarkTypeFrame` is absent from the
   published table in `docs/ui/performance.md`, which lists only label, list,
   diff and Markdown fixtures.
2. **Memory per paragraph** in `richtext` (retained heap as a function of
   paragraph count). No figure in any doc; not measured. The benchmark's
   `B/op` is per typed frame, not retention.
3. **`NoMarkdown`'s effect on multi-character IME input.** `typed()` bails when
   the committed string is not exactly one rune (`edit.go:286`), so shortcut
   triggering on IME input is limited regardless of `NoMarkdown`. Whether that
   is intentional for CJK input is not documented.
4. **Whether `Options.Block` receives blocks in a stable order/identity across
   frames.** `view.go:68` gives `(c, d, i)`; `i` is a `parser.Document` block
   index, and the doc notes parsers reparse only touched blocks — but no doc
   states that block indices are stable across an edit, so a highlight bound to
   index `i` may drift. Not verified.
5. **`Text.Equal` semantics with attachments** — source reads as pointer
   comparison through `slices.Equal` on `attrRun`; probes show `false` for equal
   images. Not documented; treat the doc's "comparable" claim as applying to
   `Attributes` without `Attachment`.
6. `maxIndent = 8` (`commands.go:286`) — resolved; `Indent` is clamped to 0…8,
   so a deeper hierarchy cannot be expressed in a flat list.
7. **`ui.Split` minimum sizes** — the doc says the divider keeps 40 DIPs from
   either edge; whether that floor is configurable was not checked.
