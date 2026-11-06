export function describe() {
  return { name: "cards-basic", version: "1.0.0", protocol: 1, runtime: "js" };
}

// 模拟"内置制卡插件"：一块文本 -> 若干卡片
export function cards(block) {
  const text = block.text || "";
  const i = text.indexOf("::");
  if (i < 0) return [];
  return [{ front: text.slice(0, i).trim(), back: text.slice(i + 2).trim(), block: block.id }];
}

export function on_event(event) {
  return { handled: event.name, ok: remit.echo(true) };
}

export function spin() { let n = 0; for (;;) { n++; } }

export function hungry() { const a = []; for (;;) { a.push(new Array(4096).fill(7)); } }

export function dynamic() { return eval("1+1"); }
