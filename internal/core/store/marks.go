package store

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

// sourceMarks is what the text sources looked like when the index last read
// them. It is stored in the index and compared at open, which is how an open
// knows it has nothing to read. A directory's mark is its modification time: it
// moves when a note or a card file is added, removed or renamed, which is what
// the library itself ever does to them. A file edited in place keeps the
// directory's mark, so a hand edit shows up after Rebuild rather than at the
// next open.
type sourceMarks struct {
	notes int64
	cards int64
	log   fileMark
}

// sourceReads counts the source files a library has read since it opened, so a
// test can prove that opening with a current index reads none of them.
type sourceReads struct {
	notes int
	cards int
	log   int
}

func (l *Library) sourceMarks() (sourceMarks, error) {
	notes, err := dirMark(filepath.Join(l.dir, notesDir))
	if err != nil {
		return sourceMarks{}, err
	}
	cards, err := dirMark(filepath.Join(l.dir, cardsDir))
	if err != nil {
		return sourceMarks{}, err
	}
	log, err := l.logFileMark()
	if err != nil {
		return sourceMarks{}, err
	}
	return sourceMarks{notes: notes, cards: cards, log: log}, nil
}

// dirMark is a directory's modification time, zero when it does not exist.
func dirMark(path string) (int64, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return info.ModTime().UnixNano(), nil
}

// logFileMark is the review log's size and modification time, zero when it does
// not exist yet.
func (l *Library) logFileMark() (fileMark, error) {
	info, err := os.Stat(l.logPath())
	if errors.Is(err, os.ErrNotExist) {
		return fileMark{}, nil
	}
	if err != nil {
		return fileMark{}, err
	}
	return fileMark{size: info.Size(), mtime: info.ModTime().UnixNano()}, nil
}

// putMarks records the sources and the scheduling parameters the index was
// built from. They are written inside the transaction that writes the index, so
// a crash never leaves the index claiming to be newer than it is.
func (l *Library) putMarks(e execer, marks sourceMarks) error {
	if err := putMeta(e, metaSchema, schemaVersion); err != nil {
		return err
	}
	if err := putMeta(e, metaParams, l.digest); err != nil {
		return err
	}
	if err := putMeta(e, metaNotes, formatInt(marks.notes)); err != nil {
		return err
	}
	return putMeta(e, metaCards, formatInt(marks.cards))
}

// putLogMark records how far into the log the index has applied.
func (l *Library) putLogMark(e execer, mark logMark) error {
	return putMeta(e, metaLog, mark.String())
}

// clearLogMark makes the next open replay the whole log, for the case where the
// file changed while it was being read and the offset can no longer be trusted.
func clearLogMark(e execer) error {
	return deleteMeta(e, metaLog)
}

func formatInt(n int64) string { return strconv.FormatInt(n, 10) }

func parseInt(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}
