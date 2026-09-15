package ui

import (
	"fmt"
	"strconv"
	"strings"
)

// Field definitions and validation for the create-topic form.
//
// Validation runs on every keystroke rather than on submit. The old form
// accepted anything until Create was pressed and then reported one error at a
// time, so a name with a stray character and a mistyped partition count took
// two round trips to discover. Here each field says what is wrong with it while
// it is being typed, and Create is simply unavailable until nothing is.

// maxTopicNameLength is Kafka's limit on a topic name.
const maxTopicNameLength = 249

// topicField describes one row of the form.
type topicField struct {
	label string
	hint  string // shown under the field while it has focus
}

var topicFields = [3]topicField{
	topicNameIdx:   {label: "Name", hint: "Letters, digits, dot, underscore and hyphen. Up to 249 characters."},
	partitionsIdx:  {label: "Partitions", hint: "The unit of parallelism: a group can have at most one consumer per partition."},
	replicationIdx: {label: "Replication", hint: "Copies of each partition across brokers. 1 means no redundancy."},
}

// validateTopicName reports what is wrong with a topic name, or "" if nothing
// is. An empty name is not an error while typing — it is simply not finished —
// so it returns "" and incomplete() keeps Create disabled instead.
func validateTopicName(name string) string {
	switch {
	case name == "":
		return ""
	case len(name) > maxTopicNameLength:
		return fmt.Sprintf("Too long: %d characters, the limit is %d", len(name), maxTopicNameLength)
	case name == "." || name == "..":
		return "Kafka reserves \".\" and \"..\""
	}

	if bad := firstIllegalRune(name); bad != 0 {
		return fmt.Sprintf("%q is not allowed: use letters, digits, . _ or -", bad)
	}
	return ""
}

// firstIllegalRune returns the first character Kafka will not accept in a topic
// name, or 0 if every character is legal.
func firstIllegalRune(name string) rune {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return r
		}
	}
	return 0
}

// validateCount reports what is wrong with a partition count or replication
// factor. Both are positive integers, and both default to 1 when left blank,
// so an empty value is valid.
func validateCount(value, what string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Sprintf("%s must be a whole number", what)
	}
	if n < 1 {
		return fmt.Sprintf("%s must be at least 1", what)
	}
	if limit > 0 && n > limit {
		return fmt.Sprintf("Only %d brokers: a replication factor above that cannot be satisfied", limit)
	}
	return ""
}

// fieldError returns the validation message for one field, or "" if it is
// valid. brokerCount is 0 until the broker list has been fetched, which
// disables the replication ceiling rather than guessing at it.
func (m CreateTopicModel) fieldError(index int) string {
	switch index {
	case topicNameIdx:
		return validateTopicName(m.inputs[topicNameIdx].Value())
	case partitionsIdx:
		return validateCount(m.inputs[partitionsIdx].Value(), "Partitions", 0)
	case replicationIdx:
		return validateCount(m.inputs[replicationIdx].Value(), "Replication factor", m.brokerCount)
	}
	return ""
}

// valid reports whether the form can be submitted: every field parses, and the
// name — the one field with no default — has been filled in.
func (m CreateTopicModel) valid() bool {
	if strings.TrimSpace(m.inputs[topicNameIdx].Value()) == "" {
		return false
	}
	for i := range m.inputs {
		if m.fieldError(i) != "" {
			return false
		}
	}
	return true
}

// countOrDefault reads a field that defaults to 1 when left blank.
func (m CreateTopicModel) countOrDefault(index int) int {
	value := strings.TrimSpace(m.inputs[index].Value())
	if value == "" {
		return 1
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// summary describes what Create will do, with the defaults filled in, so the
// two fields that may be blank still show the value they will be given.
func (m CreateTopicModel) summary() string {
	name := strings.TrimSpace(m.inputs[topicNameIdx].Value())
	if name == "" {
		return ""
	}

	partitions := m.countOrDefault(partitionsIdx)
	replication := m.countOrDefault(replicationIdx)

	return fmt.Sprintf("%s · %s · replication factor %d",
		name, pluralise(partitions, "partition"), replication)
}

// pluralise renders a count with its noun, adding "s" where English wants one.
func pluralise(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
