// A whiteboard: a page that places the library's paragraphs as a connected
// board, so a reader can walk the notes by their places instead of their titles.
//
// The page names the places and the connections; the host owns every pixel: it
// draws each node with its own button, paints the edges between them, opens the
// paragraph a node stands for, and offers "locate on board" from that
// paragraph's menu. A free canvas would need the pointer, the keyboard and the
// screen reader rebuilt in JavaScript, which is what the contract refuses
// (docs/specs/plugins/ui.md §5, ui-redesign-v2 §6.4).
//
// `go run ./cmd/remit ext test examples/plugins/whiteboard` runs its hooks
// against the fixtures in this directory, without a window.

// The board's own space, where its places sit in it, and the lines between them:
// five spots a reader walks in reading order. A library with fewer paragraphs
// fills fewer spots, and an edge whose ends are missing is left out, so the board
// keeps its shape at any size.
const BoardWidth = 1120;
const BoardHeight = 700;
const Spots = [
  { x: 60, y: 110, w: 250 },
  { x: 560, y: 60, w: 250 },
  { x: 690, y: 300, w: 250 },
  { x: 250, y: 400, w: 250 },
  { x: 480, y: 540, w: 250 },
];
const Edges = [
  [0, 1],
  [0, 2],
  [0, 4],
  [3, 2],
];

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

// places lists the board's places in the order the notes hold them, one per spot
// on the board.
function places() {
  const out = [];
  for (const note of notes()) {
    for (const block of note.blocks || []) {
      if (out.length >= Spots.length) return out;
      const text = (block.text || "").trim();
      if (!text) continue;
      out.push({
        id: "n" + (out.length + 1),
        note: note.id,
        block: block.id,
        title: note.title || "",
        text: firstLine(text),
      });
    }
  }
  return out;
}

// board is the page: one board in its own space, with the places the library has
// and the connections between them. A node names the block it stands for, and the
// words the host draws until it resolves the paragraph against the library it
// holds now.
function board() {
  const found = places();
  const nodes = found.map((place, i) => {
    return {
      id: place.id,
      note: place.note,
      block: place.block,
      x: Spots[i].x,
      y: Spots[i].y,
      w: Spots[i].w,
      title: place.title,
      text: place.text,
    };
  });
  const edges = [];
  for (const [from, to] of Edges) {
    if (from < found.length && to < found.length) edges.push([found[from].id, found[to].id]);
  }
  // A library with no paragraph yet has no board to draw: the page says so in
  // its own words rather than leaving a blank canvas.
  const content =
    found.length === 0
      ? [{ kind: "text", text: "%empty%" }]
      : [
          {
            kind: "board",
            title: "%section.board%",
            width: BoardWidth,
            height: BoardHeight,
            nodes,
            edges,
          },
        ];
  return {
    kind: "page",
    children: [
      {
        kind: "section",
        title: "%section.board%",
        description: "%section.body%",
        children: content,
      },
    ],
  };
}

// command is the one hook a contributed command and a page's data answer.
export function command(input) {
  const name = (input && input.name) || "";
  if (name === "boardData") return board();
  if (name === "about") return { result: "toast", message: "%toast.about%" };
  return { result: "none" };
}
