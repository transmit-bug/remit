package importanki

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Anki revlog kinds. Anki 2.1 writes 0..5; the legacy collection stores the
// kind in revlog.type.
const (
	revlogLearning    = 0
	revlogReview      = 1
	revlogRelearning  = 2
	revlogFiltered    = 3
	revlogManual      = 4
	revlogRescheduled = 5
)

const (
	maxWarnings  = 100
	maxTitleRune = 80
)

// Report is what one import did. Counts are what was written, so a second
// import of the same package reports zeros; media counts references in the
// notes that were just written, since the library has no media directory.
type Report struct {
	Notes            int
	Cards            int
	Reviews          int
	SkippedRows      int
	UnreachableMedia int
	Warnings         []string
}

// Import reads an Anki package and writes its notes, cards and review history
// into lib. It is safe to call twice on the same package: imported entities
// carry IDs derived from their Anki identity, so the second call finds them and
// adds nothing. Errors name the row or the zip entry they came from; a package
// that reads cleanly but holds odd data yields warnings instead.
func Import(path string, lib *store.Library) (Report, error) {
	var report Report
	if lib == nil {
		return report, errors.New("importanki: no library")
	}
	pkg, err := openPackage(path)
	if err != nil {
		return report, err
	}
	defer pkg.close()

	col, err := pkg.readCollection()
	if err != nil {
		return report, err
	}

	im := &importer{
		lib:     lib,
		report:  &report,
		media:   map[string]struct{}{},
		skipped: map[string]int{},
		decks:   map[int64]bool{},
	}
	report.Warnings = append(report.Warnings, pkg.warnings...)
	report.Warnings = append(report.Warnings, col.warnings...)
	if err := im.plan(col); err != nil {
		return report, err
	}
	if err := im.write(); err != nil {
		return report, err
	}
	im.finish()
	return report, nil
}

type importer struct {
	lib     *store.Library
	report  *Report
	notes   []*notePlan
	cards   []*cardPlan
	reviews []reviewPlan
	media   map[string]struct{}
	skipped map[string]int
	decks   map[int64]bool
	extra   int // warnings past the cap
}

// notePlan is one Anki note and the Remit note it becomes.
type notePlan struct {
	ankiID  int64
	id      core.ID
	guid    string
	title   string
	tags    string
	model   *notetype
	fields  []string // the note's text, cloze markup unwrapped
	cleaned []string // the note's text with cloze markup kept, for masking
	blocks  []core.Block
	fieldAt []int // field index -> first block position, -1 when empty

	existing    *core.Note // the library already has this note
	clozeWarned bool
}

// cardPlan is one Anki card and the Remit card it becomes.
type cardPlan struct {
	ankiID  int64
	id      core.ID
	note    *notePlan
	field   int
	blockAt int
	prompt  string
	answer  string
	status  core.Status

	skip     string // non-empty: the card cannot be created
	existing bool   // the library already has this card
	reviewed bool   // ... and its history is already in the log
}

type reviewPlan struct {
	rowID  int64
	card   *cardPlan
	at     time.Time
	rating schedule.Rating
}

// plan reads every row before anything is written, so a package that does not
// make sense fails without leaving half a library behind.
func (im *importer) plan(col *collection) error {
	models, err := col.models()
	if err != nil {
		return err
	}
	decks, named := col.deckNames()
	if !named {
		im.warn("decks: col.decks is not the expected JSON; cards are keyed by deck number")
	}

	notes := make(map[int64]*notePlan, len(col.notes))
	noteIDs := make(map[core.ID]int64, len(col.notes))
	for _, row := range col.notes {
		np, err := im.planNote(row, models)
		if err != nil {
			return err
		}
		if previous, dup := noteIDs[np.id]; dup {
			return fmt.Errorf("importanki: notes row %d: same Remit identity as notes row %d", row.id, previous)
		}
		noteIDs[np.id] = row.id
		notes[row.id] = np
		im.notes = append(im.notes, np)
	}

	cards := make(map[int64]*cardPlan, len(col.cards))
	ids := make(map[core.ID]int64, len(col.cards))
	for _, row := range col.cards {
		np, ok := notes[row.note]
		if !ok {
			return fmt.Errorf("importanki: cards row %d: no note row %d", row.id, row.note)
		}
		cp := im.planCard(row, np, decks)
		if previous, dup := ids[cp.id]; dup {
			return fmt.Errorf("importanki: cards row %d: same Remit identity as cards row %d", row.id, previous)
		}
		ids[cp.id] = row.id
		cards[row.id] = cp
		im.cards = append(im.cards, cp)
	}

	for _, row := range col.revlog {
		cp, ok := cards[row.card]
		if !ok {
			return fmt.Errorf("importanki: revlog row %d: no card row %d", row.id, row.card)
		}
		im.planReview(row, cp)
	}
	return nil
}

func (im *importer) planNote(row dbNote, models map[int64]*notetype) (*notePlan, error) {
	model, ok := models[row.mid]
	if !ok {
		return nil, fmt.Errorf("importanki: notes row %d: unknown notetype %d", row.id, row.mid)
	}
	if len(model.fields) == 0 {
		return nil, fmt.Errorf("importanki: notes row %d: %s has no fields", row.id, model.label())
	}
	if len(row.fields) > len(model.fields) {
		for _, extra := range row.fields[len(model.fields):] {
			if strings.TrimSpace(extra) != "" {
				im.warn("notes row %d: more fields than %s has; the extra text is not imported", row.id, model.label())
				break
			}
		}
	}

	guid := strings.TrimSpace(row.guid)
	key := "guid:" + guid
	if guid == "" {
		key = "note:" + strconv.FormatInt(row.id, 10)
		im.warn("notes row %d: no GUID; its Anki id is the stable key", row.id)
	}
	np := &notePlan{
		ankiID:  row.id,
		id:      stableID("note", key),
		guid:    guid,
		tags:    strings.TrimSpace(row.tags),
		model:   model,
		fields:  make([]string, len(model.fields)),
		cleaned: make([]string, len(model.fields)),
		fieldAt: make([]int, len(model.fields)),
	}
	if existing, ok := im.lib.Note(np.id); ok {
		np.existing = &existing
	}

	var hints, nested int
	for i := range model.fields {
		raw := ""
		if i < len(row.fields) {
			raw = row.fields[i]
		}
		var media []string
		np.cleaned[i] = normalizeLines(plainText(raw, &media))
		var stats clozeStats
		np.fields[i], stats = clozeText(np.cleaned[i], noClozeMask)
		hints += stats.hints
		nested += stats.nested
		if np.existing == nil {
			for _, name := range media {
				im.media[name] = struct{}{}
			}
		}
	}
	np.title = noteTitle(np.fields, guid, row.id)
	if hints > 0 {
		im.warn("note %q: %d cloze hint(s) are flattened away; a Remit card has two plain sides and no hint line", np.title, hints)
	}
	if nested > 0 {
		im.warn("note %q: %d nested cloze deletion(s) are flattened into their enclosing text", np.title, nested)
	}

	if np.existing != nil {
		np.fieldAt = mapFieldBlocks(*np.existing, np.fields)
		return np, nil
	}
	for i, field := range np.fields {
		np.fieldAt[i] = -1
		lines := splitLines(field)
		if len(lines) == 0 {
			continue
		}
		np.fieldAt[i] = len(np.blocks)
		for _, line := range lines {
			np.blocks = append(np.blocks, core.Block{Text: line})
		}
	}
	return np, nil
}

func (im *importer) planCard(row dbCard, np *notePlan, decks map[int64]string) *cardPlan {
	cp := &cardPlan{
		ankiID: row.id,
		note:   np,
		status: core.StatusNew,
	}
	cp.id = stableID("card", cardKey(np, row))
	if _, ok := im.lib.Card(cp.id); ok {
		cp.existing = true
		if state, ok := im.lib.CardState(cp.id); ok && state.Reviews > 0 {
			cp.reviewed = true
		}
	}

	tmpl, ok := np.model.templateFor(row.ord)
	if !ok {
		if len(np.model.templates) == 0 {
			cp.skip = "notetype has no templates"
			im.warn("cards row %d: %s has no template; card skipped", row.id, np.model.label())
			return cp
		}
		tmpl = np.model.templates[0]
		im.warn("cards row %d: no template ordinal %d; using %q", row.id, row.ord, tmpl.name)
	}

	field := -1
	if name := frontFieldName(tmpl.front); name != "" {
		field = np.model.fieldIndex(name)
		if field < 0 {
			im.warn("cards row %d: front template names field %q, which %s does not have", row.id, name, np.model.label())
		}
	}
	if field < 0 {
		field = 0
		im.warn("cards row %d: using the first field %q as the front", row.id, np.model.fields[0])
	}
	cp.field = field

	deck := im.deckName(decks, row.deck)
	front := np.fields[field]
	cp.blockAt = np.fieldAt[field]
	switch {
	case cp.blockAt < 0 && np.existing != nil && len(splitLines(front)) > 0:
		cp.skip = "field not in the imported note"
		im.warn("cards row %d: front field %q is not in the note already imported; card skipped", row.id, np.model.fields[field])
		return cp
	case cp.blockAt < 0:
		cp.skip = "empty front field"
		im.warn("cards row %d: front field %q is empty; card skipped", row.id, np.model.fields[field])
		return cp
	}

	if np.model.kind == notetypeCloze {
		masked, stats := clozeText(np.cleaned[field], row.ord)
		if !stats.found {
			im.warn("cards row %d: cloze c%d has no deletion in field %q", row.id, row.ord, np.model.fields[field])
		}
		if stats.masked > 1 {
			im.warn("cards row %d: cloze c%d asks %d deletions; the prompt hides them all", row.id, row.ord, stats.masked)
		}
		cp.prompt = masked
		cp.answer = np.fields[field]
		if !np.clozeWarned {
			np.clozeWarned = true
			im.warn("note %q: cloze cards read field %q for both sides; the front masks the card's deletion as [...] and the back shows the whole field", np.title, np.model.fields[field])
		}
	} else {
		if blockText(np, cp.blockAt) != front || strings.Contains(front, core.Separator) {
			// A field can span blocks, and Remit would split a "::" in it.
			// Overriding the prompt keeps the whole field on the prompt side.
			cp.prompt = front
		}
		answer := normalizeLines(plainText(renderTemplate(tmpl.back, np.values(deck, tmpl)), nil))
		if answer != "" {
			cp.answer = answer
		}
	}

	if row.kind < 0 || row.queue < 0 {
		cp.status = core.StatusSuspended
	}
	return cp
}

func (im *importer) planReview(row dbRevlog, cp *cardPlan) {
	if cp.reviewed {
		return // the card and its whole history are already in the library
	}
	if cp.skip != "" {
		im.report.SkippedRows++
		im.skipped["card skipped"]++
		return
	}
	rating, ok := reviewRating(row.kind, row.ease)
	if !ok {
		im.report.SkippedRows++
		switch row.kind {
		case revlogFiltered, revlogManual, revlogRescheduled:
			im.skipped["not a graded answer"]++
		default:
			im.skipped["unreadable row"]++
			im.warn("revlog row %d: type %d with ease %d is not a review Remit can map; skipped", row.id, row.kind, row.ease)
		}
		return
	}
	im.reviews = append(im.reviews, reviewPlan{
		rowID:  row.id,
		card:   cp,
		at:     time.UnixMilli(row.id).UTC(),
		rating: rating,
	})
}

// write creates what the plan decided, in note, card, review order, so a review
// never names a card that is not there yet.
func (im *importer) write() error {
	saved := make(map[*notePlan]core.Note, len(im.notes))
	for _, np := range im.notes {
		if np.existing != nil {
			saved[np] = *np.existing
			continue
		}
		note, err := im.lib.SaveNote(core.Note{ID: np.id, Title: np.title, Blocks: np.blocks})
		if err != nil {
			return fmt.Errorf("importanki: notes row %d: %w", np.ankiID, err)
		}
		im.report.Notes++
		saved[np] = note
	}

	for _, cp := range im.cards {
		if cp.existing || cp.skip != "" {
			continue
		}
		note := saved[cp.note]
		if cp.blockAt < 0 || cp.blockAt >= len(note.Blocks) {
			return fmt.Errorf("importanki: cards row %d: block %d is not in note %q", cp.ankiID, cp.blockAt, note.Title)
		}
		card := core.Card{
			ID:  cp.id,
			Ref: core.Reference{Block: note.Blocks[cp.blockAt].ID},
			// Anki's own wording is the closest thing to a hand edit: the
			// imported card carries its Front/Back as edited field values
			// rather than following the block's "::" reading of them.
			Type:   core.TypeBasic,
			Status: cp.status,
		}
		if cp.prompt != "" || cp.answer != "" {
			card.Fields = map[string]string{"Front": cp.prompt, "Back": cp.answer}
			card.Edited = true
		}
		if err := im.lib.SaveCard(card); err != nil {
			return fmt.Errorf("importanki: cards row %d: %w", cp.ankiID, err)
		}
		im.report.Cards++
	}

	for _, review := range im.reviews {
		if err := im.lib.Review(review.card.id, review.rating, review.at); err != nil {
			return fmt.Errorf("importanki: revlog row %d: %w", review.rowID, err)
		}
		im.report.Reviews++
	}
	return nil
}

// finish turns what the plan noticed into the report's counts and warnings.
func (im *importer) finish() {
	im.report.UnreachableMedia = len(im.media)
	if n := im.skipped["not a graded answer"]; n > 0 {
		im.warn("revlog: skipped %d rows that are not graded reviews (filtered, manual, rescheduled)", n)
	}
	if n := im.skipped["card skipped"]; n > 0 {
		im.warn("revlog: skipped %d rows for cards that could not be imported", n)
	}
	if n := im.skipped["unreadable row"]; n > 0 {
		im.warn("revlog: skipped %d rows whose type or ease is outside Anki's range", n)
	}
	if n := im.report.UnreachableMedia; n > 0 {
		im.warn("media: %d file name(s) are in the notes but no media directory exists, so no file was copied", n)
	}
	if im.extra > 0 {
		im.report.Warnings = append(im.report.Warnings, fmt.Sprintf("importanki: %d more warnings", im.extra))
	}
}

func (im *importer) warn(format string, args ...any) {
	if len(im.report.Warnings) >= maxWarnings {
		im.extra++
		return
	}
	im.report.Warnings = append(im.report.Warnings, fmt.Sprintf(format, args...))
}

func (im *importer) deckName(decks map[int64]string, id int64) string {
	if name := strings.TrimSpace(decks[id]); name != "" {
		return name
	}
	if !im.decks[id] {
		im.decks[id] = true
		im.warn("deck %d has no name in the collection", id)
	}
	return "deck " + strconv.FormatInt(id, 10)
}

// noteTitle is the note's first line of text, which is what a reader would
// call the note. The Anki GUID is the fallback.
func noteTitle(fields []string, guid string, id int64) string {
	for _, field := range fields {
		for _, line := range splitLines(field) {
			if title := truncateRunes(line, maxTitleRune); title != "" {
				return title
			}
		}
	}
	if guid != "" {
		return "Anki note " + guid
	}
	return "Anki note " + strconv.FormatInt(id, 10)
}

// values is what a back template may refer to while rendering one card.
func (np *notePlan) values(deck string, tmpl template) map[string]string {
	values := make(map[string]string, len(np.fields)+6)
	for i, name := range np.model.fields {
		values[strings.ToLower(name)] = np.fields[i]
	}
	values["deck"] = deck
	values["subdeck"] = deck
	values["tags"] = np.tags
	values["card"] = tmpl.name
	values["type"] = "Basic"
	if np.model.kind == notetypeCloze {
		values["type"] = "Cloze"
	}
	values["frontside"] = ""
	return values
}

// blockText is the text of one block as the note will hold it.
func blockText(np *notePlan, at int) string {
	if np.existing != nil && at >= 0 && at < len(np.existing.Blocks) {
		return np.existing.Blocks[at].Text
	}
	if at >= 0 && at < len(np.blocks) {
		return np.blocks[at].Text
	}
	return ""
}

// mapFieldBlocks finds where each field starts in a note the library already
// holds. Walking forward keeps two fields with the same first line apart.
func mapFieldBlocks(note core.Note, fields []string) []int {
	at := 0
	positions := make([]int, len(fields))
	for i, field := range fields {
		positions[i] = -1
		lines := splitLines(field)
		if len(lines) == 0 {
			continue
		}
		for j := at; j < len(note.Blocks); j++ {
			if note.Blocks[j].Text == lines[0] {
				positions[i] = j
				at = j + 1
				break
			}
		}
	}
	return positions
}

// reviewRating maps a revlog row to the rating Remit records. Learning and
// relearning rows only distinguished "again" from "good" buttons in Anki's
// first scheduler, so they collapse to those two; a review keeps all four.
func reviewRating(kind, ease int) (schedule.Rating, bool) {
	switch kind {
	case revlogLearning, revlogRelearning:
		if ease <= 1 {
			return schedule.Again, true
		}
		return schedule.Good, true
	case revlogReview:
		switch ease {
		case 1:
			return schedule.Again, true
		case 2:
			return schedule.Hard, true
		case 3:
			return schedule.Good, true
		case 4:
			return schedule.Easy, true
		}
	}
	return 0, false
}

// cardKey is a card's Anki identity: its note's GUID plus the template ordinal,
// or the deck and ordinal when the note has no GUID. The deck name is not part
// of it, so moving a card between decks in Anki does not mint a second Remit
// card on re-import.
func cardKey(np *notePlan, row dbCard) string {
	if np.guid != "" {
		return "guid:" + np.guid + "/ord:" + strconv.Itoa(row.ord)
	}
	return "note:" + strconv.FormatInt(np.ankiID, 10) + "/deck:" + strconv.FormatInt(row.deck, 10) + "/ord:" + strconv.Itoa(row.ord)
}

// stableID mints the kind of ID Remit validates (kind plus 128 bits) from an
// Anki identity, so re-import finds the entity it created before. ADR 0003 asks
// for random IDs; a derived one is the price of idempotence without a store
// change, and it is still stable under edits.
func stableID(kind, key string) core.ID {
	sum := sha256.Sum256([]byte("anki\x1f" + kind + "\x1f" + key))
	return core.ID(kind + "_" + hex.EncodeToString(sum[:16]))
}
