package handlers

// Revision history + restore workflow.
//
// Exposes two endpoints:
//
//   GET  /api/v1/revisions?entity_type=&entity_id=
//     List the audit_logs entries (revisions) for a given entity, newest
//     first. Only entries whose `expires_at` is still in the future are
//     returned — beyond 30 days the record is considered purged and no
//     longer restorable. Admin-only.
//
//   POST /api/v1/revisions/:id/restore
//     Apply the `before` snapshot of the given audit row to the current
//     state of the target entity, undoing whatever that audit entry
//     recorded (for an update: reinstate the before-state; for a delete:
//     recreate the row; for a create: delete it).
//
//     Enforces the 30-day retention window: revisions that have expired
//     are not restorable even if the row still exists in audit_logs.
//
//     Writes a fresh `action=restore` audit entry so the restore itself
//     is auditable and visible in the history for the same entity.
//
// This is the "rollback to any prior revision within 30 days" capability
// that was flagged as a blocker — it is broader than the approval-batch
// auto-revert, which is limited to the pending-approval workflow.

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"helios-backend/internal/approval"
	"helios-backend/internal/audit"
	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

// RetentionDays is the restore window. Entries older than this are purged
// from the user's perspective — the SQL row may still exist, but restore
// will refuse to act on it.
const RetentionDays = 30

type Revision struct {
	ID             int64           `json:"id"`
	ActorID        *int64          `json:"actor_id,omitempty"`
	ActorRole      string          `json:"actor_role,omitempty"`
	Action         string          `json:"action"`
	EntityType     string          `json:"entity_type"`
	EntityID       int64           `json:"entity_id"`
	Before         json.RawMessage `json:"before,omitempty"`
	After          json.RawMessage `json:"after,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Restorable     bool            `json:"restorable"`
	BatchID        string          `json:"batch_id,omitempty"`
	ApprovalStatus string          `json:"approval_status,omitempty"`
}

func RegisterRevisions(r *gin.RouterGroup) {
	// The restore capability is admin-only — matches the scope of the
	// audit log reader. Listing is also admin-only since the payload
	// contains full before/after snapshots which may include PII.
	g := r.Group("/revisions", auth.AuthRequired(), auth.RequireRole("administrator"))
	g.GET("", listRevisions)
	g.GET("/supported-entities", supportedEntities)
	g.POST("/:id/restore", restoreRevision)
}

func supportedEntities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"items":          approval.SupportedEntityTypes(),
		"retention_days": RetentionDays,
	})
}

func listRevisions(c *gin.Context) {
	entityType := c.Query("entity_type")
	if entityType == "" {
		fail(c, http.StatusBadRequest, "entity_type required")
		return
	}
	entityIDStr := c.Query("entity_id")
	if entityIDStr == "" {
		fail(c, http.StatusBadRequest, "entity_id required")
		return
	}
	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil || entityID <= 0 {
		fail(c, http.StatusBadRequest, "entity_id must be a positive integer")
		return
	}
	p := readPaging(c)

	rows, err := db.DB.Query(`
		SELECT id, actor_id, actor_role, action, entity_type, entity_id,
		       before_json, after_json, COALESCE(reason, ''), created_at, expires_at,
		       COALESCE(batch_id, ''), approval_status
		FROM audit_logs
		WHERE entity_type = ? AND entity_id = ? AND expires_at > NOW()
		ORDER BY id DESC
		LIMIT ? OFFSET ?`,
		entityType, entityID, p.Limit, p.Offset,
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()

	supported := approval.IsSupportedEntity(entityType)

	out := []Revision{}
	for rows.Next() {
		var r Revision
		var actor sql.NullInt64
		var role sql.NullString
		var before, after sql.NullString
		if err := rows.Scan(
			&r.ID, &actor, &role, &r.Action, &r.EntityType, &r.EntityID,
			&before, &after, &r.Reason, &r.CreatedAt, &r.ExpiresAt,
			&r.BatchID, &r.ApprovalStatus,
		); err != nil {
			dbFail(c, err)
			return
		}
		if actor.Valid {
			v := actor.Int64
			r.ActorID = &v
		}
		if role.Valid {
			r.ActorRole = role.String
		}
		if before.Valid {
			r.Before = json.RawMessage(before.String)
		}
		if after.Valid {
			r.After = json.RawMessage(after.String)
		}
		// Restore is only meaningful for rows whose entity type has a
		// reverter registered and whose action is one of create/update/delete.
		r.Restorable = supported && isRestorableAction(r.Action)
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{
		"items":          out,
		"limit":          p.Limit,
		"offset":         p.Offset,
		"retention_days": RetentionDays,
	})
}

func isRestorableAction(a string) bool {
	switch a {
	case string(audit.ActionCreate), string(audit.ActionUpdate), string(audit.ActionDelete):
		return true
	}
	return false
}

// restoreRevision re-applies the `before` snapshot of the selected audit
// row. For `create` audits the row is removed again; for `update` audits
// the original column values are written back; for `delete` audits the
// row is re-inserted.
func restoreRevision(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	// Fetch the audit row (no JOIN; pull everything we need in one shot).
	var (
		entityType     string
		entityID       int64
		action         string
		beforeJSON     sql.NullString
		afterJSON      sql.NullString
		createdAt      time.Time
		expiresAt      time.Time
		approvalStatus string
	)
	err := db.DB.QueryRow(`
		SELECT entity_type, entity_id, action, before_json, after_json,
		       created_at, expires_at, approval_status
		FROM audit_logs WHERE id = ?`, id,
	).Scan(&entityType, &entityID, &action, &beforeJSON, &afterJSON, &createdAt, &expiresAt, &approvalStatus)
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "revision not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}

	// 30-day window gate. We trust expires_at (set by audit.Write) rather
	// than computing from created_at so a future retention policy change
	// takes effect uniformly.
	if expiresAt.Before(time.Now()) {
		fail(c, http.StatusGone, "revision expired: beyond retention window")
		return
	}
	// Double-check: created_at also within 30 days, for defence in depth.
	if time.Since(createdAt) > time.Duration(RetentionDays)*24*time.Hour {
		fail(c, http.StatusGone, "revision expired: beyond retention window")
		return
	}

	if !approval.IsSupportedEntity(entityType) {
		fail(c, http.StatusBadRequest, "entity_type not restorable: "+entityType)
		return
	}
	if !isRestorableAction(action) {
		fail(c, http.StatusBadRequest, "action not restorable: "+action)
		return
	}
	// A pending audit entry is still subject to the approval workflow; do
	// not let a manual restore race the approval decision.
	if approvalStatus == audit.StatusPending {
		fail(c, http.StatusConflict, "revision is pending approval; approve or reject first")
		return
	}

	var before, after json.RawMessage
	if beforeJSON.Valid {
		before = json.RawMessage(beforeJSON.String)
	}
	if afterJSON.Valid {
		after = json.RawMessage(afterJSON.String)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()

	if err := approval.RestoreRevision(tx, entityType, action, before, after); err != nil {
		// The reverter can fail if the current state is inconsistent
		// (e.g. row already deleted in another session). Surface the
		// specific error to the admin.
		fail(c, http.StatusConflict, "restore failed: "+err.Error())
		return
	}

	// Record the restore itself. "before" is what the entity looked like
	// per the chosen revision; "after" is what we just restored to —
	// which for an update/delete revision is the `before` snapshot, and
	// for a create revision is nil (the row was deleted).
	restoredAfter := before
	if action == string(audit.ActionCreate) {
		restoredAfter = nil
	}
	if err := audit.WriteCtx(c, tx, audit.ActionRestore, entityType, entityID, after, restoredAfter); err != nil {
		dbFail(c, err)
		return
	}

	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"restored_revision_id": id,
		"entity_type":          entityType,
		"entity_id":            entityID,
		"action_restored":      action,
	})
}
