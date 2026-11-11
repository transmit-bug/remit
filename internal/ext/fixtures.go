package ext

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Fixture is one question a plugin's own fixtures ask it: the hook to ask, the
// arguments as the wire carries them — one per hook parameter — and the answer the
// hook must give, as canonical JSON (see Canonical).
//
// It is a fixture and not a call: the expected answer travels with the question,
// and CheckFixture is the only thing that ever sees the reply. That is the one
// by-name door a harness needs — a fixture in, a verdict out, never the plugin's
// answer.
type Fixture struct {
	// Hook is the hook id a plugin exports, as the call table spells it.
	Hook string
	// Args are the wire's parameters, one per hook parameter; nil when the hook
	// takes none.
	Args []any
	// Want is the answer the hook must give, in the form Canonical writes.
	Want []byte
}

// CheckFixture runs one fixture against h and reports whether the hook gave the
// answer the fixture holds, byte for byte.
//
// The hook is resolved through the call table, so a fixture drives any hook the
// table holds — identity and describe included, which no capability asks and no
// typed method names — and a name the table does not hold is refused rather than
// sent. It is the fixtures harness's door (internal/ext/exttest): the answer never
// leaves this function.
func CheckFixture(h Hook, f Fixture) error {
	var answer json.RawMessage
	if err := invoke(h, f.Hook, f.Args, &answer); err != nil {
		return err
	}
	got, err := Canonical(answer)
	if err != nil {
		return fmt.Errorf("the answer is not JSON: %w", err)
	}
	if !bytes.Equal(got, f.Want) {
		return fmt.Errorf("the hook answered\n%s\nwant\n%s", got, f.Want)
	}
	return nil
}

// invoke asks h for one hook with the wire's parameters as they already are: one
// argument per hook parameter. call is the same door with the table's adapter in
// front of it, for a typed input; a fixture's arguments are the wire's already, so
// it comes here. Like call it is unexported: a hook leaves this package as a
// capability or as a verdict, never as an answer.
func invoke(h Hook, hook string, params, out any) error {
	row, ok := hookRow(hook)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoHook, hook)
	}
	c, ok := h.(caller)
	if !ok {
		return fmt.Errorf("%w: %s: %s", ErrNoHook, h.Name(), hook)
	}
	return c.call(row.Name, params, out)
}

// Canonical is one JSON value as a comparison reads it: decoded and written back
// with sorted keys and two-space indents, so a fixture's whitespace is not part of
// the answer. An empty value is null, which is what a hook that answers nothing
// answered.
func Canonical(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("null")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(value, "", "  ")
}
