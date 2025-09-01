package kafka

import (
	"testing"
)

func TestParseTimeToMilliseconds(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		name     string
	}{
		// Already milliseconds
		{"1000", "1000", "pure number stays as-is"},
		{"86400000", "86400000", "large number stays as-is"},
		
		// Go duration formats
		{"1h", "3600000", "1 hour"},
		{"24h", "86400000", "24 hours"},
		{"30m", "1800000", "30 minutes"},
		{"1h30m", "5400000", "1 hour 30 minutes"},
		{"10s", "10000", "10 seconds"},
		{"500ms", "500", "500 milliseconds"},
		
		// Day and week formats
		{"1d", "86400000", "1 day"},
		{"7d", "604800000", "7 days"},
		{"1w", "604800000", "1 week"},
		{"2w", "1209600000", "2 weeks"},
		{"1.5d", "129600000", "1.5 days"},
		
		// With spaces
		{"1 d", "86400000", "1 day with space"},
		{"2 weeks", "1209600000", "2 weeks spelled out"},
		{"3 days", "259200000", "3 days spelled out"},
		
		// Invalid formats return as-is
		{"invalid", "invalid", "invalid format"},
		{"", "", "empty string"},
		{"abc123", "abc123", "mixed invalid"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimeToMilliseconds(tt.input)
			if result != tt.expected {
				t.Errorf("parseTimeToMilliseconds(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}