package crawler

import (
	"database/sql"
	"time"

	"helios-backend/internal/db"
)

// QuotaTimezone is the wall clock used to partition per-day quota windows.
// UTC is chosen deliberately so the reset boundary is deterministic for
// tests and does not depend on the host's local zone.
var QuotaTimezone = time.UTC

// todayDate returns the current date in the quota timezone, zeroed at midnight.
func todayDate(now time.Time) time.Time {
	t := now.In(QuotaTimezone)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, QuotaTimezone)
}

// QuotaStatus describes a job's state against its daily cap.
type QuotaStatus struct {
	JobID             int64
	DailyQuota        int
	PagesFetchedToday int
	QuotaDate         time.Time
	ResetApplied      bool // true if a day rollover caused pages_fetched_today to be reset
	Available         int  // pages remaining for today
}

// CheckAndRollover looks at a job's (pages_fetched_today, quota_date) and
// resets the counter to 0 when the calendar date (in QuotaTimezone) has
// advanced. Returns the post-rollover status.
//
// Intended to be called by the worker before enqueuing another fetch, and
// by tests that need to simulate day-rollover behaviour (pass `now` to
// control the clock).
func CheckAndRollover(jobID int64, now time.Time) (QuotaStatus, error) {
	return CheckAndRolloverTx(db.DB, jobID, now)
}

type execQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func CheckAndRolloverTx(x execQuerier, jobID int64, now time.Time) (QuotaStatus, error) {
	var (
		dailyQuota int
		today      sql.NullInt32
		qDate      sql.NullTime
	)
	err := x.QueryRow(
		`SELECT daily_quota, pages_fetched_today, quota_date FROM crawl_jobs WHERE id = ?`,
		jobID,
	).Scan(&dailyQuota, &today, &qDate)
	if err != nil {
		return QuotaStatus{}, err
	}

	current := todayDate(now)
	status := QuotaStatus{
		JobID:      jobID,
		DailyQuota: dailyQuota,
		QuotaDate:  current,
	}
	if today.Valid {
		status.PagesFetchedToday = int(today.Int32)
	}

	needsReset := true
	if qDate.Valid {
		if todayDate(qDate.Time).Equal(current) {
			needsReset = false
		}
	}
	if needsReset {
		if _, err := x.Exec(
			`UPDATE crawl_jobs SET pages_fetched_today = 0, quota_date = ? WHERE id = ?`,
			current, jobID,
		); err != nil {
			return status, err
		}
		status.PagesFetchedToday = 0
		status.ResetApplied = true
	}

	status.Available = status.DailyQuota - status.PagesFetchedToday
	if status.DailyQuota == 0 {
		// A job with daily_quota=0 means "no daily cap" — treat as effectively infinite.
		status.Available = 1<<31 - 1
	}
	return status, nil
}

// IncrementToday atomically bumps pages_fetched_today for today's window,
// assuming a prior CheckAndRollover has already ensured quota_date is current.
// Returns the new counter value.
func IncrementToday(jobID int64) (int, error) {
	res, err := db.DB.Exec(
		`UPDATE crawl_jobs
		 SET pages_fetched_today = pages_fetched_today + 1,
		     pages_fetched = pages_fetched + 1
		 WHERE id = ?`,
		jobID,
	)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, sql.ErrNoRows
	}
	var v int
	if err := db.DB.QueryRow(
		`SELECT pages_fetched_today FROM crawl_jobs WHERE id = ?`, jobID,
	).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
