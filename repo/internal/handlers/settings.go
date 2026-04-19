package handlers

import (
	"net/http"

	"helios-backend/internal/auth"
	"helios-backend/internal/settings"

	"github.com/gin-gonic/gin"
)

func RegisterSettings(r *gin.RouterGroup) {
	g := r.Group("/settings", auth.AuthRequired())
	g.GET("/approval", getApproval)
	g.PUT("/approval", auth.RequireRole("administrator"), setApproval)
}

func getApproval(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"approval_required": settings.ApprovalRequired(),
	})
}

type approvalSettingReq struct {
	Enabled bool `json:"enabled"`
}

func setApproval(c *gin.Context) {
	var req approvalSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := settings.SetApprovalRequired(req.Enabled); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"approval_required": req.Enabled})
}
