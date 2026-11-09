// A whiteboard: a page that places the library's paragraphs on a board, one node
// each, so a reader can walk the notes by their places instead of their titles.
//
// The plugin names places and nothing else — no coordinate, no colour, no node
// kind of its own. Each card carries the note's title as text, the paragraph's
// first line as markdown, and one action whose value is "<note>/<block>": the
// host reads that value, draws the node with its own widgets, opens the
// paragraph when the node is pressed, and offers "locate on board" from that
// paragraph's menu. A free canvas would need the pointer, the keyboard and the
// screen reader rebuilt in JavaScript, which is what the contract refuses
// (docs/specs/plugins/ui.md §5, ui-redesign-v2 §6.4).
//
// `go run ./cmd/remit ext test examples/plugins/whiteboard` runs its hooks
// against the fixtures in this directory, without a window.

// The board is one page of nodes: a cap, so a library of thousands of paragraphs
// still answers inside the contract's limits (500 nodes, 256 KiB).
const Limit = 24;

// notes lists the library, or nothing when this build gave the plugin no host.
function notes() {
  try {
    return remit.notes.list() || [];
  } catch (error) {
    return [];
  }
}

// firstLine is the paragraph as a node shows it: up to its first line break or
// its card separator, whichever comes first.
function firstLine(text) {
  let cut = text.length;
  const nl = text.indexOf("\n");
  if (nl >= 0 && nl < cut) cut = nl;
  const sep = text.indexOf("::");
  if (sep >= 0 && sep < cut) cut = sep;
  return text.slice(0, cut).trim();
}

// places lists the board's nodes in the order the notes hold them: the note and
// the block each one stands for, and the words the host draws while it resolves
// the paragraph against the library it holds now.
function places() {
  const out = [];
  for (const note of notes()) {
    for (const block of note.blocks || []) {
      const text = (block.text || "").trim();
      if (!text) continue;
      if (out.length >= Limit) return out;
      out.push({
        note: note.id,
        block: block.id,
        title: note.title || "",
        text: firstLine(text),
      });
    }
  }
  return out;
}

// node is one place as a card: the note's title, the paragraph's first line, and
// the action the host reads the place from.
function node(place) {
  return {
    kind: "card",
    children: [
      { kind: "text", text: place.title },
      { kind: "markdown", text: place.text },
      {
        kind: "actions",
        items: [
          {
            label: "%action.open%",
            value: place.note + "/" + place.block,
            command: "example.whiteboard.about",
          },
        ],
      },
    ],
  };
}

// boardPage is the page's data command: the tree the host draws.
function boardPage() {
  const found = places();
  const children =
    found.length === 0
      ? [{ kind: "text", text: "%empty%" }]
      : [{ kind: "grid", columns: 2, children: found.map(node) }];
  return {
    kind: "page",
    children: [
      {
        kind: "section",
        title: "%section.board%",
        description: "%section.body%",
        children,
      },
    ],
  };
}

// command is the one hook a contributed command and a page's data answer.
export function command(input) {
  const name = (input && input.name) || "";
  if (name === "boardData") return boardPage();
  if (name === "about") return { result: "toast", message: "%toast.about%" };
  return { result: "none" };
}
