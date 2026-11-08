// An importer plugin: one Obsidian-flavored Markdown file in, notes and
// cards suggested out. The host checks the answer and writes it; the plugin
// never touches the library (docs/specs/plugins/system.md §5).
//
// The format this reads: the first "# " heading is the note's title, every
// "## " heading after it opens one reviewable section, and a section holding
// lines "Q: …" then "A: …" suggests one basic card whose front is the
// section's heading and whose back is the A: line. Everything else is prose.
//
// importFile's answer is suggestions: titles and block texts, cards naming a
// note and a block by index into the same answer, and warnings for what the
// plugin itself found odd. The host mints identities, checks types and
// writes; a suggestion that fails the host's check is dropped, not fatal.

export function describe() {
  return { name: "obsidian-md", version: "0.1.0" };
}

export function importFile(input) {
  const warnings = [];
  const text = new TextDecoder().decode(base64ToBytes(input.data));
  const lines = text.split(/\r?\n/);

  const title =
    (lines.find((line) => line.startsWith("# ")) || "").replace(/^#\s*/, "").trim() ||
    input.name.replace(/\.md$/i, "");

  const blocks = [];
  const cards = [];
  let section = null; // { block: index into blocks, front, back }

  const flush = () => {
    if (!section) return;
    blocks.push(section.front);
    section.block = blocks.length - 1;
    if (section.back !== null) {
      cards.push({
        note: 0,
        block: section.block,
        typeId: "remit.basic",
        ordinal: cards.length,
        fields: { Front: section.front, Back: section.back },
      });
    } else {
      warnings.push("section without an A: line kept as prose: " + section.front);
    }
    section = null;
  };

  for (const line of lines) {
    if (line.startsWith("## ")) {
      flush();
      section = { block: -1, front: line.slice(3).trim(), back: null };
      continue;
    }
    if (section) {
      const answer = line.match(/^A:\s*(.+)$/);
      if (answer) {
        if (section.back !== null) {
          warnings.push("a second A: line in one section was dropped: " + line);
        } else {
          section.back = answer[1].trim();
        }
      }
    }
  }
  flush();

  if (blocks.length === 0) {
    blocks.push(text.trim());
  }
  return { notes: [{ title, blocks }], cards, warnings };
}

function base64ToBytes(b64) {
  const bin = atob(b64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return bytes;
}
