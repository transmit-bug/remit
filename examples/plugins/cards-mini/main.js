// A minimal card plugin: every paragraph asks one question, and the answer is
// whatever comes after the first "::".
//
// Its manifest declares no capability, because it needs nothing but the text it
// is handed. Copy this directory into <user data>/extensions/ to install it.

export function describe() {
  return { name: "cards-mini", version: "0.1.0", protocol: 1 };
}

export function derive(block) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{
    kind: "basic",
    ordinal: 0,
    front: text.slice(0, at).trim(),
    back: text.slice(at + 2).trim(),
  }];
}

export function faces(card) {
  return derive({ text: (card && card.text) || "" })[0] || { front: "", back: "" };
}

export function render(card) {
  return { front: (card && card.front) || "", back: (card && card.back) || "" };
}
