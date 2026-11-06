package design

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var iconFS embed.FS

// IconNames is every icon the app vendors, by Lucide's name, sorted.
func IconNames() []string {
	set := iconSet()
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Icon resolves the icon Lucide calls name, as "eye" or "chart-column". The set
// is parsed once and shared: an icon is a value, not a drawing, so the same one
// draws at 16 DIPs in a button and at 24 on an empty page.
//
// An unknown name is a programming error — a typo, or an icon that was never
// vendored — so it panics, as a bad SVG would.
func Icon(name string) *ui.SVG {
	if icon, ok := iconSet()[name]; ok {
		return icon
	}
	panic(fmt.Sprintf("design: no icon named %q; vendored: %s", name, strings.Join(IconNames(), ", ")))
}

var (
	iconsOnce   sync.Once
	iconsByName map[string]*ui.SVG
)

// iconSet parses every vendored icon once. The files are small and the set is
// fixed, so parsing them all costs less than tracking which ones a frame drew.
func iconSet() map[string]*ui.SVG {
	iconsOnce.Do(func() {
		entries, err := iconFS.ReadDir("icons")
		if err != nil {
			panic(fmt.Sprintf("design: reading the vendored icons: %v", err))
		}
		iconsByName = make(map[string]*ui.SVG, len(entries))
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".svg")
			if !ok {
				continue
			}
			data, err := iconFS.ReadFile(path.Join("icons", e.Name()))
			if err != nil {
				panic(fmt.Sprintf("design: reading icon %q: %v", name, err))
			}
			iconsByName[name] = ui.MustParseSVG(data)
		}
	})
	return iconsByName
}
