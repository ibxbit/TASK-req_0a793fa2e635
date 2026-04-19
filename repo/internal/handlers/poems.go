package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"helios-backend/internal/audit"
	"helios-backend/internal/auth"
	"helios-backend/internal/db"
	"helios-backend/internal/search"

	"github.com/gin-gonic/gin"
)

const entityPoem = "poem"

type Poem struct {
	ID              int64   `json:"id"`
	Title           string  `json:"title"`
	AuthorID        *int64  `json:"author_id,omitempty"`
	DynastyID       *int64  `json:"dynasty_id,omitempty"`
	MeterPatternID  *int64  `json:"meter_pattern_id,omitempty"`
	Body            string  `json:"body"`
	Preface         *string `json:"preface,omitempty"`
	Translation     *string `json:"translation,omitempty"`
	Source          *string `json:"source,omitempty"`
	Status          string  `json:"status,omitempty"`
	Version         int     `json:"version,omitempty"`
}

var allowedPoemStatus = map[string]bool{
	"draft":     true,
	"in_review": true,
	"published": true,
	"archived":  true,
}

func RegisterPoems(r *gin.RouterGroup) {
	g := r.Group("/poems", auth.AuthRequired())
	g.GET("", listPoems)
	g.GET("/:id", getPoem)

	w := g.Group("", auth.RequireRole("administrator", "content_editor"))
	w.POST("", createPoem)
	w.PUT("/:id", updatePoem)
	w.DELETE("/:id", deletePoem)
	w.POST("/bulk", bulkPoems)
}

func scanPoem(row interface{ Scan(...any) error }) (*Poem, error) {
	var p Poem
	var aID, dID, mID sql.NullInt64
	var pref, tr, src sql.NullString
	if err := row.Scan(&p.ID, &p.Title, &aID, &dID, &mID, &p.Body, &pref, &tr, &src, &p.Status, &p.Version); err != nil {
		return nil, err
	}
	if aID.Valid {
		v := aID.Int64
		p.AuthorID = &v
	}
	if dID.Valid {
		v := dID.Int64
		p.DynastyID = &v
	}
	if mID.Valid {
		v := mID.Int64
		p.MeterPatternID = &v
	}
	if pref.Valid {
		p.Preface = &pref.String
	}
	if tr.Valid {
		p.Translation = &tr.String
	}
	if src.Valid {
		p.Source = &src.String
	}
	return &p, nil
}

const poemCols = `id, title, author_id, dynasty_id, meter_pattern_id, body, preface, translation, source, status, version`

func listPoems(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(
		`SELECT `+poemCols+` FROM poems ORDER BY id DESC LIMIT ? OFFSET ?`,
		p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Poem{}
	for rows.Next() {
		pm, err := scanPoem(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *pm)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getPoem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := scanPoem(db.DB.QueryRow(`SELECT `+poemCols+` FROM poems WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func validatePoem(p *Poem) string {
	if p.Title == "" {
		return "title required"
	}
	if p.Body == "" {
		return "body required"
	}
	if p.Status == "" {
		p.Status = "draft"
	}
	if !allowedPoemStatus[p.Status] {
		return "invalid status"
	}
	return ""
}

func insertPoem(tx *sql.Tx, p *Poem, actorID *int64) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO poems (title, author_id, dynasty_id, meter_pattern_id, body, preface, translation, source, status, version, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		p.Title, nullableInt64(p.AuthorID), nullableInt64(p.DynastyID), nullableInt64(p.MeterPatternID),
		p.Body, nullableString(p.Preface), nullableString(p.Translation), nullableString(p.Source),
		p.Status, nullableInt64(actorID), nullableInt64(actorID),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updatePoemRow(tx *sql.Tx, p *Poem, actorID *int64) error {
	_, err := tx.Exec(
		`UPDATE poems SET title=?, author_id=?, dynasty_id=?, meter_pattern_id=?, body=?, preface=?, translation=?, source=?, status=?, version=version+1, updated_by=? WHERE id=?`,
		p.Title, nullableInt64(p.AuthorID), nullableInt64(p.DynastyID), nullableInt64(p.MeterPatternID),
		p.Body, nullableString(p.Preface), nullableString(p.Translation), nullableString(p.Source),
		p.Status, nullableInt64(actorID), p.ID,
	)
	return err
}

func actorIDFromCtx(c *gin.Context) *int64 {
	if s, ok := auth.CurrentSession(c); ok {
		v := s.UserID
		return &v
	}
	return nil
}

func createPoem(c *gin.Context) {
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateGeometry(raw); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var in Poem
	if err := json.Unmarshal(raw, &in); err != nil {
		fail(c, http.StatusBadRequest, "invalid payload")
		return
	}
	if msg := validatePoem(&in); msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	id, err := insertPoem(tx, &in, actorIDFromCtx(c))
	if err != nil {
		dbFail(c, err)
		return
	}
	in.ID = id
	in.Version = 1
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, entityPoem, id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	search.RefreshPoem(id)
	c.JSON(http.StatusCreated, in)
}

func updatePoem(c *gin.Context) {
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
	var in Poem
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
	before, err := scanPoem(tx.QueryRow(`SELECT `+poemCols+` FROM poems WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if in.Title == "" {
		in.Title = before.Title
	}
	if in.Body == "" {
		in.Body = before.Body
	}
	if in.Status == "" {
		in.Status = before.Status
	}
	if !allowedPoemStatus[in.Status] {
		fail(c, http.StatusBadRequest, "invalid status")
		return
	}
	in.ID = id
	if err := updatePoemRow(tx, &in, actorIDFromCtx(c)); err != nil {
		dbFail(c, err)
		return
	}
	in.Version = before.Version + 1
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, entityPoem, id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	search.RefreshPoem(id)
	c.JSON(http.StatusOK, in)
}

func deletePoem(c *gin.Context) {
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
	before, err := scanPoem(tx.QueryRow(`SELECT `+poemCols+` FROM poems WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM poems WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	batchID, needsApproval := newApprovalContext()
	if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityPoem, id, before, nil, batchID, needsApproval); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	search.RemoveFromIndex(id)
	resp := gin.H{"deleted": id}
	if m := approvalResponseMeta(batchID, needsApproval); m != nil {
		resp["approval"] = m
	}
	c.JSON(http.StatusOK, resp)
}

type bulkPoemReq struct {
	Create []Poem  `json:"create"`
	Update []Poem  `json:"update"`
	Delete []int64 `json:"delete"`
}

func bulkPoems(c *gin.Context) {
	var req bulkPoemReq
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
	actor := actorIDFromCtx(c)
	batchID, needsApproval := newApprovalContext()

	created := []Poem{}
	for _, p := range req.Create {
		if msg := validatePoem(&p); msg != "" {
			fail(c, http.StatusBadRequest, "create: "+msg)
			return
		}
		id, err := insertPoem(tx, &p, actor)
		if err != nil {
			dbFail(c, err)
			return
		}
		p.ID = id
		p.Version = 1
		if err := audit.WriteCtxApproval(c, tx, audit.ActionCreate, entityPoem, id, nil, p, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
		created = append(created, p)
	}
	for _, p := range req.Update {
		if p.ID <= 0 {
			fail(c, http.StatusBadRequest, "update requires id")
			return
		}
		before, err := scanPoem(tx.QueryRow(`SELECT `+poemCols+` FROM poems WHERE id = ? FOR UPDATE`, p.ID))
		if err == sql.ErrNoRows {
			fail(c, http.StatusNotFound, "not found in update")
			return
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if p.Title == "" {
			p.Title = before.Title
		}
		if p.Body == "" {
			p.Body = before.Body
		}
		if p.Status == "" {
			p.Status = before.Status
		}
		if !allowedPoemStatus[p.Status] {
			fail(c, http.StatusBadRequest, "invalid status in update")
			return
		}
		if err := updatePoemRow(tx, &p, actor); err != nil {
			dbFail(c, err)
			return
		}
		p.Version = before.Version + 1
		if err := audit.WriteCtxApproval(c, tx, audit.ActionUpdate, entityPoem, p.ID, before, p, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}
	for _, id := range req.Delete {
		before, err := scanPoem(tx.QueryRow(`SELECT `+poemCols+` FROM poems WHERE id = ? FOR UPDATE`, id))
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if _, err := tx.Exec(`DELETE FROM poems WHERE id = ?`, id); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityPoem, id, before, nil, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	for _, p := range created {
		search.RefreshPoem(p.ID)
	}
	for _, p := range req.Update {
		search.RefreshPoem(p.ID)
	}
	for _, id := range req.Delete {
		search.RemoveFromIndex(id)
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
