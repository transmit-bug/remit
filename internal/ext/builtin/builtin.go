// Package builtin ships the extensions Remit runs by default. They are ordinary
// extensions — a manifest, JavaScript, the same loader, the same capability
// rules and the same limits — only their files come from the binary instead of
// the library's extensions directory.
package builtin

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/transmit-bug/remit/internal/ext"
)

//go:embed cards/extension.json cards/main.js scheduler/extension.json scheduler/main.js
var files embed.FS

// Cards is the default card plugin: what a paragraph of a note asks, in the
// shapes the content model knows. It answers JSON; the host renders.
func Cards(limits ext.JSLimits) (*ext.JSRunner, error) {
	manifest, err := CardsManifest()
	if err != nil {
		return nil, err
	}
	source, err := files.ReadFile("cards/main.js")
	if err != nil {
		return nil, fmt.Errorf("ext: builtin cards: %w", err)
	}
	return ext.OpenBuiltin(manifest, source, limits)
}

// CardsManifest is what the card plugin declares: no capabilities, because it
// reads the block it is given and answers.
func CardsManifest() (ext.Manifest, error) {
	data, err := files.ReadFile("cards/extension.json")
	if err != nil {
		return ext.Manifest{}, fmt.Errorf("ext: builtin cards: %w", err)
	}
	var manifest ext.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ext.Manifest{}, fmt.Errorf("ext: builtin cards: manifest: %w", err)
	}
	return manifest, nil
}

// IsBuiltin reports whether a name belongs to an extension this app ships.
func IsBuiltin(name string) bool { return name == "cards" }

// Scheduler is the default scheduling plugin: it decides policy — retention, and
// what to call the scheduler — while the schedule's arithmetic stays the app's.
func Scheduler(limits ext.JSLimits) (*ext.JSRunner, error) {
	manifest, err := SchedulerManifest()
	if err != nil {
		return nil, err
	}
	source, err := files.ReadFile("scheduler/main.js")
	if err != nil {
		return nil, fmt.Errorf("ext: builtin scheduler: %w", err)
	}
	return ext.OpenBuiltin(manifest, source, limits)
}

// SchedulerManifest is what the scheduler plugin declares.
func SchedulerManifest() (ext.Manifest, error) {
	data, err := files.ReadFile("scheduler/extension.json")
	if err != nil {
		return ext.Manifest{}, fmt.Errorf("ext: builtin scheduler: %w", err)
	}
	var manifest ext.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ext.Manifest{}, fmt.Errorf("ext: builtin scheduler: manifest: %w", err)
	}
	return manifest, nil
}
