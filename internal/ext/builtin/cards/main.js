// The built-in card plugin: what a paragraph of a note asks, as typed cards —
// which library type a block becomes and the field values it carries. It
// answers JSON only; the host renders the faces with the type's templates.
//
// It carries no capability: it reads the block it is given and answers, like
// any other extension.
//
// The shapes it answers (spec §2.2):
//   - a "{{blank}}" paragraph is one cloze card per blank, ordinal from 0;
//   - ":::" asks both ways: two bidirectional cards with swapped fields;
//   - "::" is one basic card;
//   - a block that opens with "::: type <id>" and closes with ":::" fills
//     that type directly: one "Field: value" line per field.

const CLOZE = "remit.cloze";
const BOTH = "remit.bidirectional";
const BASIC = "remit.basic";

// blanks returns the trimmed content of each {{blank}} in text, in order.
function blanks(text) {
  const found = [];
  const re = /\{\{([\s\S]*?)\}\}/g;
  let match;
  while ((match = re.exec(text)) !== null) found.push(match[1].trim());
  return found;
}

// fieldsBlock answers a typed block: "::: type <id>" opens it, ":::" closes
// it, and every line between is "Field: value". A field the type does not
// carry is reported rather than dropped, so the host can tell the user; the
// card is still made from the fields that do fit.
function fieldsBlock(text, types) {
  const lines = text.split("\n");
  const open = /^:::\s*type\s+(\S+)\s*$/.exec(lines[0].trim());
  if (!open) return null;
  const typeId = open[1];
  const known = {};
  for (const t of types || []) {
    if (t && t.id === typeId) {
      for (const f of t.fields || []) known[f] = true;
    }
  }
  const fields = {};
  const unknown = [];
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i].trim();
    if (line === ":::") break;
    if (line === "") continue;
    const at = line.indexOf(":");
    if (at <= 0) continue;
    const name = line.slice(0, at).trim();
    const value = line.slice(at + 1).trim();
    if (known[name]) fields[name] = value;
    else unknown.push(name);
  }
  return { typeId, ordinal: 0, fields, unknown: unknown.length > 0 ? unknown : undefined };
}

export function describe() {
  return { name: "Standard cards", version: "1.0.0" };
}

// deriveBlock answers the cards a block makes, in the order they are numbered.
export function deriveBlock(block, types) {
  const text = (block && block.text) || "";

  const typed = fieldsBlock(text, types);
  if (typed) return [typed];

  const found = blanks(text);
  if (found.length > 0) {
    return found.map((_, ordinal) => ({
      typeId: CLOZE,
      ordinal,
      fields: { Text: text },
    }));
  }

  const both = text.indexOf(":::");
  if (both >= 0) {
    const front = text.slice(0, both).trim();
    const back = text.slice(both + 3).trim();
    return [
      { typeId: BOTH, ordinal: 0, fields: { Front: front, Back: back } },
      { typeId: BOTH, ordinal: 1, fields: { Front: back, Back: front } },
    ];
  }

  const single = text.indexOf("::");
  if (single >= 0) {
    return [{
      typeId: BASIC,
      ordinal: 0,
      fields: {
        Front: text.slice(0, single).trim(),
        Back: text.slice(single + 2).trim(),
      },
    }];
  }
  return [];
}
