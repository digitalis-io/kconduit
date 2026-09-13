package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

func rowsOf(values ...[]string) []table.Row {
	rows := make([]table.Row, 0, len(values))
	for _, v := range values {
		rows = append(rows, table.Row(v))
	}
	return rows
}

func firstColumn(rows []table.Row) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row[0])
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterRowsMatchesAnyCell(t *testing.T) {
	rows := rowsOf(
		[]string{"orders", "12"},
		[]string{"payments", "3"},
		[]string{"audit", "12"},
	)

	got := firstColumn(filterRows(rows, "12"))
	if want := []string{"orders", "audit"}; !equal(got, want) {
		t.Errorf("filterRows on a non-first column = %v, want %v", got, want)
	}
}

func TestFilterRowsIsCaseInsensitive(t *testing.T) {
	rows := rowsOf([]string{"Orders", "12"}, []string{"payments", "3"})

	got := firstColumn(filterRows(rows, "orders"))
	if want := []string{"Orders"}; !equal(got, want) {
		t.Errorf("filterRows(%q) = %v, want %v", "orders", got, want)
	}
}

func TestFilterRowsEmptyQueryKeepsEverything(t *testing.T) {
	rows := rowsOf([]string{"orders"}, []string{"payments"})

	if got := len(filterRows(rows, "")); got != 2 {
		t.Errorf("filterRows with an empty query kept %d rows, want 2", got)
	}
}

func TestSortRowsOrdersNumericColumnsByValue(t *testing.T) {
	// The bug this guards: every cell is a string, so a plain string sort puts
	// "10" before "9" and the lag column reads as nonsense.
	rows := rowsOf(
		[]string{"a", "9"},
		[]string{"b", "10"},
		[]string{"c", "100"},
	)

	sortRows(rows, 1, false)
	if got, want := firstColumn(rows), []string{"a", "b", "c"}; !equal(got, want) {
		t.Errorf("ascending numeric sort = %v, want %v", got, want)
	}

	sortRows(rows, 1, true)
	if got, want := firstColumn(rows), []string{"c", "b", "a"}; !equal(got, want) {
		t.Errorf("descending numeric sort = %v, want %v", got, want)
	}
}

func TestSortRowsFallsBackToTextForMixedColumns(t *testing.T) {
	rows := rowsOf(
		[]string{"b", "10"},
		[]string{"a", "n/a"},
	)

	sortRows(rows, 1, false)
	if got, want := firstColumn(rows), []string{"b", "a"}; !equal(got, want) {
		t.Errorf("text sort of a mixed column = %v, want %v", got, want)
	}
}

func TestSortRowsIgnoresOutOfRangeColumn(t *testing.T) {
	rows := rowsOf([]string{"b"}, []string{"a"})

	sortRows(rows, 4, false)
	if got, want := firstColumn(rows), []string{"b", "a"}; !equal(got, want) {
		t.Errorf("sort on a missing column reordered rows: %v, want %v", got, want)
	}
}

func TestMoveSortWalksColumnsAndDirections(t *testing.T) {
	// Each column has two places, ascending then descending, so stepping
	// forward four times over three columns lands on column 2 ascending.
	s := newTableState("filter…")

	steps := []struct {
		wantCol  int
		wantDesc bool
	}{
		{0, true},
		{1, false},
		{1, true},
		{2, false},
	}

	for i, step := range steps {
		s = s.moveSort(1, 3)
		if s.sortCol != step.wantCol || s.sortDesc != step.wantDesc {
			t.Fatalf("step %d = col %d desc %v, want col %d desc %v",
				i+1, s.sortCol, s.sortDesc, step.wantCol, step.wantDesc)
		}
	}
}

func TestMoveSortWrapsAtBothEnds(t *testing.T) {
	s := newTableState("filter…").moveSort(-1, 3)
	if s.sortCol != 2 || !s.sortDesc {
		t.Errorf("stepping back from the first place = col %d desc %v, want col 2 descending",
			s.sortCol, s.sortDesc)
	}

	s = s.moveSort(1, 3)
	if s.sortCol != 0 || s.sortDesc {
		t.Errorf("stepping forward from the last place = col %d desc %v, want col 0 ascending",
			s.sortCol, s.sortDesc)
	}
}

func TestMoveSortWithNoColumns(t *testing.T) {
	s := newTableState("filter…").moveSort(1, 0)
	if s.sortCol != 0 {
		t.Errorf("moveSort with no columns changed the column to %d", s.sortCol)
	}
}

func TestDecorateColumnsDoesNotMutateTheBase(t *testing.T) {
	// Columns are re-decorated on every redraw. If the marker were appended to
	// the shared base, titles would grow an arrow per frame.
	base := []table.Column{{Title: "Lag", Width: 4}}
	s := newTableState("filter…")

	first := decorateColumns(base, s)
	second := decorateColumns(base, s)

	if base[0].Title != "Lag" {
		t.Errorf("base title was mutated to %q", base[0].Title)
	}
	if first[0].Title != second[0].Title {
		t.Errorf("marker accumulated: %q then %q", first[0].Title, second[0].Title)
	}
	if first[0].Title != "Lag ▲" {
		t.Errorf("ascending marker = %q, want %q", first[0].Title, "Lag ▲")
	}

	s.sortDesc = true
	if got := decorateColumns(base, s)[0].Title; got != "Lag ▼" {
		t.Errorf("descending marker = %q, want %q", got, "Lag ▼")
	}
}

func TestApplyViewFiltersThenSorts(t *testing.T) {
	s := newTableState("filter…")
	s.filter.SetValue("topic")
	s.sortCol = 1

	rows := rowsOf(
		[]string{"topic-b", "2"},
		[]string{"other", "1"},
		[]string{"topic-a", "1"},
	)

	got := firstColumn(s.applyView(rows))
	if want := []string{"topic-a", "topic-b"}; !equal(got, want) {
		t.Errorf("applyView = %v, want %v", got, want)
	}
}

func TestStopFilterCanKeepOrDiscardTheQuery(t *testing.T) {
	s := newTableState("filter…")
	s.filter.SetValue("orders")

	if kept := s.stopFilter(false); !kept.active() || kept.filtering {
		t.Errorf("stopFilter(false) = query %q filtering %v, want the query kept and focus released",
			kept.filter.Value(), kept.filtering)
	}
	if cleared := s.stopFilter(true); cleared.active() {
		t.Errorf("stopFilter(true) left the query %q in place", cleared.filter.Value())
	}
}

func TestApplyViewLeavesTheCallersRowsAlone(t *testing.T) {
	// The master lists in model_rows.go are documented as unfiltered and
	// unsorted. applyView sorts in place, so if it returned the caller's own
	// slice, sorting a column would quietly reorder the master list too.
	s := newTableState("filter…")
	s.sortCol = 1

	rows := rowsOf(
		[]string{"b", "2"},
		[]string{"a", "1"},
	)
	before := firstColumn(rows)

	view := s.applyView(rows)

	if got := firstColumn(rows); !equal(got, before) {
		t.Errorf("applyView reordered the caller's rows: %v, want %v", got, before)
	}
	if got, want := firstColumn(view), []string{"a", "b"}; !equal(got, want) {
		t.Errorf("returned view = %v, want %v", got, want)
	}
}

func TestFilterRowsWithNoQueryReturnsACopy(t *testing.T) {
	rows := rowsOf([]string{"a"}, []string{"b"})
	out := filterRows(rows, "")

	out[0] = table.Row{"mutated"}
	if rows[0][0] != "a" {
		t.Errorf("writing to the filtered slice changed the caller's rows: %q", rows[0][0])
	}
}
