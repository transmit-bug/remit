// A report plugin: what a leech report would be if this app counted lapses,
// which it does not yet — so this example counts the cards a note makes and
// says when a note is close to the threshold the user set.
//
// It is also the contract's fixture: one page (stat + table + callout), two
// commands (one navigates, one toasts), two menu items and two settings, with
// English and Chinese text. `go run ./cmd/remit ext test <dir>` runs its hooks
// against the fixtures in this directory, without a window; copy it into
// <user data>/extensions/ to install it.
//
// The host draws every pixel: `command` answers a tree of layout nodes and
// semantic leaves, never a widget, a colour or a size.

// settings reads the values the host's settings form wrote into this plugin's
// own store document: {settings: {threshold: 8, includeSuspended: false}}.
function settings() {
  try {
    const raw = remit.store.get();
    if (!raw) return {};
    const document = JSON.parse(raw);
    return (document && document.settings) || {};
  } catch (error) {
    return {};
  }
}

// notes lists the library, or nothing when this build gave the plugin no host.
function notes() {
  try {
    return remit.notes.list() || [];
  } catch (error) {
    return [];
  }
}

// cardsIn counts the separators a note's text uses to make cards.
function cardsIn(note) {
  let count = 0;
  for (const block of note.blocks || []) {
    const text = block.text || "";
    for (let i = 0; i < text.length; i++) {
      if (text[i] === ":" && text[i + 1] === ":") count++;
    }
  }
  return count;
}

// leechesPage is the page's data command: the tree the host draws.
function leechesPage() {
  const found = notes();
  const threshold = Number(settings().threshold ?? 8);
  const rows = found
    .map((note) => [note.title || "", String(cardsIn(note))])
    .filter((row) => Number(row[1]) >= Math.min(threshold, 1))
    .slice(0, 100);
  let cards = 0;
  for (const note of found) cards += cardsIn(note);

  const children = [
    {
      kind: "stack",
      direction: "row",
      gap: "m",
      children: [
        { kind: "stat", label: "%stat.notes%", value: String(found.length) },
        { kind: "stat", label: "%stat.cards%", value: String(cards) },
      ],
    },
    {
      kind: "callout",
      tone: "info",
      title: "%callout.title%",
      text: "%callout.body%",
    },
  ];
  children.push(
    rows.length === 0
      ? { kind: "text", text: "%empty%", tone: "neutral" }
      : {
          kind: "table",
          columns: ["%col.note%", "%col.cards%"],
          rows,
        }
  );
  return {
    kind: "page",
    children: [
      { kind: "section", title: "%section.overview%", children },
    ],
  };
}

// command is the one hook a contributed command and a page's data answer.
export function command(input) {
  const name = (input && input.name) || "";
  if (name === "leechesData") return leechesPage();
  if (name === "showLeeches") return { result: "navigate", page: "leeches" };
  if (name === "pauseAndTag") return { result: "toast", message: "%toast.paused%" };
  return { result: "none" };
}

// on_event keeps the page fresh in a real build: the host asks again when the
// library changed (a page never polls).
export function on_event() {}
