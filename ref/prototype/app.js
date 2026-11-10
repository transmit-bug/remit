/* Remit · 功能原型
   一个 3 合 1 记忆工作台：写、读、忆共用一套块模型。
   app.js = 数据 + 路由 + 渲染 + 动作，四段，无依赖。 */

'use strict';

/* ═══ 图标 ════════════════════════════════════════════════════════════════
   一套手绘几何字形：24 网格、2px 描边、圆头圆角，只用 currentColor。 */
const ICONS = {
  today:   '<rect x="3" y="5" width="18" height="16" rx="2.5"/><path d="M8 3v4M16 3v4M3 10h18"/>',
  write:   '<path d="M4 20l4.2-1L19.5 7.7a2 2 0 0 0 0-2.8l-1.4-1.4a2 2 0 0 0-2.8 0L4 15.8z"/><path d="M14.5 5.5l4 4"/>',
  read:    '<path d="M4 5h7v15H4zM13 5h7v15h-7z"/>',
  recall:  '<rect x="3" y="4" width="18" height="16" rx="2.5"/><path d="M8 10h8M8 14h5"/>',
  stats:   '<path d="M4 20V13M10 20V6M16 20v-9M22 20H2"/>',
  settings:'<path d="M4 8h8M16 8h4M4 16h4M12 16h8"/><circle cx="14" cy="8" r="2"/><circle cx="10" cy="16" r="2"/>',
  plug:    '<path d="M9 3v5M15 3v5M6 8h12v3a6 6 0 0 1-12 0zM12 17v4"/>',
  search:  '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.6-3.6"/>',
  plus:    '<path d="M12 5v14M5 12h14"/>',
  more:    '<path d="M6 12h.01M12 12h.01M18 12h.01"/>',
  chev:    '<path d="M9 6l6 6-6 6"/>',
  down:    '<path d="M6 9l6 6 6-6"/>',
  grip:    '<path d="M9 6h.01M9 12h.01M9 18h.01M15 6h.01M15 12h.01M15 18h.01"/>',
  x:       '<path d="M6 6l12 12M18 6L6 18"/>',
  check:   '<path d="M4 12.5l5 5L20 6.5"/>',
  back:    '<path d="M19 12H5M11 6l-6 6 6 6"/>',
  quote:   '<path d="M9 6H5v6h4v6H5M19 6h-4v6h4v6h-4"/>',
  panel:   '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M15 4v16"/>',
  sidebar: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/>',
  eye:     '<path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6z"/><circle cx="12" cy="12" r="2.6"/>',
  split:   '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M12 4v16"/>',
  edit:    '<path d="M4 20l4-1 11-11-3-3L5 16z"/><path d="M15 5l3 3"/>',
  clock:   '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>',
  trash:   '<path d="M4 7h16M9 7V5h6v2M6 7l1 13h10l1-13"/>',
  up:      '<path d="M12 19V5M6 11l6-6 6 6"/>',
  dn:      '<path d="M12 5v14M6 13l6 6 6-6"/>',
  copy:    '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
  save:    '<path d="M5 4h11l3 3v13H5z"/><path d="M8 4v5h7V4M8 20v-6h8v6"/>',
  download:'<path d="M12 4v12M7 11l5 5 5-5M4 20h16"/>',
  upload:  '<path d="M12 20V8M7 13l5-5 5 5M4 4h16"/>',
  filter:  '<path d="M3 5h18l-7 8v6l-4 2v-8z"/>',
  tag:     '<path d="M4 4h8l8 8-8 8-8-8z"/><circle cx="8.5" cy="8.5" r="1.4"/>',
  image:   '<rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="8.5" cy="10" r="1.5"/><path d="M21 16l-5-5-6 6"/>',
  layers:  '<path d="M12 3l9 5-9 5-9-5z"/><path d="M3 13l9 5 9-5"/>',
  book:    '<path d="M5 4h14v16H5z"/><path d="M9 4v16"/>',
  spark:   '<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/>',
  grid:    '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
};

function icon(name, size = 20) {
  return `<svg class="od-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="--od-icon-size:${size}px">${ICONS[name] || ''}</svg>`;
}

/* ═══ 数据 ════════════════════════════════════════════════════════════════ */
const NOTES = [
  { id: 'n1', title: '增量阅读：从长文到一张卡', tags: ['方法论', '阅读'], edited: '4 天前', reading: { pos: 6 }, blocks: [
    { id: 'b1', kind: 'p', text: '长文只有两种命运：一次读完，然后遗忘；或者永远躺在库里。第三种是把它当成可以分批消化的材料。' },
    { id: 'b2', kind: 'h2', text: '问题' },
    { id: 'b3', kind: 'p', text: '收集很容易，消化很难。剪藏夹越大，越像一座没人走的图书馆。' },
    { id: 'b4', kind: 'p', text: '阅读不是一次动作，而是一条曲线：第一次读只该拿走能拿走的那部分，剩下的留给下一次。' },
    { id: 'b5', kind: 'h2', text: '节奏' },
    { id: 'b6', kind: 'p', text: '增量阅读要解决的问题不是「读得慢」，而是「读完之后什么也没留下」。' },
    { id: 'b7', kind: 'p', text: '一段话读三遍，不如同一段话隔四周读三遍。间隔让重读变成检查，而不是重复。' },
    { id: 'b8', kind: 'p', text: '所以阅读位置要记到块：下次回来，从上次停下的那一段接着走，不从头开始。' },
    { id: 'b9', kind: 'h2', text: '摘与卡' },
    { id: 'b10', kind: 'p', text: '读到值得留下的句子，就地摘录。摘录长成一篇子笔记，带着指回原块的引用。' },
    { id: 'b11', kind: 'p', text: 'cloze 要在读的当下挖空，因为理解是此刻的；回头再看，只剩形式。' },
    { id: 'b12', kind: 'p', text: '卡片不是知识本身，是一条回到原文的路。答案记不住时，下一步是回到那一段。' },
    { id: 'b13', kind: 'h2', text: '边界' },
    { id: 'b14', kind: 'p', text: '什么时候该退出队列：读懂了，或者确定不读了。两种都该移出，不该留在列表里当噪音。' },
  ] },
  { id: 'n2', title: 'FSRS 的四个评分', tags: ['FSRS'], edited: '昨天', reading: null, blocks: [
    { id: 'b1', kind: 'p', text: '评分不是自我评价，是给调度器的测量：这一次回忆的稳定度该怎么变。' },
    { id: 'b2', kind: 'h2', text: '四档在算什么' },
    { id: 'b3', kind: 'p', text: '重来把稳定性打回去，良好把它推高，容易推得更高。困难是中间那一档：想起来了，但很费劲。' },
    { id: 'b4', kind: 'p', text: '所以四档里没有「我不确定」——不确定就该按重来，让卡片明天再来一次。' },
    { id: 'b5', kind: 'h2', text: '阅读侧为什么只留三档' },
    { id: 'b6', kind: 'p', text: '三档是太难、刚好、太浅。读者对自己的困难程度分不出四档，而阅读只要一个节奏。' },
    { id: 'b7', kind: 'h2', text: '常见误用' },
    { id: 'b8', kind: 'p', text: '一直按「容易」，间隔会膨胀到没有意义；一直按「重来」，卡片永远停在第一天。' },
  ] },
  { id: 'n3', title: '块 ID 与引用的稳定', tags: ['方法论'], edited: '3 天前', reading: null, blocks: [
    { id: 'b1', kind: 'p', text: '块是笔记里最小的可寻址单位。ID 一次性铸造，写在明文里，跟着段落走。' },
    { id: 'b2', kind: 'h2', text: '为什么不用标题定位' },
    { id: 'b3', kind: 'p', text: '标题会改，会重名，会被移动。用标题做锚点，引用活不过一次改名。' },
    { id: 'b4', kind: 'p', text: '块 ID 不随顺序变化，所以拖动、拆分、合并之后，反链仍然指得准。' },
    { id: 'b5', kind: 'p', text: '代价是明文里多了一串不可读的字符。这笔账划算：可读性靠渲染补回来，引用丢了补不回来。' },
  ] },
  { id: 'n4', title: '摘录怎么写才有用', tags: ['方法论', '摘录'], edited: '刚加入', reading: { pos: 0 }, blocks: [
    { id: 'b1', kind: 'p', text: '摘录不是复制。复制留下的是原文，摘录留下的应该是一条能独立站住的想法。' },
    { id: 'b2', kind: 'h2', text: '三个条件' },
    { id: 'b3', kind: 'p', text: '离开上下文能读懂；知道自己从哪来；能长成一张卡。' },
    { id: 'b4', kind: 'p', text: '三条里缺一条，摘录就只是搬家。' },
    { id: 'b5', kind: 'h2', text: '引用' },
    { id: 'b6', kind: 'p', text: '摘录的块里带着指回原块的引用。源块没了、也没有快照，引用就是 lost——调度史还在，路断了。' },
    { id: 'b7', kind: 'p', text: '所以摘录要趁原文还在、还记得为什么摘的时候写。' },
    { id: 'b8', kind: 'h2', text: '长度' },
    { id: 'b9', kind: 'p', text: '一句话最好，三句话是上限。超过三句，说明你还没想清楚要记住哪一句。' },
  ] },
  { id: 'n5', title: '本地优先的代价', tags: ['方法论'], edited: '今天', reading: { pos: 4 }, blocks: [
    { id: 'b1', kind: 'p', text: '本地优先意味着库是一个目录，不是一台服务器的记录。' },
    { id: 'b2', kind: 'h2', text: '代价' },
    { id: 'b3', kind: 'p', text: '没有账号，就没有跨设备同步。冲突要自己解，备份要自己做。' },
    { id: 'b4', kind: 'p', text: '索引可以删掉重建，但重建的那几秒只有你自己等。' },
    { id: 'b5', kind: 'p', text: '换来的是：断网能用，十年后还能用，导出是一串 .md。' },
    { id: 'b6', kind: 'p', text: '这笔账只有在真正丢过一次数据之后才算得清。' },
  ] },
  { id: 'n6', title: '三合一的取舍', tags: ['方法论', '原型'], edited: '上周', reading: null, blocks: [
    { id: 'b1', kind: 'p', text: '笔记、卡片、阅读放进一个应用，最难的从来不是功能，是让它们共用一套模型。' },
    { id: 'b2', kind: 'h2', text: '一套模型' },
    { id: 'b3', kind: 'p', text: '笔记是有序的块；卡片是某个块的记忆角色；阅读位置是某个块上的一个点。三者指的都是块。' },
    { id: 'b4', kind: 'p', text: '所以不需要三套数据，也不需要三条调度史。一份 reviews.jsonl，重放就能推出到期与位置。' },
    { id: 'b5', kind: 'h2', text: '一个队列' },
    { id: 'b6', kind: 'p', text: 'Today 是混合队列：今天读几篇、忆几张，由同一个预算分。' },
    { id: 'b7', kind: 'p', text: '超载时先牺牲阅读：卡片有到期日，阅读没有。' },
    { id: 'b8', kind: 'h2', text: '一个入口' },
    { id: 'b9', kind: 'p', text: '复习范围不叫 deck。它是取卡的一次选择，靠笔记与标签组织，不引入容器实体。' },
    { id: 'b10', kind: 'h2', text: '边界' },
    { id: 'b11', kind: 'p', text: '不做三套同步，不做本地优先之外的账号体系，不把 AI 放进真相路径。' },
  ] },
  { id: 'n7', title: '摘录：间隔让重读变成检查', tags: ['摘录'], edited: '4 天前', excerptOf: 'n1', sourceBlock: 'b7', reading: null, blocks: [
    { id: 'b1', kind: 'p', text: '一段话读三遍，不如同一段话隔四周读三遍。间隔让重读变成检查，而不是重复。' },
    { id: 'b2', kind: 'p', text: '[[增量阅读：从长文到一张卡#^b7]]' },
  ] },
  { id: 'n8', title: '读《怎样解题》', tags: ['读书'], edited: '2 周前', reading: null, blocks: [
    { id: 'b1', kind: 'p', text: '波利亚的四步不是流程，是四个可以随时退回的位置。' },
    { id: 'b2', kind: 'h2', text: '理解题意' },
    { id: 'b3', kind: 'p', text: '未知、已知、条件——把三样写下来，很多题在这一步就已经解开一半。' },
    { id: 'b4', kind: 'h2', text: '拟定计划' },
    { id: 'b5', kind: 'p', text: '找一个你解过的、相似的题。相似的判据是「结构」而不是「题材」。' },
    { id: 'b6', kind: 'p', text: '如果卡住，回到定义；定义是这道题里唯一不会骗你的东西。' },
    { id: 'b7', kind: 'p', text: '执行之后要回看：这一步为什么成立，换个条件还成立吗。' },
  ] },
  { id: 'nd', title: '今天 · 9月18日', tags: ['日记'], edited: '今天', reading: null, daily: true, blocks: [
    { id: 'b1', kind: 'h2', text: '今天' },
    { id: 'b2', kind: 'p', text: '读《怎样解题》第四段，摘了一句：「未知、已知、条件——把三样写下来」。' },
    { id: 'b3', kind: 'p', text: '复习 12 张，错了 2 张，都来自「三合一的取舍」。那段该重写了。' },
    { id: 'b4', kind: 'h2', text: '明天' },
    { id: 'b5', kind: 'p', text: '' },
  ] },
  { id: 'n9', title: '模板：读书笔记', tags: ['模板'], edited: '1 个月前', template: true, reading: null, blocks: [
    { id: 'b1', kind: 'p', text: '## 这本书在解决什么问题' },
    { id: 'b2', kind: 'p', text: '## 核心论点' },
    { id: 'b3', kind: 'p', text: '## 我不同意的' },
    { id: 'b4', kind: 'p', text: '## 可以变成卡片的三句' },
  ] },
  { id: 'n10', title: '模板：概念卡', tags: ['模板'], edited: '1 个月前', template: true, reading: null, blocks: [
    { id: 'b1', kind: 'p', text: ':: 这个概念解决什么问题？' },
    { id: 'b2', kind: 'p', text: ':: 它和相邻概念的分界在哪？' },
    { id: 'b3', kind: 'p', text: ':: 一个反例' },
  ] },
];

const PLUGIN_TYPES = [
  { id: 'qa', name: '问答', plugin: 'tplqa', desc: '一问一答，问句在前，强调提问本身。',
    fields: ['问', '答'], front: '{{问}}', back: '{{答}}', builtin: false },
];

const CARD_TYPES = [
  { id: 'basic', name: 'basic', desc: '两字段，正面一句提示，背面一句答案。', fields: ['提示', '答案'], front: '{{提示}}', back: '{{答案}}', builtin: true },
  { id: 'bidir', name: 'bidirectional', desc: '两字段，正反互换：一个事实问两个方向。', fields: ['正面', '背面'], front: '{{正面}}', back: '{{背面}}', builtin: true },
  { id: 'cloze', name: 'cloze', desc: '一个字段，用 {{…}} 挖空，其余照读。', fields: ['正文'], front: '{{cloze:正文}}', back: '{{正文}}', builtin: true },
  { id: 'def', name: '定义', desc: '三字段：术语、定义、一个反例。', fields: ['术语', '定义', '反例'], front: '{{术语}} 是什么？', back: '{{定义}}\n\n反例：{{反例}}', builtin: false },
];

const mkCard = (id, note, block, prompt, answer, extra = {}) => ({
  id, note, block, prompt, answer, type: 'basic', state: 'review', due: '今天', ref: 'live', tags: [], ...extra,
});

const CARDS = [
  mkCard('c1', 'n1', 'b6', '增量阅读要解决的问题是什么？', '不是「读得慢」，而是「读完之后什么也没留下」。'),
  mkCard('c2', 'n1', 'b7', '为什么间隔比重读次数重要？', '间隔让重读变成检查，而不是重复。'),
  mkCard('c3', 'n1', 'b10', '读到值得留下的句子时，就地做什么？', '摘录——它长成一篇子笔记，带着指回原块的引用。', { state: 'new' }),
  mkCard('c4', 'n1', 'b11', 'cloze 要在读的当下挖空，因为 ____。', '理解是此刻的；回头再看，只剩形式。', { type: 'cloze', state: 'learning' }),
  mkCard('c5', 'n1', 'b12', '卡片的作用是什么？', '一条回到原文的路。答案记不住时，下一步是回到那一段。', { due: '明天' }),
  mkCard('c6', 'n2', 'b1', '评分测的是什么？', '这一次回忆的稳定度该怎么变。'),
  mkCard('c7', 'n2', 'b3', '四档评分分别把稳定性推向哪边？', '重来打回去，良好推高，容易推得更高，困难是中间那一档。', { state: 'learning' }),
  mkCard('c8', 'n2', 'b6', '阅读侧只留三档：____、刚好、太浅。', '太难', { type: 'cloze', state: 'new' }),
  mkCard('c9', 'n3', 'b3', '为什么不能用标题做引用锚点？', '标题会改、会重名、会被移动；引用活不过一次改名。', { due: '明天' }),
  mkCard('c10', 'n4', 'b3', '一条摘录成立的三个条件？', '离开上下文能读懂；知道自己从哪来；能长成一张卡。'),
  mkCard('c11', 'n4', 'b6', '引用变成 lost 是什么意思？', '源块没了、也没有快照；调度史还在，路断了。', { state: 'learning', ref: 'lost' }),
  mkCard('c12', 'n5', 'b3', '本地优先最大的代价是什么？', '没有账号就没有同步：冲突自己解，备份自己做。', { due: '3 天后' }),
  mkCard('c13', 'n6', 'b3', '笔记、卡片、阅读位置共用的是什么？', '块——笔记是块的序列，卡片是块的记忆角色，阅读位置是块上的一个点。'),
  mkCard('c14', 'n6', 'b6', 'Today 是什么队列？', '混合队列：今天读几篇、忆几张由同一个预算分。'),
  mkCard('c15', 'n6', 'b7', '超载时先牺牲哪一边，为什么？', '先牺牲阅读：卡片有到期日，阅读没有。', { state: 'learning' }),
  mkCard('c16', 'n6', 'b9', '复习范围为什么不是 deck？', '它是取卡的一次选择，靠笔记与标签组织，不引入容器实体。', { ref: 'frozen' }),
  mkCard('c17', 'n7', 'b1', '间隔让重读变成 ____，而不是重复。', '检查', { type: 'cloze', state: 'new' }),
  mkCard('c18', 'n8', 'b3', '解题的第一步要写下哪三样？', '未知、已知、条件。', { type: 'def' }),
  mkCard('c19', 'n8', 'b5', '什么才算「相似」的题？', '结构相似，而不是题材相似。'),
  mkCard('c20', 'n8', 'b6', '卡住的时候回到哪里？', '回到定义——它是这道题里唯一不会骗你的东西。', { due: '明天' }),
  mkCard('c21', 'n1', 'b8', '阅读位置记在哪一级？', '块：下次从停下的那一段接着走。'),
  mkCard('c22', 'n2', 'b8', '一直按「容易」会怎样？', '间隔膨胀到没有意义。', { state: 'learning' }),
  mkCard('c23', 'n3', 'b5', '块 ID 带来的代价是什么？', '明文里多了一串不可读的字符，靠渲染补回来。', { due: '3 天后' }),
  mkCard('c24', 'n5', 'b5', '本地优先换来了什么？', '断网能用，十年后能用，导出是一串 .md。', { due: '明天' }),
  mkCard('c25', 'n1', 'b6', '增量阅读解决的不是读得____，而是读完没留下。', '慢', { type: 'cloze', state: 'new' }),
];

const PLUGINS = [
  { id: 'cards', name: '内置制卡', desc: '解析 :: 与 ::: 字段块，把段落变成卡片。',
    caps: ['derive:block', 'policy:schedule'], on: true, builtin: true,
    menus: { 'note.block': ['解析为卡片', '插入字段块'], 'editor.slash': ['字段块'] } },
  { id: 'summary', name: '阅读摘要', desc: '在复习总结里补一句今天的阅读进度。',
    caps: ['summary'], on: true, builtin: true,
    menus: { 'review.card': ['给一句提示'] } },
  { id: 'hard', name: '难卡报告', desc: '按 FSRS 的败笔次数找出反复遗忘的卡，按来源聚合。',
    caps: ['review:read', 'on:event'], on: true, builtin: false,
    menus: { 'review.card': ['标记为难卡'], 'browse.row': ['只看难卡'], 'stats.block': ['本月难卡 Top 5'] } },
  { id: 'tplqa', name: '模板：问答', desc: '给库提供一种新卡片类型：一问一答，强调提问本身。',
    caps: ['types:contribute'], on: true, builtin: false, types: ['qa'],
    menus: { 'note.block': ['按问答制卡'] } },
  { id: 'capture', name: '网页捕获', desc: '把链接抓成一篇普通笔记，附来源。',
    caps: ['net:fetch', 'import'], on: false, builtin: false,
    menus: { 'toolbar.overflow': ['抓取链接…'] } },
  { id: 'export', name: '卡片导出', desc: '把卡片导出成 CSV，供别的工具读。',
    caps: ['on:event'], on: true, builtin: false,
    menus: { 'browse.row': ['导出所选为 CSV'] } },
  { id: 'whiteboard', name: '白板', desc: '（设计稿）把块摆到一块无限画布上，连线成图。',
    caps: ['notes:read', 'pages'], on: true, builtin: false,
    menus: { 'today.widget': ['新建白板'], 'toolbar.overflow': ['打开白板'] } },
];


/* ═══ 状态 ════════════════════════════════════════════════════════════════ */
const state = {
  screen: 'app',
  area: 'today',
  sub: 'notes',
  noteId: null,
  docMode: 'edit',
  panel: true,
  panelTab: 'outline',
  findOpen: false,
  currentBlock: null,
  writeFilter: 'all',
  writeQuery: '',
  tagFilter: '',
  noteSort: 'edited',
  reviewScope: 'all',
  on: new Set(['reminder', 'preview', 'return', 'agent', 'autoupdate', 'loadOnStart']),
  bodySize: 18,
  paperWidth: 760,
  maxInterval: 180,
  reviewScope: { kind: 'all', value: '' },
  importStep: 0,
  importKind: '',
  relinkCard: null,
  relinkNote: '',
  archived: new Set(),
  dismissed: new Set(),
  failed: [],
  pluginPage: 'cards',
  boardFocus: '',
  readDone: new Set(),
  readPos: 0,
  cardFilter: 'all',
  cardQuery: '',
  typeFilter: '',
  refFilter: '',
  typeSel: 'basic',
  cardCols: { interval: true, lapses: true },
  saved: [{ id: 's1', label: '今天到期且是新卡' }, { id: 's2', label: '断链，需要重连' }],
  cardSort: { key: 'due', dir: 1 },
  selected: new Set(),
  readingToCard: false,
  session: null,
  budget: { cards: 20, minutes: 30 },
  retention: 90,
  themePref: 'system',
  reviewedToday: 4,
  railWide: true,
  paletteIndex: 0,
  paletteFlat: [],
  excerptSeq: 0,
  welcomeState: 'open',
};

const $  = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
const esc = (s) => String(s == null ? '' : s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const noteById = (id) => NOTES.find((n) => n.id === id);
const cardsOf = (id) => CARDS.filter((c) => c.note === id);
const cardsOn = (id, b) => CARDS.filter((c) => c.note === id && c.block === b);
const dueCards = () => CARDS.filter((c) => c.due === '今天');
const readingQueue = () => NOTES.filter((n) => n.reading && !state.readDone.has(n.id)
  && !state.archived.has(n.id) && !state.dismissed.has(n.id));
const excerptedFrom = (noteId, blockId) => NOTES.some((n) => n.excerptOf === noteId && n.sourceBlock === blockId);

function cardTypes() {
  const extra = [];
  for (const p of PLUGINS) {
    if (!p.on || !p.types) continue;
    for (const id of p.types) {
      const t = PLUGIN_TYPES.find((x) => x.id === id);
      if (t) extra.push(t);
    }
  }
  return CARD_TYPES.concat(extra);
}

function typeById(id) { return cardTypes().find((t) => t.id === id); }

function cardsState(cards) {
  if (!cards.length) return '';
  if (cards.some((c) => c.ref === 'lost')) return 'lost';
  if (cards.some((c) => c.due === '今天')) return 'due';
  if (cards.every((c) => c.state === 'new')) return 'new';
  return 'later';
}

function pluginMenu(slot) {
  const out = [];
  for (const p of PLUGINS) {
    if (!p.on || !p.menus) continue;
    (p.menus[slot] || []).forEach((label, i) => out.push({ act: `pg:${p.id}:${slot}:${i}`, label }));
  }
  return out;
}

function cardStateWords(cards) {
  const due = cards.filter((c) => c.due === '今天').length;
  const lost = cards.filter((c) => c.ref === 'lost').length;
  const fresh = cards.filter((c) => c.state === 'new').length;
  const bits = [];
  if (due) bits.push(`${due} 张今天到期`);
  if (fresh) bits.push(`${fresh} 张新卡`);
  if (lost) bits.push(`${lost} 张断链`);
  return bits.join('，') || '都在以后';
}

function inline(text) {
  return esc(text)
    .replace(/\{\{(.+?)\}\}/g, '<mark class="cloze">$1</mark>')
    .replace(/\[\[([^\]]+?)#\^([^\]]+?)\]\]/g, '<span class="xref">↩ $1</span>')
    .replace(/\n/g, '<br>');
}

/* ═══ 组件 ════════════════════════════════════════════════════════════════ */
const chip = (label, pressed, attrs = '', count) =>
  `<button class="chip" type="button" ${attrs} aria-pressed="${pressed}">${esc(label)}${count != null ? `<span class="n">${count}</span>` : ''}</button>`;
const badge = (t, k = '') => `<span class="badge ${k ? 'badge--' + k : ''}">${esc(t)}</span>`;
const seg = (items, active, attr) =>
  `<div class="segmented" role="group">${items.map(([v, l]) =>
    `<button type="button" data-${attr}="${v}" aria-pressed="${active === v}">${l}</button>`).join('')}</div>`;
const pageHead = (title, sub, actions = '') => `
  <div class="page-head">
    <div class="titles"><h1 class="page-title">${title}</h1>${sub ? `<p class="page-sub">${sub}</p>` : ''}</div>
    ${actions ? `<div class="page-actions">${actions}</div>` : ''}
  </div>`;
const stat = (n, c, d = '') => `<div class="stat"><span class="n">${n}</span><span class="c">${c}</span>${d ? `<span class="d">${d}</span>` : ''}</div>`;
const empty = (ic, title, body, action = '') => `
  <div class="empty"><span class="eicon">${icon(ic, 20)}</span><h3>${title}</h3><p>${body}</p>${action}</div>`;
const meter = (pct) => `<div class="meter"><i style="--to:${Math.max(0, Math.min(100, pct))}%"></i></div>`;
const page = (measure, inner) => `<div class="view"><div class="stage-scroll"><div class="page"><div class="measure ${measure}">${inner}</div></div></div></div>`;

/* ═══ 外壳 ════════════════════════════════════════════════════════════════ */
const AREAS = {
  today:   { label: '今日', icon: 'today', subs: [] },
  write:   { label: '写', icon: 'write', subs: [['notes', '笔记'], ['search', '搜索'], ['tags', '标签'], ['tree', '摘录树'], ['assets', '资产'], ['templates', '模板']] },
  read:    { label: '读', icon: 'read', subs: [['queue', '阅读队列'], ['archive', '归档']] },
  recall:  { label: '忆', icon: 'recall', subs: [['queue', '队列'], ['cards', '卡片'], ['types', '类型'], ['presets', '预设'], ['history', '历史'], ['relink', '断链']] },
  stats:   { label: '统计', icon: 'stats', subs: [['overview', '总览'], ['heatmap', '热力'], ['forecast', '预测']] },
  settings:{ label: '设置', icon: 'settings', subs: [['appearance', '外观'], ['review', '复习'], ['budget', '预算'], ['keys', '快捷键'], ['plugins', '插件'], ['agent', 'Agent'], ['update', '更新'], ['about', '关于']] },
};

function renderRail() {
  const counts = {
    today: dueCards().length + readingQueue().length,
    write: NOTES.filter((n) => !n.template && !n.excerptOf).length,
    read: readingQueue().length,
    recall: dueCards().length,
  };
  $('#rail-nav').innerHTML = Object.entries(AREAS).map(([id, a]) => `
    <button class="rail-item" type="button" data-area="${id}" title="${a.label}"
      ${state.area === id ? 'aria-current="page"' : ''}>
      ${icon(a.icon, 19)}<span class="ri-label">${a.label}</span>
      ${counts[id] ? `<span class="ri-count">${counts[id]}</span>` : ''}
    </button>`).join('');

  const pages = [{ id: 'cards', label: '制卡脚本' }];
  if (PLUGINS.some((x) => x.id === 'whiteboard' && x.on)) pages.push({ id: 'board', label: '白板' });
  $('#rail-foot').innerHTML = pages.map((pg) => `
    <button class="rail-item" type="button" data-plugin-page="${pg.id}" title="${pg.label}"
      ${state.area === 'plugin' && state.pluginPage === pg.id ? 'aria-current="page"' : ''}>
      ${icon(pg.id === 'board' ? 'grid' : 'plug', 19)}<span class="ri-label">${pg.label}</span>
    </button>`).join('') + `
    <button class="rail-item rail-mini-btn" type="button" data-act="expand-rail" aria-label="展开导航">
      ${icon('chev', 18)}
    </button>`;
}

function renderTop() {
  const area = AREAS[state.area];
  let crumb = `<span>Remit</span><span class="sep">/</span><b>${area ? area.label : '插件'}</b>`;
  if (!area) crumb += `<span class="sep">/</span><span class="cur">${state.pluginPage === 'board' ? '白板' : '制卡脚本'}</span>`;
  let actions = '';

  if (state.area === 'write' && state.sub === 'doc') {
    const note = noteById(state.noteId);
    crumb = `<b>写</b><span class="sep">/</span><span class="cur">${esc(note ? note.title : '')}</span>`;
    actions = `
      ${seg([['edit', '编辑'], ['split', '分栏'], ['read', '阅读']], state.docMode, 'docmode')}
      <button class="icon-btn" type="button" data-act="toggle-panel" aria-pressed="${state.panel}" aria-label="大纲与反链">${icon('panel', 18)}</button>
      <button class="icon-btn" type="button" data-act="toggle-find" aria-pressed="${state.findOpen}" aria-label="查找">${icon('search', 18)}</button>
      <button class="icon-btn" type="button" data-act="doc-more" aria-label="更多">${icon('more', 18)}</button>`;
  } else {
    const sub = area && area.subs.find(([k]) => k === state.sub);
    if (sub) crumb += `<span class="sep">/</span><span class="cur">${sub[1]}</span>`;
    if (state.area === 'write' && state.sub === 'notes') {
      actions += `<button class="btn" type="button" data-act="new-note">${icon('plus', 14)} 新建</button>`;
    }
    actions += `<button class="btn btn--quiet" type="button" data-act="palette">${icon('search', 14)} 搜索 <span class="kbd">⌘K</span></button>`;
    actions += `<button class="icon-btn" type="button" data-act="area-more" aria-label="更多">${icon('more', 18)}</button>`;
  }

  $('#crumb').innerHTML = crumb;
  $('#topbar-actions').innerHTML = actions;

  const sn = $('#subnav');
  if (!area || !area.subs.length || state.sub === 'doc') {
    $('#app').dataset.subnav = 'off';
    sn.innerHTML = '';
  } else {
    $('#app').dataset.subnav = 'on';
    sn.innerHTML = area.subs.map(([k, l]) => `
      <button class="tab" type="button" data-sub="${k}" aria-selected="${state.sub === k}">
        ${l}${k === 'cards' ? `<span class="tab-count">${CARDS.length}</span>` : ''}
      </button>`).join('');
  }
}

function render() {
  renderRail();
  renderTop();
  const stage = $('#stage');
  stage.innerHTML = renderScreen();
  renderStatus();
  paintHeat('#heat-page');
  paintHeat('#heat-mini');
}

function go(area, sub) {
  state.area = area;
  if (sub) state.sub = sub;
  else if (AREAS[area] && AREAS[area].subs.length) state.sub = AREAS[area].subs[0][0];
  if (area !== 'write' || state.sub !== 'doc') state.noteId = state.sub === 'doc' ? state.noteId : null;
  render();
  const sc = $('.stage-scroll');
  if (sc) sc.scrollTop = 0;
}

const SCREENS = {
  'today': () => viewToday(),
  'write/notes': () => viewNotes(),
  'write/search': () => viewSearch(),
  'write/templates': () => viewTemplates(),
  'write/tags': () => viewTags(),
  'write/assets': () => viewAssets(),
  'write/tree': () => viewExcerptTree(),
  'write/doc': () => viewDoc(),
  'read/queue': () => viewReadingQueue(),
  'read/archive': () => viewArchive(),
  'recall/queue': () => viewRecallQueue(),
  'recall/cards': () => viewCards(),
  'recall/types': () => viewTypes(),
  'recall/relink': () => viewRelink(),
  'recall/presets': () => viewPresets(),
  'recall/history': () => viewHistory(),
  'stats/overview': () => viewStatsOverview(),
  'stats/heatmap': () => viewStatsHeatmap(),
  'stats/forecast': () => viewStatsForecast(),
  'settings/appearance': () => viewAppearance(),
  'settings/review': () => viewReviewSettings(),
  'settings/budget': () => viewBudget(),
  'settings/keys': () => viewKeys(),
  'settings/plugins': () => viewPlugins(),
  'settings/agent': () => viewAgent(),
  'settings/update': () => viewUpdate(),
  'settings/about': () => viewAbout(),
  'plugin': () => viewPluginPage(),
};

function renderScreen() {
  if (state.screen === 'welcome') return viewWelcome();
  if (state.area === 'plugin') return SCREENS['plugin']();
  const key = state.area === 'today' ? 'today' : `${state.area}/${state.sub}`;
  const fn = SCREENS[key] || SCREENS['today'];
  return fn();
}

/* ═══ 屏幕：欢迎 ══════════════════════════════════════════════════════════ */
function viewWelcome() {
  const err = state.welcomeState === 'error';
  const recent = [
    { p: '~/Documents/knowledge', bad: false },
    { p: '~/Documents/工作库', bad: true },
    { p: '~/Documents/读书', bad: false },
  ];
  return `
  <div class="view"><div class="welcome"><div class="welcome-card">
    <div class="welcome-brand">
      <span class="brand-mark"><span></span></span>
      <div><h1>Remit</h1><p>本地优先的记忆工作台 · 明文库，无账号</p></div>
    </div>

    ${err ? `<div class="banner banner--danger" style="margin-bottom:16px">
      ${icon('x', 14)} <span>库打不开：<span class="mono">~/Documents/工作库</span> 里没有 .remit 目录。</span>
      <span class="push"></span>
      <button class="btn btn--quiet" type="button" data-welcome="open" style="color:inherit">换一个</button>
    </div>` : ''}

    <div class="welcome-actions">
      <button class="welcome-action" type="button" data-act="enter"><span class="row-kind">${icon('layers', 15)}</span>
        <span class="wa-body"><span class="wa-t">打开库</span><span class="wa-s">选一个文件夹；索引可随时删掉重建</span></span>
        ${icon('chev', 16)}</button>
      <button class="welcome-action" type="button" data-act="enter"><span class="row-kind">${icon('plus', 15)}</span>
        <span class="wa-body"><span class="wa-t">新建库</span><span class="wa-s">建一个空目录，写入第一个 .md</span></span>
        ${icon('chev', 16)}</button>
      <button class="welcome-action" type="button" data-act="import"><span class="row-kind">${icon('upload', 15)}</span>
        <span class="wa-body"><span class="wa-t">导入 .remitlib / Markdown</span><span class="wa-s">整库归档，或一堆笔记文件</span></span>
        ${icon('chev', 16)}</button>
    </div>

    <p class="eyebrow" style="margin-bottom:8px">最近</p>
    <div class="recent">
      ${recent.map((r) => `<button class="recent-row" type="button" ${r.bad ? 'data-welcome="error"' : 'data-act="enter"'}>
        ${icon('book', 14)}<span class="rp">${esc(r.p)}</span>
        ${r.bad ? badge('打不开', 'danger') : ''}${icon('chev', 13)}</button>`).join('')}
    </div>

    <p class="dim" style="margin-top:20px;font-size:var(--t-cap);line-height:1.7">
      零遥测 · 零网络请求 · 库拷到另一台机器就能用</p>
  </div></div></div>`;
}

/* ═══ 屏幕：今日 ══════════════════════════════════════════════════════════ */
function viewToday() {
  const reading = readingQueue();
  const due = dueCards();
  const mins = Math.round(reading.length * 6 + due.length * 0.6);
  const pct = Math.min(100, Math.round((mins / state.budget.minutes) * 100));
  const unattached = NOTES.filter((n) => n.reading && n.reading.pos > 2 && !NOTES.some((e) => e.excerptOf === n.id));

  const toExcerpt = NOTES.filter((n) => n.reading && n.reading.pos > 2
    && !state.archived.has(n.id) && !state.dismissed.has(n.id)
    && !NOTES.some((e) => e.excerptOf === n.id));

  const rows = [
    ...reading.map((n) => ({ kind: 'read', id: n.id, title: n.title,
      sub: `第 ${n.reading.pos + 1} / ${n.blocks.length} 段 · ${n.edited === '刚加入' ? '刚加入队列' : '上次停在 ' + n.edited}`,
      tail: '约 6 分钟' })),
    ...toExcerpt.slice(0, 2).map((n) => ({ kind: 'excerpt', id: n.id, title: n.title,
      sub: `读到第 ${n.reading.pos + 1} / ${n.blocks.length} 段 · 还没有摘录`, tail: '约 2 分钟' })),
    { kind: 'review', id: '', title: '复习 · 全库',
      sub: `${due.length} 张待忆 · 其中 ${due.filter((c) => c.state === 'new').length} 张是新的`, tail: `约 ${Math.round(due.length * 0.6)} 分钟` },
  ];

  return page('m-paper', `
    ${pageHead('今日', '9月18日 周四 · 预算 ' + state.budget.minutes + ' 分钟',
      `<button class="btn" type="button" data-act="palette">${icon('search', 14)} 命令</button>`)}

    <div class="tile" style="margin-bottom:24px">
      <div class="od-row" style="justify-content:space-between">
        <span class="stat" style="padding:0"><span class="n tnum">${mins} 分钟</span>
          <span class="c">今天的事，按 ${state.budget.minutes} 分钟预算排</span></span>
        <span class="dim" style="font-size:var(--t-label)">${reading.length} 篇待读 · ${due.length} 张待忆</span>
      </div>
      <div style="margin-top:10px">${meter(pct)}</div>
    </div>

    <button class="row tile tile--link" type="button" data-note="nd" data-kind-row="daily"
      style="grid-template-columns:34px minmax(0,1fr) auto;margin-bottom:20px;border:1px solid var(--line);border-radius:var(--r-3);padding:12px 14px">
      <span class="row-kind">${icon('today', 14)}</span>
      <span class="body"><span class="title">今天 · 9 月 18 日</span>
        <span class="sub">读《怎样解题》第 4 段，摘了一句；复习 12 张，错 2 张</span></span>
      <span class="tail">打开 ${icon('chev', 14)}</span>
    </button>

    <div class="stat-row">
      ${stat(reading.length, '篇待读', '按到期先进先出')}
      ${stat(due.length, '张待忆', '卡片有到期日')}
      ${stat(state.reviewedToday, '今日已忆', '含阅读会话')}
    </div>

    <div class="section">
      <div class="section-head"><h2>队列</h2><span class="hint">读与忆混排，顺序由预算决定</span>
        <span class="push"></span>
        <button class="btn btn--quiet" type="button" data-act="start">${icon('chev', 14)} 开始</button></div>
      <ul class="list stagger">
        ${rows.map((r) => `<li><button class="row" type="button" data-queue="${r.kind}" data-id="${r.id}"
            style="grid-template-columns:34px minmax(0,1fr) auto">
            <span class="row-kind ${r.kind === 'read' ? 'row-kind--read' : r.kind === 'excerpt' ? 'row-kind--ex' : ''}">${r.kind === 'read' ? '读' : r.kind === 'excerpt' ? '摘' : '忆'}</span>
            <span class="body"><span class="title">${esc(r.title)}</span><span class="sub">${esc(r.sub)}</span></span>
            <span class="tail">${r.tail}${icon('chev', 14)}</span>
          </button></li>`).join('')}
      </ul>
    </div>

    ${unattached.length ? `<div class="section">
      <div class="section-head"><h2>读过但还没摘</h2><span class="hint">读完不等于记住</span></div>
      <ul class="list stagger">
        ${unattached.slice(0, 3).map((n) => `<li><button class="row" type="button" data-note="${n.id}" data-mode="read"
          style="grid-template-columns:34px minmax(0,1fr) auto">
          <span class="row-kind">${icon('quote', 14)}</span>
          <span class="body"><span class="title">${esc(n.title)}</span><span class="sub">读到第 ${n.reading.pos + 1} 段，还没有摘录</span></span>
          <span class="tail">打开 ${icon('chev', 14)}</span></button></li>`).join('')}
      </ul></div>` : ''}

    ${PLUGINS.filter((x) => x.on && x.menus && x.menus['today.widget']).map((x) => `
      <div class="section">
        <div class="section-head"><h2>${esc(x.name)}</h2><span class="hint">插件 · today.widget</span></div>
        <div class="tile" style="grid-template-columns:minmax(0,1fr) auto;align-items:center;display:grid">
          <div><b style="font-weight:500">把「三合一的取舍」摆到画布上</b>
            <p class="dim" style="font-size:var(--t-label);margin-top:3px">${esc(x.desc)}</p></div>
          ${x.menus['today.widget'].map((label, i) => `<button class="btn" type="button" data-act="pg:${x.id}:today.widget:${i}">${label}</button>`).join('')}
        </div>
      </div>`).join('')}

    <div class="section">
      <div class="section-head"><h2>明日</h2></div>
      <p class="muted" style="font-size:var(--t-label);line-height:1.7">
        1 篇到期 · 8 张到期。今天没做完的会顺延，不会堆成债。</p>
    </div>`);
}

/* ═══ 屏幕：写 ════════════════════════════════════════════════════════════ */
function viewNotes() {
  const filters = [['all', '全部'], ['reading', '在读'], ['cards', '有卡'], ['excerpt', '摘录']];
  const SORTS = { edited: '最近编辑', title: '标题', cards: '卡数' };
  const q = state.writeQuery.trim().toLowerCase();
  let list = NOTES.filter((n) => !n.template)
    .filter((n) => {
      if (state.writeFilter === 'reading') return !!n.reading;
      if (state.writeFilter === 'cards') return cardsOf(n.id).length > 0;
      if (state.writeFilter === 'excerpt') return !!n.excerptOf;
      return true;
    })
    .filter((n) => !state.tagFilter || n.tags.includes(state.tagFilter))
    .filter((n) => !q || n.title.toLowerCase().includes(q) || n.blocks.some((b) => b.text.toLowerCase().includes(q)));

  const key = state.noteSort;
  list = list.slice().sort((a, b) => {
    if (key === 'title') return a.title.localeCompare(b.title, 'zh');
    if (key === 'cards') return cardsOf(b.id).length - cardsOf(a.id).length;
    return 0;
  });

  return page('m-list', `
    ${pageHead('笔记', `${NOTES.filter((n) => !n.template).length} 篇 · ${CARDS.length} 张卡 · 明文目录`,
      `<button class="btn btn--primary" type="button" data-act="new-note">${icon('plus', 14)} 新建笔记</button>`)}

    <div class="toolbar">
      <label class="search grow">${icon('search', 16)}
        <input type="search" id="write-search" value="${esc(state.writeQuery)}" placeholder="按标题或正文搜索" aria-label="搜索笔记">
      </label>
      ${filters.map(([k, l]) => chip(l, state.writeFilter === k, `data-wfilter="${k}"`)).join('')}
      ${state.tagFilter ? chip('#' + state.tagFilter, true, 'data-act="clear-tag"') : ''}
      <span class="push"></span>
      <button class="btn btn--quiet" type="button" data-act="cycle-sort" title="切换排序">
        ${icon('filter', 14)} ${SORTS[key]}</button>
    </div>

    ${list.length ? `<ul class="list stagger">${list.map((n) => {
      const cards = cardsOf(n.id).length;
      return `<li><button class="row" type="button" data-note="${n.id}"
          style="grid-template-columns:minmax(0,1fr) auto">
          <span class="body">
            <span class="title">${esc(n.title)}${n.excerptOf ? ' <span class="badge badge--mark">摘录</span>' : ''}</span>
            <span class="sub">${esc(n.blocks[0] ? n.blocks[0].text : '')}</span>
          </span>
          <span class="tail">
            ${n.reading ? `<span class="badge badge--mark">第 ${n.reading.pos + 1}/${n.blocks.length} 段</span>` : ''}
            ${cards ? `<span class="badge badge--plain">${cards} 张卡</span>` : ''}
            <span class="dim">${esc(n.edited)}</span>
            <span class="hover-actions">
              <span class="icon-btn" data-note-open="${n.id}" data-mode="read" role="button" tabindex="0" aria-label="阅读">${icon('read', 16)}</span>
              <span class="icon-btn" data-note-menu="${n.id}" role="button" tabindex="0" aria-label="更多">${icon('more', 16)}</span>
            </span>
          </span>
        </button></li>`;
    }).join('')}</ul>` : empty('search', '没有匹配的笔记', '换个词，或者清空筛选。',
      `<button class="btn" type="button" data-act="clear-wfilter">清空筛选</button>`)}`);
}

function viewTags() {
  const counts = {};
  NOTES.filter((n) => !n.template).forEach((n) => n.tags.forEach((t) => { counts[t] = (counts[t] || 0) + 1; }));
  const rows = Object.entries(counts).sort((a, b) => b[1] - a[1]);
  return page('m-list', `
    ${pageHead('标签', `${rows.length} 个标签 · 标签属于笔记，不是容器`)}
    ${rows.length ? `<ul class="list stagger">${rows.map(([t, n]) => `
      <li><button class="row" type="button" data-tag="${esc(t)}" style="grid-template-columns:auto minmax(0,1fr) auto">
        <span class="row-kind">${icon('tag', 14)}</span>
        <span class="body"><span class="title">#${esc(t)}</span>
          <span class="sub">${NOTES.filter((x) => x.tags.includes(t) && !x.template).slice(0, 2).map((x) => x.title).join(' · ')}</span></span>
        <span class="tail">${n} 篇 ${icon('chev', 14)}</span></button></li>`).join('')}</ul>`
      : empty('tag', '还没有标签', '在笔记里加一个 #标签，它就会出现在这里。')}`);
}

function viewSearch() {
  const q = state.writeQuery.trim();
  const notes = q ? NOTES.filter((n) => n.title.includes(q) || n.blocks.some((b) => b.text.includes(q))) : [];
  const cards = q ? CARDS.filter((c) => c.prompt.includes(q) || c.answer.includes(q)) : [];

  return page('m-list', `
    ${pageHead('搜索', q ? `「${esc(q)}」的 ${notes.length + cards.length} 条结果` : '在笔记正文与卡片里找')}
    <div class="toolbar">
      <label class="search grow">${icon('search', 16)}
        <input type="search" id="search-page" value="${esc(q)}" placeholder="输入关键词，实时过滤" aria-label="搜索">
      </label>
    </div>
    ${q ? `
      <div class="section"><div class="section-head"><h2>笔记</h2><span class="hint">${notes.length} 篇</span></div>
        ${notes.length ? `<ul class="list stagger">${notes.map((n) => `<li><button class="row" type="button" data-note="${n.id}">
          <span class="body"><span class="title">${esc(n.title)}</span>
          <span class="sub">${esc(n.blocks[0].text)}</span></span>
          <span class="tail">${icon('chev', 14)}</span></button></li>`).join('')}</ul>`
          : `<p class="dim" style="font-size:var(--t-label);padding:12px 0">没有匹配的笔记。</p>`}
      </div>
      <div class="section"><div class="section-head"><h2>卡片</h2><span class="hint">${cards.length} 张</span></div>
        ${cards.length ? `<ul class="list stagger">${cards.map((c) => `<li><button class="row" type="button" data-card="${c.id}">
          <span class="body"><span class="title">${esc(c.prompt)}</span>
          <span class="sub">${esc(noteById(c.note).title)}</span></span>
          <span class="tail">${badge(c.type, 'plain')}${icon('chev', 14)}</span></button></li>`).join('')}</ul>`
          : `<p class="dim" style="font-size:var(--t-label);padding:12px 0">没有匹配的卡片。</p>`}
      </div>`
      : empty('search', '输入一个词', '标题、正文、卡片提示与答案都会参与匹配。')}`);
}

function viewTemplates() {
  const list = NOTES.filter((n) => n.template);
  return page('m-list', `
    ${pageHead('笔记模板', '带 #template 标签的普通笔记，只在新建时出现', `<button class="btn" type="button" data-act="new-note">${icon('plus', 14)} 空白笔记</button>`)}
    <ul class="list stagger">
      ${list.map((n) => `<li><button class="row" type="button" data-template="${n.id}" style="grid-template-columns:minmax(0,1fr) auto">
        <span class="body"><span class="title">${esc(n.title)}</span>
        <span class="sub">${n.blocks.length} 个块 · ${esc(n.blocks[0].text)}</span></span>
        <span class="tail">从模板新建 ${icon('chev', 14)}</span></button></li>`).join('')}
    </ul>
    <div class="section"><p class="dim" style="font-size:var(--t-label);line-height:1.7">
      副本会剥掉 #template 标记，块 ID 重新铸造——所以模板本体可以无限次开工。</p></div>`);
}

/* ═══ 屏幕：文档 ══════════════════════════════════════════════════════════ */
function viewDoc() {
  const note = noteById(state.noteId) || NOTES[0];
  if (!note) return viewNotes();
  const isRead = state.docMode === 'read';

  return `<div class="view">
    ${state.findOpen ? `<div class="findbar">
      <label class="search" style="flex:1">${icon('search', 15)}
        <input type="search" placeholder="在这篇笔记里查找" aria-label="查找"></label>
      <span class="dim" style="font-size:var(--t-cap)">0 个结果</span>
      <button class="icon-btn" type="button" data-act="toggle-find" aria-label="关闭查找">${icon('x', 16)}</button>
    </div>` : ''}
    <div class="doc" data-mode="${state.docMode}" data-panel="${state.panel ? 'on' : 'off'}">
      <div class="doc-main" id="doc-main">
        <div class="paper">
          <div class="title-input" contenteditable="${isRead ? 'false' : 'true'}" spellcheck="false"
               data-note-title data-ph="无标题">${esc(note.title)}</div>
          ${note.blocks.map((b, i) => blockHTML(note, b, i, isRead)).join('')}
        </div>
      </div>
      ${state.docMode === 'split' ? `<div class="split-pane od-scroll" id="split-pane">
        <div class="paper">${note.blocks.map((b) => `<div class="block" data-kind="${b.kind}">
          <div class="block-body">${inline(b.text)}</div></div>`).join('')}</div>
      </div>` : ''}
      ${state.panel && state.docMode !== 'split' ? panelHTML(note) : ''}
    </div>
  </div>`;
}

function blockHTML(note, b, i, isRead) {
  const cards = cardsOn(note.id, b.id);
  const ex = excerptedFrom(note.id, b.id);
  return `
  <div class="block${state.currentBlock === b.id ? ' is-current' : ''}" data-kind="${b.kind}" data-block="${b.id}">
    <span class="block-num">${i + 1}</span>
    <div class="gutter">
      <div class="g-marks">
        ${cards.length ? `<button class="badge-card" type="button" data-inspect="${b.id}"
          data-state="${cardsState(cards)}" title="${cards.length} 张卡：${cardStateWords(cards)}"
          aria-label="${cards.length} 张卡，${cardStateWords(cards)}">${cardsState(cards) === 'lost' ? '!' : ''}${cards.length}</button>` : ''}
        ${ex ? `<span class="badge-ex" title="已摘录">${icon('quote', 10)}</span>` : ''}
      </div>
      ${isRead ? '' : `<div class="g-tools">
        <button class="g-btn" type="button" data-act="insert-after" data-block="${b.id}" aria-label="在下面插入">${icon('plus', 12)}</button>
        <button class="g-btn" type="button" data-act="block-menu" data-block="${b.id}" aria-label="段落菜单">${icon('grip', 12)}</button>
      </div>`}
    </div>
    <div class="block-body" contenteditable="${isRead ? 'false' : 'true'}" spellcheck="false"
      role="textbox" aria-multiline="true" aria-label="第 ${i + 1} 段" data-body="${b.id}">${inline(b.text)}</div>
  </div>`;
}

function panelHTML(note) {
  const heads = note.blocks.map((b, i) => ({ ...b, i })).filter((b) => b.kind === 'h2' || b.kind === 'h3');
  const backs = NOTES.filter((n) => n.excerptOf === note.id);
  return `
  <aside class="panel" aria-label="大纲与反链">
    <div class="panel-head">
      ${seg([['outline', '大纲'], ['backlinks', `反链 ${backs.length || ''}`]], state.panelTab, 'paneltab')}
      <span class="push" style="margin-left:auto"></span>
      <button class="icon-btn" type="button" data-act="toggle-panel" aria-label="收起面板">${icon('chev', 16)}</button>
    </div>
    <div class="panel-body">
      ${state.panelTab === 'outline'
        ? (heads.length ? heads.map((h) => `<button class="outline-link ${h.kind === 'h3' ? 'lv3' : ''}"
            type="button" data-jump="${h.id}" aria-current="${state.currentBlock === h.id}">${esc(h.text)}</button>`).join('')
          : `<p class="panel-empty">这篇笔记还没有标题。输入 / 可以插入。</p>`)
        : (backs.length ? backs.map((n) => `<button class="backlink" type="button" data-note="${n.id}">
            <span class="bt">${esc(n.title)}</span>
            <span class="bx">${esc(n.blocks[0] ? n.blocks[0].text : '')}</span>
            <span class="bx">${cardsOf(n.id).length} 张卡 · ${NOTES.filter((x) => x.excerptOf === n.id).length ? '已再摘' : '可再摘'}</span></button>`).join('')
          : `<p class="panel-empty">还没有笔记摘录这篇。读到值得留下的句子时按 E。</p>`)}
    </div>
  </aside>`;
}

function renderStatus() {
  const bar = $('#statusbar');
  const inDoc = state.area === 'write' && state.sub === 'doc' && state.screen === 'app';
  if (!inDoc) { bar.hidden = true; return; }
  const note = noteById(state.noteId);
  if (!note) { bar.hidden = true; return; }
  const chars = note.blocks.reduce((n, b) => n + b.text.replace(/\s/g, '').length, 0);
  bar.hidden = false;
  const idx = note.blocks.findIndex((b) => b.id === state.currentBlock) + 1;
  bar.innerHTML = `<span class="strong">${chars.toLocaleString()} 字</span><span class="d"></span>
    <span>${note.blocks.length} 段</span><span class="d"></span>
    <span>${cardsOf(note.id).length} 张卡</span><span class="d"></span>
    <span>已保存</span><span class="d"></span>
    <span>${note.tags.map((t) => '#' + t).join(' ') || '无标签'}</span><span class="d"></span>
    <span>${idx > 0 ? '第 ' + idx + ' 段' : ''}</span>`;
}

/* ═══ 屏幕：读 ════════════════════════════════════════════════════════════ */
function viewReadingQueue() {
  const q = readingQueue();
  return page('m-list', `
    ${pageHead('阅读队列', '按到期先进先出；读完自动移出今天',
      `<button class="btn" type="button" data-act="add-reading">${icon('plus', 14)} 把笔记加进队列</button>
       <button class="btn btn--primary" type="button" data-act="start-reading" ${q.length ? '' : 'disabled'}>开始阅读</button>`)}
    ${q.length ? `<ul class="list stagger">${q.map((n) => {
      const pct = Math.round(((n.reading.pos + 1) / n.blocks.length) * 100);
      return `<li><button class="row" type="button" data-reading="${n.id}" style="grid-template-columns:minmax(0,1fr) 120px auto">
        <span class="body"><span class="title">${esc(n.title)}</span>
          <span class="sub">第 ${n.reading.pos + 1} / ${n.blocks.length} 段 · ${esc(n.edited)}</span></span>
        <span>${meter(pct)}</span>
        <span class="tail">继续 ${icon('chev', 14)}</span></button></li>`;
    }).join('')}</ul>` : empty('check', '今天的阅读队列空了', '读过的笔记会按 FSRS 到期回来；也可以把任意笔记手动加进队列。',
      `<button class="btn" type="button" data-act="add-reading">加入一篇</button>`)}
    <div class="section"><div class="section-head"><h2>节奏</h2></div>
      <div class="stat-row">${stat(q.length, '篇今日待读')}${stat(NOTES.filter((n) => n.reading).length, '篇在队列')}${stat(3, '档消化难度', '太难 / 刚好 / 太浅')}</div>
    </div>`);
}

/* ═══ 屏幕：忆 ════════════════════════════════════════════════════════════ */
/* ═══ 屏幕：忆 ════════════════════════════════════════════════════════════ */

const STATE_LABEL = { new: '新', learning: '学习中', review: '复习中' };
const REF_META = { live: ['跟随', 'ok'], frozen: ['快照', 'warn'], lost: ['已断', 'danger'] };

// 调度数值由卡片 id 派生：同一张卡每次打开都一样
function cardMetrics(card) {
  const h = [...card.id].reduce((a, ch) => (a * 31 + ch.charCodeAt(0)) % 997, 7);
  const lapses = h % 5;
  const interval = [1, 2, 4, 9, 17, 30, 62, 120][h % 8];
  const stability = (2 + (h % 40)) / 1.6;
  return { lapses, interval, stability: stability.toFixed(1), ease: (2.1 + (h % 13) / 10).toFixed(2) };
}

// 难卡 = 败笔次数达到阈值的卡。它是 FSRS 的一个视图，不是一种卡片类型。
const HARD_LAPSES = 3;
const isHardCard = (c) => cardMetrics(c).lapses >= HARD_LAPSES && c.state !== 'suspended';
const hardCards = (list = CARDS) => list.filter(isHardCard);

const stateCount = (list, s) => list.filter((c) => c.state === s).length;

const FILTER_KEYS = { '全部': 'all', '今天到期': 'due', '新': 'new', '学习中': 'learning', '断链': 'lost', '难卡': 'hard' };

function filterCards(list) {
  const f = state.cardFilter;
  if (f === 'due') return list.filter((c) => c.due === '今天');
  if (['new', 'learning', 'review'].includes(f)) return list.filter((c) => c.state === f);
  if (f === 'lost') return list.filter((c) => c.ref === 'lost');
  if (f === 'hard') return list.filter(isHardCard);
  return list;
}

function viewRecallQueue() {
  const due = filterCards(dueCards());
  const fresh = stateCount(due, 'new');
  const learning = stateCount(due, 'learning');
  const reviewing = stateCount(due, 'review');
  const total = Math.max(1, due.length);
  const byNote = {};
  due.forEach((c) => { byNote[c.note] = (byNote[c.note] || 0) + 1; });

  return page('m-list', `
    ${pageHead('复习队列', `${due.length} 张今天到期 · 预算 ${state.budget.cards} 张`,
      `<button class="btn" type="button" data-act="open-scope">${icon('filter', 14)} ${state.reviewScope.kind === 'all' ? '全库' : state.reviewScope.kind === 'note' ? '某篇笔记' : '#' + state.reviewScope.value}</button>
       <button class="btn btn--primary" type="button" data-act="review-due" ${due.length ? '' : 'disabled'}>开始复习</button>`)}

    <div class="tile" style="margin-bottom:20px">
      <div class="od-row" style="justify-content:space-between;margin-bottom:10px">
        <span class="stat" style="padding:0"><span class="n tnum">${Math.round(due.length * 0.6)} 分钟</span>
          <span class="c">今天的复习量</span></span>
        <span class="dim" style="font-size:var(--t-label)">新 ${fresh} · 学习中 ${learning} · 复习 ${reviewing}</span>
      </div>
      <div class="ratio" role="img" aria-label="新 ${fresh}，学习中 ${learning}，复习 ${reviewing}">
        <i data-k="read" style="--w:${Math.round(fresh / total * 100)}%"></i>
        <i data-k="recall" style="--w:${Math.round(learning / total * 100)}%"></i>
        <i data-k="excerpt" style="--w:${Math.round(reviewing / total * 100)}%"></i>
      </div>
      <div class="ratio-legend">
        <span><i data-k="read"></i>新 <b>${fresh}</b></span>
        <span><i data-k="recall"></i>学习中 <b>${learning}</b></span>
        <span><i data-k="excerpt"></i>复习 <b>${reviewing}</b></span>
        <span class="push" style="margin-left:auto"><span class="dim">超预算的顺延到明天</span></span>
      </div>
    </div>

    <div class="toolbar">
      ${['全部', '今天到期', '新', '学习中', '断链', '难卡'].map((l) =>
        chip(l, state.cardFilter === FILTER_KEYS[l], `data-qfilter="${l}"`)).join('')}
      <span class="push"></span>
      <button class="btn btn--quiet" type="button" data-act="save-search">${icon('plus', 14)} 保存为筛选</button>
    </div>

    ${state.saved.length ? `<div class="saved-row">
      ${state.saved.map((x) => `<button class="chip" type="button" data-saved="${x.id}" aria-pressed="false">
        ${icon('filter', 11)} ${esc(x.label)}</button>`).join('')}
    </div>` : ''}

    <div class="section"><div class="section-head"><h2>按笔记分</h2><span class="hint">范围靠笔记组织，不引入 deck</span></div>
      <ul class="list stagger">${Object.entries(byNote).map(([id, n]) => {
        const note = noteById(id);
        const l = due.filter((c) => c.note === id && isHardCard(c)).length;
        return `<li><button class="row" type="button" data-note="${id}" style="grid-template-columns:minmax(0,1fr) auto">
          <span class="body"><span class="title">${esc(note.title)}</span>
            <span class="sub">${note.blocks.length} 段 · ${cardsOf(id).length} 张卡${l ? ` · ${l} 张反复答错` : ''}</span></span>
          <span class="tail">${n} 张到期 ${icon('chev', 14)}</span></button></li>`;
      }).join('')}</ul>
    </div>

    <div class="section"><div class="section-head"><h2>最近 26 周</h2></div>
      <div class="heat" id="heat-mini" role="img" aria-label="复习热力图"></div></div>`);
}

function viewPresets() {
  const steps = ['1 分钟', '10 分钟', '1 天'];
  return page('m-settings', `
    ${pageHead('调度预设', 'FSRS 算间隔；这里只改用户的偏好', `<button class="btn" type="button" data-act="reset-preset">恢复默认</button>`)}

    <section class="group">
      <h2>学习步</h2><p class="lead">新卡先走这几步，全部走完才进入长期调度。</p>
      <div class="field"><div><span class="fl">步骤</span><span class="fh">按顺序执行；最后一步之后交给 FSRS。</span></div>
        <div class="fc">${steps.map((t) => `<span class="chip" aria-pressed="true">${t}</span>`).join('')}
          <button class="btn btn--quiet" type="button" data-act="add-step">${icon('plus', 13)} 加一步</button></div></div>
      <div class="field"><div><span class="fl">毕业间隔</span><span class="fh">走完学习步后第一次正式间隔。</span></div>
        <div class="fc"><div class="range"><input type="range" min="1" max="10" value="4"><output>4 天</output></div></div></div>
    </section>

    <section class="group">
      <h2>每日上限</h2>
      <div class="field"><div><span class="fl">新卡</span><span class="fh">超出的新卡不会在今天出现。</span></div>
        <div class="fc"><div class="range"><input type="range" id="budget-cards" min="5" max="60" step="5" value="${state.budget.cards}"><output>${state.budget.cards} 张</output></div></div></div>
      <div class="field"><div><span class="fl">复习</span><span class="fh">与预算共用同一个上限。</span></div>
        <div class="fc"><div class="range"><input type="range" min="20" max="300" step="10" value="120"><output>120 张</output></div></div></div>
    </section>

    <section class="group">
      <h2>失败</h2>
      <div class="field"><div><span class="fl">败笔后的重学间隔</span><span class="fh">按「重来」之后这张卡多久再出现。</span></div>
        <div class="fc"><span class="chip" aria-pressed="true">10 分钟</span><span class="chip">1 天</span></div></div>
      <div class="field"><div><span class="fl">难卡阈值</span><span class="fh">连续几次「重来」算难卡。</span></div>
        <div class="fc"><div class="range"><input type="range" min="2" max="8" value="3"><output>3 次</output></div></div></div>
    </section>

    <section class="group">
      <h2>断链</h2>
      <div class="field"><div><span class="fl">块被删掉时</span><span class="fh">卡片会失去来源，但复习历史保留。</span></div>
        <div class="fc">${seg([['keep', '保留为断链'], ['freeze', '自动转快照']], 'keep', 'brokenpref')}</div></div>
    </section>`);
}

const REVIEW_LOG = [
  ['今天 09:12', '良好', '间隔 4 天 → 9 天', 'review', '三合一的取舍'],
  ['今天 09:13', '重来', '回到 10 分钟后', 'fail', '三合一的取舍'],
  ['今天 09:14', '良好', '间隔 9 天 → 21 天', 'review', '增量阅读'],
  ['今天 09:16', '容易', '间隔 21 天 → 47 天', 'review', 'FSRS 的四个评分'],
  ['今天 09:18', '重来', '回到 10 分钟后', 'fail', '块 ID 与引用'],
  ['今天 09:22', '良好', '间隔 2 天 → 5 天', 'review', '摘录怎么写才有用'],
  ['今天 09:25', '困难', '间隔 5 天 → 6 天', 'review', '本地优先的代价'],
];

function viewHistory() {
  const grades = { 1: 2, 2: 2, 3: 8, 4: 3 };
  const max = Math.max(...Object.values(grades));
  const labels = { 1: '重来', 2: '困难', 3: '良好', 4: '容易' };
  return page('m-list', `
    ${pageHead('历史', '每一次评分都进 reviews.jsonl；这一屏只是它的一个视图')}

    <div class="stat-row">
      ${stat(15, '今天评过', '含阅读三档')}${stat('11 分钟', '用时')}${stat('2', '重来')}${stat('87%', '今日正确率')}
    </div>

    <div class="section">
      <div class="section-head"><h2>评分分布</h2><span class="hint">今天</span></div>
      <div class="dist">
        ${[1, 2, 3, 4].map((g) => `<div class="dist-row">
          <span class="dist-label">${labels[g]}</span>
          <span class="dist-bar"><i data-g="${g}" style="--w:${Math.round(grades[g] / max * 100)}%"></i></span>
          <span class="dist-num">${grades[g]}</span></div>`).join('')}
      </div>
    </div>

    <div class="section">
      <div class="section-head"><h2>今天</h2><span class="hint">按时间</span></div>
      <div class="timeline">
        ${REVIEW_LOG.map(([at, what, meta, kind, note]) => `<div class="ev" data-kind="${kind === 'fail' ? 'review' : kind}">
          <time>${at}</time><span class="node"></span>
          <span><span class="what">${what} · ${esc(note)}</span><span class="meta">${meta}</span></span>
        </div>`).join('')}
      </div>
    </div>

    <div class="section">
      <div class="section-head"><h2>本周</h2></div>
      <div class="table-wrap"><table class="grid">
        <colgroup><col style="width:86px"><col><col style="width:88px"><col style="width:88px"></colgroup>
        <thead><tr><th>日</th><th>内容</th><th>复习</th><th>正确率</th></tr></thead>
        <tbody>
          ${[['周三', '增量阅读 · 3 张', '18', '89%'], ['周二', 'FSRS · 4 张', '24', '92%'], ['周一', '摘录 · 2 张', '17', '88%']]
            .map(([d, w, n, r]) => `<tr><td class="ink">${d}</td><td>${w}</td><td>${n}</td><td>${r}</td></tr>`).join('')}
        </tbody></table></div>
    </div>`);
}

function parseQuery(raw) {
  const out = { text: '', is: [], type: '', note: '', tag: '' };
  for (const tok of raw.trim().split(/\s+/).filter(Boolean)) {
    const m = tok.match(/^(is|type|note|tag):(.+)$/i);
    if (!m) { out.text += (out.text ? ' ' : '') + tok; continue; }
    const k = m[1].toLowerCase(), v = m[2].toLowerCase();
    if (k === 'is') out.is.push(v); else out[k] = v;
  }
  return out;
}

function applyQuery(list, q) {
  let out = list;
  if (q.text) {
    const t = q.text.toLowerCase();
    out = out.filter((c) => c.prompt.toLowerCase().includes(t) || c.answer.toLowerCase().includes(t)
      || noteById(c.note).title.toLowerCase().includes(t));
  }
  if (q.type) out = out.filter((c) => c.type === q.type);
  if (q.note) out = out.filter((c) => noteById(c.note).title.toLowerCase().includes(q.note));
  if (q.tag) out = out.filter((c) => (c.tags || []).some((x) => x.toLowerCase().includes(q.tag)));
  for (const v of q.is) {
    if (['new', 'learning', 'review', 'suspended'].includes(v)) out = out.filter((c) => c.state === v);
    else if (v === 'lost') out = out.filter((c) => c.ref === 'lost');
    else if (v === 'hard') out = out.filter(isHardCard);
    else if (v === 'due') out = out.filter((c) => c.due === '今天');
  }
  return out;
}

function viewCards() {
  const q = parseQuery(state.cardQuery || '');
  let list = filterCards(CARDS.slice());
  list = applyQuery(list, q);

  const { key, dir } = state.cardSort;
  const getter = {
    prompt: (x) => x.prompt,
    note: (x) => noteById(x.note).title,
    type: (x) => x.type,
    state: (x) => x.state,
    due: (x) => x.due,
    interval: (x) => cardMetrics(x).interval,
    lapses: (x) => cardMetrics(x).lapses,
  }[key] || (() => '');
  list = list.slice().sort((a, b) => (typeof getter(a) === 'number'
    ? getter(a) - getter(b)
    : String(getter(a)).localeCompare(String(getter(b)))) * dir);

  const cols = state.cardCols;
  const th = (k, l, extra = '') => `<th ${extra}><button type="button" data-csort="${k}">${l}${state.cardSort.key === k ? (dir > 0 ? ' ↑' : ' ↓') : ''}</button></th>`;
  const group = (title, rows) => `<div class="filter-group"><span class="eyebrow">${title}</span>
    ${rows.map(([key2, label, active, attr, n]) => `
      <button class="filter-item" type="button" ${attr} aria-pressed="${active}">
        <span>${label}</span><span class="fi-n">${n}</span></button>`).join('')}</div>`;

  const tokens = [
    ...q.is.map((v) => `<span class="tok">is:${v}</span>`),
    q.type ? `<span class="tok">type:${esc(q.type)}</span>` : '',
    q.tag ? `<span class="tok">tag:${esc(q.tag)}</span>` : '',
    q.note ? `<span class="tok">note:${esc(q.note)}</span>` : '',
  ].filter(Boolean).join('');

  return page('m-wide', `
    ${pageHead('卡片', `${CARDS.length} 张 · ${dueCards().length} 张到期 · ${hardCards().length} 张难卡`,
      `<button class="btn" type="button" data-act="new-type">${icon('plus', 14)} 新建类型</button>`)}

    <div class="toolbar">
      <label class="search grow">${icon('search', 16)}
        <input type="search" id="card-search" value="${esc(state.cardQuery)}"
          placeholder="搜索，或用 is: / type: / tag: / note:" aria-label="搜索卡片"></label>
      <button class="icon-btn" type="button" data-act="search-help" aria-label="搜索语法">${icon('quote', 16)}</button>
      <span class="push"></span>
      <button class="btn btn--quiet" type="button" data-act="card-cols">${icon('columns', 14)} 列</button>
      <button class="btn btn--quiet" type="button" data-act="browse-plugin" ${pluginMenu('browse.row').length ? '' : 'disabled'}>插件动作</button>
    </div>
    ${tokens ? `<div class="query-tokens">${tokens}<span class="dim">${list.length} 条命中</span></div>` : ''}

    <div class="split-2 split-2--filter">
      <aside class="side-filters" aria-label="筛选">
        ${group('状态', [
          ['all', '全部', state.cardFilter === 'all', 'data-cfilter="all"', CARDS.length],
          ['new', '新', state.cardFilter === 'new', 'data-cfilter="new"', stateCount(CARDS, 'new')],
          ['learning', '学习中', state.cardFilter === 'learning', 'data-cfilter="learning"', stateCount(CARDS, 'learning')],
          ['review', '复习中', state.cardFilter === 'review', 'data-cfilter="review"', stateCount(CARDS, 'review')],
          ['lost', '断链', state.cardFilter === 'lost', 'data-cfilter="lost"', CARDS.filter((c) => c.ref === 'lost').length],
          ['hard', '难卡', state.cardFilter === 'hard', 'data-cfilter="hard"', hardCards().length],
        ])}
        ${group('类型', cardTypes().map((t) => [t.id, t.name, state.typeFilter === t.id, `data-typefilter="${t.id}"`, CARDS.filter((c) => c.type === t.id).length]))}
        ${group('引用', [
          ['live', '跟随块', state.refFilter === 'live', 'data-reffilter="live"', CARDS.filter((c) => c.ref === 'live').length],
          ['frozen', '快照', state.refFilter === 'frozen', 'data-reffilter="frozen"', CARDS.filter((c) => c.ref === 'frozen').length],
          ['lost', '已断', state.refFilter === 'lost', 'data-reffilter="lost"', CARDS.filter((c) => c.ref === 'lost').length],
        ])}
      </aside>

      <div class="pane">
        <div class="table-wrap">
          <table class="grid">
            <colgroup><col style="width:40px"><col><col style="width:168px"><col style="width:78px"><col style="width:74px">
              ${cols.interval ? '<col style="width:68px">' : ''}${cols.lapses ? '<col style="width:64px">' : ''}<col style="width:62px"></colgroup>
            <thead><tr><th><input type="checkbox" class="check" id="check-all" aria-label="全选"></th>
              ${th('prompt', '提示')}${th('note', '笔记')}${th('type', '类型')}${th('state', '状态')}
              ${cols.interval ? th('interval', '间隔') : ''}${cols.lapses ? th('lapses', '重来') : ''}${th('due', '到期')}</tr></thead>
            <tbody>${list.map((c) => {
              const m = cardMetrics(c);
              return `<tr data-card-row="${c.id}" aria-selected="${state.selected.has(c.id)}">
              <td><input type="checkbox" class="check" data-pick="${c.id}" ${state.selected.has(c.id) ? 'checked' : ''} aria-label="选择"></td>
              <td class="ink"><span class="od-truncate">${esc(c.prompt)}</span></td>
              <td><span class="od-truncate">${esc(noteById(c.note).title)}</span></td>
              <td>${badge(c.type, 'plain')}</td>
              <td>${STATE_LABEL[c.state] || c.state}${c.ref !== 'live' ? ' ' + badge(REF_META[c.ref][0], REF_META[c.ref][1]) : ''}</td>
              ${cols.interval ? `<td class="tnum">${m.interval} 天</td>` : ''}
              ${cols.lapses ? `<td class="tnum" style="color:${m.lapses >= HARD_LAPSES ? 'var(--danger)' : 'inherit'}">${m.lapses}</td>` : ''}
              <td style="color:${c.due === '今天' ? 'var(--mark)' : 'inherit'}">${c.due}</td>
            </tr>`;
            }).join('')}</tbody>
          </table>
          ${list.length ? '' : `<div class="empty empty--inset"><span class="eicon">${icon('search', 18)}</span>
            <h3>没有命中的卡片</h3><p>左侧换个筛选，或者清空搜索词。支持 is:new、is:hard、type:cloze、tag:、note:。</p></div>`}
        </div>

        <div class="toolbar" style="margin-top:12px">
          <span class="dim" style="font-size:var(--t-label)" id="sel-count">${state.selected.size ? `已选 ${state.selected.size} 张` : `${list.length} 张`}</span>
          <span class="push"></span>
          <button class="btn btn--quiet" type="button" data-act="bulk" ${state.selected.size ? '' : 'disabled'}>批量动作 ${state.selected.size ? `（${state.selected.size}）` : ''}</button>
          <button class="btn btn--primary" type="button" data-act="review-selected" ${state.selected.size ? '' : 'disabled'}>复习所选</button>
        </div>
      </div>
    </div>`);
}

function typePreview(t) {
  const sample = { 提示: '间隔重复的最小单位是什么？', 答案: '一个可独立回忆的问答对。',
    正面: 'FSRS', 背面: '自由间隔重复调度器', 正文: '读完之后什么也没留下', 术语: '难卡',
    定义: '连续多次答错的卡片', 反例: '只错一次的新卡' };
  const fill = (tpl) => esc(tpl.replace(/\{\{cloze:(.+?)\}\}/g, '〔$1〕')
    .replace(/\{\{(.+?)\}\}/g, (_, k) => sample[k.trim()] || '（' + k.trim() + '）')).replace(/\n/g, '<br>');
  return `<div class="preview-face"><span class="eyebrow">正面</span><p>${fill(t.front)}</p></div>
    <div class="preview-face"><span class="eyebrow">背面</span><p>${fill(t.back)}</p></div>`;
}

function viewTypes() {
  const sel = typeById(state.typeSel) || cardTypes()[0];
  return page('m-wide', `
    ${pageHead('卡片类型', '类型是库资产：字段表 + 正反两段模板，随库归档',
      `<button class="btn btn--primary" type="button" data-act="new-type">${icon('plus', 14)} 新建类型</button>`)}

    <div class="split-2 split-2--filter">
      <aside class="side-filters" aria-label="类型">
        <div class="filter-group">
          ${cardTypes().map((t) => `<button class="filter-item" type="button" data-typesel="${t.id}" aria-pressed="${sel.id === t.id}">
            <span>${esc(t.name)}</span><span class="fi-n">${CARDS.filter((c) => c.type === t.id).length}</span></button>`).join('')}
        </div>
        <p class="dim" style="font-size:var(--t-caption);line-height:1.6;margin-top:10px">
          内置三个类型随库种入，没有特权路径。</p>
      </aside>

      <div class="pane">
        <div class="od-row" style="justify-content:space-between;margin-bottom:10px">
          <div><b style="font-size:16px">${esc(sel.name)} ${sel.builtin ? badge('内置', 'plain') : badge('插件 · ' + (PLUGINS.find((p) => p.id === sel.plugin) || {}).name, 'mark')}</b>
            <p class="dim" style="font-size:var(--t-label);margin-top:2px">${esc(sel.desc)}</p></div>
          <button class="btn" type="button" data-typeedit="${sel.id}">编辑模板</button>
        </div>

        <div class="section" style="margin-top:0">
          <div class="section-head"><h2>字段</h2><span class="hint">${sel.fields.length} 个</span></div>
          <div class="od-cluster">${sel.fields.map((f) => `<span class="chip" aria-pressed="true">${esc(f)}</span>`).join('')}</div>
        </div>

        <div class="section">
          <div class="section-head"><h2>模板</h2><span class="hint">纯数据方言</span></div>
          <div class="tpl">
            <span class="eyebrow">正面</span><code>${esc(sel.front)}</code>
            <span class="eyebrow">背面</span><code>${esc(sel.back)}</code>
          </div>
        </div>

        <div class="section">
          <div class="section-head"><h2>预览</h2><span class="hint">用样例库里的值渲染</span></div>
          ${typePreview(sel)}
        </div>

        <div class="section">
          <div class="section-head"><h2>回退</h2></div>
          <p class="dim" style="font-size:var(--t-label);line-height:1.7">
            模板坏掉时回退成字段原文列表，不出白卡；用户改过的字段值才存覆盖。</p>
        </div>
      </div>
    </div>`);
}

/* ═══ 屏幕：统计 ══════════════════════════════════════════════════════════ */
const RETENTION = [78, 80, 79, 82, 84, 83, 85, 84, 86, 87, 86, 88, 87, 89, 88, 90, 89, 90, 91, 90, 91, 92, 91, 92, 93, 91];
const FORECAST = [12, 18, 9, 14, 22, 30, 41, 64, 88, 120, 96, 150, 210, 260];

function seriesPath() {
  return RETENTION.map((v, i) => ({ x: (i / (RETENTION.length - 1)) * 620 + 10, y: 128 - ((v - 70) / 30) * 104 }));
}
function retentionSVG() {
  const pts = seriesPath();
  const line = pts.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ');
  const area = `M${pts[0].x.toFixed(1)} 128 ` + pts.map((p) => `L${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ') + ` L${pts[pts.length - 1].x.toFixed(1)} 128 Z`;
  return `<svg class="chart" viewBox="0 0 640 150" role="img" aria-label="26 周保留率">
    <line class="axis" x1="0" y1="128" x2="640" y2="128"/>
    <path class="area" d="${area}"/><path class="line" d="${line}"/>
    <text class="lbl" x="0" y="145">半年</text><text class="lbl" x="596" y="145">本周</text></svg>`;
}
function forecastSVG() {
  const max = Math.max(...FORECAST), bw = 640 / FORECAST.length - 8;
  return `<svg class="chart" viewBox="0 0 640 150" role="img" aria-label="未来 14 天到期">
    ${FORECAST.map((v, i) => {
      const h = (v / max) * 104, x = i * (640 / FORECAST.length) + 4;
      return `<rect class="bar${i > 2 ? ' bar-dim' : ''}" x="${x.toFixed(1)}" y="${(124 - h).toFixed(1)}" width="${bw.toFixed(1)}" height="${h.toFixed(1)}" rx="3"/>
        ${i % 4 === 0 ? `<text class="lbl" x="${(x + bw / 2).toFixed(1)}" y="143" text-anchor="middle">${i === 0 ? '今天' : '+' + i + '天'}</text>` : ''}`;
    }).join('')}</svg>`;
}

function viewStatsOverview() {
  return page('m-paper', `
    ${pageHead('统计', '复习与阅读共用一份调度史')}
    <div class="stat-row">${stat('91%', '保留率', '目标 90%')}${stat('12', '今日到期')}${stat('3', '篇待读')}${stat(dueCards().length, '今日卡片')}</div>

    <div class="section">
      <div class="section-head"><h2>产出</h2><span class="hint">不只统计「忆」</span></div>
      <div class="stat-row">
        ${stat(NOTES.reduce((n, x) => n + x.blocks.reduce((m, b) => m + b.text.replace(/\s/g, '').length, 0), 0).toLocaleString(), '字', '写下来的')}
        ${stat(NOTES.filter((n) => n.excerptOf).length, '条摘录', '每条平均 ' + (CARDS.filter((c) => noteById(c.note) && noteById(c.note).excerptOf).length / Math.max(1, NOTES.filter((n) => n.excerptOf).length)).toFixed(1) + ' 张卡')}
        ${stat(CARDS.length, '张卡片', '来自 ' + new Set(CARDS.map((c) => c.note)).size + ' 篇笔记')}
      </div>
    </div>

    <div class="section"><div class="section-head"><h2>复习与阅读</h2><span class="hint">过去 26 周</span></div>
      <div class="heat" id="heat-mini" role="img" aria-label="热力图"></div>
      <div class="heat-legend"><span>少</span>${[0,1,2,3,4].map((l) => `<i data-lv="${l}"></i>`).join('')}<span>多</span></div>
    </div>

    <div class="section" id="ratio-7d">
      <div class="section-head"><h2>时间配比</h2><span class="hint">过去 7 天 · 读 / 忆 / 摘</span></div>
      <div class="ratio" role="img" aria-label="过去 7 天：读 58%，忆 34%，摘 8%">
        <i data-k="read" style="--w:58%"></i><i data-k="recall" style="--w:34%"></i><i data-k="excerpt" style="--w:8%"></i>
      </div>
      <div class="ratio-legend">
        <span><i data-k="read"></i>读 <b>58%</b></span>
        <span><i data-k="recall"></i>忆 <b>34%</b></span>
        <span><i data-k="excerpt"></i>摘 <b>8%</b></span>
      </div>
    </div>

    <div class="section"><div class="section-head"><h2>保留率</h2><span class="push"></span><span class="stat"><span class="n" style="font-size:var(--t-h2)">91%</span></span></div>
      ${retentionSVG()}</div>

    <div class="section"><div class="section-head"><h2>预测</h2><span class="hint">未来 14 天</span></div>
      ${forecastSVG()}</div>

    ${PLUGINS.filter((x) => x.on && x.menus && x.menus['stats.block']).map((x) => `
      <div class="section">
        <div class="section-head"><h2>${esc(x.name)}</h2><span class="hint">插件 · stats.block</span></div>
        <ul class="list" style="border-top:0">
          ${['三合一的取舍 · 4 张', '增量阅读 · 3 张', 'FSRS 的四个评分 · 2 张'].map((t) => `
            <li><div class="row" style="grid-template-columns:minmax(0,1fr);cursor:default">
              <span class="body"><span class="title" style="font-weight:400">${t}</span></span></div></li>`).join('')}
        </ul>
      </div>`).join('')}`);
}

function viewStatsHeatmap() {
  return page('m-paper', `
    ${pageHead('热力', '每天有多少次回忆；阅读会话与复习同一口径')}
    <div class="stat-row">${stat('26 周', '范围')}${stat('91%', '保留率')}${stat('4', '最长连续')}</div>
    <div class="section tile">
      <div class="heat" id="heat-page" role="img" aria-label="复习热力图"></div>
      <div class="heat-legend"><span>少</span>${[0,1,2,3,4].map((l) => `<i data-lv="${l}"></i>`).join('')}<span>多</span></div>
    </div>
    <div class="section"><div class="section-head"><h2>说明</h2></div>
      <p class="dim" style="font-size:var(--t-label);line-height:1.75">
        热力格来自 append-only 的 reviews.jsonl，删掉索引不会丢；插件只收到事件，不参与着色。</p></div>`);
}

function viewStatsForecast() {
  return page('m-paper', `
    ${pageHead('预测', '卡片到期与阅读到期合并成一条序列')}
    <div class="section tile">${forecastSVG()}</div>
    <div class="stat-row">${stat('12', '今天')}${stat('260', '峰值 · 第 14 天')}${stat('8', '明日')}</div>
    <div class="section"><div class="section-head"><h2>为什么合并</h2></div>
      <p class="dim" style="font-size:var(--t-label);line-height:1.75">
        分开看会低估负荷：同一天既有 40 张卡、又有 2 篇长文时，那是两倍的时间。</p></div>`);
}

function paintHeat(sel) {
  const el = $(sel);
  if (!el) return;
  const cells = [];
  for (let i = 0; i < 182; i++) {
    const seed = ((i + 7) * 2654435761) % 1000 / 1000;
    const s = seed * 0.7 + (i / 182) * 0.5;
    const lv = s > 0.92 ? 4 : s > 0.74 ? 3 : s > 0.56 ? 2 : s > 0.36 ? 1 : 0;
    cells.push(`<i data-lv="${lv}"></i>`);
  }
  el.innerHTML = cells.join('');
}

/* ═══ 屏幕：设置 ══════════════════════════════════════════════════════════ */
const settingsShell = (title, lead, inner) => page('m-settings', `${pageHead(title, lead)}${inner}`);

function viewAppearance() {
  return settingsShell('外观', '原生窗口跟随桌面；纸面是暖白，墨色是近黑',
    `<section class="group">
      <h2>主题</h2><p class="lead">两套独立配色，各自满足正文对比度。</p>
      <div class="field"><div><span class="fl">外观</span><span class="fh">原型里可即时切换，用来验证两套色。</span></div>
        <div class="fc">${seg([['system', '随系统'], ['light', '纸面'], ['dark', '墨色']], state.themePref, 'themepick')}</div></div>
      <div class="field"><div><span class="fl">正文尺寸</span><span class="fh">跟随桌面文本缩放，16–22。</span></div>
        <div class="fc"><div class="range"><input type="range" id="body-size" min="16" max="22" value="${state.bodySize}"><output>${state.bodySize}px</output></div></div></div>
    </section>
    <section class="group"><h2>排版</h2>
      <p class="lead">纸面 760，正文 18 / 1.85；控件 13、标签 12、注脚 11。</p>
      <div class="field"><div><span class="fl">纸面宽度</span><span class="fh">影响一行装多少字，不影响字号。</span></div>
        <div class="fc"><div class="range"><input type="range" id="paper-width" min="620" max="860" step="20" value="${state.paperWidth}"><output>${state.paperWidth}</output></div></div></div>
      <div class="field"><div><span class="fl">减少动效</span><span class="fh">跟随系统 prefers-reduced-motion。</span></div>
        <div class="fc">${badge('跟随系统', 'plain')}</div></div>
    </section>
    <section class="group"><h2>预览</h2>
      <div class="tile" style="background:var(--page)">
        <p style="font-size:var(--t-display);line-height:1.34;font-weight:600;letter-spacing:-.02em">今天读到第 7 段</p>
        <p style="font-size:var(--t-body);line-height:1.85;color:var(--ink-2);max-width:38em">
          正文是 18/1.85，一行约 37 个汉字。控件退到 13，标签 12——层级靠字号差与留白，不靠边框。</p>
        <div class="od-row" style="margin-top:6px">${badge('标记色', 'mark')}${badge('发丝线', 'plain')}${badge('墨色', 'plain')}</div>
      </div>
    </section>`);
}

function viewReviewSettings() {
  return settingsShell('复习', 'FSRS 算间隔；这里只改用户的偏好',
    `<section class="group">
      <h2>保留率</h2><p class="lead">越高，间隔越短、复习量越大。</p>
      <div class="field"><div><span class="fl">目标保留率</span><span class="fh">90% 是默认值；再高会明显增加每日量。</span></div>
        <div class="fc"><div class="range"><input type="range" id="retention" min="80" max="97" value="${state.retention}"><output>${state.retention}%</output></div></div></div>
      <div class="field"><div><span class="fl">最大间隔</span><span class="fh">超过这个天数就不再往后推。</span></div>
        <div class="fc"><div class="range"><input type="range" id="max-interval" min="30" max="365" step="30" value="${state.maxInterval}"><output>${state.maxInterval} 天</output></div></div></div>
    </section>
    <section class="group"><h2>评分</h2>
      <div class="field"><div><span class="fl">评分颜色</span><span class="fh">重来 / 困难 / 良好 / 容易，四档各有语义色。</span></div>
        <div class="fc">${badge('重来', 'danger')}${badge('困难', 'warn')}${badge('良好', 'ok')}${badge('容易', 'mark')}</div></div>
      <div class="field"><div><span class="fl">显示间隔预览</span><span class="fh">评分按钮下方显示下一次间隔。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="preview" aria-checked="${state.on.has('preview')}" aria-label="显示间隔预览"></button></div></div>
    </section>
    <section class="group"><h2>阅读侧</h2><p class="lead">阅读只留三档：太难 / 刚好 / 太浅。</p>
      <div class="field"><div><span class="fl">摘录后回到原位</span><span class="fh">摘录是旁注，不是翻页。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="return" aria-checked="${state.on.has('return')}" aria-label="摘录后回到原位"></button></div></div>
    </section>`);
}

function viewBudget() {
  return settingsShell('预算', '预算决定 Today 的顺序与截断，不是一个提醒',
    `<section class="group"><h2>每天</h2>
      <div class="field"><div><span class="fl">复习张数</span><span class="fh">超出部分顺延到明天。</span></div>
        <div class="fc"><div class="range"><input type="range" id="budget-cards" min="5" max="60" step="5" value="${state.budget.cards}"><output>${state.budget.cards} 张</output></div></div></div>
      <div class="field"><div><span class="fl">可用时间</span><span class="fh">读与忆一起算，超时先砍阅读。</span></div>
        <div class="fc"><div class="range"><input type="range" id="budget-minutes" min="10" max="90" step="5" value="${state.budget.minutes}"><output>${state.budget.minutes} 分钟</output></div></div></div>
    </section>
    <section class="group"><h2>今日分配</h2>
      <div class="tile" style="margin-top:10px">
        <div class="od-row" style="justify-content:space-between"><b>${readingQueue().length} 篇 · ${dueCards().length} 张</b><span class="dim">约 ${Math.round(readingQueue().length * 6 + dueCards().length * .6)} 分钟</span></div>
        ${meter(Math.min(100, Math.round((readingQueue().length * 6 + dueCards().length * .6) / state.budget.minutes * 100)))}
      </div>
    </section>
    <section class="group"><h2>提醒</h2>
      <div class="field"><div><span class="fl">每日提醒</span><span class="fh">有待复习或待读时才提醒，两个数一起报。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="reminder" aria-checked="${state.on.has('reminder')}" aria-label="每日提醒"></button></div></div>
      <div class="field"><div><span class="fl">托盘标题</span><span class="fh">Remit · 12 · 3（待忆 · 待读）</span></div>
        <div class="fc"><span class="mono dim">Remit · 12 · 3</span></div></div>
    </section>`);
}

function viewKeys() {
  const groups = [
    ['全局', [['⌘K', '搜索与命令'], ['⌘F', '在笔记里查找'], ['⌘N', '新建笔记'], ['⌘,', '设置']]],
    ['阅读会话', [['J / ↓', '下一段'], ['K / ↑', '上一段'], ['E', '摘录当前段'], ['C', '挖空选中'], ['S', '跳过'], ['X', '移出队列'], ['M', '在库里打开']]],
    ['复习会话', [['空格', '显示答案'], ['1–4', '评分'], ['U', '撤销上一次'], ['E', '编辑卡片'], ['Esc', '退出']]],
    ['文档', [['/', '插入菜单'], ['⌘Z / ⇧⌘Z', '撤销 / 重做'], ['⌘⇧P', '面板开关']]],
  ];
  return settingsShell('快捷键', '一套键位管三个动作；不再有第二套',
    groups.map(([g, rows]) => `<section class="group"><h2>${g}</h2>
      ${rows.map(([k, d]) => `<div class="field"><div><span class="fl">${d}</span></div>
        <div class="fc"><span class="kbd">${k}</span></div></div>`).join('')}
    </section>`).join(''));
}

function viewPlugins() {
  return settingsShell('插件', '宿主画一切；插件只给内容与声明，开关就是授权',
    `<section class="group">
      ${PLUGINS.map((p) => `<div class="field" style="grid-template-columns:minmax(0,1fr) auto auto;align-items:start">
        <div><span class="fl">${esc(p.name)} ${p.builtin ? badge('内置', 'plain') : ''}</span>
          <span class="fh">${esc(p.desc)}</span>
          <div class="od-cluster" style="gap:4px;margin-top:6px">${p.caps.map((c) => `<span class="badge">${c}</span>`).join('')}</div></div>
        <div class="fc"><button class="btn btn--quiet" type="button" data-plugin-info="${p.id}">详情</button></div>
        <div class="fc"><button class="switch" type="button" role="switch" aria-checked="${p.on}" data-plugin="${p.id}" aria-label="${esc(p.name)}"></button></div>
      </div>`).join('')}
    </section>
    <section class="group"><h2>问题</h2>
      <p class="lead">坏掉的贡献只隐藏那一条，不连带停用插件。</p>
      <ul class="list" style="border-top:0">
        ${[['网页捕获 · toolbar.overflow 条目', '能力 net:fetch 未授予'], ['卡片导出 · 统计块', '节点 stats.block 不在 apiVersion 1 里']].map(([what, why]) => `
          <li><div class="row" style="grid-template-columns:minmax(0,1fr) auto;cursor:default">
            <span class="body"><span class="title" style="font-weight:400">${what}</span>
              <span class="sub">${why}</span></span>
            <span class="tail">${badge('已隐藏', 'warn')}</span></div></li>`).join('')}
      </ul>
    </section>

    <section class="group"><h2>页面贡献</h2><p class="lead">插件提供一个宿主绘制的页面，出现在左下角。</p>
      <div class="field"><div><span class="fl">制卡脚本</span><span class="fh">来自「内置制卡」，渲染三个数字。</span></div>
        <div class="fc"><button class="btn" type="button" data-area="plugin">打开页面</button></div></div>
    </section>`);
}

function viewAgent() {
  return settingsShell('Agent API', '只读回环接口；写入需要 token，不暴露删除',
    `<section class="group"><h2>服务</h2>
      <div class="field"><div><span class="fl">监听地址</span><span class="fh">只绑回环，不对局域网开放。</span></div>
        <div class="fc"><span class="mono">127.0.0.1:7180</span></div></div>
      <div class="field"><div><span class="fl">启用</span><span class="fh">关掉之后所有 /v1 路由返回 503。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="agent" aria-checked="${state.on.has('agent')}" aria-label="启用 Agent API"></button></div></div>
      <div class="field"><div><span class="fl">写入 token</span><span class="fh">只显示一次；换一台机器要重新生成。</span></div>
        <div class="fc"><button class="btn" type="button" data-act="toast-token">生成</button></div></div>
    </section>
    <section class="group"><h2>路由</h2>
      <div class="table-wrap"><table class="grid">
        <colgroup><col style="width:80px"><col style="width:220px"><col></colgroup>
        <thead><tr><th>方法</th><th>路径</th><th>说明</th></tr></thead>
        <tbody>
          ${[['GET', '/v1/library', '计数与库信息'], ['GET', '/v1/notes', '标题、块、标签'], ['GET', '/v1/cards', '卡片与到期'], ['GET', '/v1/reading', '阅读队列'], ['POST', '/v1/reading', '加入队列（需 token）']]
            .map(([m, p, d]) => `<tr><td class="ink">${m}</td><td class="mono">${p}</td><td>${d}</td></tr>`).join('')}
        </tbody></table></div>
    </section>`);
}

function viewUpdate() {
  return settingsShell('更新', '检查新版本；不发送任何使用数据',
    `<section class="group">
      <div class="field"><div><span class="fl">当前版本</span><span class="fh">Remit 0.8 · 明文库格式 remit/2</span></div>
        <div class="fc">${badge('已是最新', 'ok')}</div></div>
      <div class="field"><div><span class="fl">自动检查</span><span class="fh">启动时请求一次发布列表。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="autoupdate" aria-checked="${state.on.has('autoupdate')}" aria-label="自动检查"></button></div></div>
      <div class="field"><div><span class="fl">检查更新</span></div>
        <div class="fc"><button class="btn" type="button" data-act="check-update">检查</button></div></div>
    </section>
    <section class="group"><h2>库</h2>
      <div class="field"><div><span class="fl">重建索引</span><span class="fh">索引可删除重建，不影响明文。</span></div>
        <div class="fc"><button class="btn" type="button" data-act="reindex">重建</button></div></div>
      <div class="field"><div><span class="fl">导出整个库</span><span class="fh">打包成 .remitlib 归档。</span></div>
        <div class="fc"><button class="btn" type="button" data-act="export-lib">导出</button></div></div>
    </section>`);
}

function viewAbout() {
  return settingsShell('关于', '本地优先的开源知识记忆工作台',
    `<section class="group">
      <div class="tile" style="margin-top:8px">
        <div class="od-row"><span class="brand-mark"><span></span></span>
          <div><b>Remit 0.8</b><p class="dim" style="font-size:var(--t-label);margin-top:2px">AGPL-3.0 · 明文库 · 零遥测</p></div></div>
      </div>
      <div class="field" style="margin-top:12px"><div><span class="fl">库位置</span></div><div class="fc"><span class="mono dim">~/Documents/knowledge/.remit</span></div></div>
      <div class="field"><div><span class="fl">笔记 / 卡片 / 队列</span></div><div class="fc"><span class="tnum">${NOTES.filter((n) => !n.template).length} / ${CARDS.length} / ${readingQueue().length}</span></div></div>
    </section>
    <section class="group"><h2>致谢</h2>
      <p class="dim" style="font-size:var(--t-label);line-height:1.8">
        图标体系参考 Lucide（ISC）；FSRS 调度算法有其公开论文；界面语言参考了留白的公开设计规范。</p>
    </section>`);
}

/* ═══ 屏幕：插件页面 ══════════════════════════════════════════════════════ */
function viewPluginPage() {
  if (state.pluginPage === 'board') return viewWhiteboard();
  return viewPluginCards();
}

function viewPluginCards() {
  return page('m-settings', `
    ${pageHead('制卡脚本', '来自「内置制卡」插件 · 宿主绘制的页面',
      pluginMenu('page.header').map((a) => `<button class="btn" type="button" data-act="${a.act}">${a.label}</button>`).join('')
      || `<button class="btn btn--quiet" type="button" data-act="run-plugin">${icon('check', 14)} 重新运行</button>`)}
    <div class="stat-row">${stat(NOTES.reduce((n, x) => n + x.blocks.filter((b) => b.text.startsWith('::')).length, 0), '可制卡的块')}${stat(CARDS.length, '已生成卡片')}${stat(4, '卡片类型')}</div>
    <div class="section tile"><div class="od-row" style="justify-content:space-between">
      <b>上一次运行</b><span class="dim">今天 09:12 · 解析 3 个块，生成 3 张卡</span>
      <button class="btn" type="button" data-act="run-plugin">重新运行</button></div>
      <div style="margin-top:10px">${meter(78)}</div>
    </div>
    <div class="section"><div class="section-head"><h2>它能做什么</h2></div>
      <ul class="list">${[['解析 :: 与 ::: 字段块', 'derive:block'], ['推荐下次到期', 'policy:schedule'], ['读取卡片事件', 'on:event']]
        .map(([d, c]) => `<li><div class="row" style="cursor:default"><span class="body"><span class="title" style="font-weight:400">${d}</span></span><span class="tail">${badge(c, 'plain')}</span></div></li>`).join('')}</ul>
    </div>`);
}

/* ═══ 白板：插件画布 ═══════════════════════════════════════════════════════
   节点是指向块的引用：点节点回那一段，从块的菜单回到这里。 */

const BOARD_W = 1120, BOARD_H = 700;

const BOARD_NODES = [
  { id: 'w1', note: 'n1', block: 'b6',  x: 60,  y: 110, w: 250 },
  { id: 'w2', note: 'n1', block: 'b10', x: 560, y: 60,  w: 250 },
  { id: 'w3', note: 'n2', block: 'b3',  x: 690, y: 300, w: 250 },
  { id: 'w4', note: 'n4', block: 'b3',  x: 250, y: 400, w: 250 },
  { id: 'w5', note: 'n6', block: 'b3',  x: 480, y: 540, w: 250 },
];
const BOARD_EDGES = [['w1', 'w2'], ['w1', 'w3'], ['w1', 'w5'], ['w4', 'w3']];
const NODE_H = 108;

function boardNode(id) { return BOARD_NODES.find((n) => n.id === id); }

function boardEdgesSVG() {
  const lines = BOARD_EDGES.map(([a, b]) => {
    const A = boardNode(a), B = boardNode(b);
    const right = B.x >= A.x;
    const x1 = right ? A.x + A.w : A.x;
    const x2 = right ? B.x : B.x + B.w;
    const y1 = A.y + NODE_H / 2, y2 = B.y + NODE_H / 2;
    const mx = (x1 + x2) / 2;
    return `<path class="bedge" d="M${x1} ${y1} C${mx} ${y1} ${mx} ${y2} ${x2} ${y2}"/>`;
  }).join('');
  return `<svg class="board-edges" viewBox="0 0 ${BOARD_W} ${BOARD_H}" preserveAspectRatio="none"
    style="position:absolute;inset:0;width:${BOARD_W}px;height:${BOARD_H}px" aria-hidden="true">${lines}</svg>`;
}

function viewWhiteboard() {
  const focus = state.boardFocus;
  return `<div class="view"><div class="stage-scroll">
    <div class="page" style="padding-bottom:32px"><div class="measure m-wide">
      ${pageHead('白板', '来自「白板」插件 · 宿主绘制的画布；节点是指向块的引用',
        `<button class="btn btn--quiet" type="button" data-act="board-fit">${icon('grid', 14)} 居中</button>
         <button class="btn" type="button" data-act="pg:whiteboard:toolbar.overflow:0">新建白板</button>`)}
      <div class="board" id="board">
        <div class="board-inner" style="width:${BOARD_W}px;height:${BOARD_H}px">
          ${boardEdgesSVG()}
          ${BOARD_NODES.map((n) => {
            const note = noteById(n.note);
            const block = note && note.blocks.find((b) => b.id === n.block);
            const cards = block ? cardsOn(n.note, n.block).length : 0;
            return `<button class="bnode${focus === n.block ? ' is-focus' : ''}" type="button"
              data-board-node="${n.id}" style="left:${n.x}px;top:${n.y}px;width:${n.w}px;height:${NODE_H}px">
              <span class="bnode-note">${esc(note ? note.title : '已删除')}</span>
              <span class="bnode-text">${esc(block ? clip(block.text, 46) : '源块已不在')}</span>
              <span class="bnode-foot">
                ${cards ? `<span class="badge badge--mark">${cards} 张卡</span>` : `<span class="badge badge--plain">无卡</span>`}
                <span class="bnode-go">回到这一段 ${icon('chev', 11)}</span>
              </span>
            </button>`;
          }).join('')}
        </div>
      </div>
      <p class="dim" style="font-size:var(--t-label);margin-top:12px;line-height:1.7">
        点节点回到它引用的那一段；在那一段的块菜单里选「在白板里定位」，会跳回这张卡并高亮它。
        画布本身不存内容——它只存引用，删掉节点不会删笔记。</p>
    </div></div>
  </div></div>`;
}

/* ═══ 会话：读 / 忆 ═══════════════════════════════════════════════════════ */
function openReading(noteId) {
  const note = noteById(noteId) || readingQueue()[0];
  if (!note) { toast('今天没有待读的笔记'); return; }
  state.session = { kind: 'read', noteId: note.id, pos: note.reading ? note.reading.pos : 0 };
  renderSession();
}

function openReview(ids) {
  const list = ids && ids.length ? CARDS.filter((c) => ids.includes(c.id)) : dueCards();
  if (!list.length) { toast('今天没有到期的卡片'); return; }
  state.session = { kind: 'review', queue: list.map((c) => c.id), idx: 0, revealed: false, ctx: false, failed: [] };
  renderSession();
}

function renderSession() {
  const el = $('#session');
  if (!state.session) { el.hidden = true; el.innerHTML = ''; return; }
  el.hidden = false;
  el.innerHTML = state.session.kind === 'read' ? readingHTML() : reviewHTML();
  const body = $('.session-body', el);
  if (body) body.scrollTop = 0;
}

function readingHTML() {
  const s = state.session;
  const note = noteById(s.noteId);
  const queue = readingQueue();
  const pos = Math.min(Math.max(0, s.pos), note.blocks.length - 1);
  const idx = queue.findIndex((n) => n.id === note.id) + 1;

  return `
  <header class="session-bar">
    <span class="session-id">读 · 第 ${Math.max(1, idx)} / ${Math.max(1, queue.length)} 篇</span>
    <span class="session-title">${esc(note.title)}</span>
    <span class="session-right">
      <span class="dim" style="font-size:var(--t-label)">第 ${pos + 1} / ${note.blocks.length} 段</span>
      <div class="meter"><i style="--to:${Math.round(((pos + 1) / note.blocks.length) * 100)}%"></i></div>
      <button class="btn btn--quiet" type="button" data-act="exit-session">退出 <span class="kbd">Esc</span></button>
    </span>
  </header>
  <div class="session-body">
    <div class="read-wrap">
      ${note.blocks.map((b, i) => {
        const cs = cardsOn(note.id, b.id).length;
        const ex = excerptedFrom(note.id, b.id);
        const last = note.reading && note.reading.pos === i && i !== pos;
        const marks = [cs ? `${cs} 卡` : '', ex ? '已摘' : '', last ? '上次停在这里' : '']
          .filter(Boolean).map((t) => `<span class="rmark${t === '上次停在这里' ? ' rmark--muted' : ''}">${t}</span>`).join('');
        return `<div class="read-block${i === pos ? ' is-pos' : ''}${cs ? ' has-card' : ''}" data-kind="${b.kind}" data-rblock="${i}">
        <span class="rnum">${i + 1}</span>
        <div class="block-body">${inline(b.text)}</div>
        ${marks ? `<span class="rmarks">${marks}</span>` : ''}
      </div>`;
      }).join('')}
    </div>
  </div>
  <footer class="session-foot">
    <div class="keys" style="display:flex;gap:6px;flex-wrap:wrap">
      <button class="key-btn" type="button" data-digest="1"><span class="kbd">1</span> 太难</button>
      <button class="key-btn" type="button" data-digest="2"><span class="kbd">2</span> 刚好</button>
      <button class="key-btn" type="button" data-digest="3"><span class="kbd">3</span> 太浅</button>
    </div>
    <span class="push"></span>
    <div class="keys" style="display:flex;gap:6px;flex-wrap:wrap">
      <label class="key-btn" style="gap:8px;cursor:pointer">
        <input type="checkbox" class="check" id="excerpt-card" ${state.readingToCard ? 'checked' : ''}> 摘录并制卡</label>
      <button class="key-btn" type="button" data-act="excerpt"><span class="kbd">E</span> 摘录</button>
      <button class="key-btn" type="button" data-act="skip"><span class="kbd">S</span> 跳过</button>
      <button class="key-btn" type="button" data-act="archive"><span class="kbd">A</span> 归档</button>
      <button class="key-btn" type="button" data-act="dismiss"><span class="kbd">D</span> 不再出现</button>
      <button class="key-btn" type="button" data-act="postpone"><span class="kbd">P</span> 推后</button>
      <button class="key-btn" type="button" data-act="open-doc"><span class="kbd">M</span> 在库里打开</button>
    </div>
  </footer>`;
}

function reviewHTML() {
  const s = state.session;
  if (s.idx >= s.queue.length) return summaryHTML();
  const card = CARDS.find((c) => c.id === s.queue[s.idx]);
  const note = noteById(card.note);
  const bi = note.blocks.findIndex((b) => b.id === card.block);
  const siblings = cardsOn(note.id, card.block).filter((x) => x.id !== card.id);
  const intervals = ['1 分钟', '10 分钟', '4 天', '12 天'];

  return `
  <header class="session-bar">
    <span class="session-id">忆 · ${s.idx + 1} / ${s.queue.length}</span>
    <span class="session-title">复习 · 全库</span>
    <span class="session-right">
      <div class="meter"><i style="--to:${Math.round((s.idx / s.queue.length) * 100)}%"></i></div>
      <button class="btn btn--quiet" type="button" data-act="exit-session">退出 <span class="kbd">Esc</span></button>
    </span>
  </header>
  <div class="session-body">
    <div style="width:100%;max-width:var(--w-review)">
      <div class="card-face">
        <p class="card-prompt">${esc(card.prompt)}</p>
        ${s.revealed ? `<hr class="card-rule"><p class="card-answer">${esc(card.answer)}</p>
          <div class="card-source">
            <span>《${esc(note.title)}》 · 第 ${bi + 1} 段</span>
            ${siblings.length ? `<span>· 同块还有 ${siblings.length} 张</span>` : ''}
            <button type="button" data-source="${card.id}">回到原文</button>
            <button type="button" data-ctx="${card.id}">${s.ctx ? '收起上下文' : '展开上下文'}</button>
          </div>
          ${s.ctx ? `<div class="ctx">
            ${[bi - 1, bi, bi + 1].filter((k) => k >= 0 && k < note.blocks.length).map((k) => `
              <div class="ctx-line${k === bi ? ' is-here' : ''}">
                <span class="ctx-num">${k + 1}</span>
                <span class="ctx-text">${inline(clip(note.blocks[k].text, 64))}</span>
              </div>`).join('')}
            ${siblings.length ? `<div class="ctx-sibs">${siblings.map((x) => `<span class="badge badge--plain">${esc(clip(x.prompt, 14))}</span>`).join('')}</div>` : ''}
          </div>` : ''}` : ''}
      </div>
      ${s.revealed ? `<div class="grades">
          ${intervals.map((t, i) => `<button class="grade" type="button" data-grade="${i + 1}" data-g="${i + 1}">
            <span class="gl">${['重来', '困难', '良好', '容易'][i]}</span><span class="gi">${t}</span></button>`).join('')}
        </div>` : `<div class="reveal-wrap">
          <button class="btn btn--primary btn--lg" type="button" data-act="reveal">显示答案 <span class="kbd">空格</span></button>
        </div>`}
    </div>
  </div>
  <footer class="session-foot">
    <span class="dim" style="font-size:var(--t-label)">${s.revealed ? '按 1–4 评分' : '想好了再翻面；想不起来也是数据'}</span>
    <span class="push"></span>
    ${pluginMenu('review.card').map((a) => `<button class="key-btn" type="button" data-act="${a.act}">${a.label}</button>`).join('')}
    <button class="key-btn" type="button" data-act="undo-review"><span class="kbd">U</span> 撤销</button>
    <button class="key-btn" type="button" data-act="edit-card" data-card="${card.id}"><span class="kbd">E</span> 编辑卡片</button>
  </footer>`;
}

function summaryHTML() {
  const n = state.session.queue.length;
  const byNote = {};
  for (const id of (state.session.failed || [])) {
    const c = CARDS.find((x) => x.id === id);
    if (c) (byNote[c.note] = byNote[c.note] || []).push(id);
  }
  const fails = Object.entries(byNote);
  return `
  <header class="session-bar"><span class="session-id">忆 · 完成</span>
    <span class="session-right"><button class="btn btn--quiet" type="button" data-act="exit-session">关闭</button></span></header>
  <div class="session-body">
    <div class="summary">
      <h2>今日忆完</h2>
      <p class="sub">${n} 张卡片 · 用时约 ${Math.round(n * 0.6)} 分钟</p>
      <div class="summary-stats">
        ${stat(n, '本次复习')}${stat(8, '明日到期')}${stat('91%', '近期保留率')}
      </div>
      <p class="plugin-line"><b>阅读摘要</b> · 今天还有 ${readingQueue().length} 篇待读，接着读会比重开一局更划算。</p>
      ${fails.length ? `<div class="fail-list">
        <span class="eyebrow">答错的按来源聚合</span>
        ${fails.map(([noteId, ids]) => `<button class="row" type="button" data-source="${ids[0]}"
          style="grid-template-columns:minmax(0,1fr) auto;border-top:1px solid var(--line)">
          <span class="body"><span class="title" style="font-weight:400">${esc(noteById(noteId).title)}</span>
            <span class="sub">${ids.length} 张没想起来</span></span>
          <span class="tail">回到原文 ${icon('chev', 14)}</span></button>`).join('')}
      </div>` : ''}
      <div style="display:flex;gap:8px;margin-top:16px">
        <button class="btn btn--primary" type="button" data-act="exit-session">回到今日</button>
        <button class="btn" type="button" data-act="start-reading">继续读</button>
      </div>
    </div>
  </div>
  <footer class="session-foot"><span class="dim" style="font-size:var(--t-label)">队列已空。明天 8 张。</span></footer>`;
}

/* ═══ 浮层 ════════════════════════════════════════════════════════════════ */
let menuOwner = null;

function openMenu(anchor, items) {
  const m = $('#menu');
  m.innerHTML = items.map((it) => it === '-' ? '<div class="menu-sep"></div>' : `
    <button class="menu-item" type="button" data-menu="${it.act}" ${it.danger ? 'data-danger="true"' : ''}>
      <span class="ml">${it.label}</span>${it.key ? `<span class="mk">${it.key}</span>` : ''}</button>`).join('');
  m.hidden = false;
  const r = anchor.getBoundingClientRect();
  const w = m.offsetWidth, h = m.offsetHeight;
  let left = Math.min(Math.max(12, r.right - w), window.innerWidth - w - 12);
  let top = r.bottom + 5;
  if (top + h > window.innerHeight - 12) top = Math.max(12, r.top - h - 5);
  m.style.left = `${left}px`; m.style.top = `${top}px`;
  menuOwner = anchor;
}
function closeMenu() { $('#menu').hidden = true; $('#menu').innerHTML = ''; menuOwner = null; }

function toast(text, actionLabel, action) {
  const t = $('#toast');
  t.innerHTML = `<span>${text}</span>${actionLabel ? `<button type="button">${actionLabel}</button>` : ''}`;
  t.hidden = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => { t.hidden = true; }, 4200);
  if (actionLabel && action) $('button', t).addEventListener('click', () => { t.hidden = true; action(); });
}

function openModal(html) {
  const d = $('#modal');
  d.innerHTML = html;
  d.showModal();
}

function closeModal() { const d = $('#modal'); if (d.open) d.close(); }

/* ═══ 动作 ════════════════════════════════════════════════════════════════ */
function openNote(id, mode = 'edit') {
  state.area = 'write'; state.sub = 'doc'; state.noteId = id; state.docMode = mode;
  const note = noteById(id);
  state.currentBlock = mode === 'read' && note.reading
    ? note.blocks[Math.min(note.reading.pos, note.blocks.length - 1)].id
    : (note.blocks[0] ? note.blocks[0].id : null);
  render();
  $('#stage').scrollTop = 0;
}

function excerptFor(noteId, blockId) {
  const note = noteById(noteId);
  const block = note.blocks.find((b) => b.id === blockId);
  if (!block) return null;
  const id = `ex${++state.excerptSeq}`;
  const child = {
    id, title: `摘录：${block.text.slice(0, 12)}${block.text.length > 12 ? '…' : ''}`,
    tags: ['摘录'], excerptOf: noteId, sourceBlock: blockId, edited: '刚刚', reading: null,
    blocks: [{ id: 'b1', kind: 'p', text: block.text },
             { id: 'b2', kind: 'p', text: `[[${note.title}#^${blockId}]]` }],
  };
  NOTES.unshift(child);
  return child;
}

function makeCard(noteId, blockId, prompt, answer, type = 'basic') {
  const id = `nc${CARDS.length + 1}`;
  CARDS.unshift({ id, note: noteId, block: blockId, prompt, answer, type, state: 'new', due: '今天', ref: 'live' });
  return id;
}

const clip = (t, n = 14) => (t.length > n ? t.slice(0, n) + '…' : t);

function insertBlock(afterId, kind = 'p', text = '') {
  const note = noteById(state.noteId);
  const i = Math.max(0, note.blocks.findIndex((b) => b.id === afterId));
  const id = `b${Date.now().toString(36).slice(-4)}`;
  note.blocks.splice(i + 1, 0, { id, kind, text });
  state.currentBlock = id;
  render();
  const el = $(`[data-body="${id}"]`);
  if (el) { el.focus(); const r = document.createRange(); r.selectNodeContents(el); r.collapse(false);
    const sel = window.getSelection(); sel.removeAllRanges(); sel.addRange(r); }
}
function moveBlock(id, dir) {
  const note = noteById(state.noteId);
  const i = note.blocks.findIndex((b) => b.id === id), j = i + dir;
  if (i < 0 || j < 0 || j >= note.blocks.length) return;
  [note.blocks[i], note.blocks[j]] = [note.blocks[j], note.blocks[i]];
  render();
}
function deleteBlock(id) {
  const note = noteById(state.noteId);
  if (note.blocks.length === 1) return toast('至少留一段');
  const live = CARDS.filter((c) => c.note === note.id && c.block === id && c.ref === 'live');
  if (live.length) {
    return openModal(`
      <div class="modal-head"><h2>删除这一段？</h2>
        <p>${live.length} 张卡片会失去来源，变成断链；复习历史保留，可以重新指向别的段落。</p></div>
      <div class="modal-body"><div class="tile" style="background:var(--page)">
        <span class="eyebrow">将要删除</span>
        <p style="font-size:15px;line-height:1.75">${inline(clip(note.blocks.find((b) => b.id === id).text, 90))}</p></div></div>
      <div class="modal-foot"><span class="push"></span>
        <button class="btn" type="button" data-dlg="close">取消</button>
        <button class="btn btn--danger" type="button" data-dlg="delete-block" data-block="${id}">删除，${live.length} 张卡转为断链</button>
      </div>`);
  }
  hardDeleteBlock(id);
}

function hardDeleteBlock(id) {
  const note = noteById(state.noteId);
  note.blocks = note.blocks.filter((b) => b.id !== id);
  CARDS.filter((c) => c.note === note.id && c.block === id).forEach((c) => { c.ref = 'lost'; });
  render();
}

function advanceReading() {
  const s = state.session;
  if (!s) return;
  const note = noteById(s.noteId);
  if (note.reading) note.reading.pos = s.pos;
  state.readDone.add(note.id);
  const rest = readingQueue();
  state.session = null;
  if (rest.length) return openReading(rest[0].id);
  toast('今天读完了', '去复习', () => openReview());
}

function doExcerpt() {
  const s = state.session;
  const note = noteById(s.noteId);
  const pos = Math.min(Math.max(0, s.pos), note.blocks.length - 1);
  const block = note.blocks[pos];
  const child = excerptFor(note.id, block.id);
  if (state.readingToCard) makeCard(child.id, child.blocks[0].id, clip(block.text, 30), `来自《${note.title}》第 ${pos + 1} 段`);
  renderSession();
  toast(state.readingToCard ? '已摘录，并制了一张卡' : '已摘录为子笔记', '打开', () => {
    state.session = null; renderSession(); openNote(child.id);
  });
}

function grade(n) {
  const s = state.session;
  if (!s.revealed) return;
  const card = CARDS.find((c) => c.id === s.queue[s.idx]);
  if (card) {
    card.state = n === 1 ? 'learning' : 'review';
    card.due = n === 1 ? '10 分钟后' : n === 2 ? '明天' : n === 3 ? '4 天后' : '12 天后';
  }
  if (n === 1 && card) s.failed.push(card.id);
  s.idx += 1; s.revealed = false; state.reviewedToday += 1;
  renderSession();
}

/* 对话框 */
function openCardDialog(noteId, blockId, cardId) {
  const note = noteById(noteId);
  const block = note.blocks.find((b) => b.id === blockId);
  const cards = cardsOn(noteId, blockId);
  const target = cardId ? CARDS.find((c) => c.id === cardId) : cards[0];
  openModal(`
    <div class="modal-head"><h2>${target ? '卡片' : '为这一段制卡'}</h2>
      <p>卡片引用这一个块；块变了，卡面跟着变（除非冻结）。</p></div>
    <div class="modal-body">
      <span class="label-sm">提示</span>
      <textarea class="textarea" id="card-prompt" aria-label="卡片提示">${esc(target ? target.prompt : block.text)}</textarea>
      <div style="height:12px"></div>
      <span class="label-sm">答案</span>
      <textarea class="textarea" id="card-answer" aria-label="卡片答案">${esc(target ? target.answer : '')}</textarea>
      <div class="od-cluster" style="margin-top:14px">
        ${badge('type · ' + (target ? target.type : 'basic'), 'plain')}
        ${badge('ref · ' + (target ? target.ref : 'live'), 'ok')}
        ${badge(`第 ${note.blocks.findIndex((b) => b.id === blockId) + 1} 段`, 'plain')}
      </div>
    </div>
    <div class="modal-foot">
      <button class="btn btn--danger" type="button" data-dlg="delete-card" data-block="${blockId}" ${target ? '' : 'disabled'}>删除卡片</button>
      <span class="push"></span>
      <button class="btn" type="button" data-dlg="close">取消</button>
      <button class="btn btn--primary" type="button" data-dlg="save-card" data-block="${blockId}">保存</button>
    </div>`);
  const p = $('#card-prompt');
  if (p) { p.focus(); p.setSelectionRange(p.value.length, p.value.length); }
}

function openTypeDialog(typeId) {
  const t = typeById(typeId);
  openModal(`
    <div class="modal-head"><h2>${t ? esc(t.name) : '新建卡片类型'}</h2>
      <p>类型是库里的明文资产；字段是标量，模板用纯数据方言。</p></div>
    <div class="modal-body">
      <span class="label-sm">名称</span>
      <input class="input" value="${esc(t ? t.name : '')}" aria-label="类型名称">
      <div style="height:12px"></div>
      <span class="label-sm">字段（每行一个）</span>
      <textarea class="textarea" aria-label="字段">${esc(t ? t.fields.join('\n') : '提示\n答案')}</textarea>
      <div style="height:12px"></div>
      <span class="label-sm">正面模板</span>
      <input class="input mono" value="${esc(t ? t.front : '{{提示}}')}" aria-label="正面模板">
      <div style="height:12px"></div>
      <span class="label-sm">背面模板</span>
      <input class="input mono" value="${esc(t ? t.back : '{{答案}}')}" aria-label="背面模板">
      <div style="height:14px"></div>
      <div class="tile" style="background:var(--page)">
        <span class="eyebrow">预览</span>
        <p style="font-size:16px;line-height:1.6">${esc(t ? t.front.replace(/\{\{cloze:(.+?)\}\}/, '「…」').replace(/\{\{(.+?)\}\}/g, '$1') : '提示')}</p>
      </div>
    </div>
    <div class="modal-foot"><span class="push"></span>
      <button class="btn" type="button" data-dlg="close">取消</button>
      <button class="btn btn--primary" type="button" data-dlg="close">保存</button></div>`);
}

function openImport() {
  const KINDS = [
    ['layers', 'lib', '.remitlib 归档', '整库：明文 + 资产 + 调度史'],
    ['upload', 'md', 'Markdown 目录', '保留相对路径与图片，块 ID 重新铸造'],
    ['book', 'anki', 'Anki 集合 (.apkg)', 'deck 映射成复习范围，不建容器实体'],
  ];
  const step = state.importStep;
  const kind = KINDS.find((k) => k[1] === state.importKind);

  const stepper = `
    <div class="steps">
      ${[['1', '选来源'], ['2', '预览'], ['3', '完成']].map(([n, label], i) => `
        <span class="step" data-on="${i === step}" data-done="${i < step}">
          <span class="dot">${i < step ? icon('check', 11) : n}</span>${label}</span>
        ${i < 2 ? '<span class="bar"></span>' : ''}`).join('')}
    </div>`;

  let body = '';
  if (step === 0) {
    body = `<div class="welcome-actions" style="margin:0">
      ${KINDS.map(([ic, id, t, d]) => `<button class="welcome-action" type="button" data-import-kind="${id}">
        <span class="row-kind">${icon(ic, 15)}</span>
        <span class="wa-body"><span class="wa-t">${t}</span><span class="wa-s">${d}</span></span>
        ${icon('chev', 16)}</button>`).join('')}</div>`;
  } else if (step === 1) {
    const preview = kind[1] === 'anki'
      ? [['笔记', '138'], ['卡片', '412'], ['deck', '3 → 复习范围'], ['媒体', '27 个']]
      : kind[1] === 'md'
        ? [['文件', '182'], ['带图', '14'], ['重复标题', '3'], ['块 ID', '重新铸造']]
        : [['笔记', '182'], ['卡片', '412'], ['资产', '41'], ['调度史', '3,204 条']];
    body = `
      <div class="tile" style="margin-bottom:12px">
        <div class="od-row" style="justify-content:space-between">
          <b>${kind[2]}</b><span class="dim">即将导入</span></div>
      </div>
      <dl class="kv">${preview.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl>
      <div style="height:14px"></div>
      <div class="field" style="border-top:1px solid var(--line)">
        <div><span class="fl">保留现有笔记</span><span class="fh">关掉就是整库替换。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="importKeep" aria-checked="${state.on.has('importKeep')}" aria-label="保留现有笔记"></button></div></div>
      <div class="field">
        <div><span class="fl">导入后重建索引</span><span class="fh">索引本来就可删可重建。</span></div>
        <div class="fc"><button class="switch" type="button" role="switch" data-toggle="importReindex" aria-checked="${state.on.has('importReindex')}" aria-label="导入后重建索引"></button></div></div>`;
  } else {
    body = `<div class="empty" style="padding:8px 0 4px">
        <span class="eicon">${icon('check', 20)}</span>
        <h3>导入完成</h3>
        <p>182 篇笔记、412 张卡片、41 个资产已写入库。索引重建用了 0.8 秒。</p>
      </div>`;
  }

  const foot = step === 0
    ? `<span class="push"></span><button class="btn" type="button" data-dlg="close">取消</button>`
    : step === 1
      ? `<button class="btn btn--quiet" type="button" data-import-back>上一步</button><span class="push"></span>
         <button class="btn" type="button" data-dlg="close">取消</button>
         <button class="btn btn--primary" type="button" data-import-next>开始导入</button>`
      : `<span class="push"></span><button class="btn btn--primary" type="button" data-dlg="close">完成</button>`;

  openModal(`
    <div class="modal-head"><h2>导入</h2><p>整库、一堆 Markdown，或一份 Anki 集合。</p></div>
    <div class="modal-body">${stepper}${body}</div>
    <div class="modal-foot">${foot}</div>`);
}

function openPluginDetail(id) {
  const p = PLUGINS.find((x) => x.id === id) || PLUGINS[0];
  openModal(`
    <div class="modal-head"><h2>${esc(p.name)}</h2><p>${esc(p.desc)}</p></div>
    <div class="modal-body">
      <div class="od-cluster">${p.caps.map((c) => `<span class="badge">${c}</span>`).join('')}</div>
      <div class="section" style="margin-top:16px">
        <div class="field" style="border-top:0"><div><span class="fl">信任级别</span><span class="fh">插件运行在应用的限额下，不是它的信任下。</span></div>
          <div class="fc">${badge(p.builtin ? '内置' : '第三方', p.builtin ? 'plain' : 'mark')}</div></div>
        <div class="field"><div><span class="fl">启动时加载</span></div>
          <div class="fc"><button class="switch" type="button" role="switch" data-toggle="loadOnStart" aria-checked="${state.on.has('loadOnStart')}" aria-label="启动时加载"></button></div></div>
      </div>
    </div>
    <div class="modal-foot"><span class="push"></span>
      <button class="btn" type="button" data-dlg="close">关闭</button></div>`);
}

/* ═══ 命令面板 ════════════════════════════════════════════════════════════ */
const COMMANDS = [
  { k: '转到', label: '今日', run: () => go('today') },
  { k: '转到', label: '笔记列表', run: () => go('write', 'notes') },
  { k: '转到', label: '阅读队列', run: () => go('read', 'queue') },
  { k: '转到', label: '复习队列', run: () => go('recall', 'queue') },
  { k: '转到', label: '卡片', run: () => go('recall', 'cards') },
  { k: '转到', label: '统计', run: () => go('stats', 'overview') },
  { k: '转到', label: '设置', run: () => go('settings', 'appearance') },
  { k: '动作', label: '开始今日复习', run: () => openReview() },
  { k: '动作', label: '继续阅读', run: () => openReading() },
  { k: '动作', label: '新建笔记', run: () => newNote() },
  { k: '动作', label: '切换外观', run: () => { state.themePref = state.themePref === 'dark' ? 'light' : 'dark'; applyTheme(); render(); } },
  { k: '动作', label: '导入', run: () => openImport() },
];

function openPalette() {
  $('#palette-icon').innerHTML = icon('search', 18);
  $('#palette-input').value = '';
  state.paletteIndex = 0;
  paintPalette('');
  $('#palette').showModal();
  $('#palette-input').focus();
}

function paintPalette(q) {
  const query = q.trim().toLowerCase();
  const groups = [];
  const cmds = COMMANDS.filter((c) => !query || c.label.toLowerCase().includes(query));
  if (cmds.length) groups.push(['命令', cmds.map((c) => ({ k: c.k, label: c.label, run: c.run }))]);
  if (query) {
    const notes = NOTES.filter((n) => n.title.toLowerCase().includes(query)).slice(0, 6)
      .map((n) => ({ k: '笔记', label: n.title, run: () => openNote(n.id) }));
    if (notes.length) groups.push(['笔记', notes]);
    const cards = CARDS.filter((c) => c.prompt.toLowerCase().includes(query) || c.answer.toLowerCase().includes(query)).slice(0, 6)
      .map((c) => ({ k: '卡片', label: c.prompt, run: () => jumpToSource(c.id) }));
    if (cards.length) groups.push(['卡片', cards]);
  }

  const flat = groups.flatMap(([, items]) => items);
  state.paletteFlat = flat;
  if (state.paletteIndex >= flat.length) state.paletteIndex = Math.max(0, flat.length - 1);

  let i = 0;
  $('#palette-list').innerHTML = flat.length ? groups.map(([g, items]) => `
    <li class="palette-group">${g}</li>
    ${items.map((it) => {
      const idx = i++;
      return `<li><button class="palette-item" type="button" data-pal="${idx}" aria-selected="${idx === state.paletteIndex}">
        <span class="pk">${it.k}</span><span class="pl">${esc(it.label)}</span></button></li>`;
    }).join('')}`).join('') : `<li><p class="panel-empty">没有匹配项</p></li>`;
}

function runPalette(idx) {
  const item = (state.paletteFlat || [])[idx];
  $('#palette').close();
  if (item) item.run();
}

/* ═══ 其它 ════════════════════════════════════════════════════════════════ */
function newNote(templateId) {
  const id = `nn${Date.now().toString().slice(-5)}`;
  const tpl = templateId ? noteById(templateId) : null;
  NOTES.unshift({
    id, title: tpl ? `${tpl.title.replace('模板：', '')} 副本` : '无标题',
    tags: [], edited: '刚刚', reading: null,
    blocks: tpl ? tpl.blocks.map((b, i) => ({ ...b, id: `b${i + 1}` })) : [{ id: 'b1', kind: 'p', text: '' }],
  });
  openNote(id);
  toast(tpl ? '已从模板新建' : '新笔记已建在库里');
}

function jumpToSource(cardId) {
  const card = CARDS.find((c) => c.id === cardId);
  state.session = null; renderSession();
  openNote(card.note, 'read');
  state.currentBlock = card.block;
  render();
  setTimeout(() => scrollToBlock(card.block), 30);
}

function scrollToBlock(id) {
  const el = $(`[data-block="${id}"]`);
  const box = $('#doc-main');
  if (!el || !box) return;
  const top = el.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop - 140;
  box.scrollTo({ top: Math.max(0, top), behavior: 'smooth' });
}

function applyTheme() {
  const dark = state.themePref === 'dark' ||
    (state.themePref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  document.documentElement.style.setProperty('--doc-body', state.bodySize + 'px');
  document.documentElement.style.setProperty('--w-paper', state.paperWidth + 'px');
}

function runAction(act, el) {
  const blockId = el && el.dataset ? el.dataset.block : null;
  switch (act) {
    case 'enter': { state.screen = 'app'; $('#app').dataset.screen = 'app'; go('today'); return; }
    case 'palette': return openPalette();
    case 'new-note': return newNote();
    case 'new-type': return openTypeDialog();
    case 'expand-rail': state.railWide = true; $('#app').dataset.rail = 'full'; return;
    case 'toggle-panel': state.panel = !state.panel; return render();
    case 'toggle-find': state.findOpen = !state.findOpen; return render();
    case 'exit-session': state.session = null; return renderSession();
    case 'start':
      if (readingQueue().length) return openReading(readingQueue()[0].id);
      return openReview();
    case 'start-reading':
      state.session = null;
      if (!readingQueue().length) return toast('今天读完了');
      return openReading(readingQueue()[0].id);
    case 'review-due': return openReview(dueForScope().map((c) => c.id));
    case 'open-scope': return openScope();
    case 'suspend-selected': {
      state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) { c.state = 'learning'; c.due = '明天'; } });
      toast(`已暂停 ${state.selected.size} 张`); state.selected.clear(); return render();
    }
    case 'save-search': {
      const label = ({ all: '全部卡片', due: '今天到期', new: '新卡', learning: '学习中', review: '复习中', lost: '断链', hard: '难卡' })[state.cardFilter] || '全部卡片';
      if (!state.saved.some((x) => x.label === label)) state.saved.push({ id: 's' + (state.saved.length + 1), label });
      render(); return toast('已保存筛选：' + label);
    }
    case 'hard-pause': {
      hardCards().forEach((c) => { c.due = '明天'; });
      toast(`已把 ${hardCards().length} 张难卡推到明天`); return render();
    }
    case 'add-step': return toast('已加一步：3 天');
    case 'search-help': return openSearchHelp();
    case 'card-cols':
      return openMenu(el, [
        { act: 'col:interval', label: (state.cardCols.interval ? '✓ ' : '　') + '间隔' },
        { act: 'col:lapses', label: (state.cardCols.lapses ? '✓ ' : '　') + '重来次数' },
      ]);
    case 'bulk': {
      const n = state.selected.size;
      return openMenu(el, [
        { act: 'bulk-suspend', label: `暂停 ${n} 张` },
        { act: 'bulk-due:tomorrow', label: '到期：推到明天' },
        { act: 'bulk-due:week', label: '到期：推一周' },
        '-',
        { act: 'bulk-type', label: '改类型…' },
        { act: 'bulk-tag', label: '加标签…' },
        { act: 'bulk-bury', label: '埋藏同类（同块的其他卡）' },
        '-',
        { act: 'bulk-freeze', label: '冻结引用' },
        { act: 'bulk-delete', label: `删除 ${n} 张`, danger: true },
      ]);
    }
    case 'bulk-type': return openBulkType();
    case 'bulk-tag': return openBulkTag();
    case 'reset-preset': return toast('已恢复默认：1 分钟 / 10 分钟 / 1 天');
    case 'board-fit': state.boardFocus = ''; return render();
    case 'browse-plugin': {
      const items = pluginMenu('browse.row');
      if (!items.length) return toast('没有启用的插件提供 browse.row');
      return openMenu(el, items);
    }
    case 'relink-all': {
      CARDS.filter((c) => c.ref === 'lost').forEach((c) => { c.ref = 'frozen'; });
      render(); return toast('已全部转为快照；需要精确重连时逐张选块');
    }
    case 'review-selected': return openReview([...state.selected]);
    case 'reveal': state.session.revealed = true; renderSession();
      { const b = $('[data-grade="3"]'); if (b) b.focus({ preventScroll: true }); } return;
    case 'skip': return advanceReading();
    case 'drop': { const n = noteById(state.session.noteId); n.reading = null; toast('已移出阅读队列'); return advanceReading(); }
    case 'archive': {
      const n = noteById(state.session.noteId);
      if (n.reading) n.reading.pos = state.session.pos;
      state.archived.add(n.id);
      toast('已归档：仍可搜索与引用，不再进队列');
      return advanceReading();
    }
    case 'dismiss': {
      const n = noteById(state.session.noteId);
      state.dismissed.add(n.id);
      toast('不再出现：要恢复可以在阅读队列的筛选里找');
      return advanceReading();
    }
    case 'postpone': {
      const n = noteById(state.session.noteId);
      if (n.reading) n.reading.pos = state.session.pos;
      toast('已推后，明天再来');
      return advanceReading();
    }
    case 'excerpt': return doExcerpt();
    case 'open-doc': { const id = state.session.noteId; state.session = null; renderSession(); return openNote(id, 'read'); }
    case 'add-reading': {
      const target = state.area === 'write' && state.noteId ? noteById(state.noteId) : NOTES.find((n) => !n.reading && !n.template);
      if (target) { target.reading = target.reading || { pos: 0 }; toast(`已加入队列：${clip(target.title, 16)}`); }
      return render();
    }
    case 'doc-more':
      return openMenu(el, [
        { act: 'add-reading', label: '加入阅读队列' },
        { act: 'export-md', label: '导出 Markdown' },
        { act: 'make-excerpt', label: '摘录当前段' },
        ...(pluginMenu('note.overflow').length ? ['-', ...pluginMenu('note.overflow')] : []),
        '-',
        { act: 'delete-note', label: '删除这篇', danger: true },
      ]);
    case 'area-more':
      return openMenu(el, [
        { act: 'import', label: '导入…' },
        { act: 'export-lib', label: '导出整库' },
        '-',
        { act: 'palette', label: '命令面板', key: '⌘K' },
        ...(pluginMenu('toolbar.overflow').length ? ['-', ...pluginMenu('toolbar.overflow')] : []),
      ]);
    case 'import': return openImport();
    case 'export-archive': closeModal(); return toast('已导出 .remitlib（明文 + 资产 + 调度史）');
    case 'export-csv': closeModal(); return toast('已导出 cards.csv，' + CARDS.length + ' 行');
    case 'export-lib': return openExport();
    case 'export-md': return toast('已导出：notes/' + clip(noteById(state.noteId).title, 12) + '.md');
    case 'make-excerpt':
      if (!state.currentBlock) return toast('先选一段');
      { const child = excerptFor(state.noteId, state.currentBlock); render();
        return toast('已摘录为子笔记', '打开', () => openNote(child.id)); }
    case 'delete-note': {
      const i = NOTES.findIndex((n) => n.id === state.noteId);
      if (i >= 0) NOTES.splice(i, 1);
      toast('笔记已删除'); return go('write', 'notes');
    }
    case 'clear-wfilter': state.writeFilter = 'all'; state.writeQuery = ''; state.tagFilter = ''; return render();
    case 'clear-tag': state.tagFilter = ''; return render();
    case 'cycle-sort': {
      const order = ['edited', 'title', 'cards'];
      state.noteSort = order[(order.indexOf(state.noteSort) + 1) % order.length];
      return render();
    }
    case 'export': return openExport();
    case 'run-plugin': toast('已重新运行：解析 3 个块，生成 3 张卡');
      return render();
    case 'clear-selection': state.selected.clear(); return render();
    case 'freeze-selected':
      state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) c.ref = 'frozen'; });
      toast(`已冻结 ${state.selected.size} 张的引用`); state.selected.clear(); return render();
    case 'undo-review': {
      const s = state.session;
      if (!s || s.idx === 0) return toast('这一步没有可撤销的评分');
      s.idx -= 1; s.revealed = false; state.reviewedToday = Math.max(0, state.reviewedToday - 1);
      return renderSession();
    }
    case 'edit-card': return openCardDialog(noteById(CARDS.find((c) => c.id === el.dataset.card).note).id,
      CARDS.find((c) => c.id === el.dataset.card).block, el.dataset.card);
    case 'insert-after': {
      state.currentBlock = blockId || state.currentBlock;
      const cur = noteById(state.noteId).blocks.find((b) => b.id === state.currentBlock);
      return openMenu(el, [
        { act: 'ins:p', label: '正文' }, { act: 'ins:h2', label: '标题' },
        { act: 'ins:quote', label: '引用' }, { act: 'ins:code', label: '代码块' },
        '-', { act: 'ins:table', label: '表格（只读）' }, { act: 'ins:formula', label: '公式' },
        cur ? { act: 'ins:card', label: '为这一段制卡' } : null,
        ...(pluginMenu('editor.slash').length ? ['-', ...pluginMenu('editor.slash')] : []),
      ].filter(Boolean));
    }
    case 'block-menu': {
      const b = noteById(state.noteId).blocks.find((x) => x.id === blockId);
      return openMenu(el, [
        { act: `up:${blockId}`, label: '上移' }, { act: `down:${blockId}`, label: '下移' },
        { act: `dup:${blockId}`, label: '复制这一段' },
        { act: `conv:${blockId}`, label: b.kind === 'h2' ? '转为正文' : '转为标题' },
        '-', { act: `ex:${blockId}`, label: '摘录这一段' }, { act: `card:${blockId}`, label: '为这一段制卡' },
        { act: `inspect:${blockId}`, label: '三种角色…' },
        ...(BOARD_NODES.some((n) => n.note === state.noteId && n.block === blockId)
          ? [{ act: `board:${blockId}`, label: '在白板里定位' }] : []),
        ...(pluginMenu('note.block').length ? ['-', ...pluginMenu('note.block')] : []),
        '-', { act: `del:${blockId}`, label: '删除', danger: true },
      ]);
    }
    case 'toast-token': return toast('token 已复制到剪贴板（原型不写入真实值）');
    case 'check-update': return toast('已是最新版本 0.8');
    case 'reindex': return toast('索引已重建 · 182 篇 / 412 张');
    default:
      if (act.startsWith('menu-open:')) return openNote(act.slice(10));
      if (act.startsWith('menu-read:')) return openNote(act.slice(10), 'read');
      if (act.startsWith('menu-queue:')) {
        const n = noteById(act.slice(11));
        if (n) { n.reading = n.reading || { pos: 0 }; render(); toast(`已加入队列：${clip(n.title, 16)}`); }
        return;
      }
      if (act.startsWith('menu-delete:')) {
        const id = act.slice(12);
        const i = NOTES.findIndex((n) => n.id === id);
        if (i >= 0) NOTES.splice(i, 1);
        render(); return toast('笔记已删除');
      }
      if (act.startsWith('up:')) return moveBlock(act.slice(3), -1);
      if (act.startsWith('down:')) return moveBlock(act.slice(5), 1);
      if (act.startsWith('dup:')) {
        const note = noteById(state.noteId);
        const i = note.blocks.findIndex((b) => b.id === act.slice(4));
        if (i >= 0) { note.blocks.splice(i + 1, 0, { ...note.blocks[i], id: `b${Date.now().toString(36).slice(-4)}` }); render(); }
        return;
      }
      if (act.startsWith('conv:')) {
        const b = noteById(state.noteId).blocks.find((x) => x.id === act.slice(5));
        if (b) { b.kind = b.kind === 'h2' ? 'p' : 'h2'; render(); } return;
      }
      if (act.startsWith('del:')) return deleteBlock(act.slice(4));
      if (act.startsWith('ex:')) {
        const child = excerptFor(state.noteId, act.slice(3)); render();
        return toast('已摘录为子笔记', '打开', () => openNote(child.id));
      }
      if (act.startsWith('card:')) {
        return openCardDialog(state.noteId, act.slice(5));
      }
      if (act.startsWith('col:')) {
        const k = act.slice(4); state.cardCols[k] = !state.cardCols[k]; return render();
      }
      if (act === 'bulk-suspend') {
        state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) { c.state = 'suspended'; c.due = '暂停'; } });
        toast(`已暂停 ${state.selected.size} 张`); state.selected.clear(); return render();
      }
      if (act.startsWith('bulk-due:')) {
        const to = act.endsWith('week') ? '1 周后' : '明天';
        state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) c.due = to; });
        toast(`${state.selected.size} 张改到${to}`); state.selected.clear(); return render();
      }
      if (act === 'bulk-bury') {
        const keys = new Set([...state.selected].map((id) => { const c = CARDS.find((x) => x.id === id); return c ? c.note + ':' + c.block : ''; }));
        let n = 0;
        CARDS.forEach((c) => { if (!state.selected.has(c.id) && keys.has(c.note + ':' + c.block) && c.due === '今天') { c.due = '明天'; n++; } });
        toast(`已埋藏 ${n} 张同块的卡`); state.selected.clear(); return render();
      }
      if (act === 'bulk-freeze') {
        state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) c.ref = 'frozen'; });
        toast(`已冻结 ${state.selected.size} 张的引用`); state.selected.clear(); return render();
      }
      if (act === 'bulk-delete') {
        const n = state.selected.size;
        for (let i = CARDS.length - 1; i >= 0; i--) if (state.selected.has(CARDS[i].id)) CARDS.splice(i, 1);
        state.selected.clear(); render(); return toast(`已删除 ${n} 张`);
      }
      if (act.startsWith('inspect:')) return openBlockInspector(state.noteId, act.slice(8));
      if (act.startsWith('board:')) {
        state.area = 'plugin'; state.pluginPage = 'board'; state.boardFocus = act.slice(6);
        return render();
      }
      if (act.startsWith('pg:')) {
        const [, pid, slot, idx] = act.split(':');
        const pl = PLUGINS.find((x) => x.id === pid);
        const label = pl && pl.menus && pl.menus[slot] ? pl.menus[slot][Number(idx)] : '';
        return toast(`插件「${pl ? pl.name : pid}」· ${label}`);
      }
      if (act.startsWith('ins:')) {
        const kind = act.slice(4);
        if (kind === 'card') return openCardDialog(state.noteId, state.currentBlock);
        const defaults = { table: '| 列 A | 列 B |\n| --- | --- |\n| 值 | 值 |', formula: 'w = (1 - ln(1 - 0.9) / S)' };
        return insertBlock(state.currentBlock, kind, defaults[kind] || '');
      }
  }
}

/* ═══ 事件 ════════════════════════════════════════════════════════════════ */
function bind() {
  $('#menu').addEventListener('click', (e) => {
    const item = e.target.closest('[data-menu]');
    const owner = menuOwner;
    closeMenu();
    if (item) runAction(item.dataset.menu, owner);
  });

  $('#rail-toggle').addEventListener('click', () => {
    state.railWide = !state.railWide;
    $('#app').dataset.rail = state.railWide ? 'full' : 'mini';
  });

  // 选区动作条
  document.addEventListener('mouseup', (e) => {
    if (e.target && e.target.closest && e.target.closest('.selbar')) return;
    setTimeout(showSelBar, 0);
  });
  document.addEventListener('scroll', hideSelBar, true);
  document.addEventListener('selectionchange', () => {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed) hideSelBar();
  });

  document.addEventListener('click', (e) => {
    const t = e.target;

    const pp = t.closest('[data-plugin-page]');
    if (pp) { state.area = 'plugin'; state.pluginPage = pp.dataset.pluginPage; state.boardFocus = ''; render(); return; }

    const area = t.closest('[data-area]');
    if (area) { const a = area.dataset.area; if (a === 'plugin') { state.area = 'plugin'; state.pluginPage = 'cards'; state.noteId = null; render(); } else go(a); return; }

    const sub = t.closest('[data-sub]');
    if (sub) { go(state.area, sub.dataset.sub); return; }

    const act = t.closest('[data-act]');
    if (act) { runAction(act.dataset.act, act); return; }

    const selBtn = t.closest('[data-sel]');
    if (selBtn) {
      const kind = selBtn.dataset.sel, bid = selBtn.dataset.block, txt = selBar ? selBar._text : '';
      hideSelBar();
      if (kind === 'cloze') return doCloze(bid, txt);
      if (kind === 'card') return openCardDialog(state.noteId, bid);
      if (kind === 'excerpt') {
        const child = excerptFor(state.noteId, bid); render();
        return toast('已摘录整段为子笔记', '打开', () => openNote(child.id));
      }
    }

    const impKind = t.closest('[data-import-kind]');
    if (impKind) { state.importKind = impKind.dataset.importKind; state.importStep = 1; return openImport(); }
    if (t.closest('[data-import-back]')) { state.importStep = 0; return openImport(); }
    if (t.closest('[data-import-next]')) { state.importStep = 2; return openImport(); }

    const scopeKind = t.closest('[data-scopekind]');
    if (scopeKind) { state.reviewScope = { kind: scopeKind.dataset.scopekind, value: '' }; return openScope(); }
    const scopeVal = t.closest('[data-scope-value]');
    if (scopeVal) { state.reviewScope.value = scopeVal.dataset.scopeValue; return openScope(); }
    const scopeStart = t.closest('[data-scope-start]');
    if (scopeStart) { closeModal(); return openReview(dueForScope().map((c) => c.id)); }

    const relink = t.closest('[data-relink]');
    if (relink) { state.relinkCard = relink.dataset.relink; return render(); }
    const relinkTo = t.closest('[data-relink-to]');
    if (relinkTo) {
      const c = CARDS.find((x) => x.id === state.relinkCard);
      if (c) { c.block = relinkTo.dataset.relinkTo; c.ref = 'live'; }
      state.relinkCard = null; render(); return toast('已重新指向，调度史不变');
    }

    const freezeBtn = t.closest('[data-freeze]');
    if (freezeBtn) {
      const c = CARDS.find((x) => x.id === freezeBtn.dataset.freeze);
      if (c) c.ref = 'frozen';
      closeModal(); render(); return toast('已冻结：保留快照，不再跟随块');
    }
    const cardEdit = t.closest('[data-card-edit]');
    if (cardEdit) {
      const c = CARDS.find((x) => x.id === cardEdit.dataset.cardEdit);
      closeModal();
      return openCardDialog(c.note, c.block, c.id);
    }

    const tog = t.closest('[data-toggle]');
    if (tog) {
      const k = tog.dataset.toggle;
      if (state.on.has(k)) state.on.delete(k); else state.on.add(k);
      tog.setAttribute('aria-checked', String(state.on.has(k)));
      return;
    }

    const tagEl = t.closest('[data-tag]');
    if (tagEl) { state.tagFilter = tagEl.dataset.tag; state.area = 'write'; state.sub = 'notes'; return render(); }

    const theme = t.closest('[data-themepick]');
    if (theme) { state.themePref = theme.dataset.themepick; applyTheme(); render(); return; }

    const dm = t.closest('[data-docmode]');
    if (dm) { state.docMode = dm.dataset.docmode; render(); return; }

    const pt = t.closest('[data-paneltab]');
    if (pt) { state.panelTab = pt.dataset.paneltab; render(); return; }

    const wf = t.closest('[data-wfilter]');
    if (wf) { state.writeFilter = wf.dataset.wfilter; render(); return; }

    const cf = t.closest('[data-cfilter]');
    if (cf) { state.cardFilter = cf.dataset.cfilter; render(); return; }

    const qf = t.closest('[data-qfilter]');
    if (qf) { state.cardFilter = FILTER_KEYS[qf.dataset.qfilter] || 'all'; render(); return; }

    const tf = t.closest('[data-typefilter]');
    if (tf) { state.typeFilter = state.typeFilter === tf.dataset.typefilter ? '' : tf.dataset.typefilter; render(); return; }

    const rf = t.closest('[data-reffilter]');
    if (rf) { state.refFilter = state.refFilter === rf.dataset.reffilter ? '' : rf.dataset.reffilter; render(); return; }

    const ts = t.closest('[data-typesel]');
    if (ts) { state.typeSel = ts.dataset.typesel; render(); return; }

    const te = t.closest('[data-typeedit]');
    if (te) return openTypeDialog(te.dataset.typeedit);

    const sv = t.closest('[data-saved]');
    if (sv) { state.cardFilter = 'hard'; render(); return toast('已套用保存的筛选：' + (state.saved.find((x) => x.id === sv.dataset.saved) || {}).label); }

    const sus = t.closest('[data-suspend]');
    if (sus) {
      const c = CARDS.find((x) => x.id === sus.dataset.suspend);
      if (c) { c.state = 'learning'; c.due = '明天'; }
      render(); return toast('已暂停这张卡');
    }

    const oc2 = t.closest('[data-open-card2]');
    if (oc2) {
      const c = CARDS.find((x) => x.id === oc2.dataset.openCard2);
      return openCardDialog(c.note, c.block, c.id);
    }

    const bt = t.closest('[data-bulk-type]');
    if (bt) {
      const n = state.selected.size;
      state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) c.type = bt.dataset.bulkType; });
      closeModal(); state.selected.clear(); render();
      return toast(`${n} 张改成 ${bt.dataset.bulkType}`);
    }

    const bts = t.closest('[data-bulk-tag-save]');
    if (bts) {
      const tag = ($('#bulk-tag-input') || {}).value || '';
      const v = tag.trim().replace(/^#/, '');
      if (v) {
        state.selected.forEach((id) => { const c = CARDS.find((x) => x.id === id); if (c) { c.tags = c.tags || []; if (!c.tags.includes(v)) c.tags.push(v); } });
        closeModal(); render(); return toast(`已加标签 #${v}`);
      }
      return;
    }

    const bp = t.closest('[data-brokenpref]');
    if (bp) {
      bp.parentElement.querySelectorAll('[data-brokenpref]').forEach((b) => b.setAttribute('aria-pressed', String(b === bp)));
      return;
    }

    const cs = t.closest('[data-csort]');
    if (cs) { const k = cs.dataset.csort; state.cardSort = { key: k, dir: state.cardSort.key === k ? -state.cardSort.dir : 1 }; render(); return; }

    const pick = t.closest('[data-pick]');
    if (pick) {
      if (pick.checked) state.selected.add(pick.dataset.pick); else state.selected.delete(pick.dataset.pick);
      const row = pick.closest('tr'); if (row) row.setAttribute('aria-selected', String(pick.checked));
      syncSelectionUI();
      return;
    }

    const noteBtn = t.closest('[data-note]');
    if (noteBtn) {
      if ($('#modal').open) closeModal();
      const menuBtn = t.closest('[data-note-menu]');
      if (menuBtn) {
        const id = noteBtn.dataset.note;
        return openMenu(menuBtn, [
          { act: 'menu-open', label: '打开' },
          { act: 'menu-read', label: '阅读态' },
          { act: 'menu-queue', label: '加入阅读队列' },
          ...(pluginMenu('sidebar.note').length ? ['-', ...pluginMenu('sidebar.note')] : []),
          '-',
          { act: 'menu-delete', label: '删除', danger: true },
        ].map((it) => it.act && it.act.startsWith('menu-')
          ? { ...it, act: `${it.act}:${id}` } : it));
      }
      const openBtn = t.closest('[data-note-open]');
      return openNote(noteBtn.dataset.note, openBtn ? openBtn.dataset.mode : (noteBtn.dataset.mode || 'edit'));
    }

    const queue = t.closest('[data-queue]');
    if (queue) return queue.dataset.queue === 'read' ? openReading(queue.dataset.id) : openReview();

    const reading = t.closest('[data-reading]');
    if (reading) return openReading(reading.dataset.reading);

    const tpl = t.closest('[data-template]');
    if (tpl) return newNote(tpl.dataset.template);

    const typeBtn = t.closest('[data-type]');
    if (typeBtn) return openTypeDialog(typeBtn.dataset.type);

    const cardRow = t.closest('[data-card-row]');
    if (cardRow && !t.closest('[data-pick]')) {
      const c = CARDS.find((x) => x.id === cardRow.dataset.cardRow);
      if (c) return openCardDetail(c.id);
    }

    const cardBtn = t.closest('[data-card]');
    if (cardBtn) return jumpToSource(cardBtn.dataset.card);

    const bnode = t.closest('[data-board-node]');
    if (bnode) {
      const n = boardNode(bnode.dataset.boardNode);
      if (!n) return;
      openNote(n.note, 'read');
      state.currentBlock = n.block;
      render();
      return setTimeout(() => scrollToBlock(n.block), 40);
    }

    const inspectEl = t.closest('[data-inspect]');
    if (inspectEl) return openBlockInspector(state.noteId, inspectEl.dataset.inspect);

    const openCardEl = t.closest('[data-open-card]');
    if (openCardEl) return openCardDetail(openCardEl.dataset.openCard);

    const blockCardEl = t.closest('[data-blockcard]');
    if (blockCardEl) { closeModal(); return openCardDialog(state.noteId, blockCardEl.dataset.blockcard); }

    const jumpBlock = t.closest('[data-jump-block]');
    if (jumpBlock) {
      closeModal();
      state.currentBlock = jumpBlock.dataset.jumpBlock;
      render();
      return setTimeout(() => scrollToBlock(jumpBlock.dataset.jumpBlock), 30);
    }

    const boardJump = t.closest('[data-board-jump]');
    if (boardJump) {
      closeModal();
      state.area = 'plugin'; state.pluginPage = 'board'; state.boardFocus = boardJump.dataset.boardJump;
      render();
      return;
    }

    const unarchive = t.closest('[data-unarchive]');
    if (unarchive) { state.archived.delete(unarchive.dataset.unarchive); render(); return toast('已恢复到阅读队列'); }

    const thaw = t.closest('[data-thaw]');
    if (thaw) {
      const c = CARDS.find((x) => x.id === thaw.dataset.thaw);
      if (c) { c.ref = 'live'; }
      render(); return toast('已解冻：重新跟随来源块');
    }

    const undismiss = t.closest('[data-undismiss]');
    if (undismiss) { state.dismissed.delete(undismiss.dataset.undismiss); render(); return toast('已恢复'); }

    const jump = t.closest('[data-jump]');
    if (jump) { state.currentBlock = jump.dataset.jump; render(); return; }

    const blockEl = t.closest('[data-block]');
    if (blockEl && !t.closest('[contenteditable="true"]') && !t.closest('.gutter')) {
      state.currentBlock = blockEl.dataset.block;
      $$('.block.is-current').forEach((b) => b.classList.remove('is-current'));
      blockEl.classList.add('is-current');
      $$('.outline-link').forEach((b) => b.setAttribute('aria-current', String(b.dataset.jump === state.currentBlock)));
      return;
    }

    const rb = t.closest('[data-rblock]');
    if (rb) { state.session.pos = Number(rb.dataset.rblock); renderSession(); return; }
    const dg = t.closest('[data-digest]');
    if (dg) { const note = noteById(state.session.noteId); if (note.reading) note.reading.pos = state.session.pos; return advanceReading(); }
    const gr = t.closest('[data-grade]');
    if (gr) return grade(Number(gr.dataset.grade));
    const ctxBtn = t.closest('[data-ctx]');
    if (ctxBtn) { state.session.ctx = !state.session.ctx; return renderSession(); }
    const src = t.closest('[data-source]');
    if (src) return jumpToSource(src.dataset.source);

    const pal = t.closest('[data-pal]');
    if (pal) return runPalette(Number(pal.dataset.pal));

    const dlg = t.closest('[data-dlg]');
    if (dlg) {
      const k = dlg.dataset.dlg;
      if (k === 'close') return closeModal();
      if (k === 'save-card') {
        const blockId = dlg.dataset.block;
        const existing = CARDS.find((c) => c.note === state.noteId && c.block === blockId);
        const prompt = $('#card-prompt').value.trim();
        const answer = $('#card-answer').value.trim();
        if (!prompt) return;
        if (existing) { existing.prompt = prompt; existing.answer = answer; }
        else makeCard(state.noteId, blockId, prompt, answer);
        closeModal(); render(); return toast('卡片已保存');
      }
      if (k === 'bulk-type-go') { closeModal(); render(); return toast('已改类型'); }
      if (k === 'delete-block') { closeModal(); const bid = dlg.dataset.block; hardDeleteBlock(bid); return toast('已删除，卡片转为断链'); }
      if (k === 'delete-card') {
        const i = CARDS.findIndex((c) => c.note === state.noteId && c.block === dlg.dataset.block);
        if (i >= 0) CARDS.splice(i, 1);
        closeModal(); render(); return toast('卡片已删除');
      }
      return;
    }

    const pluginInfo = t.closest('[data-plugin-info]');
    if (pluginInfo) return openPluginDetail(pluginInfo.dataset.pluginInfo);

    const plugin = t.closest('[data-plugin]');
    if (plugin) {
      const on = plugin.getAttribute('aria-checked') === 'true';
      plugin.setAttribute('aria-checked', String(!on));
      const p = PLUGINS.find((x) => x.id === plugin.dataset.plugin);
      if (p) p.on = !on;
      return render();
    }

    const w = t.closest('[data-welcome]');
    if (w) { state.welcomeState = w.dataset.welcome === 'error' ? 'error' : 'open'; return render(); }

    if (!t.closest('#menu')) closeMenu();
  });

  document.addEventListener('input', (e) => {
    const t = e.target;
    if (t.id === 'write-search') { state.writeQuery = t.value; render(); refocus('write-search'); return; }
    if (t.id === 'search-page') { state.writeQuery = t.value; render(); refocus('search-page'); return; }
    if (t.id === 'card-search') { state.cardQuery = t.value; render(); refocus('card-search'); return; }
    if (t.id === 'palette-input') { state.paletteIndex = 0; paintPalette(t.value); return; }
    if (t.id === 'budget-cards') { state.budget.cards = +t.value; t.nextElementSibling.textContent = `${t.value} 张`; return; }
    if (t.id === 'budget-minutes') { state.budget.minutes = +t.value; t.nextElementSibling.textContent = `${t.value} 分钟`; return; }
    if (t.id === 'retention') { state.retention = +t.value; t.nextElementSibling.textContent = `${t.value}%`; return; }
    if (t.id === 'max-interval') { state.maxInterval = +t.value; t.nextElementSibling.textContent = `${t.value} 天`; return; }
    if (t.id === 'body-size') { state.bodySize = +t.value; applyTheme(); t.nextElementSibling.textContent = `${t.value}px`; return; }
    if (t.id === 'paper-width') { state.paperWidth = +t.value; applyTheme(); t.nextElementSibling.textContent = String(t.value); return; }
    if (t.dataset && t.dataset.body) {
      const note = noteById(state.noteId);
      const b = note.blocks.find((x) => x.id === t.dataset.body);
      if (b) b.text = t.innerText;
      renderStatus();
      return;
    }
    if (t.hasAttribute && t.hasAttribute('data-note-title')) {
      const note = noteById(state.noteId);
      if (note) note.title = t.innerText.trim() || '无标题';
      renderStatus();
    }
  });

  document.addEventListener('change', (e) => {
    const t = e.target;
    if (t.id === 'excerpt-card') state.readingToCard = t.checked;
    if (t.id === 'check-all') {
      $$('[data-pick]').forEach((cb) => {
        cb.checked = t.checked;
        if (t.checked) state.selected.add(cb.dataset.pick); else state.selected.delete(cb.dataset.pick);
        const row = cb.closest('tr'); if (row) row.setAttribute('aria-selected', String(t.checked));
      });
      syncSelectionUI();
    }
  });

  document.addEventListener('keydown', (e) => {
    const mod = e.metaKey || e.ctrlKey;

    if (mod && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      if (!$('#palette').open) openPalette();
      return;
    }
    if (mod && e.key === '/') { e.preventDefault(); return openKeysHelp(); }

    if (!$('#menu').hidden) {
      const items = Array.from(document.querySelectorAll('#menu .menu-item'));
      const cur = items.indexOf(document.activeElement);
      if (e.key === 'ArrowDown') { e.preventDefault(); items[(cur + 1 + items.length) % items.length].focus(); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); items[(cur - 1 + items.length) % items.length].focus(); return; }
      if (e.key === 'Enter' && cur >= 0) { e.preventDefault(); document.activeElement.click(); return; }
    }
    if (e.key === 'Escape') {
      if (!$('#menu').hidden) return closeMenu();
      if ($('#modal').open) return closeModal();
      if ($('#palette').open) return $('#palette').close();
      if (state.session) { state.session = null; return renderSession(); }
      if (state.area === 'write' && state.sub === 'doc') return go('write', 'notes');
      return;
    }

    if ($('#palette').open) {
      if (e.key === 'ArrowDown') { e.preventDefault(); state.paletteIndex = Math.min(state.paletteIndex + 1, (state.paletteFlat || []).length - 1); paintPalette($('#palette-input').value); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); state.paletteIndex = Math.max(0, state.paletteIndex - 1); paintPalette($('#palette-input').value); return; }
      if (e.key === 'Enter') { e.preventDefault(); return runPalette(state.paletteIndex); }
      return;
    }

    const inField = /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement.tagName) || document.activeElement.isContentEditable;

    if (state.session && !inField) {
      if (state.session.kind === 'read') {
        const len = noteById(state.session.noteId).blocks.length;
        if (e.key === 'j' || e.key === 'J' || e.key === 'ArrowDown') { e.preventDefault(); state.session.pos = Math.min(state.session.pos + 1, len - 1); return renderSession(); }
        if (e.key === 'k' || e.key === 'K' || e.key === 'ArrowUp') { e.preventDefault(); state.session.pos = Math.max(0, state.session.pos - 1); return renderSession(); }
        if (['1', '2', '3'].includes(e.key)) { e.preventDefault(); const n = noteById(state.session.noteId); if (n.reading) n.reading.pos = state.session.pos; return advanceReading(); }
        if (e.key.toLowerCase() === 'e') { e.preventDefault(); return doExcerpt(); }
        if (e.key.toLowerCase() === 's') { e.preventDefault(); return advanceReading(); }
        if (e.key.toLowerCase() === 'a') { e.preventDefault(); return runAction('archive'); }
        if (e.key.toLowerCase() === 'd') { e.preventDefault(); return runAction('dismiss'); }
        if (e.key.toLowerCase() === 'p') { e.preventDefault(); return runAction('postpone'); }
        if (e.key.toLowerCase() === 'x') { e.preventDefault(); const n = noteById(state.session.noteId); n.reading = null; return advanceReading(); }
        if (e.key.toLowerCase() === 'm') { e.preventDefault(); return runAction('open-doc'); }
      } else {
        if (e.key === ' ') { e.preventDefault(); if (!state.session.revealed) runAction('reveal'); return; }
        if (state.session.revealed && ['1', '2', '3', '4'].includes(e.key)) { e.preventDefault(); return grade(Number(e.key)); }
      }
    }

    if (state.area === 'write' && state.sub === 'doc' && !inField && e.key === '/') {
      e.preventDefault();
      const anchor = $('.block.is-current .g-btn[data-act="insert-after"]') || $('.g-btn[data-act="insert-after"]');
      if (anchor) runAction('insert-after', anchor);
    }
  });

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (state.themePref === 'system') { applyTheme(); render(); }
  });
}

function syncSelectionUI() {
  const n = state.selected.size;
  const total = document.querySelectorAll('[data-pick]').length;
  const count = $('#sel-count');
  if (count) count.textContent = n ? `已选 ${n} 张` : `${total} 张`;
  const review = $('[data-act="review-selected"]');
  if (review) { review.disabled = n === 0; review.textContent = `复习所选${n ? `（${n}）` : ''}`; }
  const bulk = $('[data-act="bulk"]');
  if (bulk) { bulk.disabled = n === 0; bulk.textContent = `批量动作${n ? `（${n}）` : ''}`; }
  for (const a of ['freeze-selected', 'clear-selection']) {
    const b = $(`[data-act="${a}"]`);
    if (b) b.disabled = n === 0;
  }
}

function refocus(id) {
  const el = document.getElementById(id);
  if (!el) return;
  el.focus();
  if (el.setSelectionRange) el.setSelectionRange(el.value.length, el.value.length);
}

/* ═══ 补充界面：资产 / 断链 / 复习范围 / 卡片详情 / 快捷键 ═══════════ */

const ASSETS = [
  { hash: '9f3c0a1b', ext: 'png', size: '184 KB', usedBy: ['n1', 'n8'] },
  { hash: '6be20d44', ext: 'png', size: '72 KB', usedBy: ['n8'] },
  { hash: '1c8f47e2', ext: 'jpg', size: '412 KB', usedBy: ['n5'] },
  { hash: 'e30a95f0', ext: 'svg', size: '3 KB', usedBy: ['n1', 'n8', 'n4'] },
];

const excerptChildren = (id) => NOTES.filter((n) => n.excerptOf === id);

function treeNode(n, depth) {
  const kids = excerptChildren(n.id);
  return `<li class="tree-li">
    <button class="tree-row" type="button" data-note="${n.id}" style="padding-left:${depth * 20 + 8}px">
      <span class="tree-rail" aria-hidden="true"></span>
      <span class="tree-body">
        <span class="tree-name">${esc(n.title)}</span>
        <span class="tree-meta">${cardsOf(n.id).length} 张卡${kids.length ? ` · ${kids.length} 条再摘` : ''}</span>
      </span>
      ${icon('chev', 14)}
    </button>
    ${kids.length ? `<ul class="tree-kids">${kids.map((k) => treeNode(k, depth + 1)).join('')}</ul>` : ''}
  </li>`;
}

function viewExcerptTree() {
  const roots = NOTES.filter((n) => !n.excerptOf && excerptChildren(n.id).length);
  const orphans = NOTES.filter((n) => n.excerptOf && !noteById(n.excerptOf));
  return page('m-list', `
    ${pageHead('摘录树', `${NOTES.filter((n) => n.excerptOf).length} 条摘录 · 摘录可以再摘，形成层级`,
      `<button class="btn" type="button" data-act="start-reading">${icon('read', 14)} 继续读</button>`)}
    ${roots.length ? `<ul class="tree">${roots.map((n) => treeNode(n, 0)).join('')}</ul>`
      : empty('quote', '还没有摘录', '阅读时按 E 会把当前段落变成子笔记，带着指回原块的引用。',
        `<button class="btn" type="button" data-act="start-reading">开始读一篇</button>`)}
    ${orphans.length ? `<div class="section"><div class="section-head"><h2>源笔记已不在</h2></div>
      <p class="dim" style="font-size:var(--t-label)">${orphans.length} 条摘录的源笔记被删除了，引用会显示为断链，但摘录本身还在。</p></div>` : ''}
  `);
}

function viewArchive() {
  const list = NOTES.filter((n) => state.archived.has(n.id));
  const dismissed = NOTES.filter((n) => state.dismissed.has(n.id));
  return page('m-list', `
    ${pageHead('归档', '读完的就收起来；仍可搜索、仍可被引用', '')}
    ${list.length ? `<ul class="list stagger">${list.map((n) => `
      <li><div class="row" style="grid-template-columns:minmax(0,1fr) auto;cursor:default">
        <span class="body"><span class="title">${esc(n.title)}</span>
          <span class="sub">${cardsOf(n.id).length} 张卡 · 归档后不再进队列</span></span>
        <span class="tail">
          <button class="btn btn--quiet" type="button" data-note="${n.id}">打开</button>
          <button class="btn btn--quiet" type="button" data-unarchive="${n.id}">恢复</button>
        </span></div></li>`).join('')}</ul>`
      : empty('layers', '归档里还是空的', '阅读会话里按 A 会把读完的笔记收进来；它不会离开库，只是不再排进今天。')}
    ${dismissed.length ? `<div class="section">
      <div class="section-head"><h2>不再出现</h2><span class="hint">${dismissed.length} 篇</span></div>
      <ul class="list">${dismissed.map((n) => `<li><div class="row" style="grid-template-columns:minmax(0,1fr) auto;cursor:default">
        <span class="body"><span class="title">${esc(n.title)}</span></span>
        <span class="tail"><button class="btn btn--quiet" type="button" data-undismiss="${n.id}">恢复</button></span></div></li>`).join('')}</ul>
    </div>` : ''}
  `);
}

function viewAssets() {
  return page('m-list', `
    ${pageHead('资产', `${ASSETS.length} 个文件 · 按内容命名，同图只存一份`,
      `<button class="btn" type="button" data-act="import">${icon('upload', 14)} 导入</button>`)}
    <div class="table-wrap">
      <table class="grid">
        <colgroup><col style="width:44px"><col><col style="width:96px"><col style="width:130px"><col style="width:96px"></colgroup>
        <thead><tr><th></th><th>文件</th><th>大小</th><th>被引用</th><th>格式</th></tr></thead>
        <tbody>${ASSETS.map((a) => `<tr>
          <td><span class="asset-ext">${a.ext}</span></td>
          <td class="ink"><span class="asset-hash">${a.hash}.${a.ext}</span></td>
          <td>${a.size}</td>
          <td>${a.usedBy.length} 篇 · ${esc(clip(noteById(a.usedBy[0]).title, 10))}</td>
          <td>${badge(a.ext.toUpperCase(), 'plain')}</td>
        </tr>`).join('')}</tbody>
      </table>
    </div>
    <div class="section"><p class="dim" style="font-size:var(--t-label);line-height:1.75">
      资产随库目录一起走，拷到另一台机器引用依然成立；没有「附件」这种外挂概念。</p></div>`);
}

function viewRelink() {
  const lost = CARDS.filter((c) => c.ref === 'lost');
  const frozen = CARDS.filter((c) => c.ref === 'frozen');
  const current = lost.find((c) => c.id === state.relinkCard) || lost[0] || null;
  const note = current ? noteById(current.note) : null;

  return page('m-list', `
    ${pageHead('断链', `${lost.length} 张卡片的路断了 · ${frozen.length} 张已冻结`,
      `<button class="btn" type="button" data-act="relink-all">${icon('check', 14)} 全部重新指向</button>`)}

    ${lost.length ? `<div class="split-2">
      <div class="pane"><div class="pane-pad">
        <p class="eyebrow" style="margin-bottom:8px">路断了的卡片</p>
        <ul class="list" style="border-top:0">
          ${lost.map((c) => `<li><button class="row" type="button" data-relink="${c.id}"
            aria-current="${current && c.id === current.id}" style="grid-template-columns:minmax(0,1fr) auto">
            <span class="body"><span class="title">${esc(clip(c.prompt, 18))}</span>
              <span class="sub">原块已不在，快照也没留下</span></span>
            <span class="tail">${badge('已断', 'danger')}</span></button></li>`).join('')}
        </ul>
      </div></div>
      <div class="pane"><div class="pane-pad">
        ${current ? `
          <p class="eyebrow" style="margin-bottom:8px">重新指向《${esc(note.title)}》的哪一段</p>
          <p class="dim" style="font-size:var(--t-label);margin-bottom:12px">
            选中的块会成为这张卡的新来源；调度史不动，只换路。</p>
          <ul class="list" style="border-top:0">
            ${note.blocks.map((b, i) => `<li><button class="row" type="button" data-relink-to="${b.id}"
              style="grid-template-columns:34px minmax(0,1fr)">
              <span class="row-kind">${i + 1}</span>
              <span class="body"><span class="title" style="font-weight:400">${esc(clip(b.text, 40))}</span></span>
            </button></li>`).join('')}
          </ul>` : `<p class="dim">没有可重连的卡片。</p>`}
      </div></div>
    </div>` : empty('check', '没有断链', '所有卡片的来源块都还在。删掉一个被卡片引用的块，它会出现在这里。')}

    ${frozen.length ? `<div class="section">
      <div class="section-head"><h2>已冻结</h2><span class="hint">保留快照，不再跟随块</span></div>
      <ul class="list stagger">${frozen.map((c) => `<li><div class="row" style="grid-template-columns:minmax(0,1fr) auto;cursor:default">
        <span class="body"><span class="title">${esc(clip(c.prompt, 20))}</span>
          <span class="sub">${esc(noteById(c.note).title)}</span></span>
        <span class="tail">${badge('快照', 'warn')}
          <button class="btn btn--quiet" type="button" data-thaw="${c.id}">解冻</button></span></div></li>`).join('')}</ul>
    </div>` : ''}`);
}

function dueForScope() {
  const sc = state.reviewScope;
  if (sc.kind === 'note') return dueCards().filter((c) => c.note === sc.value);
  if (sc.kind === 'tag') {
    return dueCards().filter((c) => (noteById(c.note).tags || []).includes(sc.value));
  }
  return dueCards();
}

function openScope() {
  const sc = state.reviewScope;
  const notes = NOTES.filter((n) => dueCards().some((c) => c.note === n.id));
  const tags = [...new Set(NOTES.flatMap((n) => n.tags || []))];
  const count = dueForScope().length;

  openModal(`
    <div class="modal-head"><h2>复习范围</h2><p>范围是取卡的一次选择，不落库；organization 靠笔记与标签。</p></div>
    <div class="modal-body">
      ${seg([['all', '全库'], ['note', '某篇笔记'], ['tag', '某个标签']], sc.kind, 'scopekind')}
      <div style="height:14px"></div>
      ${sc.kind === 'note' ? `<ul class="list" style="border-top:0">${notes.map((n) => `
        <li><button class="row" type="button" data-scope-value="${n.id}" aria-current="${sc.value === n.id}"
          style="grid-template-columns:minmax(0,1fr) auto">
          <span class="body"><span class="title">${esc(n.title)}</span></span>
          <span class="tail">${dueCards().filter((c) => c.note === n.id).length} 张</span></button></li>`).join('')}</ul>`
      : sc.kind === 'tag' ? `<div class="od-cluster">${tags.map((t) => chip('#' + t, sc.value === t, `data-scope-value="${esc(t)}"`, dueCards().filter((c) => (noteById(c.note).tags || []).includes(t)).length)).join('')}</div>`
      : `<p class="dim" style="font-size:var(--t-label);line-height:1.7">全库：${dueCards().length} 张今天到期。</p>`}
    </div>
    <div class="modal-foot"><span class="dim" style="font-size:var(--t-label)">本次取 ${count} 张</span>
      <span class="push"></span>
      <button class="btn" type="button" data-dlg="close">取消</button>
      <button class="btn btn--primary" type="button" data-scope-start ${count ? '' : 'disabled'}>开始复习</button></div>`);
}

function cardHistory(card) {
  const seed = card.id.charCodeAt(1) || 3;
  const base = 2 + (seed % 5);
  return [
    { at: '今天 09:12', kind: 'review', what: '良好', meta: `间隔 ${base} 天 → ${base * 2} 天` },
    { at: `${base} 天前`, kind: 'read', what: '读过这一段', meta: '阅读位置记录' },
    { at: `${base + 3} 天前`, kind: 'review', what: '困难', meta: '间隔 1 天 → ' + base + ' 天' },
    { at: `${base + 9} 天前`, kind: 'review', what: '新卡入库', meta: '由 :: 语法生成' },
  ];
}

function openBlockInspector(noteId, blockId) {
  const note = noteById(noteId);
  const b = note.blocks.find((x) => x.id === blockId);
  if (!b) return;
  const cards = cardsOn(noteId, blockId);
  const kids = NOTES.filter((n) => n.excerptOf === noteId && n.sourceBlock === blockId);
  const idx = note.blocks.findIndex((x) => x.id === blockId) + 1;
  const isPos = note.reading && note.blocks[note.reading.pos] && note.blocks[note.reading.pos].id === blockId;
  const REF = { live: ['跟随', 'ok'], frozen: ['快照', 'warn'], lost: ['已断', 'danger'] };
  const STATE = { new: '新', learning: '学习中', review: '复习中' };
  const acts = pluginMenu('note.block');

  openModal(`
    <div class="modal-head">
      <h2>第 ${idx} 段 · 三种角色</h2>
      <p>${esc(note.title)} · ${note.blocks.length} 段${isPos ? ' · <span style="color:var(--mark)">阅读停在这里</span>' : ''}</p>
    </div>
    <div class="modal-body">
      <div class="tile" style="background:var(--page)">
        <span class="eyebrow">正文</span>
        <p style="font-size:15px;line-height:1.75">${inline(b.text) || '<span class="dim">空段</span>'}</p>
      </div>

      <div class="section" style="margin-top:16px">
        <div class="section-head"><h2>卡片 · ${cards.length}</h2>
          <span class="push"></span>
          <button class="btn btn--quiet" type="button" data-blockcard="${blockId}">为这一段制卡</button></div>
        ${cards.length ? `<ul class="list" style="border-top:0">${cards.map((c) => `
          <li><button class="row" type="button" data-open-card="${c.id}" style="grid-template-columns:minmax(0,1fr) auto">
            <span class="body"><span class="title" style="font-weight:400">${esc(clip(c.prompt, 30))}</span>
              <span class="sub">${c.type} · ${STATE[c.state] || c.state} · ${c.due}</span></span>
            <span class="tail">${badge(REF[c.ref][0], REF[c.ref][1])}</span></button></li>`).join('')}</ul>`
          : `<p class="dim" style="font-size:var(--t-label)">这一段还不是卡。选中一句话按 C 挖空，或者从菜单里制卡。</p>`}
      </div>

      <div class="section">
        <div class="section-head"><h2>摘录 · ${kids.length}</h2></div>
        ${kids.length ? kids.map((k) => `<button class="row" type="button" data-note="${k.id}"
          style="grid-template-columns:minmax(0,1fr) auto;border-top:1px solid var(--line)">
          <span class="body"><span class="title" style="font-weight:400">${esc(k.title)}</span>
            <span class="sub">${cardsOf(k.id).length} 张卡 · 可再摘</span></span>
          <span class="tail">${icon('chev', 14)}</span></button>`).join('')
          : `<p class="dim" style="font-size:var(--t-label)">没有摘录。阅读时按 E 会把这一段变成子笔记。</p>`}
      </div>

      ${acts.length ? `<div class="section">
        <div class="section-head"><h2>插件</h2><span class="hint">note.block 槽</span></div>
        <div class="od-cluster">${acts.map((a) => `<button class="btn" type="button" data-act="${a.act}">${a.label}</button>`).join('')}</div>
      </div>` : ''}
    </div>
    <div class="modal-foot"><span class="push"></span>
      <button class="btn" type="button" data-jump-block="${blockId}">在笔记里定位</button>
      ${BOARD_NODES.some((n) => n.note === noteId && n.block === blockId)
        ? `<button class="btn" type="button" data-board-jump="${blockId}">在白板里定位</button>` : ''}
      <button class="btn btn--primary" type="button" data-dlg="close">关闭</button></div>`);
}

function openCardDetail(cardId) {
  const c = CARDS.find((x) => x.id === cardId);
  if (!c) return;
  const note = noteById(c.note);
  const block = note.blocks.find((b) => b.id === c.block);
  const REF = { live: ['跟随块', 'ok'], frozen: ['快照', 'warn'], lost: ['已断', 'danger'] };

  openModal(`
    <div class="modal-head"><h2>${esc(clip(c.prompt, 26))}</h2>
      <p>${esc(note.title)} · ${badge(c.type, 'plain')} ${badge(REF[c.ref][0], REF[c.ref][1])}</p></div>
    <div class="modal-body">
      <div class="tile" style="background:var(--page)">
        <span class="eyebrow">来源块</span>
        <p style="font-size:15px;line-height:1.7">${block ? esc(clip(block.text, 80)) : '<span class="dim">这个块已经不在了</span>'}</p>
        ${block ? `<button class="btn btn--quiet" type="button" data-source="${c.id}" style="align-self:start">回到原文</button>` : ''}
      </div>
      <div style="height:16px"></div>
      <span class="eyebrow">调度史</span>
      <div class="timeline" style="margin-top:8px">
        ${cardHistory(c).map((e) => `<div class="ev" data-kind="${e.kind}">
          <time>${e.at}</time><span class="node"></span>
          <span><span class="what">${e.what}</span><span class="meta">${e.meta}</span></span>
        </div>`).join('')}
      </div>
    </div>
    <div class="modal-foot">
      <button class="btn btn--quiet" type="button" data-dlg="close">关闭</button>
      <span class="push"></span>
      <button class="btn" type="button" data-freeze="${c.id}">冻结引用</button>
      <button class="btn btn--primary" type="button" data-card-edit="${c.id}">编辑卡面</button>
    </div>`);
}

function openKeysHelp() {
  const rows = [
    ['⌘K', '搜索与命令'], ['⌘/', '这张表'], ['⌘N', '新建笔记'], ['⌘F', '在笔记里查找'],
    ['J / K', '阅读：上下移块'], ['E', '阅读：摘录当前段'], ['空格', '复习：翻面'], ['1–4', '复习：评分'],
    ['/', '文档：插入菜单'], ['Esc', '退出当前的会话或浮层'],
  ];
  openModal(`
    <div class="modal-head"><h2>快捷键</h2><p>一套键位管三个动作，不再有第二套。</p></div>
    <div class="modal-body"><dl class="kv">
      ${rows.map(([k, d]) => `<dt><span class="kbd">${k}</span></dt><dd>${d}</dd>`).join('')}
    </dl></div>
    <div class="modal-foot"><span class="push"></span>
      <button class="btn" type="button" data-dlg="close">关闭</button></div>`);
}

/* 选区动作条 */
let selBar = null;
function hideSelBar() { if (selBar) { selBar.remove(); selBar = null; } }

function showSelBar() {
  const sel = window.getSelection();
  if (!sel || sel.isCollapsed || !sel.rangeCount) return hideSelBar();
  const range = sel.getRangeAt(0);
  const host = (range.commonAncestorContainer.nodeType === 1
    ? range.commonAncestorContainer : range.commonAncestorContainer.parentElement);
  const body = host && host.closest ? host.closest('[data-body]') : null;
  if (!body) return hideSelBar();

  const text = sel.toString().trim();
  if (text.length < 2) return hideSelBar();

  const rect = range.getBoundingClientRect();
  hideSelBar();
  selBar = document.createElement('div');
  selBar.className = 'selbar';
  selBar.innerHTML = `
    <button type="button" data-sel="cloze" data-block="${body.dataset.body}">${icon('quote', 13)} 挖空</button>
    <button type="button" data-sel="card" data-block="${body.dataset.body}">${icon('recall', 13)} 制卡</button>
    <span class="sep"></span>
    <button type="button" data-sel="excerpt" data-block="${body.dataset.body}">${icon('doc', 13)} 摘录整段</button>`;
  selBar.style.left = `${Math.max(12, Math.min(rect.left, window.innerWidth - 260))}px`;
  selBar.style.top = `${Math.max(12, rect.top - 44)}px`;
  selBar._text = text;
  selBar.addEventListener('mousedown', (e) => e.preventDefault());
  document.body.appendChild(selBar);
}

function doCloze(blockId, text) {
  const note = noteById(state.noteId);
  const b = note.blocks.find((x) => x.id === blockId);
  if (!b || !text) return;
  b.text = b.text.includes(text) ? b.text.replace(text, `{{${text}}}`) : `${b.text}{{${text}}}`;
  makeCard(note.id, blockId, b.text, text, 'cloze');
  render();
  toast('已挖空并制卡');
}

/* ═══ 启动 ════════════════════════════════════════════════════════════════ */

/* 深链接：#/area/sub[/note]?mode=&tag=&theme= —— 只读，不改写地址栏 */
function applyHashRoute() {
  const raw = decodeURIComponent((location.hash || '').replace(/^#\/?/, ''));
  if (!raw) return false;
  const [path, query] = raw.split('?');
  const [area, sub, target] = path.split('/');
  if (!AREAS[area] && area !== 'plugin') return false;

  state.screen = 'app';
  state.area = area;
  if (area === 'plugin') state.pluginPage = sub || 'cards';
  if (sub) state.sub = sub;
  else if (AREAS[area] && AREAS[area].subs.length) state.sub = AREAS[area].subs[0][0];

  const qp = new URLSearchParams(query || '');
  if (target) { state.noteId = target; state.sub = 'doc'; }
  if (qp.get('note')) { state.noteId = qp.get('note'); state.sub = 'doc'; }
  if (qp.get('mode')) state.docMode = qp.get('mode');
  if (qp.get('tag')) state.tagFilter = qp.get('tag');
  if (qp.get('theme')) { state.themePref = qp.get('theme'); applyTheme(); }
  return true;
}

function init() {
  applyTheme();
  $('#rail-toggle').innerHTML = icon('sidebar', 18);
  bind();
  if (applyHashRoute()) {
    $('#app').dataset.screen = 'app';
    render();
  } else {
    $('#app').dataset.screen = 'welcome';
    state.screen = 'welcome';
    render();
  }
}

window.addEventListener('DOMContentLoaded', () => {
  try { init(); }
  catch (err) {
    console.error(err);
    const s = document.getElementById('stage');
    if (s) s.innerHTML = `<div class="empty"><h3>原型初始化失败</h3><p>${esc(err.message)}</p></div>`;
  }
});
