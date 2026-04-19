package auth

import (
	"database/sql"
	"log"
	"os"

	"helios-backend/internal/db"
)

// demoUser is a role-scoped fixture account seeded so every RBAC role has a
// known login. The plain password is only used at seed time; what is stored is
// a bcrypt hash. Credentials are intentionally documented in README.md.
type demoUser struct {
	Username string
	Password string
	Role     string
}

// demoUsers lists the fixture logins that are created on first boot.
// The administrator is kept in sync with ADMIN_USERNAME/ADMIN_PASSWORD so that
// operators can override it through .env; all other roles use stable defaults.
func demoUsers() []demoUser {
	adminUser := getenv("ADMIN_USERNAME", "admin")
	adminPass := getenv("ADMIN_PASSWORD", "admin123")
	return []demoUser{
		{Username: adminUser, Password: adminPass, Role: "administrator"},
		{Username: "editor", Password: "editor123", Role: "content_editor"},
		{Username: "reviewer", Password: "reviewer123", Role: "reviewer"},
		{Username: "marketer", Password: "marketer123", Role: "marketing_manager"},
		{Username: "crawler", Password: "crawler123", Role: "crawler_operator"},
		{Username: "member", Password: "member123", Role: "member"},
	}
}

// BootstrapAdmin ensures every RBAC role has a demo user on first boot.
// Existing users are left untouched so repeated boots are idempotent.
func BootstrapAdmin() {
	for _, u := range demoUsers() {
		if err := ensureUser(u); err != nil {
			log.Printf("bootstrap: user %s: %v", u.Username, err)
		}
	}
}

func ensureUser(u demoUser) error {
	var existing int64
	err := db.DB.QueryRow(`SELECT id FROM users WHERE username = ?`, u.Username).Scan(&existing)
	if err == nil {
		return nil // already present — don't overwrite credentials
	}
	if err != sql.ErrNoRows {
		return err
	}

	if err := ValidatePasswordPolicy(u.Password); err != nil {
		return err
	}

	var roleID int64
	if err := db.DB.QueryRow(`SELECT id FROM roles WHERE name = ?`, u.Role).Scan(&roleID); err != nil {
		return err
	}

	hash, err := HashPassword(u.Password)
	if err != nil {
		return err
	}

	if _, err := db.DB.Exec(
		`INSERT INTO users (username, password_hash, role_id, status) VALUES (?, ?, ?, 'active')`,
		u.Username, hash, roleID,
	); err != nil {
		return err
	}
	log.Printf("bootstrap: created demo user %q (role=%s)", u.Username, u.Role)
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
