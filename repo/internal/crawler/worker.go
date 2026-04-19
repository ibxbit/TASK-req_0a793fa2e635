package crawler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"helios-backend/internal/db"
)

const (
	heartbeatInterval = 10 * time.Second
	heartbeatStale    = 60 * time.Second
	workerPoll        = 2 * time.Second
	fetchTimeout      = 15 * time.Second
)

type Worker struct {
	nodeID   int64
	nodeName string
	limiter  *HostLimiter
	http     *http.Client
}

var activeWorker *Worker

// InitNode registers this process as a crawl node and starts the worker loop.
func InitNode() error {
	name := os.Getenv("HELIOS_NODE_NAME")
	if name == "" {
		h, _ := os.Hostname()
		if h == "" {
			h = "node"
		}
		name = h
	}
	host, _ := os.Hostname()

	res, err := db.DB.Exec(
		`INSERT INTO crawl_nodes (node_name, host, status, last_heartbeat_at)
		 VALUES (?, ?, 'online', NOW())
		 ON DUPLICATE KEY UPDATE
		   host = VALUES(host),
		   status = 'online',
		   last_heartbeat_at = NOW()`,
		name, host)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		// on duplicate the insertID is 0; fetch by name
		if err := db.DB.QueryRow(`SELECT id FROM crawl_nodes WHERE node_name = ?`, name).Scan(&id); err != nil {
			return err
		}
	}

	w := &Worker{
		nodeID:   id,
		nodeName: name,
		limiter:  NewHostLimiter(1 * time.Second),
		http:     &http.Client{Timeout: fetchTimeout},
	}
	activeWorker = w

	go w.heartbeatLoop()
	go w.runLoop()
	log.Printf("crawler: node '%s' (id=%d) online", name, id)
	return nil
}

func CurrentNodeID() int64 {
	if activeWorker == nil {
		return 0
	}
	return activeWorker.nodeID
}

func (w *Worker) heartbeatLoop() {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for range t.C {
		_, err := db.DB.Exec(
			`UPDATE crawl_nodes SET last_heartbeat_at = NOW(), status = 'online' WHERE id = ?`,
			w.nodeID)
		if err != nil {
			log.Printf("crawler heartbeat: %v", err)
		}
	}
}

func (w *Worker) runLoop() {
	t := time.NewTicker(workerPoll)
	defer t.Stop()
	for range t.C {
		if err := w.claimAndRunOne(); err != nil {
			log.Printf("crawler worker: %v", err)
		}
	}
}

func (w *Worker) claimAndRunOne() error {
	// Claim one job that is assigned to me OR unassigned-and-queued.
	// Use a transactional update so two workers never pick the same row.
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		jobID        int64
		jobName      string
		sourceURL    sql.NullString
		configRaw    sql.NullString
		checkpoint   sql.NullString
		attempts     int
		maxAttempts  int
		pagesFetched int
		dailyQuota   int
	)
	err = tx.QueryRow(`
		SELECT id, job_name, source_url, config, checkpoint, attempts, max_attempts,
		       pages_fetched, daily_quota
		FROM crawl_jobs
		WHERE status IN ('queued','running')
		  AND (node_id = ? OR node_id IS NULL)
		  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		  AND (scheduled_at  IS NULL OR scheduled_at  <= NOW())
		ORDER BY priority DESC, id ASC
		LIMIT 1 FOR UPDATE`, w.nodeID,
	).Scan(&jobID, &jobName, &sourceURL, &configRaw, &checkpoint, &attempts, &maxAttempts,
		&pagesFetched, &dailyQuota)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE crawl_jobs SET node_id=?, status='running', started_at=COALESCE(started_at, NOW())
		 WHERE id=?`, w.nodeID, jobID,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	job := runtimeJob{
		ID:           jobID,
		Name:         jobName,
		SourceURL:    sourceURL.String,
		Config:       parseJSON(configRaw.String),
		Checkpoint:   parseJSON(checkpoint.String),
		Attempts:     attempts,
		MaxAttempts:  maxAttempts,
		PagesFetched: pagesFetched,
		DailyQuota:   dailyQuota,
	}
	go w.execute(job)
	return nil
}

type runtimeJob struct {
	ID           int64
	Name         string
	SourceURL    string
	Config       map[string]any
	Checkpoint   map[string]any
	Attempts     int
	MaxAttempts  int
	PagesFetched int
	DailyQuota   int
}

func parseJSON(s string) map[string]any {
	if s == "" {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

// execute runs the job. Each fetched URL updates the checkpoint so a crash
// simply resumes from where it left off on the next claim.
func (w *Worker) execute(j runtimeJob) {
	start := time.Now()
	writeLog(j.ID, w.nodeID, "info", "job started", map[string]any{"attempt": j.Attempts + 1})

	pending := stringList(j.Checkpoint["pending"])
	visited := stringList(j.Checkpoint["visited"])

	if len(pending) == 0 && len(visited) == 0 {
		// First run: seed queue from config.urls or source_url
		if urls, ok := j.Config["urls"].([]any); ok {
			for _, u := range urls {
				if s, ok := u.(string); ok {
					pending = append(pending, s)
				}
			}
		}
		if len(pending) == 0 && j.SourceURL != "" {
			pending = append(pending, j.SourceURL)
		}
	}

	quotaHit := false
	for len(pending) > 0 {
		// Per-day quota check: roll over the counter if we've crossed the
		// UTC day boundary, then check we're still under cap. Cumulative
		// pages_fetched is still kept up-to-date by IncrementToday so
		// lifetime totals remain accurate for dashboards.
		quota, err := CheckAndRollover(j.ID, time.Now())
		if err != nil {
			log.Printf("crawler: quota check for job %d: %v", j.ID, err)
			// best-effort fall through — fail on pulling records is handled below
		} else if quota.DailyQuota > 0 && quota.PagesFetchedToday >= quota.DailyQuota {
			quotaHit = true
			break
		}
		url := pending[0]
		pending = pending[1:]

		host := HostOf(url)
		w.limiter.Wait(host)

		status, size, err := w.fetch(url)
		now := time.Now()
		if err != nil {
			writeLog(j.ID, w.nodeID, "error", "fetch failed: "+err.Error(),
				map[string]any{"url": url})
			recordMetric(j.ID, w.nodeID, "fetch_error", 1, "count", map[string]any{"host": host})
			// put URL back to retry within this job
			pending = append([]string{url}, pending...)
			j.Checkpoint["pending"] = pending
			j.Checkpoint["visited"] = visited
			j.Checkpoint["updated_at"] = now.Format(time.RFC3339)
			saveCheckpoint(j.ID, j.Checkpoint)
			w.failJob(j, err.Error())
			return
		}

		visited = append(visited, url)
		j.PagesFetched++
		recordMetric(j.ID, w.nodeID, "pages_fetched", 1, "count",
			map[string]any{"host": host, "status": status})
		recordMetric(j.ID, w.nodeID, "bytes_downloaded", float64(size), "bytes",
			map[string]any{"host": host})

		j.Checkpoint["pending"] = pending
		j.Checkpoint["visited"] = visited
		j.Checkpoint["updated_at"] = now.Format(time.RFC3339)
		saveCheckpoint(j.ID, j.Checkpoint)
		// IncrementToday keeps both counters in lock-step atomically.
		if _, err := IncrementToday(j.ID); err != nil {
			log.Printf("crawler: increment quota for job %d: %v", j.ID, err)
		}
	}

	duration := time.Since(start)
	recordMetric(j.ID, w.nodeID, "job_duration_ms", float64(duration/time.Millisecond), "ms", nil)

	if quotaHit {
		_, _ = db.DB.Exec(
			`UPDATE crawl_jobs SET status='paused', last_error=? WHERE id=?`,
			fmt.Sprintf("daily_quota reached (%d)", j.DailyQuota), j.ID)
		writeLog(j.ID, w.nodeID, "warn", "paused: daily quota reached", nil)
		return
	}

	_, _ = db.DB.Exec(
		`UPDATE crawl_jobs SET status='completed', finished_at=NOW(), last_error=NULL WHERE id=?`,
		j.ID)
	writeLog(j.ID, w.nodeID, "info", "job completed",
		map[string]any{"pages": j.PagesFetched, "duration_ms": duration / time.Millisecond})
}

func (w *Worker) failJob(j runtimeJob, errMsg string) {
	j.Attempts++
	if j.Attempts >= j.MaxAttempts {
		_, _ = db.DB.Exec(
			`UPDATE crawl_jobs SET status='failed', attempts=?, last_error=?, finished_at=NOW()
			 WHERE id=?`, j.Attempts, errMsg, j.ID)
		writeLog(j.ID, w.nodeID, "fatal", "job failed terminally",
			map[string]any{"attempts": j.Attempts})
		return
	}
	// Exponential backoff: 30s, 60s, 120s, 240s, 480s
	delay := time.Duration(30) * time.Second
	for i := 1; i < j.Attempts; i++ {
		delay *= 2
	}
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	_, _ = db.DB.Exec(
		`UPDATE crawl_jobs
		 SET status='queued', attempts=?, last_error=?, next_attempt_at=DATE_ADD(NOW(), INTERVAL ? SECOND), node_id=NULL
		 WHERE id=?`, j.Attempts, errMsg, int(delay.Seconds()), j.ID)
	writeLog(j.ID, w.nodeID, "warn", "retry scheduled",
		map[string]any{"attempt": j.Attempts, "max": j.MaxAttempts, "delay_sec": int(delay.Seconds())})
}

func saveCheckpoint(jobID int64, cp map[string]any) {
	b, err := json.Marshal(cp)
	if err != nil {
		return
	}
	_, _ = db.DB.Exec(`UPDATE crawl_jobs SET checkpoint=? WHERE id=?`, string(b), jobID)
}

func (w *Worker) fetch(raw string) (int, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "Helios/1.0")
	res, err := w.http.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer res.Body.Close()
	n, err := io.Copy(io.Discard, res.Body)
	if err != nil {
		return res.StatusCode, int(n), err
	}
	if res.StatusCode >= 500 {
		return res.StatusCode, int(n), fmt.Errorf("upstream %d", res.StatusCode)
	}
	return res.StatusCode, int(n), nil
}

func stringList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
