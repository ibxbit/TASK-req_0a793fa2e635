package search

import (
	"sort"
	"strings"
)

const (
	phraseBonus   = 10.0
	synonymWeight = 0.3
	cjkVariantWeight = 0.9 // slight penalty so a same-script match outranks a cross-script match
)

type Filters struct {
	AuthorID    *int64
	DynastyID   *int64
	TagID       *int64
	MeterID     *int64
	LineSnippet string
}

// Options gate non-deterministic query features. When all are false (default)
// the search produces deterministic results based solely on exact token match,
// field boosts, and phrase bonus.
type Options struct {
	ExpandSynonyms bool
	ConvertCJK     bool
	Highlight      bool
	SnippetWindow  int
}

type Hit struct {
	PoemID           int64    `json:"poem_id"`
	Title            string   `json:"title"`
	FirstLine        string   `json:"first_line,omitempty"`
	TitleHL          string   `json:"title_highlighted,omitempty"`
	FirstLineHL      string   `json:"first_line_highlighted,omitempty"`
	Snippet          string   `json:"snippet,omitempty"`
	Score            float64  `json:"score"`
	MatchedFields    []string `json:"matched_fields,omitempty"`
}

func (idx *Index) Search(q string, f Filters, opts Options, limit, offset int) []Hit {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tokens := Tokenize(q)
	scores := make(map[int64]float64)
	fieldHits := make(map[int64]map[int]struct{})

	addScore := func(docID int64, field int, delta float64) {
		scores[docID] += delta
		if fieldHits[docID] == nil {
			fieldHits[docID] = make(map[int]struct{})
		}
		fieldHits[docID][field] = struct{}{}
	}

	// Exact token matches
	for _, t := range tokens {
		for _, p := range idx.postings[t] {
			addScore(p.PoemID, p.Field, fieldBoost[p.Field]*float64(len(p.Positions)))
		}
	}

	// CJK SC↔TC variant lookups (toggleable)
	if opts.ConvertCJK {
		for _, t := range tokens {
			variants := ExpandCJKToken(t)
			for _, v := range variants {
				if v == t {
					continue
				}
				for _, p := range idx.postings[v] {
					addScore(p.PoemID, p.Field, fieldBoost[p.Field]*float64(len(p.Positions))*cjkVariantWeight)
				}
			}
		}
	}

	// Synonym expansion (toggleable)
	if opts.ExpandSynonyms {
		for _, t := range tokens {
			for _, syn := range idx.synonyms[t] {
				for _, st := range Tokenize(syn) {
					for _, p := range idx.postings[st] {
						addScore(p.PoemID, p.Field, fieldBoost[p.Field]*float64(len(p.Positions))*synonymWeight)
					}
				}
			}
		}
	}

	// Exact-phrase bonus on original tokens only (deterministic)
	if len(tokens) >= 2 {
		for docID := range scores {
			for field := FieldTitle; field <= FieldPreface; field++ {
				if idx.hasPhraseUnsafe(docID, field, tokens) {
					addScore(docID, field, phraseBonus*fieldBoost[field])
				}
			}
		}
	}

	results := []Hit{}
	if len(tokens) == 0 {
		for _, doc := range idx.docs {
			if !matchesFilters(doc, f) {
				continue
			}
			results = append(results, buildHit(doc, 0, nil, tokens, opts))
		}
	} else {
		for docID, s := range scores {
			doc, ok := idx.docs[docID]
			if !ok || !matchesFilters(doc, f) {
				continue
			}
			results = append(results, buildHit(doc, s, fieldHits[docID], tokens, opts))
		}
	}

	// Line-snippet filter
	if f.LineSnippet != "" {
		needle := f.LineSnippet
		kept := results[:0]
		for _, h := range results {
			if d := idx.docs[h.PoemID]; d != nil && strings.Contains(d.Body, needle) {
				kept = append(kept, h)
			}
		}
		results = kept
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if offset < 0 {
		offset = 0
	}
	if offset > len(results) {
		offset = len(results)
	}
	end := offset + limit
	if limit <= 0 || end > len(results) {
		end = len(results)
	}
	return results[offset:end]
}

func buildHit(doc *Doc, score float64, hitFields map[int]struct{}, tokens []string, opts Options) Hit {
	h := Hit{
		PoemID:    doc.ID,
		Title:     doc.Title,
		FirstLine: doc.FirstLine,
		Score:     score,
	}
	for fld := range hitFields {
		h.MatchedFields = append(h.MatchedFields, fieldName(fld))
	}
	sort.Strings(h.MatchedFields)

	if opts.Highlight && len(tokens) > 0 {
		// Include CJK variants in the highlight set so cross-script matches
		// still render visibly. Highlighting never affects ranking.
		hlTokens := tokens
		if opts.ConvertCJK {
			seen := map[string]struct{}{}
			for _, t := range tokens {
				for _, v := range ExpandCJKToken(t) {
					if _, ok := seen[v]; !ok {
						seen[v] = struct{}{}
						hlTokens = append(hlTokens, v)
					}
				}
			}
		}
		h.TitleHL = Highlight(doc.Title, hlTokens, DefaultHLPre, DefaultHLPost)
		h.FirstLineHL = Highlight(doc.FirstLine, hlTokens, DefaultHLPre, DefaultHLPost)
		w := opts.SnippetWindow
		if w <= 0 {
			w = 60
		}
		h.Snippet = SnippetAround(doc.Body, hlTokens, w, DefaultHLPre, DefaultHLPost)
	}
	return h
}

func (idx *Index) hasPhraseUnsafe(docID int64, field int, tokens []string) bool {
	positions := make([][]int, 0, len(tokens))
	for _, t := range tokens {
		found := false
		for _, p := range idx.postings[t] {
			if p.PoemID == docID && p.Field == field {
				positions = append(positions, p.Positions)
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, start := range positions[0] {
		ok := true
		for k := 1; k < len(positions); k++ {
			if !containsInt(positions[k], start+k) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func matchesFilters(d *Doc, f Filters) bool {
	if f.AuthorID != nil && (d.AuthorID == nil || *d.AuthorID != *f.AuthorID) {
		return false
	}
	if f.DynastyID != nil && (d.DynastyID == nil || *d.DynastyID != *f.DynastyID) {
		return false
	}
	if f.MeterID != nil && (d.MeterPatternID == nil || *d.MeterPatternID != *f.MeterID) {
		return false
	}
	if f.TagID != nil {
		found := false
		for _, t := range d.TagIDs {
			if t == *f.TagID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
