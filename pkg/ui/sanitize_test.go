package ui

import "testing"

func TestSafeCellStripsEscapeSequences(t *testing.T) {
	// The attack this guards: a consumer sets its client.id to an OSC 52
	// sequence, the id lands in the lag table, and "y" copies it — overwriting
	// the operator's clipboard on terminals that honour OSC 52.
	hostile := "\x1b]52;c;aGVsbG8=\x07consumer-1"

	got := safeCell(hostile)
	for _, r := range got {
		if isControl(r) {
			t.Fatalf("safeCell left control character %q in %q", r, got)
		}
	}
	if want := "�]52;c;aGVsbG8=�consumer-1"; got != want {
		t.Errorf("safeCell = %q, want %q", got, want)
	}
}

func TestSafeCellStripsC1Range(t *testing.T) {
	// 0x9b is CSI in the C1 range, which some terminals decode from its
	// two-byte form and act on.
	if got, want := safeCell("a\u009bb"), "a\ufffdb"; got != want {
		t.Errorf("safeCell = %q, want %q", got, want)
	}
}

func TestSafeCellLeavesPrintableTextAlone(t *testing.T) {
	for _, value := range []string{"orders", "user.events-2024", "ワクワク", "✅ Controller", ""} {
		if got := safeCell(value); got != value {
			t.Errorf("safeCell(%q) = %q, want it unchanged", value, got)
		}
	}
}

func TestSafeRowSanitisesEveryCell(t *testing.T) {
	row := safeRow("ok", "bad\x1bvalue")
	if row[0] != "ok" {
		t.Errorf("clean cell = %q, want %q", row[0], "ok")
	}
	if want := "bad�value"; row[1] != want {
		t.Errorf("hostile cell = %q, want %q", row[1], want)
	}
}
