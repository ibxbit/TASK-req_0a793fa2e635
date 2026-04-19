package idempotency

import (
	"bytes"
	"database/sql"
	"log"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

const (
	HeaderKey   = "Idempotency-Key"
	retention   = 24 * time.Hour
	sweepPeriod = 1 * time.Hour
)

func isMutating(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

type captureWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// Middleware enforces idempotent replay for requests carrying an Idempotency-Key
// header. Replays cached 2xx responses for 24 hours. Non-2xx responses are not
// cached so the client can retry with the same key.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isMutating(c.Request.Method) {
			c.Next()
			return
		}
		key := c.GetHeader(HeaderKey)
		if key == "" {
			c.Next()
			return
		}

		var uid int64
		if sess, ok := auth.CurrentSession(c); ok {
			uid = sess.UserID
		} else if cookie, err := c.Cookie(auth.SessionCookieName); err == nil && cookie != "" {
			// Idempotency middleware runs before route-level auth; read cookie directly.
			if sess, ok := auth.GetSession(cookie); ok {
				uid = sess.UserID
			}
		}
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		var statusCode int
		var body sql.NullString
		err := db.DB.QueryRow(
			`SELECT status_code, response_body FROM idempotency_keys
			 WHERE idem_key=? AND user_id=? AND method=? AND path=? AND expires_at > NOW()`,
			key, uid, c.Request.Method, path,
		).Scan(&statusCode, &body)

		if err == nil {
			c.Header("Idempotent-Replay", "true")
			if body.Valid {
				c.Data(statusCode, "application/json; charset=utf-8", []byte(body.String))
			} else {
				c.Status(statusCode)
			}
			c.Abort()
			return
		}

		cw := &captureWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = cw
		c.Next()

		status := cw.Status()
		if status >= 200 && status < 300 {
			_, insertErr := db.DB.Exec(
				`INSERT INTO idempotency_keys
				   (idem_key, user_id, method, path, status_code, response_body, expires_at)
				 VALUES (?, ?, ?, ?, ?, ?, DATE_ADD(NOW(), INTERVAL 24 HOUR))
				 ON DUPLICATE KEY UPDATE
				   status_code   = VALUES(status_code),
				   response_body = VALUES(response_body),
				   expires_at    = VALUES(expires_at)`,
				key, uid, c.Request.Method, path, status, cw.body.String(),
			)
			if insertErr != nil {
				log.Printf("idempotency store failed: %v", insertErr)
			}
		}
	}
}

// StartSweeper removes expired idempotency records hourly.
func StartSweeper() {
	go func() {
		t := time.NewTicker(sweepPeriod)
		defer t.Stop()
		sweep()
		for range t.C {
			sweep()
		}
	}()
	log.Println("idempotency sweeper started (interval=1h)")
}

func sweep() {
	if _, err := db.DB.Exec(`DELETE FROM idempotency_keys WHERE expires_at < NOW()`); err != nil {
		log.Printf("idempotency sweep: %v", err)
	}
}
