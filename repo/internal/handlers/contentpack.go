package handlers

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

type contentPack struct {
	Version   string           `json:"version"`
	BuiltAt   time.Time        `json:"built_at"`
	Poems     []map[string]any `json:"poems"`
	Authors   []map[string]any `json:"authors"`
	Dynasties []map[string]any `json:"dynasties"`
	Tags      []map[string]any `json:"tags"`
	Meters    []map[string]any `json:"meter_patterns"`
}

var (
	packMu       sync.RWMutex
	packBytes    []byte
	packETag     string
	packBuiltAt  time.Time
	packCacheTTL = 5 * time.Minute
)

func RegisterContentPacks(r *gin.RouterGroup) {
	g := r.Group("/content-packs", auth.AuthRequired())
	g.GET("/current", downloadCurrentPack)
	g.HEAD("/current", downloadCurrentPack)
}

func downloadCurrentPack(c *gin.Context) {
	data, etag, built, err := getOrBuildPack()
	if err != nil {
		dbFail(c, err)
		return
	}
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Type", "application/json; charset=utf-8")
	http.ServeContent(c.Writer, c.Request, "helios-content-pack.json", built, bytes.NewReader(data))
}

func getOrBuildPack() ([]byte, string, time.Time, error) {
	packMu.RLock()
	if packBytes != nil && time.Since(packBuiltAt) < packCacheTTL {
		b, e, t := packBytes, packETag, packBuiltAt
		packMu.RUnlock()
		return b, e, t, nil
	}
	packMu.RUnlock()

	packMu.Lock()
	defer packMu.Unlock()
	if packBytes != nil && time.Since(packBuiltAt) < packCacheTTL {
		return packBytes, packETag, packBuiltAt, nil
	}

	pack, err := buildPack()
	if err != nil {
		return nil, "", time.Time{}, err
	}
	b, err := json.Marshal(pack)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	sum := sha256.Sum256(b)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	packBytes = b
	packETag = etag
	packBuiltAt = pack.BuiltAt
	return packBytes, packETag, packBuiltAt, nil
}

func buildPack() (*contentPack, error) {
	now := time.Now()
	p := &contentPack{Version: "1", BuiltAt: now}

	// Poems (published only)
	if rows, err := db.DB.Query(
		`SELECT id, title, author_id, dynasty_id, meter_pattern_id, body,
		        COALESCE(preface,''), COALESCE(translation,''), status, version
		 FROM poems WHERE status='published' ORDER BY id`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				id, ver                int64
				title, body, pref, tr  string
				status                 string
				author, dyn, meter     sql.NullInt64
			)
			if err := rows.Scan(&id, &title, &author, &dyn, &meter, &body, &pref, &tr, &status, &ver); err != nil {
				return nil, err
			}
			p.Poems = append(p.Poems, map[string]any{
				"id":               id,
				"title":            title,
				"author_id":        nullable(author),
				"dynasty_id":       nullable(dyn),
				"meter_pattern_id": nullable(meter),
				"body":             body,
				"preface":          pref,
				"translation":      tr,
				"status":           status,
				"version":          ver,
			})
		}
	} else {
		return nil, err
	}

	if err := loadSimple(&p.Authors,
		`SELECT id, name, alt_names, dynasty_id, birth_year, death_year
		 FROM authors ORDER BY id`,
		"id", "name", "alt_names", "dynasty_id", "birth_year", "death_year"); err != nil {
		return nil, err
	}
	if err := loadSimple(&p.Dynasties,
		`SELECT id, name, start_year, end_year, description FROM dynasties ORDER BY id`,
		"id", "name", "start_year", "end_year", "description"); err != nil {
		return nil, err
	}
	if err := loadSimple(&p.Tags,
		`SELECT id, name, parent_id, description FROM genres WHERE kind='tag' ORDER BY id`,
		"id", "name", "parent_id", "description"); err != nil {
		return nil, err
	}
	if err := loadSimple(&p.Meters,
		`SELECT id, name, pattern_type, rhyme_scheme FROM meter_patterns ORDER BY id`,
		"id", "name", "pattern_type", "rhyme_scheme"); err != nil {
		return nil, err
	}
	return p, nil
}

func nullable(n sql.NullInt64) any {
	if n.Valid {
		return n.Int64
	}
	return nil
}

// loadSimple executes a query and fills out with {col: value} maps. Column
// names are provided in order matching the SELECT.
func loadSimple(out *[]map[string]any, query string, cols ...string) error {
	rows, err := db.DB.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			var x any
			ptrs[i] = &x
			vals[i] = &x
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			v := *(ptrs[i].(*any))
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[c] = v
		}
		*out = append(*out, row)
	}
	return nil
}
