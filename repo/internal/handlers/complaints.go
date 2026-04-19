package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/crypto"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

type Complaint struct {
	ID               int64      `json:"id"`
	ComplainantID    int64      `json:"complainant_id"`
	TargetType       string     `json:"target_type"`
	TargetID         *int64     `json:"target_id,omitempty"`
	Subject          string     `json:"subject"`
	Notes            string     `json:"notes,omitempty"` // plaintext (decrypted for authorized viewers)
	ArbitrationID    *int64     `json:"arbitration_id,omitempty"`
	ArbitrationCode  string     `json:"arbitration_code,omitempty"`
	ArbitratorID     *int64     `json:"arbitrator_id,omitempty"`
	Resolution       string     `json:"resolution,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

var allowedComplaintTargets = map[string]bool{
	"poem": true, "review": true, "user": true, "order": true, "other": true,
}

func RegisterComplaints(r *gin.RouterGroup) {
	g := r.Group("/complaints", auth.AuthRequired())
	g.POST("", submitComplaint)
	g.GET("/mine", listMyComplaints)

	staff := g.Group("", auth.RequireRole("administrator", "reviewer"))
	staff.GET("", listAllComplaints)
	staff.GET("/:id", getComplaint)
	staff.POST("/:id/assign", assignComplaint)
	staff.POST("/:id/resolve", resolveComplaint)
}

type complaintInput struct {
	TargetType string `json:"target_type"`
	TargetID   *int64 `json:"target_id,omitempty"`
	Subject    string `json:"subject"`
	Notes      string `json:"notes"`
}

func submitComplaint(c *gin.Context) {
	var in complaintInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Subject == "" {
		fail(c, http.StatusBadRequest, "subject required")
		return
	}
	if !allowedComplaintTargets[in.TargetType] {
		fail(c, http.StatusBadRequest, "invalid target_type")
		return
	}
	sess, _ := auth.CurrentSession(c)

	var encrypted []byte
	if in.Notes != "" {
		b, err := crypto.Encrypt([]byte(in.Notes))
		if err != nil {
			fail(c, http.StatusInternalServerError, "encryption failed")
			return
		}
		encrypted = b
	}

	var initialArb sql.NullInt64
	if id, err := lookupArbitrationID("submitted"); err == nil {
		initialArb = sql.NullInt64{Int64: id, Valid: true}
	}

	res, err := db.DB.Exec(
		`INSERT INTO complaints
		  (complainant_id, target_type, target_id, subject, notes_encrypted, encryption_scheme, arbitration_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sess.UserID, in.TargetType, nullableInt64(in.TargetID), in.Subject,
		nullBytes(encrypted), nullStrVal(encrypted != nil, crypto.SchemeName), initialArb,
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"id":                id,
		"complainant_id":    sess.UserID,
		"target_type":       in.TargetType,
		"target_id":         in.TargetID,
		"subject":           in.Subject,
		"arbitration_code":  "submitted",
		"encryption_scheme": crypto.SchemeName,
	})
}

func listMyComplaints(c *gin.Context) {
	sess, _ := auth.CurrentSession(c)
	p := readPaging(c)
	rows, err := db.DB.Query(`
		SELECT c.id, c.complainant_id, c.target_type, c.target_id, c.subject,
		       c.arbitration_id, COALESCE(s.code, ''), c.arbitrator_id,
		       COALESCE(c.resolution, ''), c.resolved_at, c.created_at
		FROM complaints c
		LEFT JOIN arbitration_status s ON s.id = c.arbitration_id
		WHERE c.complainant_id = ?
		ORDER BY c.id DESC LIMIT ? OFFSET ?`,
		sess.UserID, p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := scanComplaintRows(rows)
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func listAllComplaints(c *gin.Context) {
	p := readPaging(c)
	args := []any{}
	q := `
		SELECT c.id, c.complainant_id, c.target_type, c.target_id, c.subject,
		       c.arbitration_id, COALESCE(s.code, ''), c.arbitrator_id,
		       COALESCE(c.resolution, ''), c.resolved_at, c.created_at
		FROM complaints c
		LEFT JOIN arbitration_status s ON s.id = c.arbitration_id`
	if v := c.Query("arbitrator_id"); v != "" {
		q += " WHERE c.arbitrator_id = ?"
		args = append(args, v)
	} else if v := c.Query("status"); v != "" {
		q += " WHERE s.code = ?"
		args = append(args, v)
	}
	q += " ORDER BY c.id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	c.JSON(http.StatusOK, gin.H{"items": scanComplaintRows(rows), "limit": p.Limit, "offset": p.Offset})
}

func getComplaint(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var (
		compl        Complaint
		targetID     sql.NullInt64
		arbID, arbtr sql.NullInt64
		arbCode      sql.NullString
		resolution   sql.NullString
		resolvedAt   sql.NullTime
		notesEnc     []byte
		scheme       sql.NullString
	)
	err := db.DB.QueryRow(`
		SELECT c.id, c.complainant_id, c.target_type, c.target_id, c.subject,
		       c.notes_encrypted, c.encryption_scheme,
		       c.arbitration_id, s.code, c.arbitrator_id,
		       c.resolution, c.resolved_at, c.created_at
		FROM complaints c
		LEFT JOIN arbitration_status s ON s.id = c.arbitration_id
		WHERE c.id = ?`, id,
	).Scan(
		&compl.ID, &compl.ComplainantID, &compl.TargetType, &targetID, &compl.Subject,
		&notesEnc, &scheme,
		&arbID, &arbCode, &arbtr,
		&resolution, &resolvedAt, &compl.CreatedAt,
	)
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if targetID.Valid {
		v := targetID.Int64
		compl.TargetID = &v
	}
	if arbID.Valid {
		v := arbID.Int64
		compl.ArbitrationID = &v
	}
	if arbCode.Valid {
		compl.ArbitrationCode = arbCode.String
	}
	if arbtr.Valid {
		v := arbtr.Int64
		compl.ArbitratorID = &v
	}
	if resolution.Valid {
		compl.Resolution = resolution.String
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		compl.ResolvedAt = &t
	}
	if len(notesEnc) > 0 {
		plain, err := crypto.Decrypt(notesEnc)
		if err != nil {
			fail(c, http.StatusInternalServerError, "decryption failed")
			return
		}
		compl.Notes = string(plain)
	}
	c.JSON(http.StatusOK, compl)
}

type assignReq struct {
	ArbitratorID int64 `json:"arbitrator_id"`
}

// allowedArbitratorRoles lists the roles whose users are permitted to take
// ownership of a complaint. We look this up at the DB layer to avoid trusting
// a client-supplied arbitrator_id that happens to point at, say, a
// crawler_operator or a deleted user. The check closes a function-level
// authorization gap previously present in this handler.
var allowedArbitratorRoles = map[string]bool{
	"administrator": true,
	"reviewer":      true,
}

func assignComplaint(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req assignReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ArbitratorID <= 0 {
		fail(c, http.StatusBadRequest, "arbitrator_id required")
		return
	}

	// Resolve the target user's role and status. Rejecting here stops an
	// attacker from quietly assigning a complaint to a non-staff account
	// (which would then bypass moderation entirely).
	var (
		targetRole   string
		targetStatus string
	)
	err := db.DB.QueryRow(`
		SELECT r.name, u.status
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = ?`, req.ArbitratorID,
	).Scan(&targetRole, &targetStatus)
	if err == sql.ErrNoRows {
		fail(c, http.StatusBadRequest, "arbitrator not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if targetStatus != "active" {
		fail(c, http.StatusBadRequest, "arbitrator account not active")
		return
	}
	if !allowedArbitratorRoles[targetRole] {
		fail(c, http.StatusBadRequest, "arbitrator must be administrator or reviewer")
		return
	}

	underReviewID, _ := lookupArbitrationID("under_review")
	res, err := db.DB.Exec(
		`UPDATE complaints SET arbitrator_id = ?, arbitration_id = COALESCE(?, arbitration_id) WHERE id = ?`,
		req.ArbitratorID, sql.NullInt64{Int64: underReviewID, Valid: underReviewID > 0}, id,
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":              id,
		"arbitrator_id":   req.ArbitratorID,
		"arbitrator_role": targetRole,
	})
}

type resolveReq struct {
	ArbitrationCode string `json:"arbitration_code"`
	Resolution      string `json:"resolution"`
}

func resolveComplaint(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req resolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ArbitrationCode == "" {
		fail(c, http.StatusBadRequest, "arbitration_code required")
		return
	}
	var (
		statusID   int64
		isTerminal bool
	)
	err := db.DB.QueryRow(
		`SELECT id, is_terminal FROM arbitration_status WHERE code = ?`, req.ArbitrationCode,
	).Scan(&statusID, &isTerminal)
	if err == sql.ErrNoRows {
		fail(c, http.StatusBadRequest, "unknown arbitration_code")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}

	var resolvedAt any = nil
	if isTerminal {
		resolvedAt = time.Now()
	}
	res, err := db.DB.Exec(
		`UPDATE complaints SET arbitration_id=?, resolution=?, resolved_at=? WHERE id=?`,
		statusID, nullStrVal(req.Resolution != "", req.Resolution), resolvedAt, id,
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                id,
		"arbitration_code":  req.ArbitrationCode,
		"arbitration_id":    statusID,
		"is_terminal":       isTerminal,
		"resolution":        req.Resolution,
	})
}

// ---------- helpers ----------

func lookupArbitrationID(code string) (int64, error) {
	var id int64
	err := db.DB.QueryRow(`SELECT id FROM arbitration_status WHERE code = ?`, code).Scan(&id)
	return id, err
}

func scanComplaintRows(rows *sql.Rows) []Complaint {
	out := []Complaint{}
	for rows.Next() {
		var (
			compl      Complaint
			targetID   sql.NullInt64
			arbID      sql.NullInt64
			arbCode    string
			arbtr      sql.NullInt64
			resolution string
			resolvedAt sql.NullTime
		)
		if err := rows.Scan(&compl.ID, &compl.ComplainantID, &compl.TargetType, &targetID, &compl.Subject,
			&arbID, &arbCode, &arbtr, &resolution, &resolvedAt, &compl.CreatedAt); err != nil {
			continue
		}
		if targetID.Valid {
			v := targetID.Int64
			compl.TargetID = &v
		}
		if arbID.Valid {
			v := arbID.Int64
			compl.ArbitrationID = &v
		}
		compl.ArbitrationCode = arbCode
		if arbtr.Valid {
			v := arbtr.Int64
			compl.ArbitratorID = &v
		}
		compl.Resolution = resolution
		if resolvedAt.Valid {
			t := resolvedAt.Time
			compl.ResolvedAt = &t
		}
		out = append(out, compl)
	}
	return out
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func nullStrVal(cond bool, s string) any {
	if !cond {
		return nil
	}
	return s
}
