package ts

import (
	"testing"

	"github.com/transmit-bug/remit/internal/ext"
)

// A documented field is keyed by the contract's name for it. A key no field
// answers is a sentence that no longer reaches a reader — which is how a
// generation table rots: the field is renamed, the contract keeps the old words,
// and nothing is red.
func TestEveryDocumentedFieldIsOneTheShapeHas(t *testing.T) {
	for _, d := range hookDeclarations {
		if d.Go == nil || len(d.Fields) == 0 {
			continue
		}
		have := make(map[string]bool, d.Go.NumField())
		for i := range d.Go.NumField() {
			if name, _ := contractField(d.Go.Field(i)); name != "" {
				have[name] = true
			}
		}
		for name := range d.Fields {
			if !have[name] {
				t.Errorf("%s documents field %q, which it does not have", d.contractName(), name)
			}
		}
	}
}

// A shape the contract cannot spell would print an empty type in a signature or a
// declaration, which is a promise the host does not keep. Every shape a hook
// names, and every field of a shape this section declares, is spelled here.
func TestEveryHookShapeIsSpelled(t *testing.T) {
	for _, row := range ext.HookTable {
		for _, param := range row.Params {
			if contractType(param.Type) == "" {
				t.Errorf("%s: the contract cannot spell parameter %q (%v)", row.Name, param.Name, param.Type)
			}
		}
		for _, shape := range row.Returns {
			if contractType(shape) == "" {
				t.Errorf("%s: the contract cannot spell the answer %v", row.Name, shape)
			}
		}
	}

	for _, d := range hookDeclarations {
		if d.Go == nil {
			continue
		}
		for i := range d.Go.NumField() {
			field := d.Go.Field(i)
			if name, _ := contractField(field); name == "" {
				continue
			}
			if fieldType(field) == "" {
				t.Errorf("%s: the contract cannot spell field %q (%v)", d.contractName(), field.Name, field.Type)
			}
		}
	}
}

// A hook the contract comments is a hook the host calls: a key of hookDocs that
// names no hook in the table is a sentence printed above nothing — the same rot
// as a documented field, one level up.
func TestEveryDocumentedHookIsOneTheTableHas(t *testing.T) {
	called := make(map[string]bool, len(ext.HookTable))
	for _, row := range ext.HookTable {
		called[row.Name] = true
	}
	for hook := range hookDocs {
		if !called[hook] {
			t.Errorf("the contract documents hook %q, which the host does not call", hook)
		}
	}
}

// One contract name per declaration: two declarations under one name are a file
// whose second one narrows the first, and a plugin author reading a field of the
// wrong one.
func TestEveryDeclaredShapeHasItsOwnName(t *testing.T) {
	seen := make(map[string]bool, len(hookDeclarations))
	for _, d := range hookDeclarations {
		if d.Go == nil {
			continue
		}
		name := d.contractName()
		if seen[name] {
			t.Errorf("%s is declared twice", name)
		}
		seen[name] = true
	}
}
