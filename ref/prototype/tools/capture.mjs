#!/usr/bin/env node
// Recaptures shots/*.png from the static prototype (index.html + app.js + styles.css).
//
//   cd ref/prototype/tools
//   npm install
//   node capture.mjs            # all 40
//   node capture.mjs 08 25      # only shots whose file name starts with one of these
//
// Uses the installed Google Chrome by default (no browser download). Set
// REMIT_CAPTURE_CHANNEL= (empty) to use Playwright's bundled Chromium instead,
// after `npx playwright install chromium`.
//
// Each shot opens a fresh page at a deep link, optionally drives the prototype's
// own globals/clicks, then takes a 1440x900 @2x viewport screenshot.
// Every run asserts: no page errors, the expected screen is on screen, the PNG
// is 2880x1800 (except the deliberate element crop), and all 40 hashes differ.

import { chromium } from 'playwright';
import { createHash } from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const prototypeDir = path.resolve(here, '..');
const shotsDir = path.join(prototypeDir, 'shots');
const pageURL = pathToFileURL(path.join(prototypeDir, 'index.html')).href;

const VIEWPORT = { width: 1440, height: 900 };
const DEVICE_SCALE = 2;
const CHANNEL = process.env.REMIT_CAPTURE_CHANNEL ?? 'chrome';
const filters = process.argv.slice(2);

/* ── interactions, driven through app.js's own globals and clicks ─────────── */

const openReadingSession = (page) => page.evaluate(() => openReading('n1'));

const openReviewSession = async (page) => {
  await page.evaluate(() => openReview());
  await page.click('[data-act="reveal"]');
};

const openReviewSummary = async (page) => {
  await page.evaluate(() => openReview(['c1', 'c2', 'c3']));
  for (const grade of [1, 3, 3]) {
    await page.click('[data-act="reveal"]');
    await page.click(`[data-grade="${grade}"]`);
  }
};

const hoverRow = async (page) => {
  await page.hover('[data-note="n2"]');
};

const selectTwoCards = async (page) => {
  await page.click('[data-pick="c12"]');
  await page.click('[data-pick="c23"]');
};

const emptyReadingQueue = async (page) => {
  await page.evaluate(() => {
    ['n1', 'n4', 'n5'].forEach((id) => state.readDone.add(id));
    render();
  });
};

const archiveOneNote = async (page) => {
  await page.evaluate(() => {
    state.archived.add('n1');
    render();
  });
};

const openBlockInspectorOfB7 = (page) => page.evaluate(() => openBlockInspector('n1', 'b7'));

const openInsertMenu = async (page) => {
  await page.hover('.block[data-block="b6"] .block-body');
  await page.click('.block[data-block="b6"] [data-act="insert-after"]');
};

const openBlockMenu = async (page) => {
  await page.hover('.block[data-block="b6"] .block-body');
  // b7's card badge overlaps b6's grip, so dispatch the click on the button itself.
  await page.evaluate(() => document.querySelector('.block[data-block="b6"] [data-act="block-menu"]').click());
};

const selectFirstParagraph = async (page) => {
  await page.evaluate(() => {
    const body = document.querySelector('[data-body="b1"]');
    const range = document.createRange();
    range.selectNodeContents(body);
    const sel = getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
    showSelBar();
  });
};

const openCardEditor = (page) => page.evaluate(() => openCardDialog('n1', 'b6', 'c1'));

const openExportEntry = (page) => page.click('[data-act="doc-more"]');

const openPaletteWithQuery = async (page) => {
  await page.evaluate(() => openPalette());
  await page.fill('#palette-input', '笔记');
};

const searchHardCards = (page) => page.fill('#card-search', 'is:hard');

/* ── the 40 shots, in gallery order ──────────────────────────────────────── */

const SHOTS = [
  { file: '01-welcome.png', hash: '#/', ready: '#app[data-screen="welcome"] .welcome-card', mustSee: ['打开库', '新建库', '导入 .remitlib / Markdown'] },
  { file: '02-today.png', hash: '#/today', ready: '.page-title', mustSee: ['今日', '预算 30 分钟'] },
  { file: '03-notes.png', hash: '#/write/notes', ready: '.page-title', mustSee: ['笔记', '新建笔记', '最近编辑'] },
  { file: '04-doc-edit.png', hash: '#/write/doc/n1', ready: '.doc[data-mode="edit"]', mustSee: ['增量阅读：从长文到一张卡', '大纲'] },
  { file: '05-doc-read.png', hash: '#/write/doc/n1?mode=read', ready: '.doc[data-mode="read"]', mustSee: ['增量阅读：从长文到一张卡'] },
  { file: '06-doc-split.png', hash: '#/write/doc/n1?mode=split', ready: '.doc[data-mode="split"] #split-pane', mustSee: ['增量阅读：从长文到一张卡'] },
  { file: '07-reading-queue.png', hash: '#/read/queue', ready: '.page-title', mustSee: ['阅读队列'] },
  { file: '08-reading-session.png', hash: '#/read/queue', prepare: openReadingSession, ready: '.read-block.is-pos', mustSee: ['太难', '刚好', '太浅', '摘录'] },
  { file: '09-recall-queue.png', hash: '#/recall/queue', ready: '.page-title', mustSee: ['复习队列', '按笔记分'] },
  { file: '10-recall-session.png', hash: '#/recall/queue', prepare: openReviewSession, ready: '.card-answer', mustSee: ['按 1–4 评分', '重来', '困难', '良好', '容易', '回到原文'] },
  { file: '11-recall-summary.png', hash: '#/recall/queue', prepare: openReviewSummary, ready: '.summary', mustSee: ['今日忆完', '本次复习', '答错的按来源聚合'] },
  { file: '12-cards.png', hash: '#/recall/cards', ready: '.grid tbody tr', mustSee: ['卡片', '难卡', '到期'] },
  { file: '13-card-detail.png', hash: '#/recall/cards', prepare: (page) => page.evaluate(() => openCardDetail('c1')), ready: '#modal[open] .timeline', mustSee: ['来源块', '调度史'] },
  { file: '14-relink.png', hash: '#/recall/relink', ready: '.page-title', mustSee: ['断链', '路断了的卡片'] },
  { file: '15-types.png', hash: '#/recall/types', ready: '.page-title', mustSee: ['卡片类型', '类型是库资产'] },
  { file: '16-scope.png', hash: '#/recall/queue', prepare: (page) => page.click('[data-act="open-scope"]'), ready: '#modal[open] [data-scopekind]', mustSee: ['复习范围', '全库', '某篇笔记', '某个标签'] },
  { file: '17-stats.png', hash: '#/stats/overview', ready: '.page-title', mustSee: ['统计', '时间配比', '保留率'] },
  { file: '18-heatmap.png', hash: '#/stats/heatmap', ready: '#heat-page i', mustSee: ['热力', '26 周'] },
  { file: '19-settings-appearance.png', hash: '#/settings/appearance', ready: '.page-title', mustSee: ['外观', '正文尺寸', '纸面宽度'] },
  { file: '20-settings-plugins.png', hash: '#/settings/plugins', ready: '.page-title', mustSee: ['插件', '网页捕获', '白板'] },
  { file: '21-palette.png', hash: '#/today', prepare: openPaletteWithQuery, ready: '#palette[open] .palette-item', mustSee: ['命令', '笔记列表'] },
  { file: '22-today-dark.png', hash: '#/today?theme=dark', ready: 'html[data-theme="dark"] .page-title', mustSee: ['今日'] },
  { file: '23-hover.png', hash: '#/write/notes', prepare: hoverRow, ready: '.row:hover .hover-actions', mustSee: ['FSRS 的四个评分'] },
  { file: '24-selected.png', hash: '#/recall/cards', prepare: selectTwoCards, ready: 'tr[aria-selected="true"]', mustSee: ['已选 2 张'] },
  { file: '25-ratio.png', hash: '#/stats/overview', ready: '#ratio-7d .ratio', clip: '#ratio-7d', mustSee: ['时间配比', '读', '忆', '摘'] },
  { file: '26-empty.png', hash: '#/read/queue', prepare: emptyReadingQueue, ready: '.empty', mustSee: ['今天的阅读队列空了'] },
  { file: '27-excerpt-tree.png', hash: '#/write/tree', ready: '.tree', mustSee: ['摘录树', '再摘'] },
  { file: '28-archive.png', hash: '#/read/archive', prepare: archiveOneNote, ready: '.list .row .title', mustSee: ['归档', '增量阅读：从长文到一张卡', '恢复'] },
  { file: '29-block-inspector.png', hash: '#/write/doc/n1', prepare: openBlockInspectorOfB7, ready: '#modal[open] .timeline, #modal[open] .list', mustSee: ['三种角色', '正文', '摘录 · 1'] },
  { file: '30-insert-menu.png', hash: '#/write/doc/n1', prepare: openInsertMenu, ready: '#menu:not([hidden]) .menu-item', mustSee: ['正文', '标题', '为这一段制卡', '字段块'] },
  { file: '31-block-menu.png', hash: '#/write/doc/n1', prepare: openBlockMenu, ready: '#menu:not([hidden]) .menu-item', mustSee: ['复制这一段', '三种角色…', '在白板里定位', '删除'] },
  { file: '32-selection-bar.png', hash: '#/write/doc/n1', prepare: selectFirstParagraph, ready: '.selbar', mustSee: ['挖空', '制卡', '摘录整段'] },
  { file: '33-card-editor.png', hash: '#/write/doc/n1', prepare: openCardEditor, ready: '#modal[open] #card-prompt', mustSee: ['卡片', '提示', '答案', '第 6 段'] },
  { file: '34-export.png', hash: '#/write/doc/n1', prepare: (page) => page.evaluate(() => openExport()), ready: '#modal[open] [data-export-kind]', mustSee: ['导出', 'Markdown 目录', '.remitlib 归档', '卡片 CSV'] },
  { file: '35-import.png', hash: '#/today', prepare: (page) => page.evaluate(() => openImport()), ready: '#modal[open] .steps', mustSee: ['导入', '选来源', '.remitlib 归档', 'Markdown 目录'] },
  { file: '36-keys.png', hash: '#/today', prepare: (page) => page.evaluate(() => openKeysHelp()), ready: '#modal[open] .kv', mustSee: ['快捷键', '搜索与命令', '退出当前的会话或浮层'] },
  { file: '37-whiteboard.png', hash: '#/plugin/board', ready: '#board .bnode', mustSee: ['白板', '回到这一段'] },
  { file: '38-cards-search.png', hash: '#/recall/cards', prepare: searchHardCards, ready: '.query-tokens .tok', mustSee: ['is:hard', '10 条命中'] },
  { file: '39-recall-presets.png', hash: '#/recall/presets', ready: '.page-title', mustSee: ['调度预设', '学习步', '每日上限', '断链'] },
  { file: '40-recall-history.png', hash: '#/recall/history', ready: '.page-title', mustSee: ['历史', '评分分布', '本周'] },
];

/* ── capture ─────────────────────────────────────────────────────────────── */

const selected = SHOTS.filter((s) => !filters.length || filters.some((f) => s.file.startsWith(f)));
if (!selected.length) {
  console.error(`No shot matches ${filters.join(', ')}`);
  process.exit(1);
}

const browser = await chromium.launch(CHANNEL ? { channel: CHANNEL } : {});
const manifest = [];

for (const shot of selected) {
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE,
    colorScheme: 'light',
    reducedMotion: 'reduce',
    locale: 'zh-CN',
  });
  const page = await context.newPage();
  const failures = [];
  page.on('pageerror', (err) => failures.push(`pageerror: ${err.message}`));
  page.on('console', (msg) => {
    if (msg.type() === 'error') failures.push(`console.error: ${msg.text()}`);
  });

  await page.goto(`${pageURL}${shot.hash}`, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  // A text caret blinks, which would make byte-identical recaptures impossible.
  await page.addStyleTag({ content: '*, *::before, *::after { caret-color: transparent !important; }' });
  await page.locator('#stage').waitFor({ state: 'attached' });

  if (shot.prepare) await shot.prepare(page);
  if (shot.ready) await page.locator(shot.ready).first().waitFor({ state: 'visible', timeout: 5000 });

  const seen = await page.evaluate(() => document.body.innerText);
  for (const needle of shot.mustSee || []) {
    if (!seen.includes(needle)) failures.push(`missing on screen: ${needle}`);
  }
  if (failures.length) {
    await context.close();
    throw new Error(`${shot.file} did not render as intended:\n  ${failures.join('\n  ')}`);
  }

  const out = path.join(shotsDir, shot.file);
  if (shot.clip) await page.locator(shot.clip).screenshot({ path: out, animations: 'disabled' });
  else await page.screenshot({ path: out, animations: 'disabled' });
  await context.close();

  const bytes = await fs.readFile(out);
  const png = readPngSize(bytes);
  manifest.push({ file: shot.file, ...png, sha256: createHash('sha256').update(bytes).digest('hex') });
  console.log(`${shot.file}  ${png.width}x${png.height}  ${(bytes.length / 1024).toFixed(1)} KB`);
}

await browser.close();

/* ── verify: dimensions, uniqueness, gallery links ───────────────────────── */

let failed = false;
for (const m of manifest) {
  if (m.sha256 && manifest.filter((x) => x.sha256 === m.sha256).length > 1) {
    console.error(`duplicate image bytes: ${manifest.filter((x) => x.sha256 === m.sha256).map((x) => x.file).join(' ')}`);
    failed = true;
  }
  if (m.file === '25-ratio.png') continue;
  if (m.width !== VIEWPORT.width * DEVICE_SCALE || m.height !== VIEWPORT.height * DEVICE_SCALE) {
    console.error(`wrong size: ${m.file} is ${m.width}x${m.height}`);
    failed = true;
  }
}

if (selected.length === SHOTS.length) {
  const gallery = await fs.readFile(path.join(shotsDir, 'index.html'), 'utf8');
  for (const s of SHOTS) {
    if (!gallery.includes(`src="${s.file}"`)) {
      console.error(`gallery does not reference ${s.file}`);
      failed = true;
    }
  }
  const hashes = new Set(manifest.map((m) => m.sha256));
  console.log(`\n${manifest.length} shots · ${hashes.size} distinct hashes`);
  if (hashes.size !== manifest.length) failed = true;
}

if (failed) process.exit(1);

function readPngSize(buf) {
  if (buf.length < 24 || buf.toString('ascii', 12, 16) !== 'IHDR') throw new Error('not a PNG');
  return { width: buf.readUInt32BE(16), height: buf.readUInt32BE(20) };
}
