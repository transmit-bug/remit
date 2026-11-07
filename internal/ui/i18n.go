package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// m is the catalogs in the language this frame shows, so the view follows the
// window's locale and a locale switch never needs a second code path.
func m(c *ui.Context) locales.Messages { return locales.In(c.Locale()) }
