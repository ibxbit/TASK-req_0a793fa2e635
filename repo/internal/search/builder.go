package search

import (
	"database/sql"
	"encoding/json"
	"strings"

	"helios-backend/internal/db"
)

func firstLineOf(body string) string {
	for i, r := range body {
		if r == '\n' || r == '\r' {
			return body[:i]
		}
	}
	return body
}

func LoadAllDocs() ([]*Doc, error) {
	rows, err := db.DB.Query(`
		SELECT id, title, author_id, dynasty_id, meter_pattern_id,
		       body, COALESCE(preface, ''), COALESCE(translation, ''), status
		FROM poems WHERE status = 'published'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Doc
	tmp := make(map[int64]*Doc)
	for rows.Next() {
		var (
			d                       Doc
			author, dynasty, meter  sql.NullInt64
		)
		if err := rows.Scan(&d.ID, &d.Title, &author, &dynasty, &meter,
			&d.Body, &d.Preface, &d.Translation, &d.Status); err != nil {
			return nil, err
		}
		if author.Valid {
			v := author.Int64
			d.AuthorID = &v
		}
		if dynasty.Valid {
			v := dynasty.Int64
			d.DynastyID = &v
		}
		if meter.Valid {
			v := meter.Int64
			d.MeterPatternID = &v
		}
		d.FirstLine = firstLineOf(d.Body)
		cp := d
		out = append(out, &cp)
		tmp[cp.ID] = &cp
	}

	tagRows, err := db.DB.Query(`
		SELECT pg.poem_id, pg.genre_id
		FROM poem_genres pg
		JOIN genres g ON g.id = pg.genre_id
		WHERE g.kind = 'tag'`)
	if err != nil {
		return out, nil
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var pid, gid int64
		if err := tagRows.Scan(&pid, &gid); err != nil {
			continue
		}
		if d, ok := tmp[pid]; ok {
			d.TagIDs = append(d.TagIDs, gid)
		}
	}
	return out, nil
}

func LoadDoc(id int64) (*Doc, error) {
	var (
		d                      Doc
		author, dynasty, meter sql.NullInt64
	)
	err := db.DB.QueryRow(`
		SELECT id, title, author_id, dynasty_id, meter_pattern_id,
		       body, COALESCE(preface, ''), COALESCE(translation, ''), status
		FROM poems WHERE id = ?`, id).Scan(
		&d.ID, &d.Title, &author, &dynasty, &meter,
		&d.Body, &d.Preface, &d.Translation, &d.Status,
	)
	if err != nil {
		return nil, err
	}
	if author.Valid {
		v := author.Int64
		d.AuthorID = &v
	}
	if dynasty.Valid {
		v := dynasty.Int64
		d.DynastyID = &v
	}
	if meter.Valid {
		v := meter.Int64
		d.MeterPatternID = &v
	}
	d.FirstLine = firstLineOf(d.Body)

	tagRows, err := db.DB.Query(`
		SELECT pg.genre_id FROM poem_genres pg
		JOIN genres g ON g.id = pg.genre_id
		WHERE pg.poem_id = ? AND g.kind = 'tag'`, id)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var gid int64
			if err := tagRows.Scan(&gid); err == nil {
				d.TagIDs = append(d.TagIDs, gid)
			}
		}
	}
	return &d, nil
}

// LoadSynonyms reads dictionary_terms rows with category='synonym'.
// The `examples` JSON column is expected to be a string array of synonyms.
func LoadSynonyms() (map[string][]string, error) {
	rows, err := db.DB.Query(
		`SELECT term, examples FROM dictionary_terms WHERE category = 'synonym'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]string)
	for rows.Next() {
		var term string
		var examples sql.NullString
		if err := rows.Scan(&term, &examples); err != nil {
			continue
		}
		if !examples.Valid {
			continue
		}
		var syns []string
		if err := json.Unmarshal([]byte(examples.String), &syns); err != nil {
			continue
		}
		out[strings.ToLower(term)] = syns
	}
	return out, nil
}
