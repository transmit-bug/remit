package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// statusbar is the window's last row: one line about the page on screen, in the
// smallest type the scale has. A page with nothing to report gets the room back
// rather than an empty band, and the line is not focusable — it reports, it does
// not act.
func statusbar(c *ui.Context, tk design.Tokens, d Deps) {
	if len(d.Status) == 0 {
		return
	}
	msgs := locales.In(c.Locale())
	ui.Row(c).Height(tk.StatusbarHeight).Shrink(0).
		Padding(0, tk.SpaceL).AlignItems(ui.Center).Gap(tk.SpaceS).
		Background(tk.Surface).
		BorderWidth(tk.Hairline, 0, 0, 0).BorderColor(tk.Border).
		Label(msgs.StatusLine()).Children(func() {
		for i, part := range d.Status {
			if i > 0 {
				ui.Divider(c).Height(tk.SpaceM).AlignSelf(ui.Center)
			}
			ui.Text(c, part).FontSize(tk.TypeCaption).
				TextColor(tk.TextMuted).SingleLine()
		}
	})
}
