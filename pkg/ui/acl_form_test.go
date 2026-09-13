package ui

import (
	"strings"
	"testing"

	"github.com/digitalis-io/kconduit/pkg/kafka"
)

func TestDescribeResourceReadsAsAPhrase(t *testing.T) {
	cases := []struct {
		resourceType string
		name         string
		pattern      string
		want         string
	}{
		{"Topic", "orders", "Literal", `topic "orders"`},
		{"Topic", "orders.", "Prefixed", `topics starting with "orders."`},
		{"Group", "consumers", "Any", `group matching "consumers"`},
		{"Topic", "*", "Literal", "every topic"},
		{"Cluster", "", "Literal", "any cluster"},
	}

	for _, c := range cases {
		got := describeResource(c.resourceType, c.name, c.pattern)
		if got != c.want {
			t.Errorf("describeResource(%q, %q, %q) = %q, want %q",
				c.resourceType, c.name, c.pattern, got, c.want)
		}
	}
}

func TestACLSummaryStatesWhetherTheRuleAllowsOrDenies(t *testing.T) {
	// The summary is what the delete dialog asks the operator to confirm, so
	// Allow and Deny must not read alike.
	allow := stripStyles(aclSummary(kafka.ACL{
		Principal:      "User:alice",
		Host:           "*",
		ResourceType:   "Topic",
		ResourceName:   "orders",
		PatternType:    "Literal",
		Operation:      "Read",
		PermissionType: "Allow",
	}))

	if !strings.Contains(allow, "User:alice may read") {
		t.Errorf("allow summary = %q, want it to say the principal may read", allow)
	}
	if !strings.Contains(allow, `topic "orders"`) {
		t.Errorf("allow summary = %q, want it to name the topic", allow)
	}

	deny := stripStyles(aclSummary(kafka.ACL{
		Principal:      "User:bob",
		Host:           "10.0.0.1",
		ResourceType:   "Group",
		ResourceName:   "etl-",
		PatternType:    "Prefixed",
		Operation:      "Describe",
		PermissionType: "Deny",
	}))

	if !strings.Contains(deny, "may not describe") {
		t.Errorf("deny summary = %q, want it to say the principal may not describe", deny)
	}
	if !strings.Contains(deny, "from host 10.0.0.1") {
		t.Errorf("deny summary = %q, want it to name the host", deny)
	}
}

func TestACLSummaryShowsDashesForMissingFields(t *testing.T) {
	got := stripStyles(aclSummary(kafka.ACL{PermissionType: "Allow"}))
	if !strings.HasPrefix(got, "- may") {
		t.Errorf("summary of an empty ACL = %q, want a dash for the missing principal", got)
	}
}

func TestDraftACLSummaryJoinsTheSelectedOperations(t *testing.T) {
	got := stripStyles(draftACLSummary("User:alice", "*", "Topic", "orders",
		"Literal", "Allow", []string{"Read", "Describe"}))

	if !strings.Contains(got, "may read, describe") {
		t.Errorf("draft summary = %q, want both operations listed", got)
	}
}

func TestDraftACLSummaryIsAPromptWhileTheFormIsUntouched(t *testing.T) {
	got := stripStyles(draftACLSummary("", "*", "Topic", "", "Literal", "Allow", nil))
	if !strings.Contains(got, "Fill in the fields below") {
		t.Errorf("summary of an untouched form = %q, want a prompt", got)
	}
}

func TestDraftACLSummarySaysWhenNoOperationIsChosen(t *testing.T) {
	got := stripStyles(draftACLSummary("User:alice", "*", "Topic", "orders",
		"Literal", "Allow", nil))

	if !strings.Contains(got, "no operations chosen yet") {
		t.Errorf("draft summary = %q, want it to say no operations are chosen", got)
	}
	// "may no operations selected" was the old phrasing and is not a sentence.
	if strings.Contains(got, "may no") {
		t.Errorf("draft summary = %q, want no verb phrase when nothing is chosen", got)
	}
}

func TestACLFieldTableListsEveryField(t *testing.T) {
	table := stripStyles(aclFieldTable(kafka.ACL{
		Principal:      "User:alice",
		Host:           "*",
		ResourceType:   "Topic",
		ResourceName:   "orders",
		PatternType:    "Literal",
		Operation:      "Read",
		PermissionType: "Allow",
	}))

	for _, want := range []string{"Principal", "Host", "Resource", "Pattern", "Operation", "Permission", "Topic orders"} {
		if !strings.Contains(table, want) {
			t.Errorf("field table is missing %q:\n%s", want, table)
		}
	}
}

func TestACLFrameWidthStaysWithinBounds(t *testing.T) {
	if got := (aclFrame{width: 0}).boxWidth(); got != 72 {
		t.Errorf("width before the first resize = %d, want the 72 default", got)
	}
	if got := (aclFrame{width: 40}).boxWidth(); got != 40 {
		t.Errorf("narrow terminal = %d, want the 40 floor", got)
	}
	if got := (aclFrame{width: 300}).boxWidth(); got != 84 {
		t.Errorf("wide terminal = %d, want the 84 cap", got)
	}
}

func TestACLFormHeightStaysWithinBounds(t *testing.T) {
	if got := aclFormHeight(10); got != 15 {
		t.Errorf("short terminal = %d, want the 15 floor", got)
	}
	if got := aclFormHeight(200); got != 40 {
		t.Errorf("tall terminal = %d, want the 40 cap", got)
	}
	if got := aclFormHeight(44); got != 30 {
		t.Errorf("ordinary terminal = %d, want 30", got)
	}
}

// stripStyles removes ANSI escape sequences so assertions can be made about the
// text rather than the colours around it.
func stripStyles(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}
