package notes

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/locales"
)

// The note's pictures. An image enters the document by being dragged in as a
// file or pasted from the clipboard, is stored in the library's assets
// directory named by its content (#152), and references itself in the text by
// a relative path — ![](assets/<sha>.<ext>) — so the note stays plain text
// that resolves wherever the library goes.

// pasted is the image the paste shortcut read from the clipboard, or nil.
// The shortcut runs between passes and the document belongs to the pass, so
// the hand-off is one field read at the editor's next build — the same road
// the focus flag walks. The clipboard read is cheap and cannot race a screen
// change: an image for a note that has moved on dies here, uninserted.
var pasted *imageDrop

// imageDrop is one picture waiting for the pass that owns the document.
type imageDrop struct {
	data []byte
	name string
}

// pasteShortcut puts the editor's paste handling on the element the editor
// is built in. A shortcut around the focus claims the key before the focus
// handler runs, so ⌘V with a picture on the clipboard pastes the picture —
// which the editor's own paste would silently drop, it reading text formats
// only — and ⌘V with text runs the editor's rich pipeline by hand, in the
// order and formats its built-in paste reads: its own format first, then
// HTML, RTF, Markdown, plain text. Once the shortcut claims the key the
// editor never sees it; there is no forwarding.
func (s *State) pasteShortcut(e ui.Element, c *ui.Context, d Deps) ui.Element {
	return e.OnShortcut(ui.Cmd, ui.KeyV, func() {
		pasted = nil
		services := c.Services()
		if !hasFormat(services.ClipboardFormats(), transfer.PNG) {
			s.richPaste(c, d)
			return
		}
		data, err := services.ReadClipboardData(transfer.PNG).Read(transfer.PNG)
		if err != nil || len(data) == 0 {
			slog.Warn("remit: paste image", "err", err)
			c.Toast(locales.PasteImageFailed())
			return
		}
		pasted = &imageDrop{data: data, name: clipboardName(services)}
	})
}

// hasFormat says whether the clipboard's formats carry one.
func hasFormat(formats []transfer.Format, want transfer.Format) bool {
	for _, f := range formats {
		if f == want {
			return true
		}
	}
	return false
}

// clipboardName reads the clipboard's plain text as a pasted image's name —
// its alt text — when the text is short enough to be one: a file's name
// copied beside its bytes in Finder. Longer text and newlines are not a
// name.
func clipboardName(services ui.Services) string {
	if !hasFormat(services.ClipboardFormats(), transfer.Text) {
		return ""
	}
	text, err := services.ReadClipboardData(transfer.Text).Read(transfer.Text)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(text))
	if name == "" || len(name) > 80 || strings.ContainsAny(name, "\n\r") {
		return ""
	}
	return name
}

// richPaste is the editor's own paste pipeline, run by hand when the
// shortcut took the key and the clipboard held no picture. Formats in the
// editor's order; the first one present with readable bytes wins. The
// insertion is an edit of the app's, which the editor's Changed does not
// report, so the save is here — a handler's store write, as any menu
// action's.
func (s *State) richPaste(c *ui.Context, d Deps) {
	if s.Doc == nil || s.docNote == "" {
		return
	}
	doc := s.Doc
	services := c.Services()
	formats := services.ClipboardFormats()
	for _, format := range []transfer.Format{richtext.Format, transfer.HTML, richtext.RTF, transfer.Markdown, "text/plain"} {
		if !hasFormat(formats, format) {
			continue
		}
		data, err := services.ReadClipboardData(format).Read(format)
		if err != nil || len(data) == 0 {
			continue
		}
		switch format {
		case richtext.Format:
			var text richtext.Text
			if err := text.UnmarshalJSON(data); err != nil {
				continue
			}
			doc.Insert(text)
		case transfer.HTML:
			doc.Insert(richtext.ParseHTML(string(data)))
		case richtext.RTF:
			doc.Insert(richtext.ParseRTF(data))
		case transfer.Markdown:
			doc.Insert(richtext.ParseMarkdown(string(data)))
		default:
			doc.InsertText(string(data))
		}
		s.saveDocument(c, d)
		return
	}
}

// takeImages turns this pass's pictures — the pasted one and the picture
// files the editor element received — into stored assets and attachments at
// the caret. It runs in the editor's build pass, so the store writes stay on
// the UI thread. Files that are not pictures are a log line; failures are a
// toast. The save afterwards is app-side: an insertion the app makes does
// not trip the editor's Changed, which reports the user's edits.
func (s *State) takeImages(c *ui.Context, d Deps, files []string) {
	msgs := locales.In(c.Locale())
	stored := 0
	if pasted != nil {
		img := pasted
		pasted = nil
		if rel, ok := s.putImage(c, d, img.data, "png", img.name, msgs); ok {
			s.insertAsset(c, d, rel, img.name)
			stored++
		}
	}
	for _, path := range files {
		if s.insertImageFile(c, d, path) {
			stored++
		}
	}
	if stored > 0 {
		s.saveDocument(c, d)
	}
}

// insertImageFile stores one picture file and puts it at the caret. It is the
// one road for a drop, a paste and the slash menu's file picker: the bytes
// become a content-addressed asset, and the note keeps only its relative path.
func (s *State) insertImageFile(c *ui.Context, d Deps, path string) bool {
	msgs := locales.In(c.Locale())
	ext, ok := imageExt(path)
	if !ok {
		slog.Info("remit: file is not a picture", "file", filepath.Base(path))
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("remit: read picture", "path", path, "err", err)
		c.Toast(msgs.DropImageFailed(filepath.Base(path)))
		return false
	}
	rel, ok := s.putImage(c, d, data, ext, filepath.Base(path), msgs)
	if !ok {
		return false
	}
	s.insertAsset(c, d, rel, filepath.Base(path))
	return true
}

// imageExt names the storage extension of a picture file the editor accepts.
func imageExt(path string) (string, bool) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "jpg" {
		ext = "jpeg"
	}
	switch ext {
	case "png", "jpeg", "gif", "webp", "bmp", "svg":
		return ext, true
	}
	return "", false
}

// putImage stores one picture in the library and answers the relative path a
// note references it by.
func (s *State) putImage(c *ui.Context, d Deps, data []byte, ext, name string, msgs locales.Messages) (string, bool) {
	if d.PutAsset == nil {
		return "", false
	}
	rel, err := d.PutAsset(data, ext)
	if err != nil {
		slog.Error("remit: store picture", "err", err)
		c.Toast(msgs.ImageStoreFailed(err.Error()))
		return "", false
	}
	return rel, true
}

// insertAsset puts one stored asset at the caret: an attachment whose image
// is the stored bytes and whose URL is the note's reference — what the
// editor writes out is exactly what the note file keeps. A picture that
// cannot decode still enters as its reference in a name box: the text is
// the truth, the pixels are recoverable.
func (s *State) insertAsset(c *ui.Context, d Deps, rel, name string) {
	var img ui.ImageSource
	if d.GetAsset != nil {
		data, ok := d.GetAsset(rel)
		if ok {
			if decoded, err := decodeAssetImage(rel, data); err == nil {
				img = decoded
			}
		}
	}
	if name == "" {
		name = filepath.Base(rel)
	}
	s.Doc.InsertAttachment(&richtext.Attachment{
		Image: img,
		Name:  name,
		URL:   rel,
	})
}

// decodeAssetImage decodes a library asset as the editor draws it: SVG through
// the vector path, every other format as a bitmap.
func decodeAssetImage(rel string, data []byte) (ui.ImageSource, error) {
	if strings.HasSuffix(strings.ToLower(rel), ".svg") {
		return ui.ParseSVG(data)
	}
	return ui.DecodeBitmap(data)
}
