package approval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"helios-backend/internal/db"
)

const sweepInterval = 1 * time.Minute

type PendingBatch struct {
	BatchID   string    `json:"batch_id"`
	Entries   int       `json:"entries"`
	CreatedAt time.Time `json:"created_at"`
	Deadline  time.Time `json:"deadline"`
	Actions   []string  `json:"actions"`
}

func ListPending() ([]PendingBatch, error) {
	rows, err := db.DB.Query(`
		SELECT batch_id,
		       COUNT(*)            AS n,
		       MIN(created_at)     AS created_at,
		       MIN(approval_deadline) AS deadline,
		       GROUP_CONCAT(DISTINCT entity_type, ':', action) AS actions
		FROM audit_logs
		WHERE approval_status = 'pending' AND batch_id IS NOT NULL
		GROUP BY batch_id
		ORDER BY MIN(created_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingBatch
	for rows.Next() {
		var p PendingBatch
		var actions sql.NullString
		var deadline sql.NullTime
		if err := rows.Scan(&p.BatchID, &p.Entries, &p.CreatedAt, &deadline, &actions); err != nil {
			return nil, err
		}
		if deadline.Valid {
			p.Deadline = deadline.Time
		}
		if actions.Valid {
			p.Actions = splitActions(actions.String)
		}
		out = append(out, p)
	}
	return out, nil
}

func splitActions(s string) []string {
	out := []string{}
	cur := ""
	for _, ch := range s {
		if ch == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// Approve seals the batch — changes remain in effect.
func Approve(batchID string, approverID int64) (int64, error) {
	res, err := db.DB.Exec(`
		UPDATE audit_logs
		SET approval_status='approved', approved_by=?, approved_at=NOW()
		WHERE batch_id=? AND approval_status='pending'`,
		approverID, batchID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Reject reverts the batch immediately.
func Reject(batchID string, approverID int64) (int64, error) {
	return revertBatchInternal(batchID, approverID, "rejected")
}

// AutoRevertExpired reverts all pending batches whose approval_deadline has passed.
func AutoRevertExpired() error {
	rows, err := db.DB.Query(`
		SELECT DISTINCT batch_id FROM audit_logs
		WHERE approval_status = 'pending'
		  AND approval_deadline IS NOT NULL
		  AND approval_deadline < NOW()
		  AND batch_id IS NOT NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return err
		}
		ids = append(ids, b)
	}
	for _, b := range ids {
		if _, err := revertBatchInternal(b, 0, "reverted"); err != nil {
			log.Printf("auto-revert batch %s: %v", b, err)
		} else {
			log.Printf("auto-reverted batch %s (approval deadline exceeded)", b)
		}
	}
	return nil
}

// finalStatus is either "rejected" (admin decision) or "reverted" (timeout).
func revertBatchInternal(batchID string, approverID int64, finalStatus string) (int64, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Fetch entries — unwind in reverse creation order so dependent rows exist.
	rows, err := tx.Query(`
		SELECT id, entity_type, action, before_json, after_json
		FROM audit_logs
		WHERE batch_id = ? AND approval_status = 'pending'
		ORDER BY id DESC`,
		batchID)
	if err != nil {
		return 0, err
	}

	type row struct {
		ID         int64
		EntityType string
		Action     string
		Before     sql.NullString
		After      sql.NullString
	}
	var entries []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.EntityType, &r.Action, &r.Before, &r.After); err != nil {
			rows.Close()
			return 0, err
		}
		entries = append(entries, r)
	}
	rows.Close()

	for _, e := range entries {
		var before, after json.RawMessage
		if e.Before.Valid {
			before = json.RawMessage(e.Before.String)
		}
		if e.After.Valid {
			after = json.RawMessage(e.After.String)
		}
		if err := revertOne(tx, e.EntityType, e.Action, before, after); err != nil {
			return 0, fmt.Errorf("revert entry %d (%s/%s): %w", e.ID, e.EntityType, e.Action, err)
		}
		if approverID > 0 {
			if _, err := tx.Exec(
				`UPDATE audit_logs SET approval_status=?, approved_by=?, approved_at=NOW() WHERE id=?`,
				finalStatus, approverID, e.ID); err != nil {
				return 0, err
			}
		} else {
			if _, err := tx.Exec(
				`UPDATE audit_logs SET approval_status=?, approved_at=NOW() WHERE id=?`,
				finalStatus, e.ID); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(entries)), nil
}

// StartScheduler runs the auto-revert sweeper in the background.
func StartScheduler() {
	go func() {
		t := time.NewTicker(sweepInterval)
		defer t.Stop()
		if err := AutoRevertExpired(); err != nil {
			log.Printf("approval sweeper: %v", err)
		}
		for range t.C {
			if err := AutoRevertExpired(); err != nil {
				log.Printf("approval sweeper: %v", err)
			}
		}
	}()
	log.Println("approval auto-revert scheduler started (interval=1m, window=48h)")
}
