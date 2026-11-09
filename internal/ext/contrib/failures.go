package contrib

import "sort"

// FailureLimit is how many times one contribution may fail in a row before the
// host stops showing it. The count is per contribution, not per plugin: a broken
// page never costs the plugin its commands (docs/specs/plugins/ui.md §8).
const FailureLimit = 3

// Issue is one contribution the host is not showing, as the settings page lists
// it: which plugin declared it, which of its contributions it was, why it is not
// there, and which of the two reasons took it away.
type Issue struct {
	// Plugin is the manifest id, Contribution the command's or page's id. A
	// problem with the manifest as a whole names no contribution.
	Plugin       string
	Contribution string
	// Reason says what the host could not use.
	Reason string
	// Hidden is true when three failures took the contribution out. An entry
	// that is not hidden was dropped when the manifest was read: the host never
	// had it, so it never showed.
	Hidden bool
}

// FailureKey names one contribution's count — the plugin's manifest id and the
// contribution's id — for a log line or a message. Ids cannot hold a slash, so
// two contributions never read as one.
func FailureKey(plugin, id string) string { return plugin + "/" + id }

// failureKey is a contribution's identity in the count.
type failureKey struct{ plugin, id string }

// Failures counts each contribution's consecutive failures and remembers what the
// third one took out. One count per (plugin, contribution id), never per plugin:
// a page that throws does not carry its plugin's commands away with it
// (docs/specs/plugins/ui.md §8).
//
// It is data the window owns, never a plugin call: a handler counts into it and a
// frame reads it.
type Failures struct {
	counts  map[failureKey]int
	reasons map[failureKey]string
	hidden  map[failureKey]bool
}

// NewFailures is an empty count.
func NewFailures() *Failures {
	return &Failures{
		counts:  map[failureKey]int{},
		reasons: map[failureKey]string{},
		hidden:  map[failureKey]bool{},
	}
}

// Failed counts one failure of one contribution, keeps the reason for the
// settings page, and answers whether this is the failure that hid it.
func (f *Failures) Failed(plugin, id, reason string) bool {
	key := failureKey{plugin, id}
	if f.counts == nil {
		f.counts = map[failureKey]int{}
	}
	if f.reasons == nil {
		f.reasons = map[failureKey]string{}
	}
	if f.hidden == nil {
		f.hidden = map[failureKey]bool{}
	}
	f.counts[key]++
	if reason != "" {
		f.reasons[key] = reason
	}
	if f.hidden[key] || f.counts[key] < FailureLimit {
		return false
	}
	f.hidden[key] = true
	return true
}

// Succeeded clears a contribution's count: a contribution that answered is not
// one mistake away from being hidden.
func (f *Failures) Succeeded(plugin, id string) {
	key := failureKey{plugin, id}
	delete(f.counts, key)
	delete(f.reasons, key)
}

// Hidden reports whether three failures took the contribution out.
func (f *Failures) Hidden(plugin, id string) bool {
	return f.hidden[failureKey{plugin, id}]
}

// Count is how many failures one contribution has in a row.
func (f *Failures) Count(plugin, id string) int {
	return f.counts[failureKey{plugin, id}]
}

// Total is how many failures one plugin's contributions have in a row: the number
// the settings page shows beside the plugin.
func (f *Failures) Total(plugin string) int {
	total := 0
	for key, count := range f.counts {
		if key.plugin == plugin {
			total += count
		}
	}
	return total
}

// Reenable clears one plugin's counts and shows its contributions again: the
// settings page's action beside the failure count, and only that plugin's
// entries move.
func (f *Failures) Reenable(plugin string) {
	for key := range f.counts {
		if key.plugin == plugin {
			delete(f.counts, key)
			delete(f.reasons, key)
			delete(f.hidden, key)
		}
	}
}

// Issues is one snapshot of what the host is not showing now: what a manifest
// declared that this host could not use, by plugin id, as the drop was reported
// when the library was read, and then the contributions three failures took out.
// It reads the counts and calls nothing, so a frame may ask it.
//
// The order is stable — plugin, then contribution, then reason — because a
// settings page that reshuffles its rows on every frame reads as broken.
func (f *Failures) Issues(dropped map[string][]string) []Issue {
	issues := make([]Issue, 0, len(dropped)+len(f.hidden))
	for plugin, reasons := range dropped {
		for _, reason := range reasons {
			issues = append(issues, Issue{Plugin: plugin, Reason: reason})
		}
	}
	for key := range f.hidden {
		issues = append(issues, Issue{
			Plugin:       key.plugin,
			Contribution: key.id,
			Reason:       f.reasons[key],
			Hidden:       true,
		})
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Plugin != issues[j].Plugin {
			return issues[i].Plugin < issues[j].Plugin
		}
		if issues[i].Contribution != issues[j].Contribution {
			return issues[i].Contribution < issues[j].Contribution
		}
		if issues[i].Hidden != issues[j].Hidden {
			return !issues[i].Hidden
		}
		return issues[i].Reason < issues[j].Reason
	})
	return issues
}
