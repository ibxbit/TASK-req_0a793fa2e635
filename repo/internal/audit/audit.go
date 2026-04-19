package audit

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"helios-backend/internal/auth"

	"github.com/gin-gonic/gin"
)

const (
	retentionDays    = 30
	approvalWindow   = 48 * time.Hour
	StatusNotRequired = "not_required"
	StatusPending     = "pending"
	StatusApproved    = "approved"
	StatusRejected    = "rejected"
	StatusReverted    = "reverted"
)

type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionRestore Action = "restore"
)

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type Entry struct {
	Actor         *auth.Session
	Action        Action
	EntityType    string
	EntityID      int64
	Before        any
	After         any
	IP            string
	UserAgent     string
	Reason        string
	BatchID       string
	NeedsApproval bool
}

func NewBatchID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Write(x execer, e Entry) error {
	var actorID sql.NullInt64
	var actorRole sql.NullString
	if e.Actor != nil {
		actorID = sql.NullInt64{Int64: e.Actor.UserID, Valid: true}
		actorRole = sql.NullString{String: e.Actor.RoleName, Valid: true}
	}

	beforeJSON, err := toJSON(e.Before)
	if err != nil {
		return err
	}
	afterJSON, err := toJSON(e.After)
	if err != nil {
		return err
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(retentionDays) * 24 * time.Hour)

	var batchID sql.NullString
	if e.BatchID != "" {
		batchID = sql.NullString{String: e.BatchID, Valid: true}
	}

	approvalStatus := StatusNotRequired
	var approvalDeadline sql.NullTime
	if e.NeedsApproval {
		approvalStatus = StatusPending
		approvalDeadline = sql.NullTime{Time: now.Add(approvalWindow), Valid: true}
	}

	_, err = x.Exec(`
		INSERT INTO audit_logs
		  (actor_id, actor_role, action, entity_type, entity_id,
		   before_json, after_json, ip_address, user_agent, reason, expires_at,
		   batch_id, approval_status, approval_deadline)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		actorID, actorRole, string(e.Action), e.EntityType, e.EntityID,
		beforeJSON, afterJSON, nullStr(e.IP), nullStr(e.UserAgent), nullStr(e.Reason), expiresAt,
		batchID, approvalStatus, approvalDeadline,
	)
	return err
}

// WriteCtx logs a single action with no batch and no approval requirement.
func WriteCtx(c *gin.Context, x execer, action Action, entityType string, entityID int64, before, after any) error {
	sess, _ := auth.CurrentSession(c)
	return Write(x, Entry{
		Actor:      sess,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Before:     before,
		After:      after,
		IP:         c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})
}

// WriteCtxApproval logs an action that may require approval. Pass a batch_id
// (generate via NewBatchID) and set needsApproval per settings.
func WriteCtxApproval(c *gin.Context, x execer, action Action, entityType string, entityID int64, before, after any, batchID string, needsApproval bool) error {
	sess, _ := auth.CurrentSession(c)
	return Write(x, Entry{
		Actor:         sess,
		Action:        action,
		EntityType:    entityType,
		EntityID:      entityID,
		Before:        before,
		After:         after,
		IP:            c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
		BatchID:       batchID,
		NeedsApproval: needsApproval,
	})
}

func toJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
