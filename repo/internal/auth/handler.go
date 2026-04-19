package auth

import (
	"database/sql"
	"net/http"
	"strings"

	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userView struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/auth")
	{
		g.POST("/login", loginHandler)
		g.POST("/logout", logoutHandler)
		g.GET("/me", AuthRequired(), meHandler)
		g.POST("/register", registerHandler)
	}
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

func registerHandler(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}
	if err := ValidatePasswordPolicy(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing int64
	err := db.DB.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&existing)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	}
	if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	var roleID int64
	if err := db.DB.QueryRow(`SELECT id FROM roles WHERE name = 'member'`).Scan(&roleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "role not found"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var email any
	if req.Email != "" {
		email = req.Email
	}

	result, err := db.DB.Exec(
		`INSERT INTO users (username, password_hash, email, role_id, status) VALUES (?, ?, ?, ?, 'active')`,
		username, hash, email, roleID,
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already taken"})
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"user": userView{ID: id, Username: username, Role: "member"},
	})
}

func loginHandler(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	username := strings.TrimSpace(req.Username)

	// Reject outright if this username is currently rate-limited.
	if IsLoginLocked(username) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, try again later"})
		return
	}

	var (
		id       int64
		hash     string
		status   string
		roleName string
	)
	row := db.DB.QueryRow(`
		SELECT u.id, u.password_hash, u.status, r.name
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.username = ?`, username)
	if err := row.Scan(&id, &hash, &status, &roleName); err != nil {
		if err == sql.ErrNoRows {
			RecordLoginResult(username, false)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if status != "active" {
		RecordLoginResult(username, false)
		c.JSON(http.StatusForbidden, gin.H{"error": "account not active"})
		return
	}
	if !VerifyPassword(hash, req.Password) {
		RecordLoginResult(username, false)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	RecordLoginResult(username, true)

	sess, err := CreateSession(id, username, roleName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	_, _ = db.DB.Exec(`UPDATE users SET last_login_at = NOW() WHERE id = ?`, id)

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, sess.ID, int(IdleTimeout.Seconds()), "/", "", CookieSecure(), true)

	c.JSON(http.StatusOK, gin.H{
		"user": userView{ID: id, Username: username, Role: roleName},
	})
}

func logoutHandler(c *gin.Context) {
	cookie, err := c.Cookie(SessionCookieName)
	if err == nil && cookie != "" {
		DestroySession(cookie)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", CookieSecure(), true)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func meHandler(c *gin.Context) {
	sess, ok := CurrentSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": userView{ID: sess.UserID, Username: sess.Username, Role: sess.RoleName},
	})
}
