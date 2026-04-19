package approval

import "testing"

// splitActions parses the GROUP_CONCAT string returned from MySQL. Because the
// aggregation is done in SQL, the Go side only needs to split on comma and
// drop empty segments.
func TestSplitActions_EmptyString(t *testing.T) {
	if got := splitActions(""); len(got) != 0 {
		t.Fatalf("empty input: expected [], got %v", got)
	}
}

func TestSplitActions_SingleSegment(t *testing.T) {
	got := splitActions("dynasty:delete")
	if len(got) != 1 || got[0] != "dynasty:delete" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestSplitActions_MultipleSegmentsAndIgnoresTrailingDelim(t *testing.T) {
	got := splitActions("dynasty:delete,author:update,")
	want := []string{"dynasty:delete", "author:update"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("idx %d: got %q want %q", i, got[i], v)
		}
	}
}

func TestSplitActions_EmptySegmentsAreDropped(t *testing.T) {
	got := splitActions(",a,,b,")
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d: %q vs %q", i, got[i], want[i])
		}
	}
}
