package stats

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// block is one plugin's section of the overview: the name it goes by, and the
// items it put in the stats.block slot.
type block struct {
	Plugin string
	Name   string
	Items  []contrib.CommandRef
}

// blocksOf groups the slot's items by the plugin that declared them, in the
// order the index draws them: a plugin's items sit together under one heading,
// and a library with no such item draws no section and no empty heading.
func blocksOf(items []contrib.MenuRef) []block {
	var out []block
	at := map[string]int{}
	for _, item := range items {
		i, ok := at[item.Plugin]
		if !ok {
			i = len(out)
			at[item.Plugin] = i
			name := item.Name
			if name == "" {
				name = item.Plugin
			}
			out = append(out, block{Plugin: item.Plugin, Name: name})
		}
		out[i].Items = append(out[i].Items, item.CommandRef)
	}
	return out
}

// blocks draws the stats.block sections the overview ends with: one heading per
// plugin, one card, and its items as buttons. Every string is resolved here, in
// the frame's language, from what the library load cached — no frame calls a
// plugin, and a command runs off it like any other contribution.
func blocks(c *ui.Context, d Deps, tk design.Tokens) {
	for _, section := range blocksOf(d.Items) {
		s := section
		ui.Column(c).Gap(tk.SpaceS).Children(func() {
			sectionHead(c, tk, s.Name, "")
			kit.StyleCard(c, ui.Column(c).Padding(tk.SpaceL).Children(func() {
				ui.Row(c).Wrap().Gap(tk.SpaceS).Children(func() {
					for _, ref := range s.Items {
						button(c, tk, d, ref)
					}
				})
			}))
		})
	}
}

// button is one contributed item: its title in this frame's language, and the
// command behind it. An item the app has no runner for is shown as text — a
// control that cannot act is not drawn as one.
func button(c *ui.Context, tk design.Tokens, d Deps, ref contrib.CommandRef) {
	label := string(ref.Command.Title)
	if d.PluginText != nil {
		label = d.PluginText(c, ref.Plugin, label)
	}
	if d.Run == nil {
		ui.Text(c, label).FontSize(tk.TypeUI)
		return
	}
	ui.Button(c, label).MinHeight(tk.Control).OnClick(func() { d.Run(c, ref) })
}
