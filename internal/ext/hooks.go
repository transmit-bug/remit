package ext

import (
	"fmt"
	"reflect"

	"github.com/transmit-bug/remit/internal/core"
)

// The hook ids: the export a plugin writes. The vocabulary (HookContracts) and
// the call table are both spelled with them, so a hook is one string in one
// place. The command hook is the one id a manifest does not declare — it rides
// with contributes — which is why it is the one that is exported: the
// contribution layer names it.
const (
	hookDescribe    = "describe"
	hookDeriveBlock = "deriveBlock"
	hookIdentity    = "identity"
	hookOnEvent     = "on_event"
	hookPolicy      = "policy"
	hookSummary     = "summary"
	hookImportFile  = "importFile"

	// CommandHook is the export a plugin answers its contributed commands and its
	// pages' data with (docs/specs/plugins/ui.md §5). It rides with contributes,
	// not with implements: one hook, because a page's data command is a command.
	CommandHook = "command"
)

// Caller is a started extension as one hook call needs it: the one call shape
// both substrates answer. *Runner and *JSRunner are callers, and so is a test's
// stand-in — which is the point: what a hook's caller takes is Call, not a
// runner.
type Caller interface {
	Call(method string, params, out any) error
}

// Param is one parameter of a hook: the name the contract spells it with, and the
// shape it carries.
type Param struct {
	Name string
	Type reflect.Type
}

// HookType is one row of the hook table: a hook, the shapes its wire carries in
// and out, and the adapter that turns the hook's Go input into the parameters a
// plugin is called with.
type HookType struct {
	// Name is the hook id a plugin exports; the command hook is the one id a
	// manifest does not declare, because it rides with contributes.
	Name string
	// Params are the hook's parameters, in the order a plugin's export declares
	// them. None for a hook that takes nothing.
	Params []Param
	// Returns are the shapes the hook answers, as a union in the contract's
	// order. None for a hook that answers nothing.
	Returns []reflect.Type
	// Adapter turns the hook's Go input shape into the parameters the wire
	// carries; nil for a hook that takes nothing. It is the only place a hook's
	// input becomes arguments, so an input shape and the arguments a plugin
	// receives cannot be edited apart.
	Adapter func(in any) (any, error)
}

// HookTable is the one call table: every hook the host calls on a plugin, in the
// order the contract lists them. The vocabulary (HookContracts, what a manifest
// may declare), the six capabilities below, the fixtures harness and remit.d.ts
// all read it — so a hook cannot exist on one side of the contract and not
// another, and a hook nobody may declare cannot be called by accident.
var HookTable = []HookType{
	{
		// A manifest may declare describe: the host asks a plugin what it is in
		// its own words. It is not one of the six capabilities: no business code
		// calls it, the manifest already carries a name and a version.
		Name:    hookDescribe,
		Returns: []reflect.Type{typeOf(DescribeAnswer{})},
	},
	{
		Name: hookDeriveBlock,
		Params: []Param{
			{Name: "block", Type: typeOf(Block{})},
			{Name: "types", Type: typeOf([]CardType(nil))},
		},
		Returns: []reflect.Type{typeOf([]DerivedCard(nil))},
		Adapter: deriveArguments,
	},
	{
		// identity is in the vocabulary and in the contract, but no capability
		// calls it: the producer stamp carries the manifest's id, and a scheduler
		// names itself through policy.
		Name:    hookIdentity,
		Returns: []reflect.Type{typeOf("")},
	},
	{
		Name:    hookOnEvent,
		Params:  []Param{{Name: "event", Type: typeOf(Event{})}},
		Adapter: oneObject,
	},
	{
		Name:    hookPolicy,
		Returns: []reflect.Type{typeOf(Policy{})},
	},
	{
		Name:    hookSummary,
		Params:  []Param{{Name: "session", Type: typeOf(SummaryIn{})}},
		Returns: []reflect.Type{typeOf(Summary{})},
		Adapter: oneObject,
	},
	{
		// One command hook, two capabilities: RunCommand answers the host's result
		// and PageData answers a page's tree, which is raw JSON rather than a shape
		// ext can parse.
		Name:    CommandHook,
		Params:  []Param{{Name: "input", Type: typeOf(CommandIn{})}},
		Returns: []reflect.Type{typeOf(CommandResult{}), typeOf(PageTree(nil))},
		Adapter: oneObject,
	},
	{
		Name:    hookImportFile,
		Params:  []Param{{Name: "input", Type: typeOf(ImportIn{})}},
		Returns: []reflect.Type{typeOf(ImportSuggestions{})},
		Adapter: oneObject,
	},
}

// typeOf is the table's spelling of "this shape": a zero value, whose type is the
// shape itself.
func typeOf(v any) reflect.Type { return reflect.TypeOf(v) }

// deriveArguments is deriveBlock's adapter: its two parameters are its input's
// fields, in the contract's order, so a plugin is called with two arguments and
// not with one object.
func deriveArguments(in any) (any, error) {
	got, ok := in.(DeriveIn)
	if !ok {
		return nil, fmt.Errorf("ext: deriveBlock is asked with a DeriveIn, not %T", in)
	}
	return []any{got.Block, got.Types}, nil
}

// oneObject is every other hook's adapter: the wire takes the hook's input as one
// object, and that is the shape the caller built.
func oneObject(in any) (any, error) { return in, nil }

// hookRow is the table's row for a hook, and whether the host calls it at all.
func hookRow(hook string) (HookType, bool) {
	for _, row := range HookTable {
		if row.Name == hook {
			return row, true
		}
	}
	return HookType{}, false
}

// Invoke asks c for one hook by name, with parameters as the wire carries them:
// one argument per hook parameter. It is how an untyped caller runs any hook the
// table holds — the fixtures harness compares a plugin's answers through it — so
// a hook the host does not call cannot be run at all.
func Invoke(c Caller, hook string, params, out any) error {
	row, ok := hookRow(hook)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoHook, hook)
	}
	return c.Call(row.Name, params, out)
}

// The six capabilities the host calls on a plugin, one per job, named after what
// a plugin can do. They are separate interfaces on purpose: a caller takes the
// one it needs (the card module has no business scheduling), and a test doubles
// the one method it is about instead of a whole runner.
type (
	// Deriver derives cards from a block's text: which of the library's types the
	// text makes, and the field values each card carries.
	Deriver interface {
		DeriveBlock(in DeriveIn) ([]DerivedCard, error)
	}

	// Scheduler is what a library schedules with, and how a scheduling plugin
	// names itself.
	Scheduler interface {
		Policy() (Policy, error)
	}

	// Summariser says what a finished session is worth beyond its numbers.
	Summariser interface {
		Summary(in SummaryIn) (Summary, error)
	}

	// EventSink is told what happened. It answers nothing: an event is a
	// notification, not a veto, so the host ignores whatever a plugin makes of
	// one.
	EventSink interface {
		OnEvent(event Event) error
	}

	// Commander is the command hook's two uses: a contributed command's callback,
	// which answers a result the host applies, and a page's data command, which
	// answers its tree.
	Commander interface {
		RunCommand(in CommandIn) (CommandResult, error)
		PageData(in CommandIn) (PageTree, error)
	}

	// Importer turns one file into suggestions. The host checks them and writes;
	// a plugin that declared no importer is never asked.
	Importer interface {
		ImportFile(in ImportIn) (ImportSuggestions, error)
	}

	// Hooks is every capability the host calls on a plugin, in one interface for
	// the caller that wants all of them.
	Hooks interface {
		Deriver
		Scheduler
		Summariser
		EventSink
		Commander
		Importer
	}
)

// hooks is the six capabilities over one caller: the one implementation of the
// typed face, so the two substrates cannot answer a shape differently and a
// migration of the call sites changes what they ask, never what a plugin sees.
type hooks struct{ c Caller }

// HooksOf is every capability over c. A caller takes the one it needs:
//
//	var deriver ext.Deriver = ext.HooksOf(runner)
//
// The runner itself stays where it belongs — what leaves the seam is what a
// plugin can do, not the thing that runs it.
func HooksOf(c Caller) Hooks { return hooks{c} }

// stamped is a caller that can say who it is: the manifest id and the version of
// the build that answered. A js runner is one; a test's stand-in need not be.
type stamped interface {
	Identity() string
	Revision() string
}

// ProducerOf answers who a caller is, for the cards and notes credited to it: the
// identity stamp that travels with the capabilities, so that no caller needs the
// runner behind them. A caller that cannot say answers the empty stamp — the
// host's own, which is what a card the app made carries — rather than a stamp it
// would be wrong to write.
func ProducerOf(c Caller) core.Producer {
	stamped, ok := c.(stamped)
	if !ok {
		return core.Producer{}
	}
	return core.Producer{ID: stamped.Identity(), Version: stamped.Revision()}
}

// DeriveBlock answers the cards a text is worth, in the order they are numbered.
func (h hooks) DeriveBlock(in DeriveIn) ([]DerivedCard, error) {
	var out []DerivedCard
	err := call(h.c, hookDeriveBlock, in, &out)
	return out, err
}

// Policy answers what a plugin schedules with.
func (h hooks) Policy() (Policy, error) {
	var out Policy
	err := call(h.c, hookPolicy, nil, &out)
	return out, err
}

// Summary answers what a session is worth saying.
func (h hooks) Summary(in SummaryIn) (Summary, error) {
	var out Summary
	err := call(h.c, hookSummary, in, &out)
	return out, err
}

// OnEvent tells a plugin what happened, and ignores the answer.
func (h hooks) OnEvent(event Event) error {
	return call(h.c, hookOnEvent, event, nil)
}

// RunCommand asks a command's callback what to do, as the host's result.
func (h hooks) RunCommand(in CommandIn) (CommandResult, error) {
	var out CommandResult
	err := call(h.c, CommandHook, in, &out)
	return out, err
}

// PageData asks a page's data command for its tree, as raw JSON.
func (h hooks) PageData(in CommandIn) (PageTree, error) {
	var out PageTree
	err := call(h.c, CommandHook, in, &out)
	return out, err
}

// ImportFile asks what one file suggests.
func (h hooks) ImportFile(in ImportIn) (ImportSuggestions, error) {
	var out ImportSuggestions
	err := call(h.c, hookImportFile, in, &out)
	return out, err
}

// call asks c for one hook through the table: the row's adapter turns the typed
// input into the wire's parameters, and the answer is read into out.
func call(c Caller, hook string, in, out any) error {
	row, ok := hookRow(hook)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoHook, hook)
	}
	var params any
	if row.Adapter != nil {
		var err error
		if params, err = row.Adapter(in); err != nil {
			return err
		}
	}
	return c.Call(row.Name, params, out)
}
