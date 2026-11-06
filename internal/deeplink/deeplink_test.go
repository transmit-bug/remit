package deeplink

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
)

func TestParse(t *testing.T) {
	note := core.NewID("note")
	card := core.NewID("card")

	tests := []struct {
		name    string
		raw     string
		want    Route
		wantErr bool
	}{
		{name: "note", raw: "remit://note/" + string(note), want: Route{Kind: KindNote, ID: note}},
		{name: "card", raw: "remit://card/" + string(card), want: Route{Kind: KindCard, ID: card}},
		{name: "queue", raw: "remit://queue", want: Route{Kind: KindQueue}},
		{name: "search", raw: "remit://search?q=spacing%20effect", want: Route{Kind: KindSearch, Query: "spacing effect", Search: true}},
		{name: "wrong scheme", raw: "https://note/" + string(note), wantErr: true},
		{name: "unknown host", raw: "remit://library/" + string(note), wantErr: true},
		{name: "bad id", raw: "remit://note/not-an-id", wantErr: true},
		{name: "path in note", raw: "remit://note/../../etc/passwd", wantErr: true},
		{name: "queue with argument", raw: "remit://queue/now", wantErr: true},
		{name: "search without q", raw: "remit://search", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %+v, want an error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestStringRoundTrips(t *testing.T) {
	routes := []Route{
		{Kind: KindNote, ID: core.NewID("note")},
		{Kind: KindCard, ID: core.NewID("card")},
		{Kind: KindQueue},
		{Kind: KindSearch, Query: "spacing effect", Search: true},
	}
	for _, want := range routes {
		got, err := Parse(want.String())
		if err != nil {
			t.Errorf("Parse(%q): %v", want.String(), err)
			continue
		}
		if got != want {
			t.Errorf("round trip %q = %+v, want %+v", want.String(), got, want)
		}
	}
}

func TestStringNeverCarriesAPath(t *testing.T) {
	r := Route{Kind: KindNote, ID: core.NewID("note")}
	if strings.Contains(r.String(), "..") {
		t.Errorf("note link %q looks like a path", r.String())
	}
}
