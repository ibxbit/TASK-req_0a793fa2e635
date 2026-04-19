package search

import (
	"strings"
	"unicode"
)

// Tokenize returns lowercased tokens. CJK characters each become a unigram
// token; contiguous ASCII letters/digits form a single token.
func Tokenize(s string) []string {
	if s == "" {
		return nil
	}
	tokens := make([]string, 0, len(s)/2)
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}
	for _, r := range s {
		switch {
		case isCJK(r):
			flush()
			tokens = append(tokens, string(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			buf.WriteRune(unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return tokens
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // Extension A
		(r >= 0x20000 && r <= 0x2A6DF) // Extension B
}
