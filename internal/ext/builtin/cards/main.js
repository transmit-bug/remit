// The built-in card plugin: what a paragraph of a note asks, in the shapes the
// content model knows — one card for a separator, one per blank of a cloze, both
// directions of a paragraph written both ways. It answers with JSON only: the
// host renders.
//
// It carries no capability: it reads the block it is given and answers, like any
// other extension.

const GAP = "[…]";

// blanks returns the trimmed content of each {{blank}} in text, in order.
function blanks(text) {
  const found = [];
  const re = /\{\{([\s\S]*?)\}\}/g;
  let match;
  while ((match = re.exec(text)) !== null) found.push(match[1].trim());
  return found;
}

// clozePrompt is the paragraph with every blank shown but the one asked.
function clozePrompt(text, ask) {
  let at = 0;
  return text.replace(/\{\{([\s\S]*?)\}\}/g, (_, inner) => {
    const shown = at++ === ask ? GAP : inner.trim();
    return shown;
  }).trim();
}

export function describe() {
  return {
    name: "cards",
    version: "1.0.0",
    protocol: 1,
    produces: ["basic", "cloze", "bidirectional"],
  };
}

// derive answers the cards a block makes, in the order they are numbered.
export function derive(block) {
  const text = (block && block.text) || "";
  const found = blanks(text);
  if (found.length > 0) {
    return found.map((answer, ordinal) => ({
      kind: "cloze",
      ordinal,
      front: clozePrompt(text, ordinal),
      back: answer,
    }));
  }
  const both = text.indexOf(":::");
  if (both >= 0) {
    const front = text.slice(0, both).trim();
    const back = text.slice(both + 3).trim();
    return [
      { kind: "bidirectional", ordinal: 0, front, back },
      { kind: "bidirectional", ordinal: 1, front: back, back: front },
    ];
  }
  const single = text.indexOf("::");
  if (single >= 0) {
    return [{
      kind: "basic",
      ordinal: 0,
      front: text.slice(0, single).trim(),
      back: text.slice(single + 2).trim(),
    }];
  }
  return [];
}
