package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// committedTheme draws the window once and returns the theme it draws with, so a
// test reads what the views actually use rather than what they meant to set.
func committedTheme(t *testing.T, a *App, dark bool, prefs *ui.Preferences) *ui.Theme {
	t.Helper()
	var got *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		got = c.Theme()
	}, 1024, 720)
	if prefs != nil {
		tt.SetPreferences(*prefs)
	}
	tt.SetDark(dark)
	tt.Frame()
	if got == nil {
		t.Fatal("the window committed no theme")
	}
	return got
}

// The window follows the desktop's appearance: the same window, switched while
// it runs, draws from the other palette without being rebuilt.
func TestAppearanceFollowsTheDesktop(t *testing.T) {
	a, _ := newTestApp(t)
	var got *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		got = c.Theme()
	}, 1024, 720)
	tt.Frame()

	for _, c := range []struct {
		name string
		dark bool
		want design.Palette
	}{
		{"paper", false, design.Light()},
		{"ink", true, design.Dark()},
		{"paper again", false, design.Light()},
	} {
		tt.SetDark(c.dark)
		if got.Dark != c.dark {
			t.Fatalf("%s: the window draws with Dark=%v", c.name, got.Dark)
		}
		if !sameColor(got.Background, c.want.Page) {
			t.Errorf("%s: the page is %v, want %v", c.name, got.Background, c.want.Page)
		}
		if !sameColor(got.Text, c.want.Text) {
			t.Errorf("%s: the text is %v, want %v", c.name, got.Text, c.want.Text)
		}
	}
	tt.SetDark(false)
	if !tt.HasText("Show answer") {
		t.Fatalf("the window lost its content after switching appearance; texts %q", tt.Texts())
	}
}

// The accent is Remit's, not the one the user picked in the desktop: a system
// accent never reaches the window.
func TestAccentIsNotTheDesktops(t *testing.T) {
	desktop := ui.Preferences{Accent: ui.Hex("#00FF00")}
	for _, dark := range []bool{false, true} {
		a, _ := newTestApp(t)
		got := committedTheme(t, a, dark, &desktop)
		want := design.PaletteOf(dark).Accent
		if !sameColor(got.Accent, want) {
			t.Errorf("dark=%v: the accent is %v, want Remit's %v", dark, got.Accent, want)
		}
	}
}

// Text follows the desktop's text size, so a user who turned it up reads the
// window at that size.
func TestTextFollowsTheDesktopScale(t *testing.T) {
	a, _ := newTestApp(t)
	plain := committedTheme(t, a, false, nil)
	scaled := committedTheme(t, a, false, &ui.Preferences{TextScale: 1.5})
	if scaled.FontSize <= plain.FontSize {
		t.Errorf("text at 1.5x is %v, no larger than %v", scaled.FontSize, plain.FontSize)
	}
}

// Every mode draws in both appearances, at the size the window opens at and at
// the smallest it goes.
func TestEveryModeInBothAppearances(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := map[bool]string{false: "paper", true: "ink"}[dark]
		for _, size := range windowSizes {
			t.Run(appearance+"/"+size.name, func(t *testing.T) {
				a, _ := newTestApp(t)
				tt := ui.NewTester(a.View, size.w, size.h)
				tt.SetDark(dark)
				tt.Frame()
				tt.Advance(200 * time.Millisecond) // the first card's entry

				// The review queue, then the notes, on the real mode keys.
				assertShows(t, tt, "Show answer")
				tt.Key(ui.Cmd, ui.Key2)
				tt.Advance(200 * time.Millisecond) // the card's exit
				assertShows(t, tt, "Notebook")

				tt.Key(ui.Cmd, ui.Key3)
				assertShows(t, tt, "Browse cards")

				// Statistics, from the command palette as a user reaches it.
				tt.Key(ui.Cmd, ui.KeyK)
				tt.Type("stat")
				tt.Key(0, ui.KeyEnter)
				assertShows(t, tt, "Statistics")

				tt.Key(ui.Cmd, ui.KeyComma)
				assertShows(t, tt, "Agent API")

				tt.Key(0, ui.KeyEscape)
				tt.Advance(200 * time.Millisecond)
				if tt.FramePending() {
					t.Error("view still requests frames after it should settle")
				}
			})
		}
	}
}

// assertShows fails unless the window shows want, with what it does show.
func assertShows(t *testing.T, tt *ui.Tester, want string) {
	t.Helper()
	if !tt.HasText(want) {
		t.Fatalf("missing %q; texts %q", want, tt.Texts())
	}
}

func sameColor(a, b ui.Color) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B && a.A == b.A
}
