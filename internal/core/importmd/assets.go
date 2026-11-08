package importmd

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/transmit-bug/remit/internal/core/store"
)

// The assets of a Markdown import or export. A directory that carries an
// assets/ subtree has its files stored in the library (named by content, so
// the same picture costs one file however many files reference it); an export
// writes the assets the notes reference next to the note files, under the
// same name they have in the library, so the directory imports back and the
// references resolve.

// importAssets stores the assets a Markdown directory carries. The subtree is
// taken as it lies — an assets directory of the user's making, not one file
// format — so any file in it is stored, and the relative path it keeps is the
// one the library names it by, not where it was.
func importAssets(root string, lib *store.Library, report *Report) {
	dir := filepath.Join(root, "assets")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := lib.PutAsset(data, strings.TrimPrefix(filepath.Ext(path), ".")); err != nil {
			return err
		}
		report.Assets++
		return nil
	})
	if err != nil {
		warnf(report, "assets: %v", err)
	}
}

// exportAssets writes the assets the notes reference into dir/assets,
// keeping the names they hold in the library, so the directory's references
// resolve and importing it back stores nothing new. Missing assets are a
// warning, not an error: the references stay as written, which the notes
// already say.
func exportAssets(dir string, lib *store.Library, report *Report) {
	refs := assetRefs(lib)
	if len(refs) == 0 {
		return
	}
	out := filepath.Join(dir, "assets")
	if err := os.MkdirAll(out, 0o755); err != nil {
		warnf(report, "assets: %v", err)
		return
	}
	for _, rel := range refs {
		data, ok, err := lib.Asset(rel)
		if err != nil {
			warnf(report, "assets: %v", err)
			continue
		}
		if !ok {
			warnf(report, "assets: %s is referenced but not stored", rel)
			continue
		}
		if err := os.WriteFile(filepath.Join(out, filepath.Base(rel)), data, 0o644); err != nil {
			warnf(report, "assets: %v", err)
			continue
		}
		report.Assets++
	}
}

// exportNoteAssets writes the assets one note's source references into
// dir/assets, so a single-note export resolves its images too.
func exportNoteAssets(dir string, src []byte, lib *store.Library, report *Report) {
	refs := sourceAssetRefs(src)
	for _, rel := range refs {
		data, ok, err := lib.Asset(rel)
		if err != nil {
			warnf(report, "assets: %v", err)
			continue
		}
		if !ok {
			warnf(report, "assets: %s is referenced but not stored", rel)
			continue
		}
		if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
			warnf(report, "assets: %v", err)
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "assets", filepath.Base(rel)), data, 0o644); err != nil {
			warnf(report, "assets: %v", err)
			continue
		}
		report.Assets++
	}
}

// assetRefs is every assets reference the library's notes carry, in name
// order, once each.
func assetRefs(lib *store.Library) []string {
	var refs []string
	for _, note := range lib.Notes() {
		src, ok, err := lib.NoteSource(note.ID)
		if err != nil || !ok {
			continue
		}
		refs = append(refs, sourceAssetRefs(src)...)
	}
	sort.Strings(refs)
	return slicesCompact(refs)
}

// sourceAssetRefs is the assets references one note source carries, once
// each: image destinations inside the text.
func sourceAssetRefs(src []byte) []string {
	var refs []string
	for _, image := range imageRefs(string(src)) {
		if _, ok := assetRel(image); ok {
			refs = append(refs, image)
		}
	}
	sort.Strings(refs)
	return slicesCompact(refs)
}

// assetRel says whether dest is an asset reference the library resolves: the
// relative form a note carries when its picture lives in the assets
// directory. Everything else — a web URL, a data URI, a path elsewhere on
// disk — is not the library's to read.
func assetRel(dest string) (string, bool) {
	dest = strings.TrimSpace(dest)
	if dest == "" || !strings.HasPrefix(dest, "assets/") || strings.Contains(dest, "..") {
		return "", false
	}
	return dest, true
}

// imagesCompact — the sorted refs without duplicates.
func slicesCompact(refs []string) []string {
	out := refs[:0]
	for i, ref := range refs {
		if i == 0 || ref != refs[i-1] {
			out = append(out, ref)
		}
	}
	return out
}

// imageRefs is every image destination a Markdown source names, in the order
// the text carries them. Only the ![…](dest) inline form exists in the
// library's own files: the library renders blocks as the editor writes them.
func imageRefs(src string) []string {
	var refs []string
	for i := 0; i < len(src); {
		start := strings.Index(src[i:], "![")
		if start < 0 {
			break
		}
		start += i
		mid := strings.Index(src[start:], "](")
		if mid < 0 {
			break
		}
		// mid lands on "]", so the destination starts one past it.
		mid += start + 2
		end := strings.Index(src[mid:], ")")
		if end < 0 {
			break
		}
		end += mid
		refs = append(refs, strings.TrimSpace(src[mid:end]))
		i = end + 1
	}
	return refs
}

// copyAssets walks a source directory's assets into an export directory —
// the library is absent when plain files move, so the assets subtree is
// copied as it lies.
func copyAssets(root, dir string, report *Report) {
	in := filepath.Join(root, "assets")
	info, err := os.Stat(in)
	if err != nil || !info.IsDir() {
		return
	}
	out := filepath.Join(dir, "assets")
	if err := os.MkdirAll(out, 0o755); err != nil {
		warnf(report, "assets: %v", err)
		return
	}
	err = filepath.WalkDir(in, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		rel, err := filepath.Rel(in, path)
		if err != nil {
			return err
		}
		dst, err := os.Create(filepath.Join(out, rel))
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, src)
		if cerr := dst.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		report.Assets++
		return nil
	})
	if err != nil {
		warnf(report, "assets: %v", err)
	}
}
