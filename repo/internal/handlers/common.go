package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"helios-backend/internal/audit"
	"helios-backend/internal/settings"
	"helios-backend/internal/validation"

	"github.com/gin-gonic/gin"
)

// newApprovalContext returns (batch_id, needsApproval) for delete/bulk ops.
func newApprovalContext() (string, bool) {
	return audit.NewBatchID(), settings.ApprovalRequired()
}

// approvalResponseMeta returns the response metadata describing approval state.
func approvalResponseMeta(batchID string, needsApproval bool) gin.H {
	if !needsApproval {
		return nil
	}
	return gin.H{"status": "pending", "batch_id": batchID, "window_hours": 48}
}

type paging struct {
	Limit  int
	Offset int
}

func readPaging(c *gin.Context) paging {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return paging{Limit: limit, Offset: offset}
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func nullableInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nullableString(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func nullableInt(p *int) sql.NullInt32 {
	if p == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*p), Valid: true}
}

// validateGeometry is a defensive hook: any payload that includes a
// "geometry" key gets run through the spatial validator.
func validateGeometry(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var wrap struct {
		Geometry json.RawMessage `json:"geometry"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil // not an object; nothing to check
	}
	return validation.ValidateGeoJSON(wrap.Geometry)
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}

// dbFail logs the raw driver error server-side and returns a redacted
// 500 response so we never leak SQL state or schema details to clients.
func dbFail(c *gin.Context, err error) {
	log.Printf("db error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	fail(c, http.StatusInternalServerError, "database error")
}
