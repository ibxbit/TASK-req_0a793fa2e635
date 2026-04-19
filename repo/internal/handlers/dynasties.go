package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"helios-backend/internal/audit"
	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

const entityDynasty = "dynasty"

type Dynasty struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	StartYear   *int    `json:"start_year,omitempty"`
	EndYear     *int    `json:"end_year,omitempty"`
	Description *string `json:"description,omitempty"`
}

func RegisterDynasties(r *gin.RouterGroup) {
	g := r.Group("/dynasties", auth.AuthRequired())
	g.GET("", listDynasties)
	g.GET("/:id", getDynasty)

	w := g.Group("", auth.RequireRole("administrator", "content_editor"))
	w.POST("", createDynasty)
	w.PUT("/:id", updateDynasty)
	w.DELETE("/:id", deleteDynasty)
	w.POST("/bulk", bulkDynasties)
}

func scanDynasty(row interface{ Scan(...any) error }) (*Dynasty, error) {
	var d Dynasty
	var sy, ey sql.NullInt32
	var desc sql.NullString
	if err := row.Scan(&d.ID, &d.Name, &sy, &ey, &desc); err != nil {
		return nil, err
	}
	if sy.Valid {
		v := int(sy.Int32)
		d.StartYear = &v
	}
	if ey.Valid {
		v := int(ey.Int32)
		d.EndYear = &v
	}
	if desc.Valid {
		d.Description = &desc.String
	}
	return &d, nil
}

func listDynasties(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(
		`SELECT id, name, start_year, end_year, description FROM dynasties ORDER BY id LIMIT ? OFFSET ?`,
		p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Dynasty{}
	for rows.Next() {
		d, err := scanDynasty(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *d)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getDynasty(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	d, err := scanDynasty(db.DB.QueryRow(
		`SELECT id, name, start_year, end_year, description FROM dynasties WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

func createDynasty(c *gin.Context) {
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateGeometry(raw); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var in Dynasty
	if err := json.Unmarshal(raw, &in); err != nil || in.Name == "" {
		fail(c, http.StatusBadRequest, "name required")
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO dynasties (name, start_year, end_year, description) VALUES (?, ?, ?, ?)`,
		in.Name, nullableInt(in.StartYear), nullableInt(in.EndYear), nullableString(in.Description))
	if err != nil {
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	in.ID = id

	if err := audit.WriteCtx(c, tx, audit.ActionCreate, entityDynasty, id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateDynasty(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateGeometry(raw); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var in Dynasty
	if err := json.Unmarshal(raw, &in); err != nil {
		fail(c, http.StatusBadRequest, "invalid payload")
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()

	before, err := scanDynasty(tx.QueryRow(
		`SELECT id, name, start_year, end_year, description FROM dynasties WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if in.Name == "" {
		in.Name = before.Name
	}
	if _, err := tx.Exec(
		`UPDATE dynasties SET name=?, start_year=?, end_year=?, description=? WHERE id=?`,
		in.Name, nullableInt(in.StartYear), nullableInt(in.EndYear), nullableString(in.Description), id,
	); err != nil {
		dbFail(c, err)
		return
	}
	in.ID = id
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, entityDynasty, id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteDynasty(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()

	before, err := scanDynasty(tx.QueryRow(
		`SELECT id, name, start_year, end_year, description FROM dynasties WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM dynasties WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	batchID, needsApproval := newApprovalContext()
	if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityDynasty, id, before, nil, batchID, needsApproval); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	resp := gin.H{"deleted": id}
	if m := approvalResponseMeta(batchID, needsApproval); m != nil {
		resp["approval"] = m
	}
	c.JSON(http.StatusOK, resp)
}

type bulkDynastyReq struct {
	Create []Dynasty `json:"create"`
	Update []Dynasty `json:"update"`
	Delete []int64   `json:"delete"`
}

func bulkDynasties(c *gin.Context) {
	var req bulkDynastyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()

	batchID, needsApproval := newApprovalContext()

	created := []Dynasty{}
	for _, d := range req.Create {
		if d.Name == "" {
			fail(c, http.StatusBadRequest, "name required in create item")
			return
		}
		res, err := tx.Exec(
			`INSERT INTO dynasties (name, start_year, end_year, description) VALUES (?, ?, ?, ?)`,
			d.Name, nullableInt(d.StartYear), nullableInt(d.EndYear), nullableString(d.Description))
		if err != nil {
			dbFail(c, err)
			return
		}
		id, _ := res.LastInsertId()
		d.ID = id
		if err := audit.WriteCtxApproval(c, tx, audit.ActionCreate, entityDynasty, id, nil, d, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
		created = append(created, d)
	}

	for _, d := range req.Update {
		if d.ID <= 0 {
			fail(c, http.StatusBadRequest, "update requires id")
			return
		}
		before, err := scanDynasty(tx.QueryRow(
			`SELECT id, name, start_year, end_year, description FROM dynasties WHERE id = ? FOR UPDATE`, d.ID))
		if err == sql.ErrNoRows {
			fail(c, http.StatusNotFound, "not found in update")
			return
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if d.Name == "" {
			d.Name = before.Name
		}
		if _, err := tx.Exec(
			`UPDATE dynasties SET name=?, start_year=?, end_year=?, description=? WHERE id=?`,
			d.Name, nullableInt(d.StartYear), nullableInt(d.EndYear), nullableString(d.Description), d.ID,
		); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionUpdate, entityDynasty, d.ID, before, d, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}

	for _, id := range req.Delete {
		before, err := scanDynasty(tx.QueryRow(
			`SELECT id, name, start_year, end_year, description FROM dynasties WHERE id = ? FOR UPDATE`, id))
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if _, err := tx.Exec(`DELETE FROM dynasties WHERE id = ?`, id); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityDynasty, id, before, nil, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	resp := gin.H{
		"created": created,
		"updated": len(req.Update),
		"deleted": len(req.Delete),
	}
	if m := approvalResponseMeta(batchID, needsApproval); m != nil {
		resp["approval"] = m
	}
	c.JSON(http.StatusOK, resp)
}
