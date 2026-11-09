// Package importanki imports Anki packages (.apkg) into a Remit library.
//
// An .apkg is a zip holding a SQLite collection, a media map and numbered media
// files. This package reads both collection schemas Anki has written: schema 11
// (and what Anki 2.1 kept writing beside it), in collection.anki2 (legacy 1) or
// collection.anki21 (legacy 2), and the current schema 18 of
// collection.anki21b, which is zstd-compressed. The newest entry a package
// carries is the one read; an older package still opens, and a package with
// several entries is not read twice.
//
// Which schema a collection is in is decided by the tables and columns it has,
// never by the entry name: schema 18 moved note types and decks out of the col
// table's JSON into notetypes, fields, templates and decks. A notetype reads as
// the same shape either way — an ordered field list plus the templates that
// turn a note into two card sides.
//
// # Mapping
//
// One Anki note becomes one Remit note, one Anki card one Remit card, and each
// graded revlog row one review in the library's log.
//
//	Anki                                     Remit
//	notes row (fields joined by 0x1f)        one note via store.SaveNote
//	field text, line by line                 one block per non-empty line
//	HTML tags, entities                      plain text: tags removed, entities
//	                                         decoded, <br> and block tags break
//	                                         lines; <img src> and [sound:x] are
//	                                         kept as [img: x] and [sound: x]
//	card row + front template                one card referencing the first
//	                                         block of the field the front
//	                                         template names first
//	back template (afmt)                     the card's answer override,
//	                                         rendered with the note's fields and
//	                                         stripped to plain text
//	card with no useful front template       the first field, with a warning
//	cloze notetype (type 1)                  one card per cloze ordinal, from the
//	                                         cloze field's block; the prompt
//	                                         masks the card's deletion as [...]
//	                                         and the answer shows the field
//	complex cloze (same-number groups,       flattened: every blank of the
//	  hints, nesting)                        group is masked, a hint is dropped,
//	                                         and a nested deletion keeps its
//	                                         text; the report says each one
//	revlog type 1 (review), ease 1..4        store.Review with Again, Hard, Good,
//	                                         Easy
//	revlog type 0/2 (learning/relearning)    Again when ease <= 1, else Good
//	revlog type 3/4/5 (filtered, manual,     skipped, counted in
//	  rescheduled)                           Report.SkippedRows
//	cards.queue < 0 (suspended, buried)      core.StatusSuspended
//	note GUID, card ordinal                  hashed into the note and card ID;
//	                                         importing the same package twice
//	                                         adds nothing
//
// # What is lost
//
// Only text and grades cross. Left behind: HTML markup and CSS, template
// layout, decks and deck options, tags, note types beyond field names and
// templates, filtered decks, media bytes (ADR 0003 has no media directory, so
// file names stay in the text and are counted in Report.UnreachableMedia), and
// Anki's scheduling state (due dates, intervals, ease, leech and lapse
// counters). A cloze hint is dropped too, because a Remit card has two plain
// sides and no hint line; the report counts it rather than losing it silently.
// Reviews are replayed through Remit's FSRS, so the resulting due dates are
// Remit's, not Anki's.
//
// # Idempotence
//
// Remit IDs are normally random (ADR 0003). Imported entities instead carry an
// ID derived from their Anki identity — the note GUID for a note, the GUID plus
// template ordinal for a card — so a second import of the same package finds
// them and adds nothing. The ID is therefore derived rather than minted, but it
// is stable under edits and re-export, which is what the storage layout asks of
// it. Re-importing does not update a note that was edited in Remit.
package importanki
