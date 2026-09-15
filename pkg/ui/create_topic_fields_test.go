package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestValidateTopicNameAcceptsLegalNames(t *testing.T) {
	for _, name := range []string{"orders", "orders.v2", "user_events", "a-b.c_1", "A1"} {
		if got := validateTopicName(name); got != "" {
			t.Errorf("validateTopicName(%q) = %q, want it accepted", name, got)
		}
	}
}

func TestValidateTopicNameAllowsEmptyWhileTyping(t *testing.T) {
	// An unfinished name is not an error to show the user; Create stays
	// disabled through valid() instead.
	if got := validateTopicName(""); got != "" {
		t.Errorf("validateTopicName(\"\") = %q, want no message", got)
	}
}

func TestValidateTopicNameRejectsIllegalCharacters(t *testing.T) {
	for _, name := range []string{"my topic", "orders/v2", "orders:1", "tópico"} {
		if validateTopicName(name) == "" {
			t.Errorf("validateTopicName(%q) accepted an illegal character", name)
		}
	}
}

func TestValidateTopicNameRejectsReservedAndOverlongNames(t *testing.T) {
	if validateTopicName(".") == "" {
		t.Error("validateTopicName(\".\") was accepted, want it rejected")
	}
	if validateTopicName("..") == "" {
		t.Error("validateTopicName(\"..\") was accepted, want it rejected")
	}

	tooLong := strings.Repeat("a", maxTopicNameLength+1)
	if validateTopicName(tooLong) == "" {
		t.Errorf("a %d-character name was accepted, the limit is %d",
			len(tooLong), maxTopicNameLength)
	}
	if got := validateTopicName(strings.Repeat("a", maxTopicNameLength)); got != "" {
		t.Errorf("a name at exactly the limit was rejected: %q", got)
	}
}

func TestValidateCountAcceptsBlankAndPositiveIntegers(t *testing.T) {
	for _, value := range []string{"", "1", "12", " 3 "} {
		if got := validateCount(value, "Partitions", 0); got != "" {
			t.Errorf("validateCount(%q) = %q, want it accepted", value, got)
		}
	}
}

func TestValidateCountRejectsNonNumbersAndZero(t *testing.T) {
	for _, value := range []string{"two", "1.5", "0", "-1"} {
		if validateCount(value, "Partitions", 0) == "" {
			t.Errorf("validateCount(%q) was accepted, want it rejected", value)
		}
	}
}

func TestValidateCountEnforcesTheBrokerCeiling(t *testing.T) {
	if got := validateCount("4", "Replication factor", 3); got == "" {
		t.Error("a replication factor of 4 on 3 brokers was accepted")
	}
	if got := validateCount("3", "Replication factor", 3); got != "" {
		t.Errorf("a replication factor equal to the broker count was rejected: %q", got)
	}
	// A limit of 0 means the broker list has not arrived, so nothing is capped.
	if got := validateCount("99", "Replication factor", 0); got != "" {
		t.Errorf("the ceiling was enforced before the broker count was known: %q", got)
	}
}

// newTestForm builds a form with the given field values, bypassing the network.
func newTestForm(t *testing.T, name, partitions, replication string, brokers int) CreateTopicModel {
	t.Helper()

	m := NewCreateTopicModel(nil, 100, 40)
	m.inputs[topicNameIdx].SetValue(name)
	m.inputs[partitionsIdx].SetValue(partitions)
	m.inputs[replicationIdx].SetValue(replication)
	m.brokerCount = brokers
	return m
}

func TestFormIsInvalidUntilTheNameIsFilledIn(t *testing.T) {
	if newTestForm(t, "", "", "", 3).valid() {
		t.Error("an empty form reports itself valid; Create would be enabled with no name")
	}
	if !newTestForm(t, "orders", "", "", 3).valid() {
		t.Error("a name with both counts left blank should be valid: they default to 1")
	}
}

func TestFormIsInvalidWhileAnyFieldIsWrong(t *testing.T) {
	cases := map[string]CreateTopicModel{
		"illegal name":             newTestForm(t, "my topic", "1", "1", 3),
		"partitions not a number":  newTestForm(t, "orders", "two", "1", 3),
		"replication over brokers": newTestForm(t, "orders", "1", "9", 3),
	}

	for name, form := range cases {
		if form.valid() {
			t.Errorf("%s: form reports itself valid", name)
		}
	}
}

func TestCountOrDefaultFallsBackToOne(t *testing.T) {
	form := newTestForm(t, "orders", "", "nonsense", 3)

	if got := form.countOrDefault(partitionsIdx); got != 1 {
		t.Errorf("blank partitions = %d, want 1", got)
	}
	if got := form.countOrDefault(replicationIdx); got != 1 {
		t.Errorf("unparseable replication = %d, want 1", got)
	}
}

func TestSummarySpellsOutTheDefaults(t *testing.T) {
	// The point of the summary is that the two fields which may be left blank
	// still show the value they will be given.
	form := newTestForm(t, "orders", "", "", 3)
	if got, want := form.summary(), "orders · 1 partition · replication factor 1"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}

	form = newTestForm(t, "orders", "6", "3", 3)
	if got, want := form.summary(), "orders · 6 partitions · replication factor 3"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestSummaryIsEmptyWithoutAName(t *testing.T) {
	if got := newTestForm(t, "", "6", "3", 3).summary(); got != "" {
		t.Errorf("summary = %q, want it empty until the topic is named", got)
	}
}

func TestPluralise(t *testing.T) {
	if got, want := pluralise(1, "partition"), "1 partition"; got != want {
		t.Errorf("pluralise = %q, want %q", got, want)
	}
	if got, want := pluralise(2, "partition"), "2 partitions"; got != want {
		t.Errorf("pluralise = %q, want %q", got, want)
	}
}

func TestCreateButtonIsDimmedUntilTheFormIsValid(t *testing.T) {
	// Tests run without a TTY, where lipgloss picks the Ascii profile and drops
	// every colour — which would make the two renderings identical and the
	// assertion meaningless. Force a colour profile for the duration.
	restore := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(restore)

	incomplete := newTestForm(t, "", "", "", 3)
	complete := newTestForm(t, "orders", "", "", 3)

	if incomplete.renderButtons() == complete.renderButtons() {
		t.Error("Create looks the same whether or not the form can be submitted")
	}
	if !strings.Contains(incomplete.renderButtons(), string(theme.Muted)) {
		t.Error("Create is not dimmed while the form is incomplete")
	}
}
