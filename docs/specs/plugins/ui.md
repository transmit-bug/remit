# 插件 UI 贡献 · commands / menus / pages / settings

插件只能**声明**界面，画一切的是宿主。这份稿子冻结 `contributes` 的形状、上下文表达式、
菜单槽位、页面节点树与失败隔离；实现在 #146，契约变更走 `apiVersion`。

前置：manifest 与钩子词汇表（[`system.md`](system.md) §4/#139）、卡片类型与
字段（ADR 0008/#140）。本稿不碰卡片模型——复习卡信息仍只读 `card.type` / `fields` /
`ordinal` / `producer`，没有 `cards.describe`，也没有 `cards.render`。

> **现状（2026-11-08，#146 与 #145 落地）**：已实现。`contributes` 随 manifest 读入，
> `internal/ext/contrib` 解析 / 校验 / 索引 / 求值 `when`；命令进命令面板与宿主菜单槽位，
> 页面由 `internal/ui/screens/pluginpage` 用宿主组件绘制，设置项落在设置页的宿主表单里。
> `remit ext check <dir>` 跑同一份解析；`remit ext test <dir>` 用插件自带的 `fixtures/`
> 逐字比对钩子答案；`remit.d.ts` 由宿主的表生成并随 `apiVersion` 版本化。库的每次写入
> 都会让开着的页面重取一次数据（§9）。§4 的每个槽位都有接收方（W9），除 `status.item`（预留）；
> 仍未做的：`status.item`（预留，随 `apiVersion` 升级）、`activation`（manifest 的触发
> 时机仍是加载即用）。

## 1. 原则

| 原则 | 落法 |
|---|---|
| **静态声明，启用时校验** | `contributes` 与 `implements` 同级：宿主不启动插件就能读取、校验、索引；运行时只有命令回调与页面数据命令回到插件 |
| **宿主持有组件与布局** | 页面是语义块 + 布局节点的树；插件给结构，宿主给 token、组件、焦点、i18n 与窄窗口重排 |
| **永不在帧里调用插件** | `View` 只读缓存；页面数据在进入 / 刷新 / 事件时取，命令在运行时 goroutine 执行后回填 |
| **逐贡献失败隔离** | 计数按 `(plugin, contribution id)`；坏一条只隐藏那一条，不连带停用插件或钩子 |
| **未知只降级，不崩** | 未知节点/叶就地占位；未知槽位是校验错误（宿主不知道放哪） |

`docs/specs/ui-composition.md` 的组件优先规则仍然有效：插件不能自带 DOM、CSS、画布、
图标包或主题；也因此键盘、焦点、读屏角色与 720×520 退化由宿主一次做对。

## 2. Manifest
```json
{
  "id": "acme.leeches",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "capabilities": ["notes:read", "review:read", "store:own"],
  "implements": { "on_event": 1 },
  "contributes": {
    "commands": [
      {
        "id": "acme.leeches.show",
        "title": "%command.show%",
        "category": "%category.review%",
        "icon": "bug",
        "keybinding": "mod+shift+l",
        "when": "library.open && (mode == review || mode == browse)",
        "action": "command:acme.showLeeches"
      }
    ],
    "menus": {
      "review.card": [
        { "command": "acme.leeches.suspendTag", "group": "plugin", "order": 100, "when": "card.present" }
      ]
    },
    "pages": [
      {
        "id": "acme.leeches",
        "title": "%page.leeches%",
        "icon": "bug",
        "nav": "plugins",
        "when": "library.open",
        "data": "command:acme.leechesData"
      }
    ],
    "settings": [
      { "key": "threshold", "type": "number", "title": "%setting.threshold%", "default": 8 }
    ],
    "importers": [
      {
        "format": "acme.obsidian",
        "extensions": ["md", "canvas"],
        "label": "%importer.label%"
      }
    ],
    "locales": {
      "zh-Hans": { "command.show": "水蛭报告", "page.leeches": "水蛭", "setting.threshold": "水蛭阈值", "importer.label": "Obsidian 导出" }
    }
  }
}
```

- 命令 `id` 全局唯一，`remit.` 前缀保留给宿主。
- `title` / `category` 是 `%message.key%` 而非字面量；缺 key 时显示 key 本身。
- `icon` 只能取宿主图标集的名字；未知名字 = 该条校验失败并被隐藏。
- `keybinding` 可选；冲突由宿主拒绝，并在校验结果里指出双方。
- `action` 两种：宿主动作白名单（`remit.command.openSettings` 这类，宿主代码）或
  `command:<id>` 回调。插件不能注册新的宿主动作。
- `nav` 目前只有 `"plugins"`（进入插件页面分组）与 `"hidden"`；具体排序与分组归 #129，
  本契约只保证存在一个插件页面分组。
- `importers` 声明该插件能导入的数据格式（#153）：`format` 是稳定标识（跨版本不变，
  与命令 id 同样的命名纪律，`remit.` 前缀保留给宿主）；`extensions` 是文件扩展名
  （无点、小写），宿主的文件对话框与拖拽按它认文件；`label` 走 `%key%`，是导入入口
  里这一格的名字。声明 `importers` 的插件必须同时 `implements` `importFile` 钩子；
  反之 implements 了钩子却不声明的，宿主照常加载但导入入口不出现。

## 3. 上下文与 `when`

宿主冻结的初始键（新增 = `apiVersion` 兼容变更，删除/改义 = 破坏性）：

| Key | Values |
|---|---|
| `mode` | `notes` / `review` / `browse` / `stats` / `settings`（插件页面及其它状态为空串） |
| `area` | `today` / `write` / `read` / `recall` / `stats` / `settings` / `plugin`（窗口的哪一部分；`notes` 一屏含三个 area，所以 `mode` 说不了） |
| `sub` | 子页的稳定 id（`queue`、`cards`…），单屏 area 为空串 |
| `library.open` | bool |
| `note.open` | bool |
| `note.selection` | bool |
| `card.present` | bool |
| `card.status` | `new` / `learning` / `review` / `suspended` / `reading`（卡片自身的状态；`lost` 是引用状态） |
| `browse.selection` | bool |
| `scope.active` | bool |
| `page.id` | string |
| `plugin.id` | string |

表达式只允许 `!`、`&&`、`||`、`==`、`!=`、括号与字符串/数字字面量——没有函数调用，没有属性
路径。未知键 = `false` + 一条校验警告；解析失败 = 该贡献隐藏。

## 4. 菜单槽位

槽位由宿主拥有，插件不能新建：

`toolbar.overflow` · `note.overflow` · `note.block` · `editor.slash` · `review.card` ·
`browse.row` · `sidebar.note` · `page.header` · `today.widget` · `stats.block`

`status.item`（状态行）**预留**：声明它通过校验、槽项也保留，但本版宿主不画它，所以它出现在
插件设置页的「问题」里，而不是界面上；接收方随一次 `apiVersion` 升级到来。

每项是 `{command, group, order, when}`。`group` 取宿主组（`primary` / `secondary`）或
`plugin`；同组按 `order` 升序。位置、分隔线与溢出行为由 #129 定，契约只保证槽位存在。

每个槽位的接收方（`ui-redesign-v2.md` §6.3 是同一张表的界面口径）；接收方测试在
`internal/ui/receivers_test.go`（`stats.block` 的那条在 `internal/ui/stats_test.go`）：

| 槽位 | 宿主落点 | 接收方 |
|---|---|---|
| `toolbar.overflow`、`note.overflow` | 窗口的 `⋯` 菜单，两条槽位在这一处合并，同一命令只出现一次 | 有（`TestOverflowMenuDrawsTheToolbarAndNoteSlots`） |
| `note.block` | 段落菜单的底部，以及块检查器的「插件」段 | 有（`TestBlockMenuDrawsTheNoteBlockSlot`） |
| `editor.slash` | `/` 插入菜单的底部，随查询一起过滤 | 有（`TestSlashMenuDrawsTheEditorSlashSlot`） |
| `review.card` | 复习会话的动作行 | 有（`TestReviewFooterDrawsTheReviewCardSlot`） |
| `browse.row` | 卡片表工具行的「插件动作」，`browse.selection` 决定它开不开 | 有（`TestCardTableDrawsTheBrowseRowSlot`） |
| `sidebar.note` | 笔记行菜单的底部 | 有（`TestNoteRowMenuDrawsTheSidebarNoteSlot`） |
| `page.header` | 插件页页头 | 有（`TestPageHeaderDrawsThePageHeaderSlot`） |
| `today.widget` | 今日页每个插件的一段 | 有（`TestTodayDrawsTheTodayWidgetSlot`） |
| `stats.block` | 统计·总览每个插件的一段 | 有（`TestStatsBlocksComeFromPlugins`） |
| `status.item` | **不画**：条目保留，声明进设置页「问题」 | **预留**：无接收方，随一次 `apiVersion` 升级到来 |

槽项只读宿主的缓存：`when` 在库打开时编译一次，接收方每帧求值、从不调用插件（§9）。

## 5. 页面：语义块 + 布局节点

页面 `data` 命令返回一棵节点树：

```json
{
  "kind": "page",
  "children": [
    {
      "kind": "section",
      "title": "%section.overview%",
      "children": [
        { "kind": "stack", "direction": "row", "gap": "m", "children": [
          { "kind": "stat", "label": "%stat.leeches%", "value": "12" },
          { "kind": "stat", "label": "%stat.suspended%", "value": "3" }
        ]},
        { "kind": "grid", "columns": 2, "children": [
          { "kind": "table", "columns": ["%col.card%", "%col.lapses%"], "rows": [["…", "9"]] },
          { "kind": "markdown", "text": "…" }
        ]}
      ]
    }
  ]
}
```

**布局节点**（只表达结构）

| 节点 | 字段 | 语义 |
|---|---|---|
| `section` | `title?`, `description?`, `children` | 一组内容，带标题 |
| `stack` | `direction: row\|column`, `gap`, `children` | 顺序排列 |
| `grid` | `columns: 1..4`（提示）, `children` | 网格；窄窗口收成一列 |
| `split` | `ratio: 1\|2`, 恰好两个 `children` | 两栏；窄窗口叠成一栏 |
| `tabs` | `tabs: [{label, children}]` | 标签页 |
| `card` | `title?`, `children` | 浮起的一组内容 |

**语义叶**：`markdown` · `text` · `stat` · `table` · `list` · `tree` · `form` · `chart` ·
`callout` · `actions`。

- 只允许 `gap: xs|s|m|l` 与 `tone: neutral|info|warn|danger|success` 两个 token。
- **禁止** `width` / `height` / `color` / `font` / `margin` / `padding` / `position`、原始
  HTML/CSS、脚本、自定义绘制。
- 焦点顺序 = 文档顺序；`section` / `tabs` 必须有标签；不允许键盘陷阱。
- 上限：深度 ≤ 8，节点 ≤ 500，`table.rows` ≤ 500，单页 JSON ≤ 256 KiB。超限 = 页面错误
  态，不是空白。
- 窄窗口：`grid` 收一列，`split` 叠一栏，`stack row` 放不下时换行，`tabs` 保留；
  **720×520 下不允许横向滚动**。
- 未知节点/叶：就地显示含 `kind` 的错误占位，其余照常渲染；同一 `(plugin, kind)` 只记
  一条日志。

## 6. 文案与设置

- `contributes.locales` 内联在 manifest，或插件目录的 `locales/<lang>.json`；宿主默认 `en`，
  缺 key 显示 key。
- 设置项沿用 `contributes.settings`，由宿主表单渲染，值写 `store:own`（插件 id 命名空间），
  不经过插件代码。类型管理界面是宿主 UI（#144），不是插件页面。
- 内置类型/字段名是内容，不进插件文案目录；插件页面里的用户数据也不翻译。

## 7. 卡片信息：类型/字段，不是插件钩子

复习卡的信息面板只读 `card.type`、`card.fields`、`card.ordinal`、`card.producer`。
没有 `cards.describe`、没有 `cards.render`；插件想展示更多，贡献页面或命令，不注入卡面。
这条边界防止插件契约长出第二套卡片模型。

## 8. 失败隔离

| 情况 | 宿主行为 |
|---|---|
| manifest 里某条贡献非法 | 只丢弃该条；插件仍可启用；错误进插件设置页与日志 |
| 页面数据抛错/超时（1s） | 页面错误态 + 重试；连续 3 次隐藏该页面（导航项消失） |
| 命令回调抛错/超时 | toast + 日志；连续 3 次隐藏该命令；其他命令不受影响 |
| 未知页面节点/叶 | 该节点错误占位，其余渲染 |
| 插件停用/卸载 | 入口消失；`store:own` 数据保留 |
| 钩子（derive/schedule）失败 | 由 #143 的计数处理；**不**连带停用 UI 贡献，反之亦然 |
| 页面/命令数据超限 | 错误态；不进入帧 |

计数按 `(plugin, contribution id)`，三次规则只隐藏那一条；所有错误写进同一份插件日志，
设置页显示失败计数与「重新启用」。

## 9. 永不在帧里调用插件

- `contributes` 在启用时解析、校验、缓存。
- 页面 `data` 是一个命令：进入页面、显式刷新或 `onEvent` 触发时调用；结果按
  `(plugin, page, context-hash)` 缓存。
- `View` 只读缓存树；无缓存 = 骨架 + 触发一次加载，绝不同步等待。
- 命令回调在运行时 goroutine 执行，结果（`navigate` / `refresh` / `toast` / `none`）回到
  UI 线程应用；命令不能直接改 `App` 状态。
- 一个慢命令只阻塞它自己的结果，不阻塞输入、滚动或复习。

## 10. 版本与工具

- `apiVersion` 沿用最低版本语义（`system.md` §9）；`contributes` schema 是其一部分。
- 新增叶/节点/上下文键 = 兼容；删除/改义 = 破坏性 +1。未知节点/叶降级为占位，未知槽位
  是校验错误。
- `remit.d.ts` 是契约产物：从宿主的钩子 / 能力 / 贡献 / 节点 / 上下文键表生成，随
  `apiVersion` 版本化；`go test ./internal/ext/ts` 在代码与文件不一致时变红。
- `remit ext check <dir>` / `remit ext test <dir>` 校验：manifest 与 `contributes` 能解析、
  命令 id 唯一、菜单槽位已知、命令存在、`when` 可解析、图标存在、页面节点已知、上限、
  locale 键齐全；`ext test` 再把每个声明的钩子按 fixture 跑一遍。命令在仓库里是
  `go run ./cmd/remit ext …`（不是 `go tool remit ext …`）。
- 示例插件 `examples/plugins/leeches` 同时是测试 fixture：一个页面（stat + table +
  callout）、两个命令、三个菜单项、两个设置项、en/zh-Hans 文案，`fixtures/` 的 fixture
  逐字比对（`internal/ext/exttest`）。

## 11. 怎么测

好测试只测外部行为：磁盘明文、headless View 的文本/盒子/焦点、命令的副作用、fixtures 的
逐字比对；不 mock 宿主，不做像素基线。

1. **新的纯解析缝** `internal/ext/contrib`：`Parse(manifest) → Contributions + []error`、
   `When(expr, context) → bool`、`ValidatePage(node)`。不依赖窗口、不启动插件。
2. **既有 manifest 校验缝**：未知钩子/槽位/图标/`when` 错误逐条列出。
3. **既有 `ui.NewTester` 缝**：真实 `App` + 示例插件，驱动命令面板、菜单、页面与失败路径；
   默认与 720×520 各一遍。
4. **既有插件 fixtures 缝**：示例插件的页面数据输出逐字比对。
5. **既有 capability 缝**：页面数据命令走 `notes:read`，未授权时错误态。

要测什么：`when` 表驱动（含未知键为 false）；命令出现在面板与正确槽位、宿主动作生效、
`navigate/refresh/toast` 生效、抛错 toast + 计数、三次后隐藏；页面在 1280 与 720 的收敛
（grid 一列、split 一叠、无横向滚动）、焦点顺序、tabs 标签、未知节点占位；页面数据抛错 →
错误态 + 重试 → 三次后导航消失；插件卸载入口消失而数据保留；假命令阻塞时 `View` 仍返回
骨架且帧内没有插件调用；zh-Hans 命中与缺 key 回退；超深/超节点/超大 → 错误态。

## 12. 分期

| 期 | 内容 | 依赖 |
|---|---|---|
| A | `contributes` 进 manifest + `internal/ext/contrib` 解析/校验/`when` | #139 |
| B | 命令：面板项、菜单槽位、宿主动作白名单、回调结果与三次隐藏 | A，#129 的槽位位置 |
| C | 页面：节点树渲染、数据命令、缓存、错误态与上限、窄窗口收敛 | A，#142 的 `notes:read` |
| D | 设置表单、locales 落地、示例插件、`ext check` / `remit.d.ts` 接线 | C，#144，#145 |

每期一个 PR；A 与 B 可先落地而不阻塞 C。

## 13. 不做

- 自定义绘制、画布、webview、CSS、插件自带图标/主题。
- 插件新建菜单槽位或顶栏段。
- 卡片面渲染与 `cards.describe` / `cards.render`（类型/模板归 #138/ADR 0008）。
- 导航的最终排序、分组与降级（#129）。
- 插件市场、签名、`net:fetch`。
- 每帧推送数据的页面；页面数据只在导航/刷新/事件时取。
- 用户内容（类型名、字段名、笔记文本）的翻译。
