package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

func RegisterAudit(r *gin.RouterGroup) {
	g := r.Group("/audit-logs",
		auth.AuthRequired(),
		auth.RequireRole("administrator"),
	)
	g.GET("", listAuditLogs)
}

type auditRow struct {
	ID               int64           `json:"id"`
	ActorID          *int64          `json:"actor_id,omitempty"`
	ActorRole        string          `json:"actor_role,omitempty"`
	Action           string          `json:"action"`
	EntityType       string          `json:"entity_type"`
	EntityID         *int64          `json:"entity_id,omitempty"`
	Before           json.RawMessage `json:"before,omitempty"`
	After            json.RawMessage `json:"after,omitempty"`
	BatchID          string          `json:"batch_id,omitempty"`
	ApprovalStatus   string          `json:"approval_status"`
	ApprovalDeadline *time.Time      `json:"approval_deadline,omitempty"`
	ApprovedBy       *int64          `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time      `json:"approved_at,omitempty"`
	IPAddress        string          `json:"ip_address,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

func listAuditLogs(c *gin.Context) {
	p := readPaging(c)
	where := []string{}
	args := []any{}
	if v := c.Query("entity_type"); v != "" {
		where = append(where, "entity_type = ?")
		args = append(args, v)
	}
	if v := c.Query("action"); v != "" {
		where = append(where, "action = ?")
		args = append(args, v)
	}
	if v := c.Query("actor_id"); v != "" {
		where = append(where, "actor_id = ?")
		args = append(args, v)
	}
	if v := c.Query("batch_id"); v != "" {
		where = append(where, "batch_id = ?")
		args = append(args, v)
	}
	if v := c.Query("approval_status"); v != "" {
		where = append(where, "approval_status = ?")
		args = append(args, v)
	}
	if v := c.Query("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			where = append(where, "created_at >= ?")
			args = append(args, t)
		}
	}
	q := `SELECT id, actor_id, actor_role, action, entity_type, entity_id,
	        before_json, after_json, batch_id, approval_status,
	        approval_deadline, approved_by, approved_at, ip_address, created_at
	      FROM audit_logs`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()

	out := []auditRow{}
	for rows.Next() {
		var (
			r             auditRow
			actorID       sql.NullInt64
			actorRole     sql.NullString
			entityID      sql.NullInt64
			before, after sql.NullString
			batchID       sql.NullString
			deadline      sql.NullTime
			approvedBy    sql.NullInt64
			approvedAt    sql.NullTime
			ip            sql.NullString
		)
		if err := rows.Scan(&r.ID, &actorID, &actorRole, &r.Action, &r.EntityType,
			&entityID, &before, &after, &batchID, &r.ApprovalStatus,
			&deadline, &approvedBy, &approvedAt, &ip, &r.CreatedAt); err != nil {
			dbFail(c, err)
			return
		}
		if actorID.Valid {
			v := actorID.Int64
			r.ActorID = &v
		}
		if actorRole.Valid {
			r.ActorRole = actorRole.String
		}
		if entityID.Valid {
			v := entityID.Int64
			r.EntityID = &v
		}
		if before.Valid {
			r.Before = json.RawMessage(before.String)
		}
		if after.Valid {
			r.After = json.RawMessage(after.String)
		}
		if batchID.Valid {
			r.BatchID = batchID.String
		}
		if deadline.Valid {
			t := deadline.Time
			r.ApprovalDeadline = &t
		}
		if approvedBy.Valid {
			v := approvedBy.Int64
			r.ApprovedBy = &v
		}
		if approvedAt.Valid {
			t := approvedAt.Time
			r.ApprovedAt = &t
		}
		if ip.Valid {
			r.IPAddress = ip.String
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}
