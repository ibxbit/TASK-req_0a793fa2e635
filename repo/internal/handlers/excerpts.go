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

const entityExcerpt = "excerpt"

type Excerpt struct {
	ID             int64   `json:"id"`
	PoemID         int64   `json:"poem_id"`
	StartOffset    uint32  `json:"start_offset"`
	EndOffset      uint32  `json:"end_offset"`
	ExcerptText    string  `json:"excerpt_text"`
	Annotation     *string `json:"annotation,omitempty"`
	AnnotationType string  `json:"annotation_type,omitempty"`
	AuthorID       *int64  `json:"author_id,omitempty"`
}

var allowedAnnotationType = map[string]bool{
	"note":        true,
	"commentary":  true,
	"translation": true,
	"reference":   true,
}

func RegisterExcerpts(r *gin.RouterGroup) {
	g := r.Group("/excerpts", auth.AuthRequired())
	g.GET("", listExcerpts)
	g.GET("/:id", getExcerpt)

	w := g.Group("", auth.RequireRole("administrator", "content_editor"))
	w.POST("", createExcerpt)
	w.PUT("/:id", updateExcerpt)
	w.DELETE("/:id", deleteExcerpt)
	w.POST("/bulk", bulkExcerpts)
}

const excerptCols = `id, poem_id, start_offset, end_offset, excerpt_text, annotation, annotation_type, author_id`

func scanExcerpt(row interface{ Scan(...any) error }) (*Excerpt, error) {
	var e Excerpt
	var ann sql.NullString
	var aID sql.NullInt64
	if err := row.Scan(&e.ID, &e.PoemID, &e.StartOffset, &e.EndOffset, &e.ExcerptText, &ann, &e.AnnotationType, &aID); err != nil {
		return nil, err
	}
	if ann.Valid {
		e.Annotation = &ann.String
	}
	if aID.Valid {
		v := aID.Int64
		e.AuthorID = &v
	}
	return &e, nil
}

func listExcerpts(c *gin.Context) {
	p := readPaging(c)
	var rows *sql.Rows
	var err error
	if poemID := c.Query("poem_id"); poemID != "" {
		rows, err = db.DB.Query(
			`SELECT `+excerptCols+` FROM excerpts WHERE poem_id = ? ORDER BY start_offset LIMIT ? OFFSET ?`,
			poemID, p.Limit, p.Offset)
	} else {
		rows, err = db.DB.Query(
			`SELECT `+excerptCols+` FROM excerpts ORDER BY id DESC LIMIT ? OFFSET ?`, p.Limit, p.Offset)
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Excerpt{}
	for rows.Next() {
		e, err := scanExcerpt(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *e)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getExcerpt(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	e, err := scanExcerpt(db.DB.QueryRow(`SELECT `+excerptCols+` FROM excerpts WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

func validateExcerpt(e *Excerpt) string {
	if e.PoemID <= 0 {
		return "poem_id required"
	}
	if e.ExcerptText == "" {
		return "excerpt_text required"
	}
	if e.EndOffset < e.StartOffset {
		return "end_offset must be >= start_offset"
	}
	if e.AnnotationType == "" {
		e.AnnotationType = "note"
	}
	if !allowedAnnotationType[e.AnnotationType] {
		return "invalid annotation_type"
	}
	return ""
}

func insertExcerpt(tx *sql.Tx, e *Excerpt) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO excerpts (poem_id, start_offset, end_offset, excerpt_text, annotation, annotation_type, author_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.PoemID, e.StartOffset, e.EndOffset, e.ExcerptText,
		nullableString(e.Annotation), e.AnnotationType, nullableInt64(e.AuthorID),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateExcerptRow(tx *sql.Tx, e *Excerpt) error {
	_, err := tx.Exec(
		`UPDATE excerpts SET poem_id=?, start_offset=?, end_offset=?, excerpt_text=?, annotation=?, annotation_type=?, author_id=? WHERE id=?`,
		e.PoemID, e.StartOffset, e.EndOffset, e.ExcerptText,
		nullableString(e.Annotation), e.AnnotationType, nullableInt64(e.AuthorID), e.ID,
	)
	return err
}

func createExcerpt(c *gin.Context) {
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateGeometry(raw); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var in Excerpt
	if err := json.Unmarshal(raw, &in); err != nil {
		fail(c, http.StatusBadRequest, "invalid payload")
		return
	}
	if msg := validateExcerpt(&in); msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	id, err := insertExcerpt(tx, &in)
	if err != nil {
		dbFail(c, err)
		return
	}
	in.ID = id
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, entityExcerpt, id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateExcerpt(c *gin.Context) {
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
	var in Excerpt
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
	before, err := scanExcerpt(tx.QueryRow(`SELECT `+excerptCols+` FROM excerpts WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if in.PoemID == 0 {
		in.PoemID = before.PoemID
	}
	if in.ExcerptText == "" {
		in.ExcerptText = before.ExcerptText
	}
	if in.AnnotationType == "" {
		in.AnnotationType = before.AnnotationType
	}
	if msg := validateExcerpt(&in); msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	in.ID = id
	if err := updateExcerptRow(tx, &in); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, entityExcerpt, id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteExcerpt(c *gin.Context) {
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
	before, err := scanExcerpt(tx.QueryRow(`SELECT `+excerptCols+` FROM excerpts WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM excerpts WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	batchID, needsApproval := newApprovalContext()
	if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityExcerpt, id, before, nil, batchID, needsApproval); err != nil {
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

type bulkExcerptReq struct {
	Create []Excerpt `json:"create"`
	Update []Excerpt `json:"update"`
	Delete []int64   `json:"delete"`
}

func bulkExcerpts(c *gin.Context) {
	var req bulkExcerptReq
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

	created := []Excerpt{}
	for _, e := range req.Create {
		if msg := validateExcerpt(&e); msg != "" {
			fail(c, http.StatusBadRequest, "create: "+msg)
			return
		}
		id, err := insertExcerpt(tx, &e)
		if err != nil {
			dbFail(c, err)
			return
		}
		e.ID = id
		if err := audit.WriteCtxApproval(c, tx, audit.ActionCreate, entityExcerpt, id, nil, e, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
		created = append(created, e)
	}
	for _, e := range req.Update {
		if e.ID <= 0 {
			fail(c, http.StatusBadRequest, "update requires id")
			return
		}
		before, err := scanExcerpt(tx.QueryRow(`SELECT `+excerptCols+` FROM excerpts WHERE id = ? FOR UPDATE`, e.ID))
		if err == sql.ErrNoRows {
			fail(c, http.StatusNotFound, "not found in update")
			return
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if e.PoemID == 0 {
			e.PoemID = before.PoemID
		}
		if e.ExcerptText == "" {
			e.ExcerptText = before.ExcerptText
		}
		if e.AnnotationType == "" {
			e.AnnotationType = before.AnnotationType
		}
		if msg := validateExcerpt(&e); msg != "" {
			fail(c, http.StatusBadRequest, "update: "+msg)
			return
		}
		if err := updateExcerptRow(tx, &e); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionUpdate, entityExcerpt, e.ID, before, e, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}
	for _, id := range req.Delete {
		before, err := scanExcerpt(tx.QueryRow(`SELECT `+excerptCols+` FROM excerpts WHERE id = ? FOR UPDATE`, id))
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if _, err := tx.Exec(`DELETE FROM excerpts WHERE id = ?`, id); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityExcerpt, id, before, nil, batchID, needsApproval); err != nil {
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
