package crawler

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// openTestDB opens a real MySQL connection for quota tests. Tests are skipped
// when HELIOS_DB_DSN is not set, enabling local unit runs without a live DB.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("HELIOS_DB_DSN")
	if dsn == "" {
		t.Skip("HELIOS_DB_DSN not set; skipping DB-backed quota tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// insertTestJob seeds a crawl_jobs row and registers cleanup. quotaDate may be
// nil to leave the column NULL (first-run scenario).
func insertTestJob(t *testing.T, db *sql.DB, dailyQuota, pagesToday int, quotaDate *time.Time) int64 {
	t.Helper()
	var qd any
	if quotaDate != nil {
		qd = *quotaDate
	}
	res, err := db.Exec(
		`INSERT INTO crawl_jobs (job_name, daily_quota, pages_fetched_today, quota_date)
		 VALUES (?, ?, ?, ?)`,
		fmt.Sprintf("test_quota_%d", time.Now().UnixNano()), dailyQuota, pagesToday, qd,
	)
	if err != nil {
		t.Fatalf("insert test job: %v", err)
	}
	id, _ := res.LastInsertId()
	t.Cleanup(func() { db.Exec(`DELETE FROM crawl_jobs WHERE id = ?`, id) })
	return id
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

// TestQuota_FirstRunResetsCounterAndSetsDate: NULL quota_date → reset applied,
// DB row updated, Available reflects full daily quota.
func TestQuota_FirstRunResetsCounterAndSetsDate(t *testing.T) {
	db := openTestDB(t)
	id := insertTestJob(t, db, 100, 0, nil)

	now := day(2026, 4, 19)
	status, err := CheckAndRolloverTx(db, id, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.ResetApplied {
		t.Fatalf("first call with NULL quota_date should trigger reset; got %+v", status)
	}
	if status.PagesFetchedToday != 0 || status.Available != 100 {
		t.Fatalf("unexpected status after first run: %+v", status)
	}

	// Verify the DB row was actually updated.
	var pToday int
	var qDate sql.NullTime
	if err := db.QueryRow(
		`SELECT pages_fetched_today, quota_date FROM crawl_jobs WHERE id = ?`, id,
	).Scan(&pToday, &qDate); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if pToday != 0 {
		t.Fatalf("pages_fetched_today should be 0 in DB after reset, got %d", pToday)
	}
	if !qDate.Valid {
		t.Fatalf("quota_date should be set in DB after reset")
	}
}

// TestQuota_SameDayDoesNotReset: quota_date already equals today → no UPDATE,
// counter preserved, Available = quota - fetched.
func TestQuota_SameDayDoesNotReset(t *testing.T) {
	db := openTestDB(t)
	today := day(2026, 4, 19)
	id := insertTestJob(t, db, 100, 42, &today)

	status, err := CheckAndRolloverTx(db, id, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.ResetApplied {
		t.Fatalf("same-day run must NOT reset; got %+v", status)
	}
	if status.PagesFetchedToday != 42 || status.Available != 58 {
		t.Fatalf("unexpected status on same-day run: %+v", status)
	}

	// Confirm DB counter was not touched.
	var pToday int
	if err := db.QueryRow(
		`SELECT pages_fetched_today FROM crawl_jobs WHERE id = ?`, id,
	).Scan(&pToday); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if pToday != 42 {
		t.Fatalf("pages_fetched_today should remain 42, got %d", pToday)
	}
}

// TestQuota_DayRolloverResetsToZero: quota_date is yesterday → counter reset to
// 0 in DB, Available = full daily quota.
func TestQuota_DayRolloverResetsToZero(t *testing.T) {
	db := openTestDB(t)
	yesterday := day(2026, 4, 18)
	today := day(2026, 4, 19)
	id := insertTestJob(t, db, 100, 99, &yesterday)

	status, err := CheckAndRolloverTx(db, id, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.ResetApplied {
		t.Fatalf("day rollover must trigger reset; got %+v", status)
	}
	if status.PagesFetchedToday != 0 || status.Available != 100 {
		t.Fatalf("unexpected status after rollover: %+v", status)
	}

	var pToday int
	if err := db.QueryRow(
		`SELECT pages_fetched_today FROM crawl_jobs WHERE id = ?`, id,
	).Scan(&pToday); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if pToday != 0 {
		t.Fatalf("pages_fetched_today should be 0 in DB after rollover, got %d", pToday)
	}
}

// TestQuota_CapHitStopsFurtherWork: pages_fetched_today == daily_quota → Available = 0.
func TestQuota_CapHitStopsFurtherWork(t *testing.T) {
	db := openTestDB(t)
	today := day(2026, 4, 19)
	id := insertTestJob(t, db, 100, 100, &today)

	status, err := CheckAndRolloverTx(db, id, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Available != 0 {
		t.Fatalf("available should be 0 at cap, got %d", status.Available)
	}
	if !(status.DailyQuota > 0 && status.PagesFetchedToday >= status.DailyQuota) {
		t.Fatalf("caller gate condition not satisfied: %+v", status)
	}
}

// TestQuota_ZeroMeansUnlimited: daily_quota = 0 → Available = MaxInt32 regardless
// of pages_fetched_today.
func TestQuota_ZeroMeansUnlimited(t *testing.T) {
	db := openTestDB(t)
	today := day(2026, 4, 19)
	id := insertTestJob(t, db, 0, 1_000_000, &today)

	status, err := CheckAndRolloverTx(db, id, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Available != 1<<31-1 {
		t.Fatalf("daily_quota=0 should yield unlimited Available (%d), got %d", 1<<31-1, status.Available)
	}
}

func TestTodayDate_IsMidnightUTC(t *testing.T) {
	got := todayDate(day(2026, 4, 19))
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Fatalf("todayDate should zero time of day: %v", got)
	}
	if got.Location() != time.UTC {
		t.Fatalf("todayDate should be UTC: %v", got.Location())
	}
}
