package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
)

// Sanitising of strings that come from the cluster.
//
// Topic names, consumer group ids, ACL principals and, worst of all, a
// consumer's client.id are chosen by whoever connects to Kafka, not by the
// operator reading them here. Kafka places no restriction on the bytes in a
// client.id, and joining a group needs no admin rights, so any one of these can
// arrive carrying terminal escape sequences.
//
// That matters twice over. Rendered raw, an escape sequence can rewrite the
// screen or the window title; copied to the clipboard with "y", an OSC 52
// sequence can overwrite the operator's clipboard on the terminals that honour
// it. Every cell that reaches a table or the clipboard therefore goes through
// safeCell first.

// replacement stands in for a control character that was removed.
const replacement = '�'

// safeCell replaces the control characters that let cluster-supplied text
// escape its cell: the C0 range, DEL, and the C1 range a terminal may decode
// from its two-byte form. Printable text, including non-Latin scripts and
// emoji, is left alone.
func safeCell(value string) string {
	if !needsSanitising(value) {
		return value
	}

	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if isControl(r) {
			b.WriteRune(replacement)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// needsSanitising reports whether value contains anything safeCell would
// replace. Almost every string is clean, and this keeps those allocation-free.
func needsSanitising(value string) bool {
	for _, r := range value {
		if isControl(r) {
			return true
		}
	}
	return false
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

// safeRow builds a table row from cluster-supplied values.
func safeRow(values ...string) table.Row {
	row := make(table.Row, len(values))
	for i, v := range values {
		row[i] = safeCell(v)
	}
	return row
}
