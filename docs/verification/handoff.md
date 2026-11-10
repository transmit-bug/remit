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
- [ ] The tray menu's Start review brings the window forward in review mode;
      Pause reminders flips the checkbox and survives a restart.
- [ ] The default hotkey starts a review from another app and focuses the
      window; changing it in Settings re-binds it; a taken combination shows an
      inline error and the old binding keeps working.
- [ ] With notifications on, a due card produces at most one reminder a day;
      pausing produces none.
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
- [ ] Settings start and stop the agent API live; the token can be rotated and
      the address file points at a live loopback port.
- [ ] Check now says what it found, and with the setting off no request leaves
      the machine.

## Plugins

- [ ] Copying `examples/plugins/leeches` into `<user data>/extensions/` makes it
      appear in Settings with its two capabilities; approving it and reopening
      the library puts its page and its commands in the window.
- [ ] A plugin page reshapes at 720×520 without horizontal scrolling, and its
      table reads correctly with a screen reader.
- [ ] `go run ./cmd/remit ext test <a plugin directory outside the repo>` passes,
      and a deliberately changed hook answer fails naming the fixture.

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

## The redesign's surfaces

The five checks `docs/specs/ui-redesign-v2.md` §9.4 adds on top of the list above
(the first five of the list in `docs/verification/ui-acceptance.md`, 全局验收 · 人工交接,
whose items 6-8 are the appearance, accessibility and `manual` passes):

- [ ] Rail fold: drag the window to 960 wide — the rail is the 64 icon strip, an
      icon is clickable and names itself in its tooltip, the counts still show;
      drag back to 1200 and the labels return. At 980 and below neither fold
      control is drawn, since neither can change anything there.
- [ ] Whiteboard round trip: a node opens read mode at its own paragraph; the
      paragraph's block menu goes back to the board and the node is highlighted.
- [ ] Plugin failure: break the page data of `examples/plugins/leeches`, and the
      page says why and offers Retry; three failures take its rail item away
      while Settings still lists it with the reason, and Re-enable brings the
      contribution back.
- [ ] Archive and dismiss: A/D in a reading session, then the archive page
      restores the note to Today; the state survives a restart.
- [ ] Presets: change the retention, close the window, reopen — the value is
      still there; add an unknown key to `remit.json` by hand, reopen, and it has
      not been dropped.
