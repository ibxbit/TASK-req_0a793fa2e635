package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

type Review struct {
	ID                int64  `json:"id"`
	PoemID            int64  `json:"poem_id"`
	UserID            int64  `json:"user_id"`
	Rating            uint8  `json:"rating"`
	RatingAccuracy    uint8  `json:"rating_accuracy"`
	RatingReadability uint8  `json:"rating_readability"`
	RatingValue       uint8  `json:"rating_value"`
	Title             string `json:"title,omitempty"`
	Content           string `json:"content,omitempty"`
	Status            string `json:"status"`
}

const reviewCols = `id, poem_id, user_id, rating, rating_accuracy, rating_readability, rating_value,
	COALESCE(title, ''), COALESCE(content, ''), status`

var allowedReviewStatus = map[string]bool{
	"pending": true, "approved": true, "rejected": true, "hidden": true,
}

func RegisterReviews(r *gin.RouterGroup) {
	g := r.Group("/reviews", auth.AuthRequired())
	g.GET("", listReviews)
	g.GET("/:id", getReview)
	g.POST("", createReview)
	g.PUT("/:id", updateReview)
	g.DELETE("/:id", deleteReview)
	g.POST("/:id/moderate", auth.RequireRole("administrator", "reviewer"), moderateReview)
}

func scanReview(row interface{ Scan(...any) error }) (*Review, error) {
	var r Review
	if err := row.Scan(&r.ID, &r.PoemID, &r.UserID,
		&r.Rating, &r.RatingAccuracy, &r.RatingReadability, &r.RatingValue,
		&r.Title, &r.Content, &r.Status); err != nil {
		return nil, err
	}
	return &r, nil
}

func validateRating(v uint8) bool { return v >= 1 && v <= 5 }

func computeOverall(a, r, v uint8) uint8 {
	sum := uint16(a) + uint16(r) + uint16(v)
	return uint8((sum + 1) / 3) // rounded average
}

func listReviews(c *gin.Context) {
	p := readPaging(c)
	args := []any{}
	where := []string{}
	if v := c.Query("poem_id"); v != "" {
		where = append(where, "poem_id = ?")
		args = append(args, v)
	}
	if v := c.Query("user_id"); v != "" {
		where = append(where, "user_id = ?")
		args = append(args, v)
	}
	if v := c.Query("status"); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	q := `SELECT ` + reviewCols + ` FROM reviews`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *r)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getReview(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := scanReview(db.DB.QueryRow(`SELECT `+reviewCols+` FROM reviews WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

type reviewInput struct {
	PoemID            int64  `json:"poem_id"`
	RatingAccuracy    uint8  `json:"rating_accuracy"`
	RatingReadability uint8  `json:"rating_readability"`
	RatingValue       uint8  `json:"rating_value"`
	Title             string `json:"title"`
	Content           string `json:"content"`
}

func createReview(c *gin.Context) {
	var in reviewInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if in.PoemID <= 0 {
		fail(c, http.StatusBadRequest, "poem_id required")
		return
	}
	if !validateRating(in.RatingAccuracy) || !validateRating(in.RatingReadability) || !validateRating(in.RatingValue) {
		fail(c, http.StatusBadRequest, "each rating must be 1..5")
		return
	}

	sess, _ := auth.CurrentSession(c)
	overall := computeOverall(in.RatingAccuracy, in.RatingReadability, in.RatingValue)

	res, err := db.DB.Exec(
		`INSERT INTO reviews (poem_id, user_id, rating, rating_accuracy, rating_readability, rating_value, title, content, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		in.PoemID, sess.UserID, overall, in.RatingAccuracy, in.RatingReadability, in.RatingValue,
		nullableString(&in.Title), nullableString(&in.Content),
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	out := Review{
		ID: id, PoemID: in.PoemID, UserID: sess.UserID,
		Rating: overall, RatingAccuracy: in.RatingAccuracy,
		RatingReadability: in.RatingReadability, RatingValue: in.RatingValue,
		Title: in.Title, Content: in.Content, Status: "pending",
	}
	c.JSON(http.StatusCreated, out)
}

func updateReview(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	sess, _ := auth.CurrentSession(c)

	var before Review
	if err := db.DB.QueryRow(
		`SELECT user_id FROM reviews WHERE id = ?`, id,
	).Scan(&before.UserID); err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		dbFail(c, err)
		return
	}
	if before.UserID != sess.UserID && sess.RoleName != "administrator" {
		fail(c, http.StatusForbidden, "not your review")
		return
	}

	var in reviewInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if !validateRating(in.RatingAccuracy) || !validateRating(in.RatingReadability) || !validateRating(in.RatingValue) {
		fail(c, http.StatusBadRequest, "each rating must be 1..5")
		return
	}
	overall := computeOverall(in.RatingAccuracy, in.RatingReadability, in.RatingValue)
	if _, err := db.DB.Exec(
		`UPDATE reviews SET rating=?, rating_accuracy=?, rating_readability=?, rating_value=?, title=?, content=? WHERE id=?`,
		overall, in.RatingAccuracy, in.RatingReadability, in.RatingValue,
		nullableString(&in.Title), nullableString(&in.Content), id,
	); err != nil {
		dbFail(c, err)
		return
	}
	r, _ := scanReview(db.DB.QueryRow(`SELECT `+reviewCols+` FROM reviews WHERE id = ?`, id))
	c.JSON(http.StatusOK, r)
}

func deleteReview(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	sess, _ := auth.CurrentSession(c)

	var userID int64
	err := db.DB.QueryRow(`SELECT user_id FROM reviews WHERE id = ?`, id).Scan(&userID)
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if userID != sess.UserID && sess.RoleName != "administrator" && sess.RoleName != "reviewer" {
		fail(c, http.StatusForbidden, "not allowed")
		return
	}
	if _, err := db.DB.Exec(`DELETE FROM reviews WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

type moderateReq struct {
	Status string `json:"status"`
}

func moderateReview(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req moderateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if !allowedReviewStatus[req.Status] {
		fail(c, http.StatusBadRequest, "invalid status")
		return
	}
	res, err := db.DB.Exec(`UPDATE reviews SET status=? WHERE id=?`, req.Status, id)
	if err != nil {
		dbFail(c, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": req.Status})
}
