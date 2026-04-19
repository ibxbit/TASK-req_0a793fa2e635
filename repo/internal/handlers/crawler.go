package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

func RegisterCrawler(r *gin.RouterGroup) {
	g := r.Group("/crawl", auth.AuthRequired(), auth.RequireRole("administrator", "crawler_operator"))
	g.GET("/nodes", listCrawlNodes)
	g.GET("/jobs", listCrawlJobs)
	g.GET("/jobs/:id", getCrawlJob)
	g.GET("/jobs/:id/metrics", jobMetrics)
	g.GET("/jobs/:id/logs", jobLogs)
	g.POST("/jobs", createCrawlJob)
	g.POST("/jobs/:id/pause", pauseJob)
	g.POST("/jobs/:id/resume", resumeJob)
	g.POST("/jobs/:id/cancel", cancelJob)
	g.POST("/jobs/:id/reset", resetJob)
}

// ---------- nodes ----------

type crawlNode struct {
	ID              int64      `json:"id"`
	Name            string     `json:"node_name"`
	Host            string     `json:"host"`
	Status          string     `json:"status"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
}

func listCrawlNodes(c *gin.Context) {
	rows, err := db.DB.Query(
		`SELECT id, node_name, host, status, last_heartbeat_at FROM crawl_nodes ORDER BY id`)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []crawlNode{}
	for rows.Next() {
		var n crawlNode
		var hb sql.NullTime
		if err := rows.Scan(&n.ID, &n.Name, &n.Host, &n.Status, &hb); err != nil {
			dbFail(c, err)
			return
		}
		if hb.Valid {
			t := hb.Time
			n.LastHeartbeatAt = &t
		}
		out = append(out, n)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// ---------- jobs ----------

type crawlJob struct {
	ID            int64           `json:"id"`
	JobName       string          `json:"job_name"`
	NodeID        *int64          `json:"node_id,omitempty"`
	SourceURL     string          `json:"source_url,omitempty"`
	Config        json.RawMessage `json:"config,omitempty"`
	Checkpoint    json.RawMessage `json:"checkpoint,omitempty"`
	Status        string          `json:"status"`
	Priority      int             `json:"priority"`
	Attempts      int             `json:"attempts"`
	MaxAttempts   int             `json:"max_attempts"`
	PagesFetched  int             `json:"pages_fetched"`
	DailyQuota    int             `json:"daily_quota"`
	LastError     string          `json:"last_error,omitempty"`
	ScheduledAt   *time.Time      `json:"scheduled_at,omitempty"`
	NextAttemptAt *time.Time      `json:"next_attempt_at,omitempty"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

func scanCrawlJob(row interface{ Scan(...any) error }) (*crawlJob, error) {
	var (
		j           crawlJob
		node        sql.NullInt64
		src, cfg    sql.NullString
		cp          sql.NullString
		errMsg      sql.NullString
		sched, next sql.NullTime
		start, fin  sql.NullTime
	)
	if err := row.Scan(&j.ID, &j.JobName, &node, &src, &cfg, &cp, &j.Status, &j.Priority,
		&j.Attempts, &j.MaxAttempts, &j.PagesFetched, &j.DailyQuota, &errMsg,
		&sched, &next, &start, &fin, &j.CreatedAt); err != nil {
		return nil, err
	}
	if node.Valid {
		v := node.Int64
		j.NodeID = &v
	}
	if src.Valid {
		j.SourceURL = src.String
	}
	if cfg.Valid {
		j.Config = json.RawMessage(cfg.String)
	}
	if cp.Valid {
		j.Checkpoint = json.RawMessage(cp.String)
	}
	if errMsg.Valid {
		j.LastError = errMsg.String
	}
	if sched.Valid {
		t := sched.Time
		j.ScheduledAt = &t
	}
	if next.Valid {
		t := next.Time
		j.NextAttemptAt = &t
	}
	if start.Valid {
		t := start.Time
		j.StartedAt = &t
	}
	if fin.Valid {
		t := fin.Time
		j.FinishedAt = &t
	}
	return &j, nil
}

const crawlJobCols = `id, job_name, node_id, source_url, config, checkpoint, status, priority,
	attempts, max_attempts, pages_fetched, daily_quota, last_error,
	scheduled_at, next_attempt_at, started_at, finished_at, created_at`

func listCrawlJobs(c *gin.Context) {
	p := readPaging(c)
	args := []any{}
	q := `SELECT ` + crawlJobCols + ` FROM crawl_jobs`
	if s := c.Query("status"); s != "" {
		q += " WHERE status = ?"
		args = append(args, s)
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)
	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []crawlJob{}
	for rows.Next() {
		j, err := scanCrawlJob(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *j)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getCrawlJob(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	j, err := scanCrawlJob(db.DB.QueryRow(`SELECT `+crawlJobCols+` FROM crawl_jobs WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, j)
}

type crawlJobInput struct {
	JobName     string          `json:"job_name"`
	SourceURL   string          `json:"source_url"`
	Config      json.RawMessage `json:"config"`
	Priority    int             `json:"priority"`
	MaxAttempts int             `json:"max_attempts"`
	DailyQuota  int             `json:"daily_quota"`
	ScheduledAt *time.Time      `json:"scheduled_at,omitempty"`
}

func createCrawlJob(c *gin.Context) {
	var in crawlJobInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if in.JobName == "" {
		fail(c, http.StatusBadRequest, "job_name required")
		return
	}
	if in.SourceURL == "" && len(in.Config) == 0 {
		fail(c, http.StatusBadRequest, "source_url or config.urls required")
		return
	}
	if in.MaxAttempts <= 0 {
		in.MaxAttempts = 5
	}
	if in.MaxAttempts > 5 {
		in.MaxAttempts = 5
	}
	if in.DailyQuota <= 0 {
		in.DailyQuota = 10000
	}

	var cfg any
	if len(in.Config) > 0 {
		cfg = string(in.Config)
	}

	sess, _ := auth.CurrentSession(c)
	var createdBy any = nil
	if sess != nil {
		createdBy = sess.UserID
	}
	var sched any = nil
	if in.ScheduledAt != nil {
		sched = *in.ScheduledAt
	}

	res, err := db.DB.Exec(
		`INSERT INTO crawl_jobs
		   (job_name, source_url, config, status, priority, max_attempts, daily_quota, scheduled_at, created_by)
		 VALUES (?, ?, ?, 'queued', ?, ?, ?, ?, ?)`,
		in.JobName, nullStrVal(in.SourceURL != "", in.SourceURL), cfg,
		in.Priority, in.MaxAttempts, in.DailyQuota, sched, createdBy,
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	j, _ := scanCrawlJob(db.DB.QueryRow(`SELECT `+crawlJobCols+` FROM crawl_jobs WHERE id = ?`, id))
	c.JSON(http.StatusCreated, j)
}

func pauseJob(c *gin.Context)  { transitionJob(c, "paused",  []string{"queued", "running"}) }
func resumeJob(c *gin.Context) { transitionJob(c, "queued",  []string{"paused", "failed"}) }
func cancelJob(c *gin.Context) { transitionJob(c, "cancelled", []string{"queued", "running", "paused", "failed"}) }

func transitionJob(c *gin.Context, toStatus string, fromStatuses []string) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	placeholders := ""
	args := []any{toStatus}
	for i, s := range fromStatuses {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	args = append(args, id)
	res, err := db.DB.Exec(
		`UPDATE crawl_jobs SET status = ?, next_attempt_at = NULL
		 WHERE status IN (`+placeholders+`) AND id = ?`, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(c, http.StatusConflict, "job not in a transitionable state")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": toStatus})
}

func resetJob(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	_, err := db.DB.Exec(
		`UPDATE crawl_jobs
		 SET status='queued', attempts=0, next_attempt_at=NULL, last_error=NULL,
		     checkpoint=NULL, pages_fetched=0, started_at=NULL, finished_at=NULL, node_id=NULL
		 WHERE id=?`, id)
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": "queued", "reset": true})
}

// ---------- metrics / logs ----------

func jobMetrics(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := db.DB.Query(
		`SELECT metric_name, SUM(metric_value), MAX(unit), COUNT(*)
		 FROM crawl_metrics WHERE job_id = ?
		 GROUP BY metric_name ORDER BY metric_name`, id)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	type agg struct {
		Name  string  `json:"name"`
		Total float64 `json:"total"`
		Unit  string  `json:"unit,omitempty"`
		Count int64   `json:"samples"`
	}
	out := []agg{}
	for rows.Next() {
		var a agg
		var u sql.NullString
		if err := rows.Scan(&a.Name, &a.Total, &u, &a.Count); err != nil {
			dbFail(c, err)
			return
		}
		if u.Valid {
			a.Unit = u.String
		}
		out = append(out, a)
	}

	var status string
	var startedAt, finishedAt sql.NullTime
	var attempts, pagesFetched, dailyQuota int
	_ = db.DB.QueryRow(
		`SELECT status, started_at, finished_at, attempts, pages_fetched, daily_quota
		 FROM crawl_jobs WHERE id = ?`, id,
	).Scan(&status, &startedAt, &finishedAt, &attempts, &pagesFetched, &dailyQuota)

	var durationMs int64
	if startedAt.Valid {
		end := time.Now()
		if finishedAt.Valid {
			end = finishedAt.Time
		}
		durationMs = end.Sub(startedAt.Time).Milliseconds()
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":        id,
		"status":        status,
		"attempts":      attempts,
		"pages_fetched": pagesFetched,
		"daily_quota":   dailyQuota,
		"duration_ms":   durationMs,
		"metrics":       out,
	})
}

func jobLogs(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p := readPaging(c)
	rows, err := db.DB.Query(
		`SELECT id, level, message, context, logged_at FROM crawl_logs
		 WHERE job_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		id, p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	type logRow struct {
		ID       int64           `json:"id"`
		Level    string          `json:"level"`
		Message  string          `json:"message"`
		Context  json.RawMessage `json:"context,omitempty"`
		LoggedAt time.Time       `json:"logged_at"`
	}
	out := []logRow{}
	for rows.Next() {
		var r logRow
		var ctx sql.NullString
		if err := rows.Scan(&r.ID, &r.Level, &r.Message, &ctx, &r.LoggedAt); err != nil {
			dbFail(c, err)
			return
		}
		if ctx.Valid {
			r.Context = json.RawMessage(ctx.String)
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}
