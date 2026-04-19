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

const entityAuthor = "author"

type Author struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	AltNames  *string `json:"alt_names,omitempty"`
	DynastyID *int64  `json:"dynasty_id,omitempty"`
	BirthYear *int    `json:"birth_year,omitempty"`
	DeathYear *int    `json:"death_year,omitempty"`
	Biography *string `json:"biography,omitempty"`
}

func RegisterAuthors(r *gin.RouterGroup) {
	g := r.Group("/authors", auth.AuthRequired())
	g.GET("", listAuthors)
	g.GET("/:id", getAuthor)

	w := g.Group("", auth.RequireRole("administrator", "content_editor"))
	w.POST("", createAuthor)
	w.PUT("/:id", updateAuthor)
	w.DELETE("/:id", deleteAuthor)
	w.POST("/bulk", bulkAuthors)
}

func scanAuthor(row interface{ Scan(...any) error }) (*Author, error) {
	var a Author
	var alt, bio sql.NullString
	var dyn sql.NullInt64
	var by, dy sql.NullInt32
	if err := row.Scan(&a.ID, &a.Name, &alt, &dyn, &by, &dy, &bio); err != nil {
		return nil, err
	}
	if alt.Valid {
		a.AltNames = &alt.String
	}
	if dyn.Valid {
		v := dyn.Int64
		a.DynastyID = &v
	}
	if by.Valid {
		v := int(by.Int32)
		a.BirthYear = &v
	}
	if dy.Valid {
		v := int(dy.Int32)
		a.DeathYear = &v
	}
	if bio.Valid {
		a.Biography = &bio.String
	}
	return &a, nil
}

func listAuthors(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(
		`SELECT id, name, alt_names, dynasty_id, birth_year, death_year, biography
		 FROM authors ORDER BY id LIMIT ? OFFSET ?`,
		p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Author{}
	for rows.Next() {
		a, err := scanAuthor(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *a)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getAuthor(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := scanAuthor(db.DB.QueryRow(
		`SELECT id, name, alt_names, dynasty_id, birth_year, death_year, biography
		 FROM authors WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

func insertAuthor(tx *sql.Tx, a *Author) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO authors (name, alt_names, dynasty_id, birth_year, death_year, biography)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.Name, nullableString(a.AltNames), nullableInt64(a.DynastyID),
		nullableInt(a.BirthYear), nullableInt(a.DeathYear), nullableString(a.Biography),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateAuthorRow(tx *sql.Tx, a *Author) error {
	_, err := tx.Exec(
		`UPDATE authors SET name=?, alt_names=?, dynasty_id=?, birth_year=?, death_year=?, biography=? WHERE id=?`,
		a.Name, nullableString(a.AltNames), nullableInt64(a.DynastyID),
		nullableInt(a.BirthYear), nullableInt(a.DeathYear), nullableString(a.Biography), a.ID,
	)
	return err
}

func createAuthor(c *gin.Context) {
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateGeometry(raw); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var in Author
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
	id, err := insertAuthor(tx, &in)
	if err != nil {
		dbFail(c, err)
		return
	}
	in.ID = id
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, entityAuthor, id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateAuthor(c *gin.Context) {
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
	var in Author
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
	before, err := scanAuthor(tx.QueryRow(
		`SELECT id, name, alt_names, dynasty_id, birth_year, death_year, biography
		 FROM authors WHERE id = ? FOR UPDATE`, id))
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
	if err := updateAuthorRow(tx, &in); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, entityAuthor, id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteAuthor(c *gin.Context) {
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
	before, err := scanAuthor(tx.QueryRow(
		`SELECT id, name, alt_names, dynasty_id, birth_year, death_year, biography
		 FROM authors WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM authors WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	batchID, needsApproval := newApprovalContext()
	if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityAuthor, id, before, nil, batchID, needsApproval); err != nil {
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

type bulkAuthorReq struct {
	Create []Author `json:"create"`
	Update []Author `json:"update"`
	Delete []int64  `json:"delete"`
}

func bulkAuthors(c *gin.Context) {
	var req bulkAuthorReq
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

	created := []Author{}
	for _, a := range req.Create {
		if a.Name == "" {
			fail(c, http.StatusBadRequest, "name required in create item")
			return
		}
		id, err := insertAuthor(tx, &a)
		if err != nil {
			dbFail(c, err)
			return
		}
		a.ID = id
		if err := audit.WriteCtxApproval(c, tx, audit.ActionCreate, entityAuthor, id, nil, a, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
		created = append(created, a)
	}
	for _, a := range req.Update {
		if a.ID <= 0 {
			fail(c, http.StatusBadRequest, "update requires id")
			return
		}
		before, err := scanAuthor(tx.QueryRow(
			`SELECT id, name, alt_names, dynasty_id, birth_year, death_year, biography
			 FROM authors WHERE id = ? FOR UPDATE`, a.ID))
		if err == sql.ErrNoRows {
			fail(c, http.StatusNotFound, "not found in update")
			return
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if a.Name == "" {
			a.Name = before.Name
		}
		if err := updateAuthorRow(tx, &a); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionUpdate, entityAuthor, a.ID, before, a, batchID, needsApproval); err != nil {
			dbFail(c, err)
			return
		}
	}
	for _, id := range req.Delete {
		before, err := scanAuthor(tx.QueryRow(
			`SELECT id, name, alt_names, dynasty_id, birth_year, death_year, biography
			 FROM authors WHERE id = ? FOR UPDATE`, id))
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			dbFail(c, err)
			return
		}
		if _, err := tx.Exec(`DELETE FROM authors WHERE id = ?`, id); err != nil {
			dbFail(c, err)
			return
		}
		if err := audit.WriteCtxApproval(c, tx, audit.ActionDelete, entityAuthor, id, before, nil, batchID, needsApproval); err != nil {
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
