---
name: mygo-motion
description: Motion in MyGo UI — values, poses, transitions, keyframes, timelines and gestures, and how a test drives them. Use when a change should move — hover or press feedback, rows added, removed, reordered or swiped away, an entrance or exit, numbers or content that change, a spinner or progress loop, drag to move or dismiss, shared-element flights, a choreographed intro, or an animation a test must advance. Static layout is mygo-components; painting pixels is mygo-drawing.
---

# Move it with MyGo

The view says where things are; motion finds the way. Every frame rebuilds
the view and reads animated values; MyGo draws the next frame only while
something moves, and none while the window is out of sight.

Scope: making a change move. Which widget carries the meaning is
`../mygo-components`; painting beyond widgets is `../mygo-drawing`; build
and test commands are `../mygo-usage`.

## Where the rest is

| Material | Reached for |
|---|---|
| `$MYGO/docs/ui/motion.md`, where `MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)` | everything here in depth: gestures, staggering, out-of-sight rules |
| `$MYGO/docs/ui/transitions.md` | layout-change motion, enter/exit, shared elements |
| `$MYGO/docs/ui/transforms.md` | `Translate`/`Scale`/`Rotate`/`Skew`, 3D, `BackfaceHidden`, coordinate rules |
| `$MYGO/examples/timeline`, `$MYGO/examples/gallery` (same `MYGO`) | one playhead driving an intro; the Motion page — springs retargeted and thrown, keyframe sequences, shared elements |

Version protocol at `../mygo-usage`: read what you use in the copy
`go list -m -f '{{.Dir}}'` names.

## Reconcile first

Most motion is already built in; hand-rolling it repeats the hand-rolled
widget mistake:

| The change | Already moves by |
|---|---|
| Element moves, resizes or recolours because layout says so | `Transition(ui.ElementTransition{…})` on each element the change moves |
| Text, icon or image content swaps | `ContentTransition` — fade, icon grow, `Numeric` rolling digits, or `Morph` turning one text into another character by character |
| Rows enter or leave a list | `Enter`/`Exit` `ui.Motion` on the rows |
| Spinner, progress, pressed and hover states | the widget's own — check before adding any |

Done when: every motion in the change names its mechanism — transition,
value, pose, keyframes, timeline or draw — and why none of the rows above
covers it.

## The timing table

Durations live as package-level named values, not inline literals — motion
as spec, tuned in one place. 2048 does this (`slide`, `pop`, `rise`):

```go
var (
	slide  = ui.Animation{Duration: 100 * time.Millisecond, Ease: ui.EaseInOut}
	pop    = []ui.Keyframe{{Value: 0}, {At: 100 * time.Millisecond, Value: 1.2, Ease: ui.EaseInOut}, {At: 200 * time.Millisecond, Value: 1}}
	riseIn = []ui.Keyframe{{Value: 0}, {At: 600 * time.Millisecond, Value: 1, Ease: ui.EaseIn}}
)
```

Springs: presets `ui.Smooth` (no bounce), `ui.Snappy` (a little),
`ui.Bouncy` (visible); describe them as perceived —
`SpringOptions{Duration, Bounce}` — over physical
`Stiffness`/`Damping`/`Mass`. A spring keeps its velocity when its target
changes mid-flight.

Done when: no duration literal appears twice, and every spring is a preset
or a named value.

## Target, not tween

State holds the target; the view reads the value; MyGo draws frames until
the value arrives. Nothing schedules frames by hand:

```go
target := float32(0)
if app.open {
	target = 200
}
card.Translate(card.AnimateTo("x", target, ui.Snappy), 0)
```

The key (`"x"`) is identity: the same key frame to frame keeps the value
and a spring's velocity through a retarget. One key per meaning; typed
keys for per-item values (`c.Key(riseKey(a.rise))`). `Settled(key)` asks
whether a value rests; `Arrived(key)` is true in the one frame it landed —
dismiss from there, never on a timer.

| Need | Mechanism | The rule that bites |
|---|---|---|
| A value drives transform, size, opacity or colour | `AnimateTo(key, target, anim)`; `Animate(key, target, d)` for curves; `AnimateColor` for colours | Reading it runs the view every moving frame |
| Lift on hover, press scale — no relayout | `AnimatePose(ui.Pose{X, Y, Scale, Rotate, Fade}, anim)` | Paint-only frames, the cheapest motion; share one pose among elements that move alike |
| Appears or disappears | `ElementTransition{Enter, Exit: &ui.Motion{…}}`, `Collapse`; `Motion.Blur` blurs the way; `AfterExits` holds one back until its siblings have left | `Enter` plays only in a parent that existed last frame; the exiting element is a copy under its siblings — no pointer, no focus |
| Flies from where another element showed | `Match:` a shared identity of your own type, `Fly:` the Magic Move — corners, opacity, colours and 3D blend on the way; pages: `Router.Transition = TransitionMatch` | One match at a time; the element gone leaves by its own `Exit` |
| Brought into view, gliding | `el.ScrollIntoView(…)` — start, center or end | scroll math off the view's own state |
| Brings itself in as it scrolls | `InView()` → `AnimateTo`; `ScrollState` for scroll-linked | — |
| Timed sequence on one value | `Keyframes(name, kfs, ui.KeyframeOptions{Run: …})` | A `Run` change replays; first time must be zero, times increasing; `Delay`/`Repeat`/`Alternate` |
| Many values, one playhead | `card.Timeline(key, ui.TimelineOptions{Run: …})` + `tl.Tween` | Placements `AfterPrevious`/`WithPrevious`/`AtLabel`/`AtTime`; `Paused`/`Reverse`/`Seek`/`Progress` take effect the frame they are said; `Settled`/`Arrived` chain state |
| Drag, throw, swipe away | `el.Draggable(&offset, ui.DraggableOptions{Axis, Bounds, Snap})` | Remove for real only when `!Transitioning()`; `DragOffset()` follows for backdrops; by hand: `Jump`, `DragEnded`, `Project`, `Rubberband` |
| Brings itself in as it scrolls | `InView()` → `AnimateTo`; `ScrollState` for scroll-linked | — |
| A change must not show as motion | `c.DisableAnimations()` before building it | Restoring state, jumping to a search result |

## Frames and cost

- Frames that run the view (read `AnimateTo`, `Keyframes`, `DragOffset`)
  differ from frames that only paint (`AnimatePose`, `ContentTransition`,
  a `Draw` with `p.AnimationFrame()`). Prefer paint-only on large views.
- Out of sight, nothing draws and the view does not run: springs and
  timelines continue from where the time puts them, and a target changed
  meanwhile animates when the window shows. Compute motion from the clock —
  take `changed time.Time` where the state changes, `at(c.Now())` in the
  view — never from frame counts.
- Polling, saving, jobs: never from the view, `After`, or `Draw` — they
  stall out of sight. Run them from the app; show results with
  `Window.Update`.

## Less motion

`c.Preferences().ReduceMotion`: values, poses, transitions and timelines
snap to their ends; colours and fades still ease; `Loop` keeps spinning,
as the system's spinners do. A fade or blur that must keep moving under
it: `NonMotion`. Motion computed by hand reads the preference too.

## Testing

Protocol and `ui.NewTester` at `../mygo-usage`; the two clocks apply in
full. `tt.Advance(d)` drives animation, keyframe and timeline time; call
`tt.Frame()` after changing state from outside the view;
`tt.FramePending()` is false wherever the view should settle.

Done when: the test advances the clock rather than sleeping, drives the
real gesture (`tt.ClickAt`, `tt.Scroll`) rather than a helper, and settles
— or asserts the loop that intentionally never does.
