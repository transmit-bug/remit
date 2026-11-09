package stats

import (
	"math"
	"time"

	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/store"
)

// MinutesPerExcerpt prices taking one excerpt. It is the time share's own
// estimate: the queue prices a sitting (MinutesPerReading, MinutesPerReview),
// and an excerpt is the work that follows one.
const MinutesPerExcerpt = 2.0

// Production is how much a library holds: the characters of note text, the
// notes that carry cards, the notes that are excerpts of another note, the
// cards, and how many cards one excerpt's worth of reading turned into.
// CardsPerExcerpt is zero when there is no excerpt to divide by.
type Production struct {
	Characters      int
	Notes           int
	Excerpts        int
	Cards           int
	CardsPerExcerpt float64
}

// ProductionOf counts a library's production from the derived index: block text
// and cards in SQL, the excerpt edge from the links table. No note file is
// opened, so the cost does not follow the size of the library.
func ProductionOf(lib *store.Library) Production {
	volume, err := lib.Volume()
	if err != nil {
		return Production{} // an index that cannot be read counts nothing
	}
	out := Production{
		Characters: volume.Characters,
		Notes:      volume.Notes,
		Excerpts:   volume.Excerpts,
		Cards:      volume.Cards,
	}
	if volume.Excerpts > 0 {
		out.CardsPerExcerpt = float64(volume.Cards) / float64(volume.Excerpts)
	}
	return out
}

// TimeShare is how a range's minutes divide between reading, reviewing and
// taking excerpts. The three are one picture: the same live log prices the
// heatmap, the forecast and the tray.
type TimeShare struct {
	Read    int // minutes spent reading notes
	Review  int // minutes spent answering cards
	Excerpt int // minutes spent taking excerpts
}

// TimeShareOf prices [from, to) — a zero bound is open — from one source: the
// live log for the sessions read and the cards answered, so a voided review
// costs nothing, and the index's save stamp for the excerpts taken, because the
// log has no excerpt event.
//
// Reading and reviewing are counted per review and priced by the queue's
// estimate, not measured: the log records that a session happened, not how long
// it took. Each line is rounded once, so the three segments of the bar add up to
// the minutes the range is worth.
func TimeShareOf(lib *store.Library, from, to time.Time) TimeShare {
	var out TimeShare
	events, err := lib.LiveReviews(from, to)
	if err != nil {
		return TimeShare{}
	}
	var read, review int
	for _, ev := range events {
		if !isReview(ev) {
			continue // a queue change is intent, not a sitting
		}
		if ev.IsReading() {
			read++
		} else {
			review++
		}
	}
	out.Read = roundMinutes(float64(read) * queue.MinutesPerReading)
	out.Review = roundMinutes(float64(review) * queue.MinutesPerReview)
	if excerpts, err := lib.ExcerptsBetween(from, to); err == nil {
		out.Excerpt = roundMinutes(float64(excerpts) * MinutesPerExcerpt)
	}
	return out
}

// roundMinutes is the estimate as a whole minute, the unit the time share's bar
// is drawn in.
func roundMinutes(minutes float64) int {
	if minutes <= 0 {
		return 0
	}
	return int(math.Round(minutes))
}
