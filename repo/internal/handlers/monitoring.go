package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"
	"helios-backend/internal/monitoring"

	"github.com/gin-gonic/gin"
)

func RegisterMonitoring(r *gin.RouterGroup) {
	g := r.Group("/monitoring",
		auth.AuthRequired(),
		auth.RequireRole("administrator"),
	)
	g.GET("/metrics", listMetrics)
	g.GET("/metrics/summary", metricsSummary)
	g.GET("/crashes", listCrashes)
	g.GET("/crashes/:id", getCrash)
}

// ---------- metrics ----------

func listMetrics(c *gin.Context) {
	p := readPaging(c)
	args := []any{}
	q := `SELECT id, service, metric_name, metric_value, unit, tags, recorded_at FROM performance_metrics`
	where := ""
	if name := c.Query("name"); name != "" {
		where += " WHERE metric_name = ?"
		args = append(args, name)
	}
	if since := c.Query("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			if where == "" {
				where += " WHERE recorded_at >= ?"
			} else {
				where += " AND recorded_at >= ?"
			}
			args = append(args, t)
		}
	}
	q += where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()

	type row struct {
		ID         int64           `json:"id"`
		Service    string          `json:"service"`
		Name       string          `json:"metric_name"`
		Value      float64         `json:"value"`
		Unit       string          `json:"unit,omitempty"`
		Tags       json.RawMessage `json:"tags,omitempty"`
		RecordedAt time.Time       `json:"recorded_at"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		var unit sql.NullString
		var tags sql.NullString
		if err := rows.Scan(&r.ID, &r.Service, &r.Name, &r.Value, &unit, &tags, &r.RecordedAt); err != nil {
			dbFail(c, err)
			return
		}
		if unit.Valid {
			r.Unit = unit.String
		}
		if tags.Valid {
			r.Tags = json.RawMessage(tags.String)
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func metricsSummary(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT m.metric_name, m.metric_value, m.unit, m.recorded_at
		FROM performance_metrics m
		JOIN (
		  SELECT metric_name, MAX(id) AS max_id
		  FROM performance_metrics
		  WHERE tags IS NULL
		  GROUP BY metric_name
		) latest ON latest.max_id = m.id
		ORDER BY m.metric_name`)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	type gauge struct {
		Name       string    `json:"name"`
		Value      float64   `json:"value"`
		Unit       string    `json:"unit,omitempty"`
		RecordedAt time.Time `json:"recorded_at"`
	}
	out := []gauge{}
	for rows.Next() {
		var g gauge
		var unit sql.NullString
		if err := rows.Scan(&g.Name, &g.Value, &unit, &g.RecordedAt); err != nil {
			dbFail(c, err)
			return
		}
		if unit.Valid {
			g.Unit = unit.String
		}
		out = append(out, g)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// ---------- crashes ----------

func listCrashes(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(
		`SELECT id, service, environment, error_type, error_message, context, occurred_at, resolved
		 FROM crash_reports ORDER BY id DESC LIMIT ? OFFSET ?`,
		p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()

	type entry struct {
		ID         int64           `json:"id"`
		Service    string          `json:"service"`
		Env        string          `json:"environment"`
		ErrorType  string          `json:"error_type"`
		ErrorMsg   string          `json:"error_message"`
		Context    json.RawMessage `json:"context,omitempty"`
		OccurredAt time.Time       `json:"occurred_at"`
		Resolved   bool            `json:"resolved"`
	}
	out := []entry{}
	for rows.Next() {
		var e entry
		var ctx sql.NullString
		var resolved int
		if err := rows.Scan(&e.ID, &e.Service, &e.Env, &e.ErrorType, &e.ErrorMsg, &ctx, &e.OccurredAt, &resolved); err != nil {
			dbFail(c, err)
			return
		}
		if ctx.Valid {
			e.Context = json.RawMessage(ctx.String)
		}
		e.Resolved = resolved != 0
		out = append(out, e)
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     out,
		"crash_dir": monitoring.CrashDir(),
	})
}

func getCrash(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	var (
		service, env, errType, errMsg, stack string
		ctxRaw                               sql.NullString
		occurred                             time.Time
	)
	err = db.DB.QueryRow(
		`SELECT service, environment, error_type, error_message, stack_trace, context, occurred_at
		 FROM crash_reports WHERE id = ?`, id,
	).Scan(&service, &env, &errType, &errMsg, &stack, &ctxRaw, &occurred)
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}

	var ctxMap map[string]any
	if ctxRaw.Valid {
		_ = json.Unmarshal([]byte(ctxRaw.String), &ctxMap)
	}

	resp := gin.H{
		"id":            id,
		"service":       service,
		"environment":   env,
		"error_type":    errType,
		"error_message": errMsg,
		"stack_trace":   stack,
		"context":       ctxMap,
		"occurred_at":   occurred,
	}

	// Serve authoritative disk copy if available. If the on-disk report is
	// unreadable we log it instead of leaking raw filesystem errors to the
	// client (which could reveal internal paths).
	if diskPath, ok := ctxMap["disk_path"].(string); ok && diskPath != "" {
		if r, err := monitoring.ReadReport(diskPath); err == nil {
			resp["disk_copy"] = r
		} else {
			log.Printf("crash report read failed for id=%d: %v", id, err)
		}
	}
	c.JSON(http.StatusOK, resp)
}
