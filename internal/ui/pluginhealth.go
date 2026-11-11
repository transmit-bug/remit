package ui

import (
	"log/slog"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/plugin"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
)

// reenable is the settings page's one action over a plugin, which is two
// counters: the supervisor clears the plugin-level count and opens the runner
// again, and the contribution layer clears its counts and its hidden set. The
// counts stay separate — a broken page never takes the plugin's commands with it
// — so the page draws one button over both (docs/specs/plugin-system.md P1:
// 重新启用是一个用户动作、两个内部计数).
func (a *App) reenable(c *ui.Context, name string) {
	if a.plugins != nil {
		if err := a.plugins.Reenable(name); err != nil {
			slog.Warn("remit: a plugin would not start again", "plugin", name, "err", err)
			a.prefs.PluginErr = m(c).PluginReenableFailed(err.Error())
		}
	}
	a.reenablePlugin(c, name)
}

// settingsHealth is the supervisor's answer as the page draws it: the same state
// names, the same raw reason, and the roles a built-in plays instead.
func settingsHealth(h plugin.Health) screensettings.PluginHealth {
	out := screensettings.PluginHealth{State: string(h.State), Failures: h.Failures, Reason: h.Reason}
	for _, fallback := range h.Fallback {
		out.Fallback = append(out.Fallback, screensettings.PluginFallback{
			Role:   fallback.Role,
			Player: fallback.Player,
		})
	}
	return out
}
