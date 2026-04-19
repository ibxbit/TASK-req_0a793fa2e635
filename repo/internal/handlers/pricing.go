package handlers

import (
	"net/http"

	"helios-backend/internal/auth"
	"helios-backend/internal/pricing"

	"github.com/gin-gonic/gin"
)

func RegisterPricing(r *gin.RouterGroup) {
	g := r.Group("/pricing", auth.AuthRequired())
	g.POST("/quote", quoteHandler)
}

func quoteHandler(c *gin.Context) {
	var req pricing.QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	if len(req.Items) == 0 {
		fail(c, http.StatusBadRequest, "items required")
		return
	}
	// Never trust a client-supplied user_id for member/discount eligibility:
	// always derive it from the authenticated session so attackers can't quote
	// as another member. Marketing managers may override for back-office quotes.
	sess, _ := auth.CurrentSession(c)
	if sess != nil {
		if req.UserID != nil && *req.UserID != sess.UserID && sess.RoleName != "marketing_manager" && sess.RoleName != "administrator" {
			fail(c, http.StatusForbidden, "cannot quote for another user")
			return
		}
		if req.UserID == nil {
			uid := sess.UserID
			req.UserID = &uid
		}
	}

	res, err := pricing.Quote(req)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, res)
}
