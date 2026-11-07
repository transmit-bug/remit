package core_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/template"
)

func TestBuiltinTypes(t *testing.T) {
	t.Helper()
	got := core.BuiltinTypes()

	wantIDs := []core.ID{core.TypeBasic, core.TypeBidirectional, core.TypeCloze}
	if len(got) != len(wantIDs) {
		t.Fatalf("BuiltinTypes() returned %d types, want %d", len(got), len(wantIDs))
	}
	for i, ty := range got {
		if ty.ID != wantIDs[i] {
			t.Errorf("type %d ID = %q, want %q", i, ty.ID, wantIDs[i])
		}
		if ty.Name == "" {
			t.Errorf("type %q has empty Name", ty.ID)
		}
		if len(ty.Fields) == 0 {
			t.Errorf("type %q has no fields", ty.ID)
		}
		seen := map[string]bool{}
		for _, f := range ty.Fields {
			if f == "" {
				t.Errorf("type %q has an empty field name", ty.ID)
			}
			if seen[f] {
				t.Errorf("type %q has duplicate field %q", ty.ID, f)
			}
			seen[f] = true
		}
		if ty.Version < 1 {
			t.Errorf("type %q Version = %d, want >= 1", ty.ID, ty.Version)
		}
		for _, src := range []string{ty.Front, ty.Back} {
			if _, err := template.Compile(src); err != nil {
				t.Errorf("type %q: template %q does not compile: %v", ty.ID, src, err)
			}
		}
	}
}

func TestBuiltinTypesFieldSets(t *testing.T) {
	t.Helper()
	tests := []struct {
		id   core.ID
		want []string
	}{
		{core.TypeBasic, []string{"Front", "Back"}},
		{core.TypeBidirectional, []string{"Front", "Back"}},
		{core.TypeCloze, []string{"Text"}},
	}
	got := map[core.ID][]string{}
	for _, ty := range core.BuiltinTypes() {
		got[ty.ID] = ty.Fields
	}
	for _, tt := range tests {
		fields, ok := got[tt.id]
		if !ok {
			t.Fatalf("BuiltinTypes() is missing %q", tt.id)
		}
		if len(fields) != len(tt.want) {
			t.Fatalf("%q Fields = %v, want %v", tt.id, fields, tt.want)
		}
		for i := range tt.want {
			if fields[i] != tt.want[i] {
				t.Errorf("%q Fields[%d] = %q, want %q", tt.id, i, fields[i], tt.want[i])
			}
		}
	}
}

func TestBuiltinTypeTemplates(t *testing.T) {
	t.Helper()
	types := map[core.ID]core.Type{}
	for _, ty := range core.BuiltinTypes() {
		types[ty.ID] = ty
	}
	tests := []struct {
		name  string
		id    core.ID
		in    template.Input
		front bool // true: assert Front template, false: Back
		want  string
	}{
		{
			name: "basic front",
			id:   core.TypeBasic, front: true,
			in:   template.Input{Fields: map[string]string{"Front": "2+2?", "Back": "4"}, Side: template.SideFront},
			want: "2+2?",
		},
		{
			name: "basic back",
			id:   core.TypeBasic, front: false,
			in:   template.Input{Fields: map[string]string{"Front": "2+2?", "Back": "4"}, Side: template.SideBack},
			want: "4",
		},
		{
			name: "bidirectional swapped fields act as the other direction",
			id:   core.TypeBidirectional, front: true,
			in:   template.Input{Fields: map[string]string{"Front": "4", "Back": "2+2?"}, Side: template.SideFront},
			want: "4",
		},
		{
			name: "cloze front masks the ordinal blank",
			id:   core.TypeCloze, front: true,
			in:   template.Input{Fields: map[string]string{"Text": "Berlin is {{the capital}} of Germany"}, Ordinal: 0, Side: template.SideFront},
			want: "Berlin is […] of Germany",
		},
		{
			name: "cloze back reveals",
			id:   core.TypeCloze, front: false,
			in:   template.Input{Fields: map[string]string{"Text": "Berlin is {{the capital}} of Germany"}, Ordinal: 0, Side: template.SideBack},
			want: "Berlin is the capital of Germany",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			ty := types[tt.id]
			src := ty.Front
			if !tt.front {
				src = ty.Back
			}
			tpl, err := template.Compile(src)
			if err != nil {
				t.Fatalf("Compile(%q): %v", src, err)
			}
			out, err := tpl.Render(tt.in)
			if err != nil {
				t.Fatalf("Render(%v): %v", tt.in, err)
			}
			if out.Markdown != tt.want {
				t.Errorf("Markdown = %q, want %q", out.Markdown, tt.want)
			}
		})
	}
}
