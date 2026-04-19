package search

import (
	"reflect"
	"testing"
)

func TestTokenize_ASCII(t *testing.T) {
	got := Tokenize("Hello, World! 123")
	want := []string{"hello", "world", "123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTokenize_CJKUnigrams(t *testing.T) {
	got := Tokenize("春眠不觉晓")
	want := []string{"春", "眠", "不", "觉", "晓"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTokenize_MixedContent(t *testing.T) {
	got := Tokenize("poem 春  abc")
	want := []string{"poem", "春", "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTokenize_Empty(t *testing.T) {
	if got := Tokenize(""); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestTokenize_LowercaseASCII(t *testing.T) {
	got := Tokenize("HELLO")
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("expected [hello], got %v", got)
	}
}

func TestTokenize_PunctuationOnly(t *testing.T) {
	got := Tokenize("... !!!")
	if len(got) != 0 {
		t.Fatalf("expected no tokens, got %v", got)
	}
}
