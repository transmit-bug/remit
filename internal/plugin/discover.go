package plugin

import (
	"errors"
	"log/slog"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// Info is one plugin this build can run, as the settings page lists it: what its
// manifest declares, read without starting anything.
type Info struct {
	Name         string
	ID           string
	Version      string
	Capabilities []string // the capability names the manifest declares
	Roles        []string // the role names the manifest implements
	Builtin      bool     // this build ships it
}

// Discover lists the plugins this build can run: the two it ships, then what the
// user installed. A plugin that will not load is reported and left out, so the
// page lists what works.
func (s *Supervisor) Discover() ([]Info, error) {
	found, problems := s.all()
	out := make([]Info, 0, len(found))
	for _, e := range found {
		out = append(out, info(e))
	}
	return out, problems
}

// Manifests lists what the plugin directory holds, with what each manifest
// declares, without starting any of it: the window parses the enabled plugins'
// pages, commands, menus and settings from these, so a plugin it does not run
// still costs nothing.
func (s *Supervisor) Manifests() ([]ext.Extension, error) {
	return s.all()
}

// all is the app's own plugins followed by the plugin directory's: what the
// settings page and the contribution index both read. A manifest that will not
// parse is reported and skipped, so a broken plugin costs only itself.
func (s *Supervisor) all() ([]ext.Extension, error) {
	out := make([]ext.Extension, 0, 4)
	var problems []error
	for _, manifest := range []func() (ext.Manifest, error){builtin.CardsManifest, builtin.SchedulerManifest} {
		m, err := manifest()
		if err != nil {
			problems = append(problems, err)
			continue
		}
		out = append(out, ext.Extension{Manifest: m})
	}
	found, err := ext.Discover(s.dir)
	out = append(out, found...)
	problems = append(problems, err)
	return out, errors.Join(problems...)
}

// find answers the installed extension a manifest name names, as discovery sees
// it.
func (s *Supervisor) find(name string) (ext.Extension, bool) {
	found, err := ext.Discover(s.dir)
	if err != nil {
		slog.Warn("remit: plugin discovery", "err", err)
	}
	for _, candidate := range found {
		if candidate.Manifest.Name == name {
			return candidate, true
		}
	}
	return ext.Extension{}, false
}

// info is one manifest as the settings page lists it.
func info(e ext.Extension) Info {
	roles := make([]string, 0, len(core.Roles()))
	for _, role := range core.Roles() {
		if ext.Plays(e, role) {
			roles = append(roles, role)
		}
	}
	return Info{
		Name:         e.Manifest.Name,
		ID:           e.Manifest.ID,
		Version:      e.Manifest.Version,
		Capabilities: append([]string(nil), e.Manifest.Capabilities...),
		Roles:        roles,
		Builtin:      builtin.IsBuiltin(e.Manifest.Name),
	}
}
