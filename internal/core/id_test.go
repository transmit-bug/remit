package core

import "testing"

func TestNewID(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		valid bool
	}{
		{"note", "note", true},
		{"card", "card", true},
		{"empty kind", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := NewID(tt.kind)
			if got := id.Valid(); got != tt.valid {
				t.Fatalf("NewID(%q).Valid() = %v, want %v", tt.kind, got, tt.valid)
			}
			if tt.valid && id.Kind() != tt.kind {
				t.Errorf("Kind() = %q, want %q", id.Kind(), tt.kind)
			}
		})
	}
}

func TestIDValid(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want bool
	}{
		{"minted", NewID("card"), true},
		{"empty", "", false},
		{"no kind", "_0123456789abcdef0123456789abcdef", false},
		{"short", "card_0123", false},
		{"not hex", "card_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", false},
		{"path", ID("../notes/today.md"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.Valid(); got != tt.want {
				t.Errorf("%q.Valid() = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestNewIDIsUnique(t *testing.T) {
	seen := make(map[ID]bool, 1024)
	for range 1024 {
		id := NewID("note")
		if seen[id] {
			t.Fatalf("duplicate ID %q", id)
		}
		seen[id] = true
	}
}
