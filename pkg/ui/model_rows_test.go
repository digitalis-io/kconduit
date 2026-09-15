package ui

import "testing"

func TestHumanBytesScales(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
	}

	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTopicColumnsGiveSpareWidthToTheName(t *testing.T) {
	wide := topicColumns(160)
	if wide[0].Width <= topicsColumns[0].Width {
		t.Errorf("name column on a wide table = %d, want more than the base %d",
			wide[0].Width, topicsColumns[0].Width)
	}

	// A table too narrow to grow the name must not shrink it to nothing.
	narrow := topicColumns(10)
	if narrow[0].Width != topicsColumns[0].Width {
		t.Errorf("name column on a narrow table = %d, want the base %d",
			narrow[0].Width, topicsColumns[0].Width)
	}

	if topicsColumns[0].Width != 30 {
		t.Errorf("topicColumns mutated the shared base to width %d", topicsColumns[0].Width)
	}
}
