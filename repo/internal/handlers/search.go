package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/search"

	"github.com/gin-gonic/gin"
)

func RegisterSearch(r *gin.RouterGroup) {
	g := r.Group("/search", auth.AuthRequired())
	g.GET("", searchHandler)
	g.GET("/suggest", suggestHandler)
	g.POST("/reindex", auth.RequireRole("administrator"), reindexHandler)
}

func optInt64(c *gin.Context, key string) *int64 {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func boolParam(c *gin.Context, key string) bool {
	v := strings.ToLower(c.Query(key))
	return v == "1" || v == "true" || v == "yes"
}

func searchHandler(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	p := readPaging(c)

	f := search.Filters{
		AuthorID:    optInt64(c, "author_id"),
		DynastyID:   optInt64(c, "dynasty_id"),
		TagID:       optInt64(c, "tag_id"),
		MeterID:     optInt64(c, "meter_id"),
		LineSnippet: strings.TrimSpace(c.Query("snippet")),
	}

	opts := search.Options{
		ExpandSynonyms: boolParam(c, "syn"),
		ConvertCJK:     boolParam(c, "cjk"),
		Highlight:      boolParam(c, "highlight"),
	}
	if w, err := strconv.Atoi(c.Query("snippet_window")); err == nil && w > 0 {
		opts.SnippetWindow = w
	}

	start := time.Now()
	hits := search.Engine().Search(q, f, opts, p.Limit, p.Offset)
	duration := time.Since(start)

	resp := gin.H{
		"query":  q,
		"hits":   hits,
		"count":  len(hits),
		"limit":  p.Limit,
		"offset": p.Offset,
		"options": gin.H{
			"expand_synonyms": opts.ExpandSynonyms,
			"convert_cjk":     opts.ConvertCJK,
			"highlight":       opts.Highlight,
		},
	}

	// "Did you mean" is attached when the query is non-empty but weak.
	if q != "" && len(hits) < 3 {
		if sugs, err := search.Suggest(q, 5); err == nil && len(sugs) > 0 {
			resp["did_you_mean"] = sugs
		}
	}

	// Async log to query_history (best effort — never blocks response).
	var uid *int64
	if sess, ok := auth.CurrentSession(c); ok {
		uid = &sess.UserID
	}
	go search.LogQuery(uid, q, len(hits), int(duration/time.Millisecond))

	c.JSON(http.StatusOK, resp)
}

func suggestHandler(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	sugs, err := search.Suggest(q, limit)
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"query": q, "suggestions": sugs})
}

func reindexHandler(c *gin.Context) {
	if err := search.Rebuild(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reindexed": true,
		"docs":      search.Engine().Size(),
	})
}
