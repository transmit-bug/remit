# What a headless test cannot prove

Remit's suite drives the real views through `ui.NewTester`, which renders in
memory: it proves text, boxes, focus order, that a click reached a handler, and
that a review wrote a review. It cannot prove that pixels reach a GPU, that a
native dialog round-trips, or that the operating system honours a registration.
This is the list a person runs before calling a build good.

## Rendering

- [ ] `go run ./cmd/remit` opens a window at 1024×720 with no blank frame, and
      the text is crisp on a Retina display.
- [ ] Switching light/dark in the system changes the window live.
- [ ] Reviewing a card animates the advance (~120 ms) and does not flicker.
- [ ] With "Reduce motion" on, the same transition is instant and loses nothing.

## Platform

- [ ] The tray icon appears, shows `Remit · N`, and the number drops after a
      review.
- [ ] The tray menu's Start review brings the window forward in review mode.
- [ ] `Cmd/Ctrl+Shift+R` starts a review from another app. With a combination
      already taken, the log names it and the shortcut stays unbound.
- [ ] Clicking `remit://note/<id>`, `remit://card/<id>`, `remit://queue` opens
      the right place in the running app; an unknown link logs and does nothing.
- [ ] Double-clicking a `.remitlib` in Finder imports it once and opens it; the
      second double-click opens the copy instead of importing again.
- [ ] A second launch (`open -n`, or the binary twice) focuses the first one.

## Native surfaces

- [ ] The macOS app menu's About item opens the About window, and the license
      link opens the browser.
- [ ] An installed build launches without a Gatekeeper prompt (signed and
      notarized), or with the documented warning when unsigned.
- [ ] An agent token created by the app is readable only by its owner, and the
      address file points at a live loopback port.

## Data

- [ ] A library copied by hand (without `.remit/`) opens with its due dates.
- [ ] Editing a note's block updates every card that references it.
- [ ] Deleting the referenced block from the note's file leaves the card lost
      with its due date intact.
- [ ] An `.apkg` from a current Anki release imports without a panic, and the
      report's skipped counts look sane.

Anything that fails here is a bug, not a known limitation; anything that cannot
be run (no second machine, no signing identity) is recorded as unverified rather
than assumed.
