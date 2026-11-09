// Package doctor checks a library for the damage a plaintext library can take
// from a hand edit: a note renamed out of the shape an open reads, a block ID
// duplicated, a card whose type is gone, a log line an editor broke, an image a
// note names but the library does not hold.
//
// It reads the files itself and never opens the library. Open is not a read:
// it rebuilds a stale index, seeds the built-in types a library is missing and
// makes the directories the layout expects, so a check through it would report
// a library it had already changed — and a check must write nothing. What the
// doctor does borrow is the file format: store's decoders, the index's own
// status query and its rebuild, so a card, a type, a log line and the index are
// read by the one implementation that writes them (ADR 0003, ADR 0008).
//
// Repair is deliberately narrow. The sources of truth — notes, cards, types,
// assets, remit.json and reviews.jsonl — are never touched, because there is no
// way to rewrite one of them without inventing what the user meant. Only what
// an open derives again, and what a write abandoned, is repaired.
package doctor

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// The severities a finding carries; the order they are declared in is the order
// of how much they matter.
const (
	// SeverityError marks damage that changes what the library holds or does:
	// an unreadable card, a block ID in two places, a log line that stops the
	// replay.
	SeverityError = "error"
	// SeverityWarning marks damage the library survives: a stale derived index,
	// a marker that names nothing, a void that cancels nothing.
	SeverityWarning = "warning"
	// SeverityInfo marks a legitimate state or housekeeping: a card whose block
	// is gone keeps its schedule, an asset no note references, an index an open
	// will build.
	SeverityInfo = "info"
)

// The codes a finding carries. They are the checker's stable vocabulary: a
// script greps for one, and a code never changes meaning.
const (
	CodeConfigMissing = "config.missing" // no remit.json
	CodeConfigFormat  = "config.format"  // remit.json is not this build's format

	CodeIndexMissing = "index.missing" // the derived index is absent
	CodeIndexSchema  = "index.schema"  // the derived index is another build's, or unreadable
	CodeIndexTemp    = "index.temp"    // a temporary file a write left in .remit/

	CodeNoteMisnamed   = "note.misnamed"   // the name is not <slug>-<id>.md
	CodeNoteUnreadable = "note.unreadable" // the file cannot be read
	CodeNoteMarker     = "note.marker"     // a marker-shaped token that is not a block ID
	CodeBlockDuplicate = "block.duplicate" // one block ID in more than one place

	CodeCardMisnamed   = "card.misnamed"   // the name is not <card-id>.json
	CodeCardUnreadable = "card.unreadable" // the file cannot be read
	CodeCardInvalid    = "card.invalid"    // the file is not a card the store reads
	CodeCardReference  = "card.reference"  // the card names no block ID
	CodeCardType       = "card.type"       // the card's type is not in types/
	CodeCardLost       = "card.lost"       // the block is gone and no snapshot was kept

	CodeTypeInvalid  = "type.invalid"  // the file is not a type the store reads
	CodeTypeMisnamed = "type.misnamed" // the file's name and the ID it declares disagree

	CodeReviewMalformed = "review.malformed" // a line that is not a log event
	CodeReviewTruncated = "review.truncated" // a last line without a line ending
	CodeReviewSeq       = "review.seq"       // a sequence that does not count up
	CodeReviewVoid      = "review.void"      // a void that cancels no review

	CodeAssetMissing = "asset.missing" // a note names an asset the library does not hold
	CodeAssetOrphan  = "asset.orphan"  // an asset no note names
)

// Finding is one defect, or one legitimate state worth knowing.
type Finding struct {
	// Code is what kind of thing is wrong, from the Code* constants.
	Code string `json:"code"`
	// Severity is SeverityError, SeverityWarning or SeverityInfo.
	Severity string `json:"severity"`
	// Path is the file the finding is about, relative to the library and
	// slash-separated, so a report reads the same on every platform.
	Path string `json:"path"`
	// Detail is what the check saw, in the words the file format uses.
	Detail string `json:"detail"`
	// Fixable says Fix repairs it without a source file changing.
	Fixable bool `json:"fixable"`
}

// Report is what a check found and what it read. Findings are ordered by path,
// then code, then detail, so two runs over the same library print the same
// report.
type Report struct {
	Findings []Finding `json:"findings"`
	// Notes and Cards count the files an open would read: the ones whose names
	// carry the library's identity. Reviews counts the log's events.
	Notes   int `json:"notes"`
	Cards   int `json:"cards"`
	Reviews int `json:"reviews"`
}

// Clean reports whether the library needs no attention.
func (r Report) Clean() bool { return len(r.Findings) == 0 }

// Check reads the library at dir and reports what it finds. It writes nothing.
// An error means the library could not be inspected at all — a directory that
// is not there, a notes directory that cannot be listed — not a defect it found.
func Check(dir string) (Report, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return Report{Findings: []Finding{}}, fmt.Errorf("doctor: %w", err)
	}
	if !info.IsDir() {
		return Report{Findings: []Finding{}}, fmt.Errorf("doctor: %s is not a directory", dir)
	}

	c := &check{
		dir:    dir,
		report: Report{Findings: []Finding{}},
		blocks: map[core.ID]string{},
		types:  map[core.ID]bool{},
		assets: map[string]bool{},
		refs:   map[string]map[string]bool{},
	}
	for _, step := range []func() error{
		c.checkConfig,
		c.checkTypes,
		c.checkNotes,
		c.checkCards,
		c.checkLog,
		c.checkAssets,
		c.checkIndex,
	} {
		if err := step(); err != nil {
			return Report{Findings: []Finding{}}, err
		}
	}
	sort.SliceStable(c.report.Findings, func(i, j int) bool {
		a, b := c.report.Findings[i], c.report.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Detail < b.Detail
	})
	return c.report, nil
}

// Fix repairs what a library can be repaired of without touching its sources:
// a missing or stale derived index is built again, and temporary files a write
// left inside .remit/ are removed. Notes, cards, types, assets, remit.json and
// reviews.jsonl are never rewritten, so the report returned is the library as
// it stands afterwards — what is left in it needs a person.
//
// An error means the check could not run or the repair could not finish; the
// report then still names what the check found.
func Fix(dir string) (Report, error) {
	before, err := Check(dir)
	if err != nil {
		return Report{Findings: []Finding{}}, err
	}
	if !repairable(before.Findings) {
		return before, nil
	}
	if needsRebuild(before.Findings) {
		// Rebuilding throws .remit/ away, which is where a stale temporary file
		// lies, so the rebuild is the whole repair.
		if err := store.RebuildIndex(dir, store.Options{}); err != nil {
			return before, fmt.Errorf("doctor: rebuild the index: %w", err)
		}
		return Check(dir)
	}
	if err := removeTemps(dir); err != nil {
		return before, fmt.Errorf("doctor: remove temporary files: %w", err)
	}
	return Check(dir)
}

// check is one pass over one library: the findings so far, and what a later
// check needs to know about an earlier one.
type check struct {
	dir    string
	report Report
	// blocks is where each block ID was first seen, so a second occurrence is
	// named against the first.
	blocks map[core.ID]string
	// types is every type ID the library holds.
	types map[core.ID]bool
	// assets is every asset file the library holds, by file name.
	assets map[string]bool
	// refs is every asset a note names, by file name, against the notes that
	// name it.
	refs map[string]map[string]bool
	// notesFailed counts the note files that could not be read: their assets are
	// unknown, so no asset is called orphaned while one exists.
	notesFailed int
}

func (c *check) add(code, severity, file, detail string, fixable bool) {
	c.report.Findings = append(c.report.Findings, Finding{
		Code: code, Severity: severity, Path: file, Detail: detail, Fixable: fixable,
	})
}

// repairable says whether Fix has anything to do.
func repairable(findings []Finding) bool {
	for _, f := range findings {
		if f.Fixable {
			return true
		}
	}
	return false
}

// needsRebuild says whether the derived index has to be built again.
func needsRebuild(findings []Finding) bool {
	for _, f := range findings {
		if f.Fixable && (f.Code == CodeIndexMissing || f.Code == CodeIndexSchema) {
			return true
		}
	}
	return false
}

// removeTemps removes the temporary files a write left inside .remit/. The
// index itself, its write-ahead log and its journal are the store's, not stale
// noise, so they stay.
func removeTemps(dir string) error {
	indexDir := filepath.Join(dir, indexDirName)
	entries, err := os.ReadDir(indexDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), tempPrefix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(indexDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

const (
	// indexDirName is the library's directory of derived state.
	indexDirName = ".remit"
	// indexName is the derived index's file name, as the store writes it.
	indexName = "index.sqlite"
	// tempPrefix is how os.CreateTemp names the file an atomic write stages
	// through; a crash between the create and the rename leaves one behind.
	tempPrefix = ".tmp-"
	// assetPrefix is how a note names an asset: a path relative to the library.
	assetPrefix = "assets/"
)

// checkConfig reads remit.json: a library's format version is the one thing an
// open cannot infer.
func (c *check) checkConfig() error {
	data, err := os.ReadFile(filepath.Join(c.dir, "remit.json"))
	if errors.Is(err, fs.ErrNotExist) {
		c.add(CodeConfigMissing, SeverityError, "remit.json", "the library has no remit.json", false)
		return nil
	}
	if err != nil {
		return fmt.Errorf("doctor: %w", err)
	}
	var config struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		c.add(CodeConfigFormat, SeverityError, "remit.json", "not JSON: "+err.Error(), false)
		return nil
	}
	if config.Format != store.FormatVersion {
		c.add(CodeConfigFormat, SeverityError, "remit.json",
			fmt.Sprintf("format %q, this build reads %q", config.Format, store.FormatVersion), false)
	}
	return nil
}

// checkTypes reads types/: a card cannot render without the type its file
// declares, and a file whose name and declared ID disagree is one an open reads
// as the ID it declares, not as the name it was saved under.
func (c *check) checkTypes() error {
	entries, err := os.ReadDir(filepath.Join(c.dir, "types"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue // the filesystem's business, not the library's
		}
		rel := path.Join("types", name)
		if entry.IsDir() {
			c.add(CodeTypeInvalid, SeverityError, rel, "a directory where a type file belongs", false)
			continue
		}
		if filepath.Ext(name) != ".json" {
			c.add(CodeTypeInvalid, SeverityError, rel, "a type file ends .json, so an open never reads this one", false)
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.dir, "types", name))
		if err != nil {
			c.add(CodeTypeInvalid, SeverityError, rel, err.Error(), false)
			continue
		}
		typ, err := store.DecodeType(name, data)
		if err != nil {
			c.add(CodeTypeInvalid, SeverityError, rel, err.Error(), false)
			continue
		}
		c.types[typ.ID] = true
		if declared, file := string(typ.ID), strings.TrimSuffix(name, ".json"); declared != file {
			c.add(CodeTypeMisnamed, SeverityError, rel,
				fmt.Sprintf("the file is %s.json but declares id %q, so an open reads it as that type", file, declared), false)
		}
	}
	return nil
}

// checkNotes reads notes/: the name is the identity an open reads, the markers
// in the text are the identity a card follows, and the text is where an asset
// is named.
func (c *check) checkNotes() error {
	entries, err := os.ReadDir(filepath.Join(c.dir, "notes"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		rel := path.Join("notes", name)
		if entry.IsDir() {
			c.add(CodeNoteMisnamed, SeverityError, rel, "a directory where a note file belongs", false)
			continue
		}
		if _, _, ok := store.SplitNoteName(name); !ok {
			c.add(CodeNoteMisnamed, SeverityError, rel, "the name is not <slug>-<id>.md, so an open skips the note", false)
			continue
		}
		c.report.Notes++
		src, err := os.ReadFile(filepath.Join(c.dir, "notes", name))
		if err != nil {
			c.notesFailed++
			c.add(CodeNoteUnreadable, SeverityError, rel, err.Error(), false)
			continue
		}
		c.checkNoteSource(rel, src)
	}
	return nil
}

// checkNoteSource reads one note's text: its markers, and the assets it names.
func (c *check) checkNoteSource(rel string, src []byte) {
	_, blocks := store.ParseNoteSource(src)
	for _, block := range blocks {
		if block.ID == "" {
			continue
		}
		first, seen := c.blocks[block.ID]
		if !seen {
			c.blocks[block.ID] = rel
			continue
		}
		detail := fmt.Sprintf("%s is also in %s, so one of them lost its history", block.ID, first)
		if first == rel {
			detail = fmt.Sprintf("%s is in this note twice, so one of them lost its history", block.ID)
		}
		c.add(CodeBlockDuplicate, SeverityError, rel, detail, false)
	}
	for _, line := range brokenMarkers(src) {
		c.add(CodeNoteMarker, SeverityWarning, rel, line, false)
	}
	for _, ref := range assetPaths(string(src)) {
		name := strings.TrimPrefix(ref, assetPrefix)
		if c.refs[name] == nil {
			c.refs[name] = map[string]bool{}
		}
		c.refs[name][rel] = true
	}
}

// checkCards reads cards/: every file the store would read, and for each what
// its block and its type say about whether it can render.
func (c *check) checkCards() error {
	entries, err := os.ReadDir(filepath.Join(c.dir, "cards"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		rel := path.Join("cards", name)
		if entry.IsDir() {
			c.add(CodeCardMisnamed, SeverityError, rel, "a directory where a card file belongs", false)
			continue
		}
		id, ok := store.SplitCardName(name)
		if !ok {
			c.add(CodeCardMisnamed, SeverityError, rel, "the name is not <card-id>.json, so an open skips the card", false)
			continue
		}
		c.report.Cards++
		data, err := os.ReadFile(filepath.Join(c.dir, "cards", name))
		if err != nil {
			c.add(CodeCardUnreadable, SeverityError, rel, err.Error(), false)
			continue
		}
		card, _, err := store.DecodeCard(id, data)
		if err != nil {
			c.add(CodeCardInvalid, SeverityError, rel, err.Error(), false)
			continue
		}
		c.checkCard(rel, card)
	}
	return nil
}

// checkCard says what one decoded card needs of the library around it.
func (c *check) checkCard(rel string, card core.Card) {
	block := card.Ref.Block
	valid := block.Valid() && block.Kind() == core.BlockKind
	switch {
	case block == "":
		c.add(CodeCardReference, SeverityError, rel, "the card names no block", false)
	case !valid:
		c.add(CodeCardReference, SeverityError, rel, fmt.Sprintf("%q is not a block ID", block), false)
	}
	if !c.types[card.Type] {
		c.add(CodeCardType, SeverityError, rel, fmt.Sprintf("type %s is not in types/", card.Type), false)
	}
	if !valid || card.Ref.Frozen != nil {
		// An invalid reference is already a defect of its own, and a snapshot is
		// the card's own text: neither is the lost state.
		return
	}
	if _, live := c.blocks[block]; !live {
		c.add(CodeCardLost, SeverityInfo, rel,
			fmt.Sprintf("%s is in no note and the card keeps no snapshot: the card keeps its schedule and can be pointed at another block", block), false)
	}
}

// checkLog reads reviews.jsonl the way the store streams it: line by line,
// stopping at a last line a write never finished.
func (c *check) checkLog() error {
	const name = "reviews.jsonl"
	file, err := os.Open(filepath.Join(c.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 64*1024)
	var (
		line    int
		prev    int64
		voids   []int64
		cancels = map[int64]bool{}
	)
	for {
		raw, readErr := reader.ReadBytes('\n')
		if len(raw) > 0 {
			line++
			if raw[len(raw)-1] != '\n' {
				c.add(CodeReviewTruncated, SeverityWarning, name,
					fmt.Sprintf("line %d has no line ending: a write that never finished, so the event is not applied", line), false)
				break
			}
			event, ok, err := store.DecodeLogLine(raw)
			switch {
			case err != nil:
				c.add(CodeReviewMalformed, SeverityError, name, fmt.Sprintf("line %d: %v", line, err), false)
			case ok:
				c.report.Reviews++
				if prev != 0 && event.Seq <= prev {
					c.add(CodeReviewSeq, SeverityError, name, fmt.Sprintf("line %d: seq %d after %d", line, event.Seq, prev), false)
				}
				prev = event.Seq
				if event.IsVoid() {
					voids = append(voids, event.Void)
					continue
				}
				// A void cancels a review, so only a review's sequence is one a
				// void can name: cancelling a queue event or a void cancels
				// nothing.
				if event.Kind == schedule.Reviewed {
					cancels[event.Seq] = true
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return readErr
		}
	}
	c.checkVoids(name, voids, cancels)
	return nil
}

// checkVoids names every void that cancels nothing: an undo whose review is not
// in the log is one the replay ignores.
func (c *check) checkVoids(name string, voids []int64, cancels map[int64]bool) {
	for _, target := range voids {
		if cancels[target] {
			continue
		}
		c.add(CodeReviewVoid, SeverityWarning, name,
			fmt.Sprintf("a void names seq %d, which is not a review in the log, so it cancels nothing", target), false)
	}
}

// checkAssets compares the two directions: an asset a note names must exist,
// and an asset the library holds should be named by a note.
func (c *check) checkAssets() error {
	entries, err := os.ReadDir(filepath.Join(c.dir, assetPrefix))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		c.assets[entry.Name()] = true
	}

	for _, name := range sortedKeys(c.refs) {
		if c.assets[name] {
			continue
		}
		for _, note := range sortedKeys(c.refs[name]) {
			c.add(CodeAssetMissing, SeverityError, note,
				fmt.Sprintf("%s%s is not in the library", assetPrefix, name), false)
		}
	}
	if c.notesFailed > 0 {
		// A note that could not be read may name any of them.
		return nil
	}
	for _, name := range sortedKeys(c.assets) {
		if len(c.refs[name]) > 0 {
			continue
		}
		c.add(CodeAssetOrphan, SeverityInfo, path.Join(assetPrefix, name), "no note names it", false)
	}
	return nil
}

// checkIndex reads .remit/: the derived index, and what a write abandoned there.
func (c *check) checkIndex() error {
	state, err := store.InspectIndex(c.dir)
	if err != nil {
		return err
	}
	switch {
	case !state.Present:
		c.add(CodeIndexMissing, SeverityInfo, path.Join(indexDirName, indexName),
			"the derived index is gone; an open builds it again", true)
	case !state.Current:
		detail := "the index declares no schema, which is an index this build cannot read"
		if state.Schema != "" {
			detail = fmt.Sprintf("the index is schema %q, and this build writes another", state.Schema)
		}
		c.add(CodeIndexSchema, SeverityWarning, path.Join(indexDirName, indexName), detail, true)
	}

	entries, err := os.ReadDir(filepath.Join(c.dir, indexDirName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), tempPrefix) {
			continue
		}
		c.add(CodeIndexTemp, SeverityInfo, path.Join(indexDirName, entry.Name()),
			"a write left a temporary file here", true)
	}
	return nil
}

// brokenMarkers names the lines that end with a marker-shaped token which is
// not a block ID: the block they end carries no identity, so a card that once
// pointed at it is lost.
func brokenMarkers(src []byte) []string {
	var out []string
	for i, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSuffix(line, "\r")
		token, ok := markerShaped(line)
		if !ok {
			continue
		}
		out = append(out, fmt.Sprintf("line %d: %q is not a block ID", i+1, token))
	}
	return out
}

// markerShaped returns the last token of a line when it starts the marker the
// content model defines but is not a block ID. A lone sigil is left alone: it
// is prose or arithmetic, not a marker anyone tried to write.
func markerShaped(line string) (string, bool) {
	trimmed := strings.TrimRight(line, " \t")
	cut := strings.LastIndexAny(trimmed, " \t")
	token := trimmed[cut+1:]
	if !strings.HasPrefix(token, core.MarkerSigil) || token == core.MarkerSigil {
		return "", false
	}
	if id, _, ok := core.SplitBlockMarker(trimmed); ok && id.Kind() == core.BlockKind {
		return "", false
	}
	return token, true
}

// assetPaths is every assets/ path a note's text carries, as a Markdown image
// names one. Only the relative form the library resolves counts: a web URL or a
// path outside assets/ is not the library's to hold.
func assetPaths(src string) []string {
	var out []string
	for at := 0; at < len(src); {
		found := strings.Index(src[at:], "]("+assetPrefix)
		if found < 0 {
			break
		}
		at += found + len("](")
		end := strings.Index(src[at:], ")")
		if end < 0 {
			break
		}
		ref := strings.TrimSpace(src[at : at+end])
		at += end + 1
		if strings.Contains(ref, "..") {
			continue // a note cannot make the library read outside its assets
		}
		out = append(out, ref)
	}
	return out
}

// sortedKeys returns a map's keys in order, so every report reads the same.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
