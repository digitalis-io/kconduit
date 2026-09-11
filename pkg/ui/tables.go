package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// tableState is the per-tab view state that sits between the data KConduit
// fetched and the rows the table draws: which text is being filtered for, and
// which column the rows are ordered by.
//
// It deliberately works on the rendered rows rather than on the typed data, so
// one implementation serves the topics, brokers, groups, and ACL tables.
type tableState struct {
	filter    textinput.Model
	filtering bool // the filter input currently has focus
	sortCol   int
	sortDesc  bool
}

func newTableState(placeholder string) tableState {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = "/"
	ti.CharLimit = 120
	return tableState{filter: ti, sortCol: 0}
}

// query is the filter text, lower-cased for matching.
func (s tableState) query() string {
	return strings.ToLower(strings.TrimSpace(s.filter.Value()))
}

// active reports whether a filter is narrowing the rows.
func (s tableState) active() bool { return s.query() != "" }

// startFilter focuses the filter input.
func (s tableState) startFilter() (tableState, tea.Cmd) {
	s.filtering = true
	s.filter.Focus()
	return s, textinput.Blink
}

// stopFilter blurs the input, optionally discarding what was typed.
func (s tableState) stopFilter(clear bool) tableState {
	s.filtering = false
	s.filter.Blur()
	if clear {
		s.filter.SetValue("")
	}
	return s
}

// moveSort steps the ordering one place, forwards or backwards.
//
// Each column occupies two places, ascending then descending, so > walks
// column 0 ascending, column 0 descending, column 1 ascending, and so on, and <
// walks back. Two keys therefore reach every ordering; moving the column and
// reversing it separately would need a third.
func (s tableState) moveSort(delta, columns int) tableState {
	if columns <= 0 {
		return s
	}

	places := columns * 2
	current := s.sortCol * 2
	if s.sortDesc {
		current++
	}

	next := ((current+delta)%places + places) % places
	s.sortCol = next / 2
	s.sortDesc = next%2 == 1
	return s
}

// applyView filters and sorts rows for display. The caller keeps the unfiltered
// data; this returns what the table should show.
func (s tableState) applyView(rows []table.Row) []table.Row {
	out := filterRows(rows, s.query())
	sortRows(out, s.sortCol, s.sortDesc)
	return out
}

// filterRows keeps rows where any cell contains query.
func filterRows(rows []table.Row, query string) []table.Row {
	if query == "" {
		return rows
	}
	out := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), query) {
				out = append(out, row)
				break
			}
		}
	}
	return out
}

// sortRows orders rows in place by column col.
//
// Cells are strings, but most of these columns hold numbers — partition counts,
// lag, broker ids. Comparing those as text puts 10 before 9, so a column whose
// values all parse as integers is compared numerically and everything else
// falls back to a case-insensitive string compare.
func sortRows(rows []table.Row, col int, desc bool) {
	if len(rows) < 2 {
		return
	}

	numeric := true
	for _, row := range rows {
		if col >= len(row) {
			return
		}
		if _, err := strconv.ParseInt(strings.TrimSpace(row[col]), 10, 64); err != nil {
			numeric = false
			break
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i][col], rows[j][col]
		less := false
		if numeric {
			ai, _ := strconv.ParseInt(strings.TrimSpace(a), 10, 64)
			bi, _ := strconv.ParseInt(strings.TrimSpace(b), 10, 64)
			less = ai < bi
		} else {
			less = strings.ToLower(a) < strings.ToLower(b)
		}
		if desc {
			return !less
		}
		return less
	})
}

// decorateColumns returns base with a direction marker on the sorted column.
// The base titles are never mutated, so repeated redraws cannot accumulate
// markers.
func decorateColumns(base []table.Column, s tableState) []table.Column {
	out := make([]table.Column, len(base))
	copy(out, base)
	if s.sortCol >= 0 && s.sortCol < len(out) {
		marker := " ▲"
		if s.sortDesc {
			marker = " ▼"
		}
		out[s.sortCol].Title += marker
	}
	return out
}
