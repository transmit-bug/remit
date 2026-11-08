package notes

import "testing"

// A GFM header separator makes a table; a fenced block that shows one is
// still code and must be left to the editor as code.
func TestMarkdownTableRecognizesOnlyRealTables(t *testing.T) {
	table := "| Name | Size |\n| :--- | ---: |\n| alpha | 1 |"
	if got, ok := markdownTable(table); !ok || got != table {
		t.Fatalf("table = (%q, %v)", got, ok)
	}
	for _, text := range []string{
		"```\n| Name | Size |\n| --- | --- |\n```",
		"| Name | Size |\n| not a separator |",
		"just prose | with a pipe",
	} {
		if _, ok := markdownTable(text); ok {
			t.Errorf("%q was taken for a table", text)
		}
	}
}

// Table shape is what the editor's read-only placeholder says: columns by
// header and data rows, not counting the delimiter.
func TestTableShape(t *testing.T) {
	if cols, rows := tableShape("| A | B | C |\n| --- | --- | --- |\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |"); cols != 3 || rows != 3 {
		t.Errorf("shape = %d×%d, want 3×3", cols, rows)
	}
}
