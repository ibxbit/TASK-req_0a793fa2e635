package unittests

import (
	"strings"
	"testing"

	"helios-backend/internal/search"
)

func TestSearch_TokenizeMixedContent(t *testing.T) {
	got := search.Tokenize("Hello 春 World 123")
	want := []string{"hello", "春", "world", "123"}
	if len(got) != len(want) {
		t.Fatalf("len: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestSearch_CJKVariantExpansion(t *testing.T) {
	got := search.ExpandCJKToken("国")
	foundSimplified := false
	foundTraditional := false
	for _, v := range got {
		if v == "国" {
			foundSimplified = true
		}
		if v == "國" {
			foundTraditional = true
		}
	}
	if !foundSimplified || !foundTraditional {
		t.Fatalf("expected both SC and TC variants, got %v", got)
	}
}

func TestSearch_HighlightWrapsMatches(t *testing.T) {
	out := search.Highlight("春眠不觉晓", []string{"眠"},
		search.DefaultHLPre, search.DefaultHLPost)
	if !strings.Contains(out, "<mark>眠</mark>") {
		t.Fatalf("expected CJK highlight, got %q", out)
	}
}

func TestSearch_HighlightCaseInsensitiveASCII(t *testing.T) {
	out := search.Highlight("Hello World", []string{"world"}, "<m>", "</m>")
	if out != "Hello <m>World</m>" {
		t.Fatalf("case-insensitive highlight failed: %q", out)
	}
}
