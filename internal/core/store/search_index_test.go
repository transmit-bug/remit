package store

import (
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// TestSearchIsAnsweredByTheFTSIndex is the cost promise written down: a search
// reads the full-text index and never the cards or the blocks table. It asks
// SQLite for the plan of the statements the search really runs, so a rewrite
// that turns the lookup into a scan — the bug this guards — fails here instead
// of on a hundred-thousand-card library.
func TestSearchIsAnsweredByTheFTSIndex(t *testing.T) {
	lib, err := Create(t.TempDir(), Options{Now: time.Now})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()
	note, err := lib.SaveNote(core.Note{Title: "Entropy", Blocks: []core.Block{{Text: "entropy grows"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if _, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}}); err != nil {
		t.Fatalf("card: %v", err)
	}
	index := lib.index

	countSQL, countArgs := searchMatchCountSQL("entropy", "")
	rankSQL, rankArgs := searchMatchRankSQL("entropy", "", 10, 0)
	hydrateArgs := append([]any{`"entropy"`}, int64(1), int64(2))

	statements := []struct {
		name  string
		query string
		args  []any
	}{
		{"the count", countSQL, countArgs},
		{"the ranking", rankSQL, rankArgs},
		{"the page's text", hydrateSQL("", 2), hydrateArgs},
	}
	for _, statement := range statements {
		t.Run(statement.name, func(t *testing.T) {
			plan := queryPlan(t, index, statement.query, statement.args)
			if !strings.Contains(plan, "VIRTUAL TABLE INDEX") {
				t.Errorf("the plan does not read the full-text index:\n%s", plan)
			}
			for _, table := range []string{"cards", "blocks"} {
				if strings.Contains(plan, "SCAN "+table) || strings.Contains(plan, "SEARCH "+table) {
					t.Errorf("the plan walks %s:\n%s", table, plan)
				}
			}
		})
	}
}

// queryPlan is SQLite's answer to how a statement runs.
func queryPlan(t *testing.T, index *index, query string, args []any) string {
	t.Helper()
	rows, err := index.db.Query(`EXPLAIN QUERY PLAN `+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("explain: %v", err)
		}
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain: %v", err)
	}
	return plan.String()
}
