// A minimal card plugin: every paragraph asks one question, and the answer is
// whatever comes after the first "::".
//
// Its manifest declares no capability, because it needs nothing but the block
// it is handed. Copy this directory into <user data>/extensions/ to install it.
//
// deriveBlock is the typed contract (spec §5): the plugin picks a card type
// from the library's types and fills its fields. The host checks the answer,
// stamps it with this plugin's identity and writes it; the card's faces are
// rendered later by the host's template engine, not by the plugin.

export function describe() {
  return { name: "cards-mini", version: "0.1.0" };
}

export function deriveBlock(block, types) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{
    typeId: "remit.basic",
    ordinal: 0,
    fields: {
      Front: text.slice(0, at).trim(),
      Back: text.slice(at + 2).trim(),
    },
  }];
}
