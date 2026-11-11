package ts

import (
	"reflect"
	"strings"
)

// This file is the one translation from a Go type to the contract's TypeScript:
// a field's name and optionality, and a type's spelling. Everything the hook
// section declares goes through it, so a field the host adds is a field the
// contract has, and a field it makes optional is optional there too.

// contractField reads a struct field's contract name and optionality from the
// JSON tag the wire already uses: omitempty is what makes a field a plugin may
// leave out, so optionality is one fact in one place.
func contractField(field reflect.StructField) (name, optional string) {
	tag, ok := field.Tag.Lookup("json")
	if !ok || tag == "-" {
		return "", ""
	}
	name, rest, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	if strings.Contains(rest, "omitempty") {
		optional = "?"
	}
	return name, optional
}

// contractType spells one Go type the way the contract does. The rules are the
// ones the hooks' shapes need and no more: a type outside them spells nothing,
// and the section's test says so rather than a signature printing an empty type.
//
// A string whose values are a closed set is spelled by the field's remit tag, not
// here: reflection cannot see string literals, and a generator that guessed them
// would be a second list of enumerations.
func contractType(t reflect.Type) string {
	if t == nil {
		return ""
	}
	if spelling, ok := spellings[t]; ok {
		return spelling
	}
	if name, ok := declaredName(t); ok {
		return name
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Float64:
		return "number"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			// []byte is the bytes, base64 on the wire: Go's own JSON marshalling
			// does the encoding, so a caller hands a plugin bytes and never base64.
			return "string"
		}
		return contractType(t.Elem()) + "[]"
	case reflect.Map:
		if t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.String {
			return "Record<string, string>"
		}
	case reflect.Struct:
		return inlineObject(t)
	}
	return ""
}

// declaredName is the contract's name for a type this section declares.
func declaredName(t reflect.Type) (string, bool) {
	for _, d := range hookDeclarations {
		if d.Go == t {
			return d.contractName(), true
		}
	}
	return "", false
}

// inlineObject is a struct the contract does not declare on a line of its own:
// its fields, where a signature names it — describe's answer.
func inlineObject(t reflect.Type) string {
	fields := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		name, optional := contractField(field)
		if name == "" {
			continue
		}
		fields = append(fields, name+optional+": "+contractType(field.Type))
	}
	return "{ " + strings.Join(fields, "; ") + " }"
}

// fieldType is one field's contract type: the union its remit tag declares, when
// it declares one, else the type reflection spells.
func fieldType(field reflect.StructField) string {
	values, ok := field.Tag.Lookup("remit")
	if !ok {
		return contractType(field.Type)
	}
	literals := strings.Split(values, "|")
	for i, value := range literals {
		literals[i] = quoted(value)
	}
	return strings.Join(literals, " | ")
}
