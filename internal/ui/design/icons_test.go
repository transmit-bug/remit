package design_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// Every vendored icon parses: a broken file would panic the first frame that
// drew it.
func TestEveryIconParses(t *testing.T) {
	names := design.IconNames()
	if len(names) != 38 {
		t.Fatalf("vendored %d icons, want the 38 the app uses: %v", len(names), names)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if icon := design.Icon(name); icon == nil {
				t.Fatal("parsed to nil")
			}
		})
	}
}

// The set is indexed by name, in a stable order, so a test can enumerate it.
func TestIconNames(t *testing.T) {
	names := design.IconNames()
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("names are not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

// One icon is one value: asking twice hands back the same parsed drawing, so a
// frame drawing the same icon twice parses nothing.
func TestIconIsShared(t *testing.T) {
	first, second := design.Icon("eye"), design.Icon("eye")
	if first != second {
		t.Error("the same icon parsed twice")
	}
}

// A name that was never vendored is a programming error, and says which names
// exist so that the typo is obvious.
func TestUnknownIconPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("an unknown name did not panic")
		}
		if msg, ok := r.(string); !ok || msg == "" {
			t.Fatalf("the panic said %v", r)
		}
	}()
	design.Icon("no-such-icon")
}
