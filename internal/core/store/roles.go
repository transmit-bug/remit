package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/transmit-bug/remit/internal/core"
)

// The plugins that play each role while a library names none: the two
// extensions this app ships, by manifest name (internal/ext/builtin).
const (
	DefaultCardsPlugin     = "cards"
	DefaultSchedulerPlugin = "scheduler"
)

// Errors a role write may answer.
var (
	// ErrUnknownRole is a role the host does not have.
	ErrUnknownRole = errors.New("store: no such plugin role")
	// ErrPluginNotEnabled is a role naming a plugin this library has not
	// approved: a role chooses among enabled plugins, never around approval.
	ErrPluginNotEnabled = errors.New("store: the plugin is not enabled in this library")
)

// DefaultRole answers the built-in plugin that plays role.
func DefaultRole(role string) (string, bool) {
	switch role {
	case core.RoleCardsDerive:
		return DefaultCardsPlugin, true
	case core.RoleReviewSchedule:
		return DefaultSchedulerPlugin, true
	}
	return "", false
}

// Roles answers the plugin each role names for this library. A role the
// library leaves unset — or names a plugin it no longer enables — reads as the
// built-in, so a hand-edited remit.json can never leave a role dangling.
func (l *Library) Roles() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rolesLocked()
}

// SetRole chooses the plugin that plays role in this library. An empty name, or
// the role's built-in, takes the choice back out of remit.json: the built-in is
// what an unset role means. The plugin must be enabled in this library, because
// a role chooses among plugins the user approved.
func (l *Library) SetRole(role, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	def, ok := DefaultRole(role)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownRole, role)
	}
	next := make(map[string]string, len(l.roles)+1)
	for held, plugin := range l.roles {
		if l.enabledLocked(plugin) {
			next[held] = plugin
		}
	}
	if name == "" || name == def {
		delete(next, role)
	} else {
		if !l.enabledLocked(name) {
			return fmt.Errorf("%w: %q", ErrPluginNotEnabled, name)
		}
		next[role] = name
	}
	held := l.roles
	l.roles = next
	if err := l.writeConfigLocked(); err != nil {
		l.roles = held
		return err
	}
	return nil
}

// ReadRoles answers the roles a library directory declares, with the built-in
// for every one it leaves unset: the app resolves plugins before it opens the
// library, because what schedules has to be known before the index replays.
// A directory without a readable config reads as the defaults; opening the
// library will report the real problem.
func ReadRoles(dir string) map[string]string {
	roles := map[string]string{}
	for _, role := range core.Roles() {
		if def, ok := DefaultRole(role); ok {
			roles[role] = def
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, configName))
	if err != nil {
		return roles
	}
	config, err := readConfig(data)
	if err != nil {
		return roles
	}
	enabled := make(map[string]bool, len(config.Extensions))
	for _, name := range config.Extensions {
		enabled[name] = true
	}
	for role, name := range config.Roles {
		def, ok := DefaultRole(role)
		if !ok || name == "" {
			continue
		}
		// A role chooses among what the library approved, so a name it no
		// longer enables is the built-in rather than a plugin nothing approved.
		if name == def || enabled[name] {
			roles[role] = name
		}
	}
	return roles
}

// rolesLocked is Roles without the lock, for the writes that already hold it.
func (l *Library) rolesLocked() map[string]string {
	roles := make(map[string]string, len(core.Roles()))
	for _, role := range core.Roles() {
		def, _ := DefaultRole(role)
		roles[role] = def
		if name := l.roles[role]; name != "" && l.enabledLocked(name) {
			roles[role] = name
		}
	}
	return roles
}

// enabledLocked reports whether the library enables a plugin.
func (l *Library) enabledLocked(name string) bool {
	if name == DefaultCardsPlugin || name == DefaultSchedulerPlugin {
		return true // the app ships them, so they are always there
	}
	for _, enabled := range l.extensions {
		if enabled == name {
			return true
		}
	}
	return false
}
