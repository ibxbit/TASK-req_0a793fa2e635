package search

import (
	"sort"
	"strings"
)

const (
	DefaultHLPre  = "<mark>"
	DefaultHLPost = "</mark>"
)

// Highlight wraps each occurrence of any token in text with pre/post markers.
// Operates on runes so CJK substring boundaries are preserved. Case-insensitive
// for ASCII.
func Highlight(text string, tokens []string, pre, post string) string {
	if text == "" || len(tokens) == 0 {
		return text
	}
	if pre == "" {
		pre = DefaultHLPre
	}
	if post == "" {
		post = DefaultHLPost
	}

	runes := []rune(text)
	lower := []rune(strings.ToLower(text))

	type span struct{ start, end int }
	var spans []span

	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		needle := []rune(strings.ToLower(tok))
		n := len(needle)
		if n == 0 || n > len(lower) {
			continue
		}
		for i := 0; i <= len(lower)-n; i++ {
			match := true
			for j := 0; j < n; j++ {
				if lower[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				spans = append(spans, span{i, i + n})
			}
		}
	}

	if len(spans) == 0 {
		return text
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}

	var b strings.Builder
	cursor := 0
	for _, s := range merged {
		b.WriteString(string(runes[cursor:s.start]))
		b.WriteString(pre)
		b.WriteString(string(runes[s.start:s.end]))
		b.WriteString(post)
		cursor = s.end
	}
	b.WriteString(string(runes[cursor:]))
	return b.String()
}

// SnippetAround returns up to `window` runes of context around the first match
// of any token, with that match highlighted. Falls back to the first `window`
// runes of text when no token is found.
func SnippetAround(text string, tokens []string, window int, pre, post string) string {
	if text == "" {
		return ""
	}
	runes := []rune(text)
	lower := []rune(strings.ToLower(text))

	bestStart, bestLen := -1, 0
	for _, tok := range tokens {
		needle := []rune(strings.ToLower(tok))
		if len(needle) == 0 || len(needle) > len(lower) {
			continue
		}
		for i := 0; i <= len(lower)-len(needle); i++ {
			match := true
			for j := 0; j < len(needle); j++ {
				if lower[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				bestStart = i
				bestLen = len(needle)
				break
			}
		}
		if bestStart >= 0 {
			break
		}
	}

	if bestStart < 0 {
		end := window
		if end > len(runes) {
			end = len(runes)
		}
		return string(runes[:end])
	}

	half := window / 2
	start := bestStart - half
	if start < 0 {
		start = 0
	}
	end := bestStart + bestLen + half
	if end > len(runes) {
		end = len(runes)
	}
	return Highlight(string(runes[start:end]), tokens, pre, post)
}
