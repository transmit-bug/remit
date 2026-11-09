package design

// The glyphs a keycap writes a key with: the symbol the platform prints on the
// key, so one chord reads the same on every page, and the keyboard help and the
// palette's hints cannot drift apart.
const (
	KeyCmd   = "⌘"
	KeyShift = "⇧"
	KeyAlt   = "⌥"
	KeyCtrl  = "⌃"
	KeyEnter = "↵"
	KeyEsc   = "esc"
	KeySpace = "space"
	KeyUp    = "↑"
	KeyDown  = "↓"
	// KeyRange names a run of keys — the areas, the grades — as one cap.
	KeyRange = "…"
	// KeySeparator is what two keys of one meaning are joined by: J / K.
	KeySeparator = " / "
)

// Chord is a chord's parts as the one string a keycap shows: ⌘K, ⇧⌘Z.
func Chord(parts ...string) string {
	out := ""
	for _, part := range parts {
		out += part
	}
	return out
}
