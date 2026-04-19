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

const (
	entityTag = "tag"
	tagKind   = "tag"
)

type Tag struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	ParentID    *int64  `json:"parent_id,omitempty"`
	Description *string `json:"description,omitempty"`
}

func RegisterTags(r *gin.RouterGroup) {
	g := r.Group("/tags", auth.AuthRequired())
	g.GET("", listTags)
	g.GET("/:id", getTag)

	w := g.Group("", auth.RequireRole("administrator", "content_editor"))
	w.POST("", createTag)
	w.PUT("/:id", updateTag)
	w.DELETE("/:id", deleteTag)
	w.POST("/bulk", bulkTags)
}

const tagCols = `id, name, parent_id, description`

func scanTag(row interface{ Scan(...any) error }) (*Tag, error) {
	var t Tag
	var pID sql.NullInt64
	var desc sql.NullString
	if err := row.Scan(&t.ID, &t.Name, &pID, &desc); err != nil {
		return nil, err
	}
	if pID.Valid {
		v := pID.Int64
		t.ParentID = &v
	}
	if desc.Valid {
		t.Description = &desc.String
	}
	return &t, nil
}

func listTags(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(
		`SELECT `+tagCols+` FROM genres WHERE kind=? ORDER BY id LIMIT ? OFFSET ?`,
		tagKind, p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *t)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getTag(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := scanTag(db.DB.QueryRow(`SELECT `+tagCols+` FROM genres WHERE id=? AND kind=?`, id, tagKind))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func insertTag(tx *sql.Tx, t *Tag) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO genres (name, kind, parent_id, description) VALUES (?, ?, ?, ?)`,
		t.Name, tagKind, nullableInt64(t.ParentID), nullableString(t.Description))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateTagRow(tx *sql.Tx, t *Tag) error {
	_, err := tx.Exec(
		`UPDATE genres SET name=?, parent_id=?, description=? WHERE id=? AND kind=?`,
		t.Name, nullableInt64(t.ParentID), nullableString(t.Description), t.ID, tagKind)
	return err
}

func createTag(c *gin.Context) {
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateGeometry(raw); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var in Tag
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
	id, err := insertTag(tx, &in)
	if err != nil {
		dbFail(c, err)
		return
	}
	in.ID = id
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, entityTag, id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateTag(c *gin.Context) {
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
	var in Tag
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
	before, err := scanTag(tx.QueryRow(
		`SELECT `+tagCols+` FROM genres WHERE id=? AND kind=? FOR UPDATE`, id, tagKind))
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
	in.ID = id
	if err := updateTagRow(tx, &in); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, entityTag, id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteTag(c *gin.Context) {
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
	before, err := scanTag(tx.QueryRow(
		`SELECT `+tagCols+` FROM genres WHERE id=? AND kind=? FOR UPDATE`, id, tagKind))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM genres WHERE id=? AND kind=?`, id, tagKind); err != nil {
		dbFail(c, err)
		return
	}
	batchID, needsApproval := newApprovalContext()
	if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityTag, id, before, nil, batchID, needsApproval); err != nil {
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

type bulkTagReq struct {
	Create []Tag   `json:"create"`
	Update []Tag   `json:"update"`
	Delete []int64 `json:"delete"`
}

func bulkTags(c *gin.Context) {
	var req bulkTagReq
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

	created := []Tag{}
	for _, t := range req.Create {
		if t.Name == "" {
			fail(c, http.StatusBadRequest, "name required in create item")
			return
		}
		id, err := insertTag(tx, &t)
		if err != nil {
			dbFail(c, err)
			return
		}
		t.ID = id
		if err := audit.WriteCtxApproval(c, tx, audit.ActionCreate, entityTag, id, nil, t, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
		created = append(created, t)
	}
	for _, t := range req.Update {
		if t.ID <= 0 {
			fail(c, http.StatusBadRequest, "update requires id")
			return
		}
		before, err := scanTag(tx.QueryRow(
			`SELECT `+tagCols+` FROM genres WHERE id=? AND kind=? FOR UPDATE`, t.ID, tagKind))
		if err == sql.ErrNoRows {
			fail(c, http.StatusNotFound, "not found in update")
			return
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if t.Name == "" {
			t.Name = before.Name
		}
		if err := updateTagRow(tx, &t); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionUpdate, entityTag, t.ID, before, t, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}
	for _, id := range req.Delete {
		before, err := scanTag(tx.QueryRow(
			`SELECT `+tagCols+` FROM genres WHERE id=? AND kind=? FOR UPDATE`, id, tagKind))
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if _, err := tx.Exec(`DELETE FROM genres WHERE id=? AND kind=?`, id, tagKind); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityTag, id, before, nil, batchID, needsApproval); err != nil {
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
