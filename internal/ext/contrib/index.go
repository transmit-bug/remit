package contrib

import (
	"fmt"
	"sort"
)

// CommandRef is one contributed command and the plugin that declared it: what
// the palette lists and what a menu item points at.
type CommandRef struct {
	// Plugin is the manifest id and Name the manifest name, the two things the
	// host needs to log, translate and call.
	Plugin  string
	Name    string
	Command Command
}

// PageRef is one contributed page and the plugin that declared it.
type PageRef struct {
	Plugin string
	Name   string
	Page   Page
}

// Index is the enabled plugins' contributions merged, which is what the window
// asks: a command by id, a page by id, each slot's items in drawing order. Two
// plugins claiming one id or one keybinding is the later plugin's loss, by
// manifest name, and is reported — a library is what one plugin claims, never
// an order two of them compete over.
type Index struct {
	plugins []Contributions
}

// BuildIndex merges the enabled plugins' contributions and reports what one of
// them lost to another: a command id or a keybinding two plugins claimed.
func BuildIndex(plugins []Contributions) (Index, []error) {
	sorted := make([]Contributions, len(plugins))
	copy(sorted, plugins)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Plugin < sorted[j].Plugin
	})

	var problems []error
	ids := map[string]string{}  // command id -> the plugin that claimed it
	keys := map[string]string{} // keybinding -> the plugin that claimed it
	for i := range sorted {
		plugin := &sorted[i]
		// The index adjusts what it merges, never the caller's copy.
		menus := make(map[string][]MenuItem, len(plugin.Menus))
		for slot, items := range plugin.Menus {
			menus[slot] = append([]MenuItem(nil), items...)
		}
		plugin.Menus = menus
		kept := make([]Command, 0, len(plugin.Commands))
		for _, command := range plugin.Commands {
			if owner, taken := ids[command.ID]; taken {
				problems = append(problems, fmt.Errorf(
					"contributes.commands: %s declares command %q, which %s already declared; the later one is hidden",
					plugin.Name, command.ID, owner))
				continue
			}
			if command.Keybinding != "" {
				if owner, taken := keys[command.Keybinding]; taken {
					problems = append(problems, fmt.Errorf(
						"contributes.commands: %s binds %q, which %s already binds; the later one is unbound",
						plugin.Name, command.Keybinding, owner))
					command.Keybinding = ""
				} else {
					keys[command.Keybinding] = plugin.Name
				}
			}
			ids[command.ID] = plugin.Name
			kept = append(kept, command)
		}
		plugin.Commands = kept

		// A menu item is a place for one of this plugin's commands: one whose
		// command went to another plugin has nowhere to point.
		own := make(map[string]bool, len(kept))
		for _, command := range kept {
			own[command.ID] = true
		}
		for slot, items := range plugin.Menus {
			menu := make([]MenuItem, 0, len(items))
			for _, item := range items {
				if own[item.Command] {
					menu = append(menu, item)
				}
			}
			plugin.Menus[slot] = menu
		}
	}
	return Index{plugins: sorted}, problems
}

// Plugins is every indexed plugin, by manifest name: the order the window lists
// them in.
func (i Index) Plugins() []Contributions { return i.plugins }

// Plugin answers one plugin's contributions by manifest id.
func (i Index) Plugin(id string) (Contributions, bool) {
	for _, plugin := range i.plugins {
		if plugin.Plugin == id {
			return plugin, true
		}
	}
	return Contributions{}, false
}

// Commands lists every command the window may show, by plugin and then by id.
// Whether one shows now is its When, which the caller evaluates with the
// frame's context.
func (i Index) Commands() []CommandRef {
	var out []CommandRef
	for _, plugin := range i.plugins {
		for _, command := range plugin.Commands {
			out = append(out, CommandRef{Plugin: plugin.Plugin, Name: plugin.Name, Command: command})
		}
	}
	return out
}

// Command answers one command by id, with the plugin that declared it.
func (i Index) Command(id string) (CommandRef, bool) {
	for _, plugin := range i.plugins {
		if command, ok := plugin.Command(id); ok {
			return CommandRef{Plugin: plugin.Plugin, Name: plugin.Name, Command: command}, true
		}
	}
	return CommandRef{}, false
}

// Pages lists every contributed page, by plugin and then by id.
func (i Index) Pages() []PageRef {
	var out []PageRef
	for _, plugin := range i.plugins {
		for _, page := range plugin.Pages {
			out = append(out, PageRef{Plugin: plugin.Plugin, Name: plugin.Name, Page: page})
		}
	}
	return out
}

// Page answers one contributed page by its plugin and its id.
func (i Index) Page(plugin, id string) (PageRef, bool) {
	contributions, ok := i.Plugin(plugin)
	if !ok {
		return PageRef{}, false
	}
	page, ok := contributions.Page(id)
	if !ok {
		return PageRef{}, false
	}
	return PageRef{Plugin: contributions.Plugin, Name: contributions.Name, Page: page}, true
}

// PageByID answers the first contributed page with that id, which is what a
// command's navigate result names without a plugin.
func (i Index) PageByID(id string) (PageRef, bool) {
	for _, ref := range i.Pages() {
		if ref.Page.ID == id {
			return ref, true
		}
	}
	return PageRef{}, false
}

// Menu lists one slot's commands in the order the host draws them: its primary
// group, then its secondary, then the plugins', each by order.
func (i Index) Menu(slot string) []CommandRef {
	var out []CommandRef
	for _, plugin := range i.plugins {
		for _, item := range plugin.Menus[slot] {
			command, ok := i.Command(item.Command)
			if !ok {
				continue
			}
			out = append(out, command)
		}
	}
	return out
}
