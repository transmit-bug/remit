// Package cards turns a block's text into card faces. The card plugin
// derives typed cards — which type a block becomes, with which field values —
// and the host template engine renders the faces. This is the one door every
// caller goes through — the app, the agent API and the golden set — so a card
// reads the same everywhere (ADR 0007's "shapes live in one place").
package cards

import (
	"log/slog"
	"sync"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/template"
	"github.com/transmit-bug/remit/internal/ext"
)

// Resolver derives cards from block text and renders their faces with the
// library's types. A nil plugin is allowed: a library without a card plugin
// makes no new cards, and a card reads its paragraph's own words.
//
// One resolver is shared by every goroutine that serves an agent API request,
// so deriving and facing are safe to run at once.
type Resolver struct {
	plugin *ext.JSRunner
	// The type tables are written by New and read-only afterwards, so a face
	// reads them — and renders on them — without a lock.
	types map[core.ID]core.Type
	front map[core.ID]*template.Template
	back  map[core.ID]*template.Template

	mu sync.Mutex
	// asked caches what the plugin answered for a text, failures included, so
	// a paragraph is asked once however many of its cards are on screen — and
	// a plugin that throws is not retried every frame. Its answers are shared
	// with every caller, so none of them may mutate one.
	asked map[string]answer
	// deriving holds the derivations in flight, one per text, so a text being
	// asked is waited for rather than asked a second time.
	deriving map[string]*derivation
}

// answer is the cached reply for one text.
type answer struct {
	cards []core.Derived
	ok    bool
}

// derivation is one text's plugin call in flight; done closes once its answer
// is in the cache.
type derivation struct {
	done  chan struct{}
	cards []core.Derived
	ok    bool
}

// New builds a resolver over the plugin and the library's types. A type whose
// templates do not compile — a user's edit can break them — renders as the
// field list rather than taking the review down.
func New(plugin *ext.JSRunner, types []core.Type) *Resolver {
	r := &Resolver{
		plugin:   plugin,
		types:    make(map[core.ID]core.Type, len(types)),
		front:    make(map[core.ID]*template.Template, len(types)),
		back:     make(map[core.ID]*template.Template, len(types)),
		asked:    map[string]answer{},
		deriving: map[string]*derivation{},
	}
	for _, typ := range types {
		r.types[typ.ID] = typ
		r.front[typ.ID] = compile(typ, typ.Front, "front")
		r.back[typ.ID] = compile(typ, typ.Back, "back")
	}
	return r
}

// compile parses one side of a type's templates, reporting a bad one instead
// of failing the library.
func compile(typ core.Type, src, side string) *template.Template {
	tpl, err := template.Compile(src)
	if err != nil {
		slog.Error("remit: a card type's template does not compile", "type", typ.ID, "side", side, "err", err)
		return nil
	}
	return tpl
}

// Answer is a card's rendered faces, with what the review view may add.
type Answer struct {
	Front, Back string
	Hint        string
	TypeAnswer  string
	State       core.LinkState
}

// derivedEntry is one card the plugin answered, as the extension speaks it.
type derivedEntry struct {
	TypeID  string            `json:"typeId"`
	Ordinal int               `json:"ordinal"`
	Fields  map[string]string `json:"fields"`
	Unknown []string          `json:"unknown"`
}

// Derive asks the plugin what a text makes. ok is false when there is no
// plugin or it failed: a paragraph then makes no new card, and nothing the
// user sees turns red over it. The answer is stamped with the plugin's
// identity and cached by text — the failure too, so it is asked once, and a
// text already being asked is waited for rather than asked again. The answer
// is shared, so no caller may mutate it, and the plugin a derivation calls
// must not reach back into this resolver's Derive — a card plugin's runner is
// given no host that derives.
func (r *Resolver) Derive(text string) (cards []core.Derived, ok bool) {
	if r == nil || r.plugin == nil {
		return nil, false
	}
	r.mu.Lock()
	if got, hit := r.asked[text]; hit {
		r.mu.Unlock()
		return got.cards, got.ok
	}
	if wait, busy := r.deriving[text]; busy {
		r.mu.Unlock()
		<-wait.done
		return wait.cards, wait.ok
	}
	wait := &derivation{done: make(chan struct{})}
	r.deriving[text] = wait
	r.mu.Unlock()

	// The wait is settled even if the plugin call lets a panic out, so a
	// waiter is never left blocked on a text nobody is asking.
	defer func() {
		r.mu.Lock()
		wait.cards, wait.ok = cards, ok
		r.asked[text] = answer{cards: cards, ok: ok}
		delete(r.deriving, text)
		r.mu.Unlock()
		close(wait.done)
	}()

	// The plugin call stays outside the lock: another text derives on the
	// runner's other runtime instead of queueing behind this one.
	return r.ask(text)
}

// ask runs the plugin for one text. It takes no lock, so two texts derive at
// once on the runner's two runtimes rather than queueing behind each other.
func (r *Resolver) ask(text string) ([]core.Derived, bool) {
	types := make([]any, 0, len(r.types))
	for _, typ := range r.types {
		types = append(types, map[string]any{
			"id": string(typ.ID), "fields": typ.Fields, "version": typ.Version,
		})
	}
	var entries []derivedEntry
	block := map[string]any{"text": text}
	if err := r.plugin.Call("deriveBlock", []any{block, types}, &entries); err != nil {
		slog.Error("remit: the card plugin could not derive", "err", err)
		return nil, false
	}
	producer := core.Producer{ID: r.plugin.Identity(), Version: r.plugin.Revision()}
	out := make([]core.Derived, 0, len(entries))
	for _, entry := range entries {
		id := core.ID(entry.TypeID)
		if _, known := r.types[id]; !known {
			slog.Warn("remit: the card plugin answers an unknown type", "type", entry.TypeID)
			continue
		}
		if len(entry.Unknown) > 0 {
			slog.Warn("remit: fields the type does not carry were dropped", "type", entry.TypeID, "fields", entry.Unknown)
		}
		out = append(out, core.Derived{Type: id, Ordinal: entry.Ordinal, Fields: entry.Fields, Producer: producer})
	}
	return out, true
}

// Face answers what a card asks and accepts. The fields come from the
// paragraph — the block for a live reference, the snapshot for a frozen one —
// with the card's own edited fields laid on top; the type's templates render
// the two sides. A paragraph that asks nothing of this card leaves its own
// words, unsplit; a template that fails leaves the field list, never a
// blank.
func (r *Resolver) Face(card core.Card, src core.Blocks) Answer {
	text, state := card.Ref.Text(src)
	entry, derived := r.entry(text, card.Type, card.Ordinal)
	if !derived {
		// No plugin, no answer, or the paragraph asks nothing this card can
		// use (an imported card whose block is the question itself): the card
		// reads its own words, unsplit, with its edits laid on top.
		front, back := text, ""
		if card.Fields["Front"] != "" {
			front = card.Fields["Front"]
		}
		if card.Fields["Back"] != "" {
			back = card.Fields["Back"]
		}
		return Answer{Front: front, Back: back, State: state}
	}
	fields := map[string]string{}
	for _, name := range r.fieldsOf(card.Type) {
		fields[name] = ""
	}
	for key, value := range entry.Fields {
		fields[key] = value
	}
	for key, value := range card.Fields {
		if value != "" {
			fields[key] = value
		}
	}
	front, back := r.front[card.Type], r.back[card.Type]
	if _, known := r.types[card.Type]; !known || front == nil || back == nil {
		list := template.Fallback(fields)
		return Answer{Front: list, Back: list, State: state}
	}
	frontOut, frontErr := front.Render(template.Input{Fields: fields, Ordinal: card.Ordinal, Side: template.SideFront})
	backOut, backErr := back.Render(template.Input{Fields: fields, Ordinal: card.Ordinal, Side: template.SideBack})
	if frontErr != nil || backErr != nil {
		slog.Error("remit: a card's template failed to render", "type", card.Type,
			"front", frontErr, "back", backErr)
		list := template.Fallback(fields)
		return Answer{Front: list, Back: list, State: state}
	}
	return Answer{
		Front: frontOut.Markdown, Back: backOut.Markdown,
		Hint: frontOut.Hint, TypeAnswer: frontOut.TypeAnswer,
		State: state,
	}
}

// entry finds the derived card that holds this place of this text. A card
// whose place the paragraph no longer asks — a cloze whose blank is gone —
// takes the fields of its type's first answer, so the template can ask what
// it can, the way the card plugin's faces hook did.
func (r *Resolver) entry(text string, typ core.ID, ordinal int) (core.Derived, bool) {
	derived, ok := r.Derive(text)
	if !ok {
		return core.Derived{}, false
	}
	for _, d := range derived {
		if d.Type == typ && d.Ordinal == ordinal {
			return d, true
		}
	}
	for _, d := range derived {
		if d.Type == typ {
			return d, true
		}
	}
	return core.Derived{}, false
}

// fieldsOf lists the field names a type carries, so every field renders — an
// empty one as empty, not as a template failure.
func (r *Resolver) fieldsOf(typ core.ID) []string {
	if t, ok := r.types[typ]; ok {
		return t.Fields
	}
	return nil
}
