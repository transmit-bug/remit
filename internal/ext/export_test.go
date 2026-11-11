package ext

import (
	"encoding/json"
	"fmt"
)

// This file is compiled into the package's test build alone — its name ends in
// _test.go — and it holds the two things the seam keeps to itself: the wire's
// by-name door, and a caller that records what a capability sent. Both exist for
// tests here and in ext_test; neither is in the app.

// CallForTest drives one export of a started extension by name, with parameters as
// the wire carries them. A test uses it to stand in for a substrate — this
// package's own tests ask modules that export "spin" and "list", which no hook
// names — and no code outside a test can, which is what keeps a by-name call out
// of the app.
func CallForTest(h Hook, method string, params, out any) error {
	c, ok := h.(caller)
	if !ok {
		return fmt.Errorf("%w: %s: %s", ErrNoHook, h.Name(), method)
	}
	return c.call(method, params, out)
}

// RecorderForTest is a started extension with no substrate behind it: it records
// the hook it was asked and the parameters it was handed, and answers what a test
// queued, decoded the way both substrates decode a reply. It is how a test reads
// what a capability puts on the wire without an engine.
type RecorderForTest struct {
	Method string
	Params any
	Asked  int
	// Answer is the plugin's reply; the caller's out reads it through JSON.
	Answer any
}

// Name is what a hook log would call it.
func (r *RecorderForTest) Name() string { return "recorder" }

// Close is a no-op: there is nothing behind it to release.
func (r *RecorderForTest) Close() error { return nil }

func (r *RecorderForTest) call(method string, params, out any) error {
	r.Asked++
	r.Method, r.Params = method, params
	if out == nil || r.Answer == nil {
		return nil
	}
	encoded, err := json.Marshal(r.Answer)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}
