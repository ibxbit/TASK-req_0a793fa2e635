package search

import (
	"sort"
	"strings"

	"helios-backend/internal/db"
)

type Suggestion struct {
	Term     string `json:"term"`
	Distance int    `json:"distance"`
	Source   string `json:"source"`
}

// Suggest returns "did you mean" candidates, drawn from query_history and
// dictionary_terms, ranked by Levenshtein distance.
func Suggest(query string, limit int) ([]Suggestion, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	type src struct {
		term   string
		source string
	}
	seen := make(map[string]string) // term -> source

	rows, err := db.DB.Query(`
		SELECT query_text FROM query_history
		WHERE query_text <> ''
		GROUP BY query_text
		ORDER BY MAX(executed_at) DESC
		LIMIT 500`)
	if err == nil {
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err == nil {
				if _, ok := seen[s]; !ok {
					seen[s] = "query_history"
				}
			}
		}
		rows.Close()
	}

	rows2, err := db.DB.Query(`SELECT term FROM dictionary_terms LIMIT 2000`)
	if err == nil {
		for rows2.Next() {
			var s string
			if err := rows2.Scan(&s); err == nil {
				if _, ok := seen[s]; !ok {
					seen[s] = "dictionary"
				}
			}
		}
		rows2.Close()
	}

	qRunes := []rune(q)
	threshold := len(qRunes) / 2
	if threshold < 1 {
		threshold = 1
	}

	type scored struct {
		s src
		d int
	}
	var list []scored
	for term, source := range seen {
		if term == q {
			continue
		}
		d := levenshtein(qRunes, []rune(term))
		if d <= threshold {
			list = append(list, scored{src{term, source}, d})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].d != list[j].d {
			return list[i].d < list[j].d
		}
		return list[i].s.term < list[j].s.term
	})

	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]Suggestion, 0, len(list))
	for _, s := range list {
		out = append(out, Suggestion{Term: s.s.term, Distance: s.d, Source: s.s.source})
	}
	return out, nil
}

func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			c := prev[j] + 1
			if curr[j-1]+1 < c {
				c = curr[j-1] + 1
			}
			if prev[j-1]+cost < c {
				c = prev[j-1] + cost
			}
			curr[j] = c
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// LogQuery persists a search to query_history; fire-and-forget.
func LogQuery(userID *int64, queryText string, resultCount int, durationMs int) {
	var user any = nil
	if userID != nil {
		user = *userID
	}
	_, _ = db.DB.Exec(
		`INSERT INTO query_history (user_id, query_text, result_count, duration_ms) VALUES (?, ?, ?, ?)`,
		user, queryText, resultCount, durationMs,
	)
}
