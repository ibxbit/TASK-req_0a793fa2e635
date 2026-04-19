package search

import (
	"strings"
	"testing"
)

func TestHighlight_ASCII_CaseInsensitive(t *testing.T) {
	out := Highlight("Hello World", []string{"world"}, "<m>", "</m>")
	if out != "Hello <m>World</m>" {
		t.Fatalf("got %q", out)
	}
}

func TestHighlight_MultipleOccurrences(t *testing.T) {
	out := Highlight("foo bar foo", []string{"foo"}, "<m>", "</m>")
	if strings.Count(out, "<m>foo</m>") != 2 {
		t.Fatalf("expected 2 marks, got %q", out)
	}
}

func TestHighlight_OverlappingSpansMerged(t *testing.T) {
	// 'abab' searched with 'aba' — matches at 0 and would match at 2 (overlap)
	out := Highlight("ababa", []string{"aba"}, "<m>", "</m>")
	// overlaps are merged so there's one continuous mark covering the union
	if strings.Count(out, "<m>") != 1 {
		t.Fatalf("expected single merged mark, got %q", out)
	}
}

func TestHighlight_CJK(t *testing.T) {
	out := Highlight("春眠不觉晓", []string{"眠"}, "<m>", "</m>")
	if out != "春<m>眠</m>不觉晓" {
		t.Fatalf("got %q", out)
	}
}

func TestHighlight_NoMatch_Unchanged(t *testing.T) {
	in := "nothing here"
	if out := Highlight(in, []string{"xyz"}, "<m>", "</m>"); out != in {
		t.Fatalf("expected unchanged, got %q", out)
	}
}

func TestSnippetAround_FallsBackToPrefix(t *testing.T) {
	out := SnippetAround("hello world, this is long text", []string{"zzz"}, 10, "<m>", "</m>")
	if len([]rune(out)) > 10 {
		t.Fatalf("snippet length exceeded window: %q", out)
	}
}
