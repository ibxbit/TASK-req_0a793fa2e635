package handlers

import (
	"net/http"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

type ArbitrationStatus struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Label      string `json:"label"`
	IsTerminal bool   `json:"is_terminal"`
	SortOrder  int    `json:"sort_order"`
}

func RegisterArbitration(r *gin.RouterGroup) {
	g := r.Group("/arbitration", auth.AuthRequired())
	g.GET("/statuses", listArbitrationStatuses)
}

func listArbitrationStatuses(c *gin.Context) {
	rows, err := db.DB.Query(
		`SELECT id, code, label, is_terminal, sort_order
		 FROM arbitration_status ORDER BY sort_order, id`)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []ArbitrationStatus{}
	for rows.Next() {
		var s ArbitrationStatus
		var term int
		if err := rows.Scan(&s.ID, &s.Code, &s.Label, &term, &s.SortOrder); err != nil {
			dbFail(c, err)
			return
		}
		s.IsTerminal = term != 0
		out = append(out, s)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}
