package notes

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// AssetRef is a note that carries a file: its identity and its title.
type AssetRef struct {
	ID    core.ID
	Title string
}

// AssetRow is one file in the library's assets directory: what it is, how large,
// and the notes that carry it. A file nothing references is still a row — it is
// in the library, and the page says so rather than hiding it.
type AssetRow struct {
	// Name is the file's name inside assets/.
	Name string
	// Ext is its extension without the dot, lower-cased.
	Ext string
	// Bytes is its size.
	Bytes int64
	// Notes are the notes that reference it, in title order.
	Notes []AssetRef
}

// AssetsView draws the assets page (S08): every file the library holds, with the
// notes that carry it. Choosing a row opens the first note that references the
// file.
func AssetsView(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	var assets []AssetRow
	if d.AssetRows != nil {
		assets = d.AssetRows()
	}

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.BrowseWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
				ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, msgs.WriteAssetsTitle()).FontSize(tk.TypeH1).Bold()
					ui.Text(c, msgs.WriteAssetsHint()).
						FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
				})
				if d.ImportAssets != nil {
					ui.Button(c, msgs.WriteAssetsImport()).MinHeight(tk.Control).
						OnClick(func() { d.ImportAssets(c) })
				}
			})
			if len(assets) == 0 {
				kit.Empty(c, "file-text", msgs.WriteAssetsEmpty(), func() {
					if d.ImportAssets == nil {
						return
					}
					ui.Button(c, msgs.WriteAssetsImport()).MinHeight(tk.Control).
						OnClick(func() { d.ImportAssets(c) })
				})
				return
			}
			// A row is a way into the note that carries the file: choosing one
			// opens the first, which is the note the column names.
			s.assetList.Key = func(row int) any { return assets[row].Name }
			s.assetList.Label = func(row int) string { return assets[row].Name }
			chosen := -1
			s.assetList.Selected = &chosen
			// A table's columns are sized in controls: what a name needs, what
			// a size or an extension needs, and room for a note's title beside
			// its count.
			columns := []ui.TableColumn{
				{Title: msgs.WriteAssetsKind(), MinWidth: tk.Control * 2, MaxWidth: tk.Control * 3},
				{Title: msgs.WriteAssetsFile()},
				{Title: msgs.WriteAssetsSize(), Width: tk.Control * 3, Align: ui.End},
				{Title: msgs.WriteAssetsFormat(), Width: tk.Control * 3},
				{Title: msgs.WriteAssetsRefs(), MinWidth: tk.Control * 6},
			}
			table := ui.Table(c, &s.assetList, columns, len(assets), func(row, col int) {
				assetCell(c, tk, msgs, assets[row], col)
			}).Grow(1).Label(msgs.WriteAssetsTitle())
			if table.Changed() && chosen >= 0 && chosen < len(assets) {
				if notes := assets[chosen].Notes; len(notes) > 0 && d.OpenNote != nil {
					d.OpenNote(notes[0].ID)
				}
			}
		})
	})
}

// assetCell is one cell of the assets table: the kind of file, its name, its
// size, its extension, and who carries it.
func assetCell(c *ui.Context, tk design.Tokens, msgs locales.Messages, asset AssetRow, col int) {
	switch col {
	case 0:
		ui.Badge(c, assetKind(msgs, asset.Ext))
	case 1:
		ui.Text(c, asset.Name).FontSize(tk.TypeMono).SingleLine()
	case 2:
		ui.Text(c, sizeOf(asset.Bytes)).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
	case 3:
		ui.Text(c, asset.Ext).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
	case 4:
		ui.Text(c, refsOf(msgs, asset)).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted).SingleLine()
	}
}

// assetKind names what a file is by its extension: the badge a row shows.
func assetKind(msgs locales.Messages, ext string) string {
	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "heic", "avif":
		return msgs.WriteKindImage()
	case "mp3", "m4a", "wav", "ogg", "flac", "aac", "opus":
		return msgs.WriteKindSound()
	}
	return msgs.WriteKindOther()
}

// refsOf is who carries a file: how many notes, then the first of them so that
// the row says where the picture is used. A file nothing references says so.
func refsOf(msgs locales.Messages, asset AssetRow) string {
	if len(asset.Notes) == 0 {
		return msgs.WriteAssetUnreferenced()
	}
	return msgs.WriteAssetRefs(len(asset.Notes)) + " · " + asset.Notes[0].Title
}

// sizeOf is a file's size in the unit a reader uses: bytes below a kilobyte,
// then kilobytes and megabytes with one decimal.
func sizeOf(bytes int64) string {
	switch {
	case bytes < 1024:
		return strconv.FormatInt(bytes, 10) + " B"
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/(1024))
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}
