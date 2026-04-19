package handlers

import (
	"net/http"

	"helios-backend/internal/approval"
	"helios-backend/internal/auth"

	"github.com/gin-gonic/gin"
)

func RegisterApprovals(r *gin.RouterGroup) {
	g := r.Group("/approvals", auth.AuthRequired(), auth.RequireRole("administrator"))
	g.GET("", listApprovals)
	g.POST("/:batch_id/approve", approveBatch)
	g.POST("/:batch_id/reject", rejectBatch)
}

func listApprovals(c *gin.Context) {
	items, err := approval.ListPending()
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func approveBatch(c *gin.Context) {
	batchID := c.Param("batch_id")
	if batchID == "" {
		fail(c, http.StatusBadRequest, "batch_id required")
		return
	}
	sess, _ := auth.CurrentSession(c)
	n, err := approval.Approve(batchID, sess.UserID)
	if err != nil {
		dbFail(c, err)
		return
	}
	if n == 0 {
		fail(c, http.StatusNotFound, "no pending entries for batch")
		return
	}
	c.JSON(http.StatusOK, gin.H{"approved": n, "batch_id": batchID})
}

func rejectBatch(c *gin.Context) {
	batchID := c.Param("batch_id")
	if batchID == "" {
		fail(c, http.StatusBadRequest, "batch_id required")
		return
	}
	sess, _ := auth.CurrentSession(c)
	n, err := approval.Reject(batchID, sess.UserID)
	if err != nil {
		dbFail(c, err)
		return
	}
	if n == 0 {
		fail(c, http.StatusNotFound, "no pending entries for batch")
		return
	}
	c.JSON(http.StatusOK, gin.H{"reverted": n, "batch_id": batchID})
}
