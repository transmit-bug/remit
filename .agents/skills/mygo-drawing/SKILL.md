---
name: mygo-drawing
description: Custom painting in MyGo UI — the Draw/DrawOver painter, paths, gradients, sprite regions, offscreen canvases, shader effects and shaped text, with the cost rules that keep drawing cheap. Use when pixels go beyond what widgets, ui/chart and SVG icons compose — a custom control's face, a waveform or instrument no chart covers, sprite sheets or contact sheets, glow, blur or distortion effects, a panning and zooming map or diagram, text laid out cell by cell or paged, or a frame-cost pass. Widget choice is mygo-components; animation is mygo-motion.
---

# Draw it yourself

`Draw` paints on an element after its background, `DrawOver` after its
children, with a `*ui.Painter` in the window's DIPs. One rule governs all
of it: **draw-only** — MyGo may call a `Draw` more than once a frame, so a
`Draw` only paints: it writes no state, starts no work, asks for no
`Window.Update`.

Scope: pixels beyond widgets. Which widget carries the meaning is
`../mygo-components`; animating values is `../mygo-motion`; build and test
commands are `../mygo-usage`.

## Where the rest is

| Material | Reached for |
|---|---|
| `$MYGO/docs/ui/drawing.md`, where `MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)` | the painter, paths, bitmaps, canvases, shaped text, draw-driven animation |
| `$MYGO/docs/ui/shaders.md` | Go-syntax shaders, element effects, the backdrop |
| `$MYGO/docs/ui/rendering.md` | GPU/CPU backends, `MYGO_GPU`, `MYGO_FRAME_STATS` |
| `$MYGO/docs/ui/performance.md` | steady-frame costs and benchmark fixtures |
| `$MYGO/docs/ui/text-input-client.md` | `ShapeText`/`ShapeRichText` retained geometry |
| `$MYGO/examples/chart-streaming`, `piano`, `effort-slider`, `blackhole` (same `MYGO`) | the tiers live: live data, a custom control, a shader pass |

Version protocol at `../mygo-usage`: read what you use in the copy
`go list -m -f '{{.Dir}}'` names.

## Reconcile in pixels

Name the tier before writing a `Draw` — cheapest first:

| Tier | It is | The tell it retires |
|---|---|---|
| 0 | A widget, `ui/chart`, or an SVG icon covers it | the hand-rolled status dot again, in painter form |
| 1 | A control's face | the control stays `ButtonBase`/`Box` with handlers; `Draw` paints only its face (piano keys, the effort-slider track) |
| 2 | Data visualisation no chart covers | live waveforms painted in `Area.Draw` while the data streams |
| 3 | Text laid out yourself | grid cells from `Shape` + `Glyphs`; book pages from `ShapeText`/`ShapeRichText` → `TextLayout` |
| 4 | Effects and art | `shader.MustCompile` passes over an element or the window |

2048 is the tier-0 proof: a whole game — board, tiles, swipe input, pop
animations — with no `Draw` at all: a `Box` grid, keyframes and
`HandleInput`.

Done when: the change names its tier, and tier 0 is ruled out by naming
the widget or chart that lost.

## The painter

| Need | Call |
|---|---|
| Rectangles, borders, shadows | `p.Fill`, `p.FillGradient`, `p.Stroke`, `p.Shadow`, `p.InsetShadow` |
| Lines and shapes | `p.Line`, `p.FillPath`, `p.StrokePath`, and the gradient variants |
| A path drawn in | `path.Trim(from, to)` — animate `to` from 0 to 1, as SwiftUI's `trim` |
| A path from SVG path data | `ui.ParsePath` / `ui.MustParsePath` |
| A ring with its hole; a point inside a shape | `Path.FillRule(ui.EvenOdd)`; `Path.Contains(pt)` |
| The pointer taken only inside a shape | `Element.HitTest(fn)` — a round button hits inside its circle |
| Stroke details | `Path.Cap` (`ButtCap`/`SquareCap`), `Path.Join` (+`MiterLimit`), `Path.Dash` |
| Images and sprites | `p.Image`, `p.ImageRegion(bitmap, src, dst, ui.ImageOptions{Tint, FlipX, …})` — a region outside the view draws nothing, so sheets need no own culling |
| Text | `p.Text`, `p.RichText`, after `p.MeasureText` to centre; `p.Glyphs` for runes placed by hand; `p.TextLayout` for a shaped layout |
| Focus | `p.FocusRing` — for widgets that draw their own |
| Icons | `p.Icon(svg, rect, tint)` |

## The cost rules

- Draw-only. Reads may change between the calls of one frame; the state a
  `Draw` sees is the view's last run — change it elsewhere and call
  `Window.Update`.
- An element out of view is not painted and asks for no frames.
- Paint-only frames come from the `Draw`: `p.AnimationFrame()` every frame
  that moves, `p.After(d)` for changes in steps,
  `p.AnimationFrameRate(fps)` when fewer frames look the same (a level
  meter, a slow wave) — the window draws at the fastest rate anything asks.
  Pacing with `p.After` alone draws unevenly.
- Out of sight, nothing draws: compute from `p.Now()`, never from frame
  counts, and a start time belongs where the state changes.
- `Loop(key, period, ease)` returns a restarting progress for spinners and
  pulses.

## Canvases

`ui.NewCanvas(w, h)` is a bitmap painted on the GPU: draw it once inside a
`Draw` (`grid.Draw(p, fn)`), then blit `grid.Bitmap()` every frame for the
price of an image. A canvas's pixels are its units — draw at
`Painter.Scale` for window sharpness, or smaller with `Pixelated` for
pixel art. A tint or opacity on a canvas colours or fades a whole layer at
once.

## Pan and zoom

`p.Transform(ui.Translation(-x, -y).Then(ui.Scaling(z, z)), ox, oy, fn)`
draws a map or diagram's content through one view transform; clips made
inside turn with it. Input is not transformed: the pointer arrives in
element coordinates — map to content with the inverse.

## Shaders

`shader.MustCompile` compiles a Go-syntax shader (`Fragment(dst vec4, pos
vec2) vec4`, a `Time` uniform fed from `p.Now()`) and draws on Metal,
Direct3D, OpenGL, WebGL 2, and the CPU where none runs. A shader also
fills shapes in place of a colour: `waves.FillPath`, `waves.Text`,
`waves.Fill`. `Element.Effect` (entry points `ColorEffect`,
`DistortionEffect`, `LayerEffect`, with `MaxSampleOffset` for readers
outside the box) renders the element to a bitmap and draws the effect over
it; clicks, focus and accessibility stay where the element is.

## Verify

Build and test as `../mygo-usage` says; `tt.Image()` catches drawing that
parses but never renders, and a minimum-size pass catches a drawing that
only looks right at one size. `MYGO_FRAME_STATS=1` logs every frame over
8 ms with its build/layout/paint/present split.

Done when: every `Draw` is draw-only (no state writes, no work started),
the tier is named with tier 0 ruled out, frames are asked only while
something moves, and the change's own interaction stays inside its frame
budget.
