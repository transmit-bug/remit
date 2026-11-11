package ext_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/transmit-bug/remit/internal/ext"
)

// The vocabulary a manifest may declare and the call table the host calls are two
// lists on purpose: one says what a plugin may claim, the other what the host can
// call. A hook in one and not the other is a hook nobody reaches — a manifest
// accepting a hook no code answers, or a host calling a hook no manifest may name
// — so they are compared here rather than trusted. Adding a hook to the
// vocabulary without its shapes is red on the first loop.
func TestTheHookTableHasEveryHookTheVocabularyDeclares(t *testing.T) {
	rows := make(map[string]ext.HookType, len(ext.HookTable))
	for _, row := range ext.HookTable {
		if _, twice := rows[row.Name]; twice {
			t.Errorf("the call table holds %q twice", row.Name)
		}
		rows[row.Name] = row
	}

	// The hooks whose wire carries one direction only. They are named here so
	// that a hook which loses its shapes is red — a nil pair is only right where
	// the contract says the hook takes or answers nothing.
	takesNothing := map[string]bool{"describe": true, "identity": true, "policy": true}
	answersNothing := map[string]bool{"on_event": true}

	for hook := range ext.HookContracts {
		row, ok := rows[hook]
		if !ok {
			t.Errorf("the vocabulary declares %q and the call table has no shapes for it", hook)
			continue
		}
		if len(row.Params) == 0 && !takesNothing[hook] {
			t.Errorf("%q takes no parameter and is not one of the hooks that take none", hook)
		}
		if len(row.Returns) == 0 && !answersNothing[hook] {
			t.Errorf("%q answers nothing and is not one of the hooks that answer nothing", hook)
		}
	}
	for hook := range rows {
		if _, declarable := ext.HookContracts[hook]; !declarable && hook != ext.CommandHook {
			t.Errorf("the call table holds %q, which no manifest may declare", hook)
		}
	}

	// Every shape a row names is a shape: an unnamed parameter or a nil return is
	// a hook whose contract cannot be written.
	for _, row := range ext.HookTable {
		for _, param := range row.Params {
			if param.Name == "" || param.Type == nil {
				t.Errorf("%q has a parameter without a name or a shape: %+v", row.Name, param)
			}
		}
		for _, shape := range row.Returns {
			if shape == nil {
				t.Errorf("%q answers with a shape that is nil", row.Name)
			}
		}
	}
}

// A hook's Go input reaches the wire as that hook's own parameters: deriveBlock as
// two arguments, summary as one object, policy as none — and an importer's bytes
// as base64, because Go's JSON encoding does that and no caller should. The
// built-in card plugin is what breaks when deriveBlock is called with one object
// instead of two: it reads the block's text from its first argument.
func TestTheTableSpellsEachHooksArguments(t *testing.T) {
	types := []ext.CardType{{ID: "remit.basic", Fields: []string{"Front", "Back"}, Version: 1}}
	cases := []struct {
		what string
		hook string
		call func(ext.Hooks)
		want string
	}{
		{
			what: "deriveBlock's two arguments",
			hook: "deriveBlock",
			call: func(h ext.Hooks) {
				_, _ = h.DeriveBlock(ext.DeriveIn{Block: ext.Block{Text: "a :: b"}, Types: types})
			},
			want: `[{"text":"a :: b"},[{"id":"remit.basic","fields":["Front","Back"],"version":1}]]`,
		},
		{
			what: "policy's none",
			hook: "policy",
			call: func(h ext.Hooks) { _, _ = h.Policy() },
			want: `null`,
		},
		{
			what: "summary's one object",
			hook: "summary",
			call: func(h ext.Hooks) { _, _ = h.Summary(ext.SummaryIn{Reviewed: 3, DueTomorrow: 1, Accuracy: 0.5}) },
			want: `{"reviewed":3,"dueTomorrow":1,"accuracy":0.5}`,
		},
		{
			what: "on_event's event",
			hook: "on_event",
			call: func(h ext.Hooks) { _ = h.OnEvent(ext.Event{Kind: "note.saved", ID: "note_1"}) },
			want: `{"kind":"note.saved","id":"note_1"}`,
		},
		{
			what: "a command's input",
			hook: "command",
			call: func(h ext.Hooks) {
				_, _ = h.RunCommand(ext.CommandIn{Name: "remit.x", Context: ext.ContextJSON{"mode": "notes"}})
			},
			want: `{"name":"remit.x","context":{"mode":"notes"}}`,
		},
		{
			what: "an importer's file, base64 on the wire",
			hook: "importFile",
			call: func(h ext.Hooks) {
				_, _ = h.ImportFile(ext.ImportIn{Name: "a.md", Data: []byte("# hi"), Format: "md"})
			},
			want: `{"name":"a.md","data":"IyBoaQ==","format":"md"}`,
		},
	}

	for _, one := range cases {
		caller := &ext.RecorderForTest{}
		one.call(ext.HooksOf(caller))
		if caller.Asked != 1 || caller.Method != one.hook {
			t.Errorf("%s asked %q %d times, want %q once", one.what, caller.Method, caller.Asked, one.hook)
			continue
		}
		sent, err := json.Marshal(caller.Params)
		if err != nil {
			t.Errorf("%s: the parameters are not JSON: %v", one.what, err)
			continue
		}
		if string(sent) != one.want {
			t.Errorf("%s reached the wire as\n%s\nwant\n%s", one.what, sent, one.want)
		}
	}
}

// What a plugin answers is read back through the same JSON the wire carries, so a
// reply decodes into the hook's own type or the call fails. A page's tree is the
// trap this catches: it is raw JSON, and a shape defined over []byte rather than
// an alias would try to read the answer as base64 text.
func TestEveryCapabilityReadsTheAnswerTheWireCarries(t *testing.T) {
	var (
		policy    ext.Policy
		cards     []ext.DerivedCard
		summary   ext.Summary
		command   ext.CommandResult
		page      ext.PageTree
		suggested ext.ImportSuggestions
	)

	cases := []struct {
		what   string
		call   func(ext.Hooks) error
		answer any
		got    func() any
		want   any
	}{
		{
			what:   "a policy",
			answer: map[string]any{"family": "fsrs", "version": "1", "retention": 0.9},
			call: func(h ext.Hooks) (err error) {
				policy, err = h.Policy()
				return err
			},
			got:  func() any { return policy },
			want: ext.Policy{Family: "fsrs", Version: "1", Retention: 0.9},
		},
		{
			what: "derived cards",
			answer: []map[string]any{
				{"typeId": "remit.basic", "ordinal": 0, "fields": map[string]string{"Front": "a", "Back": "b"}},
			},
			call: func(h ext.Hooks) (err error) {
				cards, err = h.DeriveBlock(ext.DeriveIn{Block: ext.Block{Text: "a :: b"}})
				return err
			},
			got:  func() any { return cards },
			want: []ext.DerivedCard{{TypeID: "remit.basic", Fields: map[string]string{"Front": "a", "Back": "b"}}},
		},
		{
			what:   "a summary",
			answer: map[string]any{"tone": "quiet", "lines": []string{"one more"}},
			call: func(h ext.Hooks) (err error) {
				summary, err = h.Summary(ext.SummaryIn{Reviewed: 1})
				return err
			},
			got:  func() any { return summary },
			want: ext.Summary{Tone: "quiet", Lines: []string{"one more"}},
		},
		{
			what:   "a command's result",
			answer: map[string]any{"result": "toast", "message": "done"},
			call: func(h ext.Hooks) (err error) {
				command, err = h.RunCommand(ext.CommandIn{Name: "remit.x"})
				return err
			},
			got:  func() any { return command },
			want: ext.CommandResult{Result: "toast", Message: "done"},
		},
		{
			what:   "a page's tree, as raw JSON",
			answer: map[string]any{"kind": "page", "title": "Today"},
			call: func(h ext.Hooks) (err error) {
				page, err = h.PageData(ext.CommandIn{Name: "remit.x"})
				return err
			},
			got:  func() any { return string(page) },
			want: `{"kind":"page","title":"Today"}`,
		},
		{
			what:   "an importer's suggestions",
			answer: map[string]any{"notes": []map[string]any{{"title": "a", "blocks": []string{"b"}}}, "warnings": []string{"one line skipped"}},
			call: func(h ext.Hooks) (err error) {
				suggested, err = h.ImportFile(ext.ImportIn{Name: "a.md", Format: "md"})
				return err
			},
			got: func() any { return suggested },
			want: ext.ImportSuggestions{
				Notes:    []ext.SuggestedNote{{Title: "a", Blocks: []string{"b"}}},
				Warnings: []string{"one line skipped"},
			},
		},
	}

	for _, one := range cases {
		caller := &ext.RecorderForTest{Answer: one.answer}
		if err := one.call(ext.HooksOf(caller)); err != nil {
			t.Errorf("%s did not read back: %v", one.what, err)
			continue
		}
		if got := one.got(); !reflect.DeepEqual(got, one.want) {
			t.Errorf("%s read back as\n%+v\nwant\n%+v", one.what, got, one.want)
		}
	}
}

// fakeDeriver is a card plugin with no engine behind it: deriving is the
// interface's whole surface, so three lines replace a JavaScript runtime, a
// library and a window.
type fakeDeriver struct{ asked []ext.DeriveIn }

func (f *fakeDeriver) DeriveBlock(in ext.DeriveIn) ([]ext.DerivedCard, error) {
	f.asked = append(f.asked, in)
	return []ext.DerivedCard{{
		TypeID: "remit.basic",
		Fields: map[string]string{"Front": in.Block.Text, "Back": "the gaps"},
	}}, nil
}

// Deriving is what a Deriver is, and nothing else: a hand-written fake drives it.
// This fails to compile — not at run time — the day the interface starts leaking
// a runner through a parameter, a return or a method, which is exactly the seam
// the card module must be able to test without an engine.
func TestAFakeDeriverDrivesDerivationWithoutAnEngine(t *testing.T) {
	var deriver ext.Deriver = &fakeDeriver{}
	got, err := deriver.DeriveBlock(ext.DeriveIn{
		Block: ext.Block{Text: "spacing :: the gaps"},
		Types: []ext.CardType{{ID: "remit.basic", Fields: []string{"Front", "Back"}, Version: 1}},
	})
	if err != nil {
		t.Fatalf("a fake derivation failed: %v", err)
	}
	if len(got) != 1 || got[0].TypeID != "remit.basic" || got[0].Fields["Back"] != "the gaps" {
		t.Fatalf("the fake answered %+v", got)
	}

	asked := deriver.(*fakeDeriver).asked
	if len(asked) != 1 || asked[0].Block.ID != "" || asked[0].Types[0].ID != "remit.basic" {
		t.Fatalf("the fake was asked %+v", asked)
	}
}

// standIn is a plugin with no wire behind it: a name, a close, and the one
// capability it answers itself.
type standIn struct{ fakeDeriver }

func (s *standIn) Name() string { return "stand-in" }
func (s *standIn) Close() error { return nil }

// A stand-in from another package is asked the capability it answers itself, since
// the wire's door is unexported: HooksOf is a shape over a started extension, and a
// test still doubles the one method it is about instead of a whole runner (#270;
// docs/specs/plugin-system.md, P2: 测试替身只假一个方法).
func TestAStandInIsAskedTheCapabilityItAnswers(t *testing.T) {
	fake := &standIn{}
	got, err := ext.HooksOf(fake).DeriveBlock(ext.DeriveIn{Block: ext.Block{Text: "spacing :: the gaps"}})
	if err != nil {
		t.Fatalf("the stand-in was not asked: %v", err)
	}
	if len(got) != 1 || got[0].Fields["Front"] != "spacing :: the gaps" {
		t.Fatalf("the stand-in answered %+v", got)
	}
	if len(fake.asked) != 1 {
		t.Fatalf("the stand-in was asked %d times, want once", len(fake.asked))
	}
}

// A stand-in that answers no capability it is asked carries no such hook: the
// answer is ErrNoHook, not a zero value and not a panic. The regression this
// guards: a Hook with no wire behind it looking like a plugin that answered.
func TestAStandInThatAnswersNothingSaysNoSuchHook(t *testing.T) {
	var stand ext.Hook = &standIn{}
	if _, err := ext.HooksOf(stand).Policy(); !errors.Is(err, ext.ErrNoHook) {
		t.Fatalf("a stand-in with no wire answered a policy: %v", err)
	}
	if err := ext.HooksOf(stand).OnEvent(ext.Event{Kind: "note.saved"}); !errors.Is(err, ext.ErrNoHook) {
		t.Fatalf("a stand-in with no wire took an event: %v", err)
	}
}
