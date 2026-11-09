// Package contrib reads what a plugin declares for the window: its commands,
// its menu items, its pages, its settings and its own strings, all in the
// manifest, so that the host can validate and index them without starting the
// plugin (docs/specs/plugins/ui.md).
//
// It is deliberately pure: no window, no store, no runner. One contribution
// stands or falls alone — Parse answers the ones it could keep and a problem for
// each one it could not — and nothing here ever runs a plugin's code. What the
// window draws is the host's, from the node tree ParsePage reads at run time.
package contrib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/transmit-bug/remit/internal/ext"
)

// The menu slots the host owns. A manifest may put an item in one of them and
// nowhere else: a slot is a place in the window, and a plugin cannot invent one.
const (
	SlotToolbarOverflow = "toolbar.overflow"
	SlotNoteOverflow    = "note.overflow"
	SlotNoteBlock       = "note.block"
	SlotEditorSlash     = "editor.slash"
	SlotReviewCard      = "review.card"
	SlotBrowseRow       = "browse.row"
	SlotSidebarNote     = "sidebar.note"
	SlotPageHeader      = "page.header"
	SlotTodayWidget     = "today.widget"
	SlotStatsBlock      = "stats.block"
	// SlotStatusItem is the status line's slot. It is reserved: the contract
	// names it and a manifest declaring it validates, but this host draws no
	// status item until an apiVersion says where (ui.md §4).
	SlotStatusItem = "status.item"
)

// Slots are the menu slots the host draws: its own controls and the plugin
// panels the window has a receiver for today.
var Slots = []string{
	SlotToolbarOverflow, SlotNoteOverflow, SlotNoteBlock, SlotEditorSlash,
	SlotReviewCard, SlotBrowseRow, SlotSidebarNote, SlotPageHeader,
	SlotTodayWidget, SlotStatsBlock,
}

// ReservedSlots are the slots the contract names for a later apiVersion. A
// manifest's items there validate and are kept — a host that draws them reads
// the same manifest — but this one draws nothing in them, so the declaration is
// a problem the settings page lists rather than a contribution anyone sees.
var ReservedSlots = []string{SlotStatusItem}

// SlotReserved reports whether slot is one the contract reserves: a place a
// manifest may name, and this host shows nowhere.
func SlotReserved(slot string) bool {
	for _, reserved := range ReservedSlots {
		if reserved == slot {
			return true
		}
	}
	return false
}

// Nav is where a contributed page appears in the window.
type Nav string

const (
	// NavPlugins puts the page in the host's plugin-pages group.
	NavPlugins Nav = "plugins"
	// NavHidden keeps the page out of navigation: a command opens it.
	NavHidden Nav = "hidden"
)

// Navs are every navigation choice a page has.
var Navs = []Nav{NavPlugins, NavHidden}

// SettingType is what a contributed setting holds.
type SettingType string

const (
	SettingBool   SettingType = "bool"
	SettingNumber SettingType = "number"
	SettingText   SettingType = "text"
)

// SettingTypes are every value type a contributed setting may hold.
var SettingTypes = []SettingType{SettingBool, SettingNumber, SettingText}

// HostActions are the app's own actions a contributed command may run instead
// of answering a callback. The list is the host's: a name outside it is a
// validation error, so a plugin points at an action that exists and never
// invents one (docs/specs/plugins/ui.md §2).
var HostActions = map[string]bool{
	"remit.command.openSettings": true,
	"remit.command.reviewCards":  true,
	"remit.command.editNotes":    true,
	"remit.command.browse":       true,
	"remit.command.stats":        true,
	"remit.command.newNote":      true,
	"remit.command.makeCard":     true,
}

// Options is what the parser needs to know about the app it lands in.
type Options struct {
	// Icon reports whether the host's icon set carries a name. nil accepts
	// every name, which only a test does — the app and the checker pass the
	// window's set.
	Icon func(name string) bool
}

// Message is a plugin string. "%key%" names an entry of the plugin's own
// catalog; anything else is a literal.
type Message string

// Text answers the message in locale through the plugin's catalog. A key the
// plugin did not translate shows the key itself, so a gap is visible rather
// than blank.
func (m Message) Text(catalog Catalog, locale string) string {
	raw := string(m)
	key, ok := strings.CutPrefix(raw, "%")
	if !ok {
		return raw
	}
	key, ok = strings.CutSuffix(key, "%")
	if !ok || key == "" {
		return raw
	}
	return catalog.Text(locale, key)
}

// Catalog is a plugin's own strings: language, then key. It is the plugin's
// file, not the app's catalog — a plugin's interface is its own to translate.
type Catalog map[string]map[string]string

// Text answers key in the closest language the catalog holds: the exact locale,
// its base language, English, then the key.
func (c Catalog) Text(locale, key string) string {
	for _, language := range languages(locale) {
		if text, ok := c[language][key]; ok {
			return text
		}
	}
	return key
}

// Has reports whether any language translates key.
func (c Catalog) Has(key string) bool {
	for _, messages := range c {
		if _, ok := messages[key]; ok {
			return true
		}
	}
	return false
}

// languages is the fallback chain a message is looked up in, closest first.
func languages(locale string) []string {
	out := []string{}
	if locale != "" {
		out = append(out, locale)
		if base, _, ok := strings.Cut(locale, "-"); ok && base != locale {
			out = append(out, base)
		}
	}
	return append(out, "en")
}

// Contributions is one plugin's declared UI, indexed by what the host asks for.
type Contributions struct {
	// Plugin is the manifest id: the plugin's namespace in the store and in the
	// ids of what it declares. Name is the manifest name, what a runner is
	// asked for.
	Plugin string
	Name   string

	Catalog  Catalog
	Commands []Command
	// Menus holds each declared slot's items, sorted by group and order.
	Menus    map[string][]MenuItem
	Pages    []Page
	Settings []Setting
	// Importers are the data formats the plugin can bring in, sorted by
	// format; the import entries list them.
	Importers []Importer
}

// Text resolves a message the plugin wrote, in locale.
func (c Contributions) Text(locale, raw string) string {
	return Message(raw).Text(c.Catalog, locale)
}

// Command answers one of the plugin's commands by id.
func (c Contributions) Command(id string) (Command, bool) {
	for _, command := range c.Commands {
		if command.ID == id {
			return command, true
		}
	}
	return Command{}, false
}

// Page answers one of its pages by id.
func (c Contributions) Page(id string) (Page, bool) {
	for _, page := range c.Pages {
		if page.ID == id {
			return page, true
		}
	}
	return Page{}, false
}

// Command is one command a plugin contributes: what the palette lists, what a
// menu slot points at, and what runs when it is chosen.
type Command struct {
	ID       string
	Title    Message
	Category Message
	Icon     string
	// Keybinding is the accelerator the plugin asks for, "" when it asks for
	// none. The host indexes it and refuses two plugins claiming one.
	Keybinding string
	// When decides whether the command shows; the zero value shows it always.
	When Expr
	// Action is what running it does.
	Action Action
}

// Action is a command's act: one of the host's own actions, or a callback the
// plugin answers in its runtime.
type Action struct {
	// Host is a host action's name, "" when the command answers a callback.
	Host string
	// Name is the callback's name: what the plugin's command hook is asked for.
	Name string
}

// MenuItem is one command in one of the host's slots.
type MenuItem struct {
	Command string
	// Group orders the item among the slot's: the host's primary and secondary
	// groups, or the plugin group that follows them.
	Group string
	Order int
	When  Expr
}

// Page is one page a plugin contributes: a navigation entry, and the command
// that answers its tree.
type Page struct {
	ID    string
	Title Message
	Icon  string
	Nav   Nav
	When  Expr
	// Data is the callback that answers the page's node tree.
	Data string
}

// Importer is one data format a plugin can bring into the library: what the
// import entries show, and which files the host hands it. Declaring one is
// not an authorization — the host still validates every suggestion the
// plugin's importFile hook answers before anything is written (#153).
type Importer struct {
	// Format is the stable identity of the format ("acme.obsidian"); the
	// remit. prefix is the host's.
	Format string
	// Extensions are the file extensions that identify the format, without
	// the dot, lower-case.
	Extensions []string
	// Label is the name the import entry shows.
	Label Message
}

// Setting is one value a plugin asks the host to edit for it. The host keeps it
// in the plugin's own store and never calls the plugin to write it.
type Setting struct {
	Key   string
	Type  SettingType
	Title Message
	// Default is the value before the user changes it: a bool, a float64 or a
	// string, as Type says.
	Default any
}

// Parse indexes one plugin's contributions. It starts nothing: the manifest is
// the whole input. A contribution that does not hold up is dropped on its own,
// with a problem naming it, so one bad entry never costs the plugin its others;
// a warning about a newer host's context key is a problem that keeps the
// contribution.
func Parse(e ext.Extension, opts Options) (Contributions, []error) {
	out := Contributions{
		Plugin:  e.Manifest.ID,
		Name:    e.Manifest.Name,
		Catalog: Catalog{},
		Menus:   map[string][]MenuItem{},
	}
	raw := bytes.TrimSpace(e.Manifest.Contributes)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return out, nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return out, []error{fmt.Errorf("contributes: %w", err)}
	}
	p := &parser{catalog: out.Catalog, opts: opts, used: map[string]bool{}}

	// Locales first: every other contribution is named by one.
	if body, ok := top["locales"]; ok {
		var inline Catalog
		if err := json.Unmarshal(body, &inline); err != nil {
			p.problem("contributes.locales: %v", err)
		} else {
			mergeCatalog(out.Catalog, inline)
		}
	}
	if e.Dir != "" {
		if err := readCatalogFiles(filepath.Join(e.Dir, "locales"), out.Catalog); err != nil {
			p.problem("%v", err)
		}
	}

	// Commands come first: a menu item names one.
	if body, ok := top["commands"]; ok {
		p.declared = p.commands(body)
		out.Commands = p.declared
	}
	for _, kind := range sortedRawKeys(top) {
		switch kind {
		case "commands", "locales":
			// read above
		case "menus":
			out.Menus = p.menus(top[kind])
		case "pages":
			out.Pages = p.pages(top[kind])
		case "settings":
			out.Settings = p.settings(top[kind])
		case "importers":
			out.Importers = p.importers(top[kind])
		default:
			p.problem("contributes.%s: no such contribution kind; the host hides it", kind)
		}
	}
	for _, key := range sortedSet(p.used) {
		if !out.Catalog.Has(key) {
			p.problem("contributes.locales: no language translates %%%s%%", key)
		}
	}
	sort.Slice(out.Commands, func(i, j int) bool { return out.Commands[i].ID < out.Commands[j].ID })
	sort.Slice(out.Pages, func(i, j int) bool { return out.Pages[i].ID < out.Pages[j].ID })
	sort.Slice(out.Settings, func(i, j int) bool { return out.Settings[i].Key < out.Settings[j].Key })
	sort.Slice(out.Importers, func(i, j int) bool { return out.Importers[i].Format < out.Importers[j].Format })
	return out, p.problems
}

// Check reads one plugin directory and reports what its manifest and its
// contributions declare that this host cannot use: the answer `ext check`
// prints, and the same parse the window does before it shows any of it.
func Check(dir string, opts Options) []error {
	e, err := ext.Load(dir)
	if err != nil {
		return []error{err}
	}
	_, problems := Parse(e, opts)
	return problems
}

// parser collects what one manifest could not offer, and the keys its messages
// named.
type parser struct {
	problems []error
	catalog  Catalog
	opts     Options
	used     map[string]bool
	// declared are the plugin's own commands, which a menu item must name.
	declared []Command
}

func (p *parser) problem(format string, args ...any) {
	p.problems = append(p.problems, fmt.Errorf(format, args...))
}

// commandWire, menuItemWire and the rest are the manifest's shapes: one struct
// each, decoded strictly, so a field the contract does not have is a problem
// naming the contribution rather than a silently ignored one.
type commandWire struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Category   string `json:"category"`
	Icon       string `json:"icon"`
	Keybinding string `json:"keybinding"`
	When       string `json:"when"`
	Action     string `json:"action"`
}

type menuItemWire struct {
	Command string `json:"command"`
	Group   string `json:"group"`
	Order   int    `json:"order"`
	When    string `json:"when"`
}

type pageWire struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Icon  string `json:"icon"`
	Nav   string `json:"nav"`
	When  string `json:"when"`
	Data  string `json:"data"`
}

type settingWire struct {
	Key     string          `json:"key"`
	Type    string          `json:"type"`
	Title   string          `json:"title"`
	Default json.RawMessage `json:"default"`
}

func (p *parser) commands(raw json.RawMessage) []Command {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		p.problem("contributes.commands: %v", err)
		return nil
	}
	commands := make([]Command, 0, len(items))
	ids := map[string]bool{}
	for i, item := range items {
		where := fmt.Sprintf("contributes.commands[%d]", i)
		var w commandWire
		if err := decodeStrict(item, &w); err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		command, err := p.command(w, where)
		if err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		if ids[command.ID] {
			p.problem("%s: command %q is declared twice", where, command.ID)
			continue
		}
		ids[command.ID] = true
		commands = append(commands, command)
	}
	return commands
}

// command checks one command, or says why it does not hold up.
func (p *parser) command(w commandWire, where string) (Command, error) {
	if !validID(w.ID) {
		return Command{}, fmt.Errorf("id %q is not a name", w.ID)
	}
	if strings.HasPrefix(w.ID, "remit.") {
		return Command{}, fmt.Errorf("id %q: the remit. prefix is the host's", w.ID)
	}
	if err := p.useMessage(w.Title); err != nil {
		return Command{}, fmt.Errorf("title: %w", err)
	}
	if w.Category != "" {
		if err := p.useMessage(w.Category); err != nil {
			return Command{}, fmt.Errorf("category: %w", err)
		}
	}
	if w.Icon != "" && p.opts.Icon != nil && !p.opts.Icon(w.Icon) {
		return Command{}, fmt.Errorf("icon %q: the host has no such icon", w.Icon)
	}
	when, err := p.when(w.When, where)
	if err != nil {
		return Command{}, err
	}
	action, err := parseAction(w.Action)
	if err != nil {
		return Command{}, err
	}
	if w.Keybinding != "" {
		if err := validKeybinding(w.Keybinding); err != nil {
			return Command{}, fmt.Errorf("keybinding %q: %w", w.Keybinding, err)
		}
	}
	return Command{
		ID:         w.ID,
		Title:      Message(w.Title),
		Category:   Message(w.Category),
		Icon:       w.Icon,
		Keybinding: w.Keybinding,
		When:       when,
		Action:     action,
	}, nil
}

// when compiles a contribution's expression, reporting the keys this host does
// not know as problems that keep the contribution.
func (p *parser) when(src, where string) (Expr, error) {
	expr, warnings, err := Compile(src)
	for _, warning := range warnings {
		p.problem("%s: %s", where, warning)
	}
	return expr, err
}

// parseAction reads a command's act: command:<name> answers a callback, and a
// remit.command.* name is one of the host's own, from its allowlist.
func parseAction(raw string) (Action, error) {
	if name, ok := strings.CutPrefix(raw, "command:"); ok {
		if strings.TrimSpace(name) == "" {
			return Action{}, errors.New("action: command: names nothing")
		}
		return Action{Name: strings.TrimSpace(name)}, nil
	}
	if raw == "" {
		return Action{}, errors.New("action: a command needs one, command:<name> or a remit.command.* action")
	}
	if HostActions[raw] {
		return Action{Host: raw}, nil
	}
	return Action{}, fmt.Errorf("action %q: the host owns its actions, and has none by that name", raw)
}

func (p *parser) menus(raw json.RawMessage) map[string][]MenuItem {
	menus := map[string][]MenuItem{}
	var slots map[string][]json.RawMessage
	if err := json.Unmarshal(raw, &slots); err != nil {
		p.problem("contributes.menus: %v", err)
		return menus
	}
	for _, slot := range sortedRawKeys(slots) {
		switch {
		case SlotReserved(slot):
			// Kept, not dropped: the items are what a later apiVersion draws,
			// and this host validates them now so that day surfaces nothing new.
			p.problem("contributes.menus.%s: the slot is reserved for a later apiVersion; nothing shows it here", slot)
		case !knownSlot(slot):
			p.problem("contributes.menus.%s: the host has no such slot", slot)
			continue
		}
		items := make([]MenuItem, 0, len(slots[slot]))
		for i, item := range slots[slot] {
			where := fmt.Sprintf("contributes.menus.%s[%d]", slot, i)
			var w menuItemWire
			if err := decodeStrict(item, &w); err != nil {
				p.problem("%s: %v", where, err)
				continue
			}
			if _, ok := commandsOf(p, w.Command); !ok {
				p.problem("%s: no command %q is declared", where, w.Command)
				continue
			}
			if w.Group != "" && w.Group != "primary" && w.Group != "secondary" && w.Group != "plugin" {
				p.problem("%s: group %q, the host has primary, secondary and plugin", where, w.Group)
				continue
			}
			when, err := p.when(w.When, where)
			if err != nil {
				p.problem("%s: %v", where, err)
				continue
			}
			items = append(items, MenuItem{Command: w.Command, Group: w.Group, Order: w.Order, When: when})
		}
		sort.SliceStable(items, func(i, j int) bool {
			if groupOrder(items[i].Group) != groupOrder(items[j].Group) {
				return groupOrder(items[i].Group) < groupOrder(items[j].Group)
			}
			if items[i].Order != items[j].Order {
				return items[i].Order < items[j].Order
			}
			return items[i].Command < items[j].Command
		})
		menus[slot] = items
	}
	return menus
}

// commandsOf answers whether this plugin declared the command.
func commandsOf(p *parser, id string) (Command, bool) {
	for _, command := range p.declared {
		if command.ID == id {
			return command, true
		}
	}
	return Command{}, false
}

func (p *parser) pages(raw json.RawMessage) []Page {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		p.problem("contributes.pages: %v", err)
		return nil
	}
	pages := make([]Page, 0, len(items))
	ids := map[string]bool{}
	for i, item := range items {
		where := fmt.Sprintf("contributes.pages[%d]", i)
		var w pageWire
		if err := decodeStrict(item, &w); err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		page, err := p.page(w, where)
		if err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		if ids[page.ID] {
			p.problem("%s: page %q is declared twice", where, page.ID)
			continue
		}
		ids[page.ID] = true
		pages = append(pages, page)
	}
	return pages
}

func (p *parser) page(w pageWire, where string) (Page, error) {
	if !validID(w.ID) {
		return Page{}, fmt.Errorf("id %q is not a name", w.ID)
	}
	if err := p.useMessage(w.Title); err != nil {
		return Page{}, fmt.Errorf("title: %w", err)
	}
	if w.Icon != "" && p.opts.Icon != nil && !p.opts.Icon(w.Icon) {
		return Page{}, fmt.Errorf("icon %q: the host has no such icon", w.Icon)
	}
	nav := Nav(w.Nav)
	switch {
	case nav == "":
		nav = NavPlugins
	case !knownNav(nav):
		return Page{}, fmt.Errorf("nav %q, the host has plugins and hidden", w.Nav)
	}
	when, err := p.when(w.When, where)
	if err != nil {
		return Page{}, err
	}
	data := callbackName(w.Data)
	if data == "" {
		return Page{}, errors.New("data: a page needs the command that answers its tree")
	}
	return Page{ID: w.ID, Title: Message(w.Title), Icon: w.Icon, Nav: nav, When: when, Data: data}, nil
}

func (p *parser) settings(raw json.RawMessage) []Setting {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		p.problem("contributes.settings: %v", err)
		return nil
	}
	settings := make([]Setting, 0, len(items))
	keys := map[string]bool{}
	for i, item := range items {
		where := fmt.Sprintf("contributes.settings[%d]", i)
		var w settingWire
		if err := decodeStrict(item, &w); err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		setting, err := p.setting(w)
		if err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		if keys[setting.Key] {
			p.problem("%s: setting %q is declared twice", where, setting.Key)
			continue
		}
		keys[setting.Key] = true
		settings = append(settings, setting)
	}
	return settings
}

func (p *parser) setting(w settingWire) (Setting, error) {
	if !validID(w.Key) {
		return Setting{}, fmt.Errorf("key %q is not a name", w.Key)
	}
	if err := p.useMessage(w.Title); err != nil {
		return Setting{}, fmt.Errorf("title: %w", err)
	}
	setting := Setting{Key: w.Key, Type: SettingType(w.Type), Title: Message(w.Title)}
	value, err := settingDefault(setting.Type, w.Default)
	if err != nil {
		return Setting{}, err
	}
	setting.Default = value
	return setting, nil
}

// importerWire is the manifest's shape of one importer: a format the host can
// name, extensions to recognize, and a label the import entry shows.
type importerWire struct {
	Format     string   `json:"format"`
	Extensions []string `json:"extensions"`
	Label      string   `json:"label"`
}

func (p *parser) importers(raw json.RawMessage) []Importer {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		p.problem("contributes.importers: %v", err)
		return nil
	}
	importers := make([]Importer, 0, len(items))
	formats := map[string]bool{}
	for i, item := range items {
		where := fmt.Sprintf("contributes.importers[%d]", i)
		var w importerWire
		if err := decodeStrict(item, &w); err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		importer, err := p.importer(w)
		if err != nil {
			p.problem("%s: %v", where, err)
			continue
		}
		if formats[importer.Format] {
			p.problem("%s: format %q is declared twice", where, importer.Format)
			continue
		}
		formats[importer.Format] = true
		importers = append(importers, importer)
	}
	return importers
}

// importer checks one importer, or says why it does not hold up: a format
// name the host can carry, at least one extension, a label the catalog
// translates.
func (p *parser) importer(w importerWire) (Importer, error) {
	if !validID(w.Format) {
		return Importer{}, fmt.Errorf("format %q is not a name", w.Format)
	}
	if strings.HasPrefix(w.Format, "remit.") {
		return Importer{}, fmt.Errorf("format %q: the remit. prefix is the host's", w.Format)
	}
	if len(w.Extensions) == 0 {
		return Importer{}, errors.New("extensions: an importer names at least one file extension")
	}
	exts := make([]string, 0, len(w.Extensions))
	for _, ext := range w.Extensions {
		ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
		if ext == "" || strings.ContainsAny(ext, "./\\") {
			return Importer{}, fmt.Errorf("extension %q is not a file extension", ext)
		}
		exts = append(exts, ext)
	}
	if err := p.useMessage(w.Label); err != nil {
		return Importer{}, fmt.Errorf("label: %w", err)
	}
	return Importer{Format: w.Format, Extensions: exts, Label: Message(w.Label)}, nil
}

// settingDefault reads a setting's default as its type says: absent means the
// type's zero, and a value of another type is a validation error rather than a
// control that shows nothing.
func settingDefault(typ SettingType, raw json.RawMessage) (any, error) {
	absent := len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
	switch typ {
	case SettingBool:
		if absent {
			return false, nil
		}
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("default: %v, a bool setting takes true or false", err)
		}
		return v, nil
	case SettingNumber:
		if absent {
			return float64(0), nil
		}
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("default: %v, a number setting takes a number", err)
		}
		return v, nil
	case SettingText:
		if absent {
			return "", nil
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("default: %v, a text setting takes a string", err)
		}
		return v, nil
	case "":
		return nil, errors.New("type: a setting needs one: bool, number or text")
	}
	return nil, fmt.Errorf("type %q, the host has bool, number and text", typ)
}

// knownNav reports whether the window has a place for a page's nav choice.
func knownNav(n Nav) bool {
	for _, known := range Navs {
		if known == n {
			return true
		}
	}
	return false
}

// useMessage checks that raw is the %key% a message must be, and records the key
// so that a missing translation is one problem on the plugin, not one per use.
func (p *parser) useMessage(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("a title is a message, and empty names nothing")
	}
	key, ok := strings.CutPrefix(raw, "%")
	if !ok {
		return fmt.Errorf("%q is a literal; a plugin's text is %%key%% of its own catalog", raw)
	}
	key, ok = strings.CutSuffix(key, "%")
	if !ok || key == "" {
		return fmt.Errorf("%q is not a %%key%%", raw)
	}
	p.used[key] = true
	return nil
}

// callbackName reads the name a command: action or a page's data names, with
// or without the prefix.
func callbackName(raw string) string {
	name, ok := strings.CutPrefix(strings.TrimSpace(raw), "command:")
	if ok {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(raw)
}

// validID reports whether a contributed id is a name the host can route and log:
// letters, digits, dots, dashes and underscores.
func validID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !isIdentPart(c) {
			return false
		}
	}
	return isIdentStart(id[0]) || id[0] >= '0' && id[0] <= '9'
}

// validKeybinding checks the accelerator's shape, not its meaning: modifiers
// the host knows, each once, and one key. Whether the combination is taken is
// the index's question, across every plugin.
func validKeybinding(binding string) error {
	parts := strings.Split(binding, "+")
	if len(parts) == 0 || len(parts) > 5 {
		return errors.New("a keybinding is modifiers and one key, as mod+shift+l")
	}
	key := parts[len(parts)-1]
	seen := map[string]bool{}
	for _, modifier := range parts[:len(parts)-1] {
		switch modifier {
		case "mod", "shift", "alt", "ctrl":
			if seen[modifier] {
				return fmt.Errorf("%s is named twice", modifier)
			}
			seen[modifier] = true
		default:
			return fmt.Errorf("%q is not a modifier; the host has mod, shift, alt and ctrl", modifier)
		}
	}
	if len(key) == 1 && key[0] >= ' ' && key[0] < 0x7f {
		return nil
	}
	switch strings.ToLower(key) {
	case "space", "enter", "escape", "tab", "backspace", "delete", "home", "end",
		"pageup", "pagedown", "up", "down", "left", "right", "comma", "period", "slash", "minus", "plus", "equal":
		return nil
	}
	return fmt.Errorf("%q is not a key name the host knows", key)
}

func knownSlot(slot string) bool {
	for _, known := range Slots {
		if known == slot {
			return true
		}
	}
	return false
}

func groupOrder(group string) int {
	switch group {
	case "primary":
		return 0
	case "secondary":
		return 1
	}
	return 2
}

// decodeStrict reads one contribution, refusing a field the contract does not
// have: a command cannot name a widget, and a page node cannot name a width.
func decodeStrict(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// readCatalogFiles reads locales/<lang>.json in the plugin's directory, the
// inline catalog winning where both hold a key.
func readCatalogFiles(dir string, catalog Catalog) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("contributes.locales: %w", err)
	}
	for _, entry := range entries {
		language, ok := strings.CutSuffix(entry.Name(), ".json")
		if entry.IsDir() || !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("contributes.locales.%s: %w", language, err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			return fmt.Errorf("contributes.locales.%s: %w", language, err)
		}
		if catalog[language] == nil {
			catalog[language] = map[string]string{}
		}
		for key, text := range messages {
			if _, taken := catalog[language][key]; taken {
				continue
			}
			catalog[language][key] = text
		}
	}
	return nil
}

func mergeCatalog(into, from Catalog) {
	for language, messages := range from {
		if into[language] == nil {
			into[language] = map[string]string{}
		}
		for key, text := range messages {
			into[language][key] = text
		}
	}
}

func sortedRawKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSet(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
