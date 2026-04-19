package monitoring

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

const (
	envCrashDir     = "HELIOS_CRASH_DIR"
	defaultCrashDir = "/data/crashes"
)

var crashDir string

// InitCrashDir prepares the on-disk crash report directory.
func InitCrashDir() error {
	crashDir = os.Getenv(envCrashDir)
	if crashDir == "" {
		crashDir = defaultCrashDir
	}
	if err := os.MkdirAll(crashDir, 0o700); err != nil {
		return err
	}
	log.Printf("monitoring crash dir: %s", crashDir)
	return nil
}

func CrashDir() string { return crashDir }

type CrashReport struct {
	ID           string         `json:"id"`
	Service      string         `json:"service"`
	Environment  string         `json:"environment"`
	ErrorType    string         `json:"error_type"`
	ErrorMessage string         `json:"error_message"`
	StackTrace   string         `json:"stack_trace"`
	Request      map[string]any `json:"request,omitempty"`
	UserID       *int64         `json:"user_id,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
	DiskPath     string         `json:"disk_path"`
}

// OnPanic is a gin.CustomRecovery handler. It writes a crash report to disk
// (primary storage, per requirement) and adds an index row in crash_reports.
func OnPanic(c *gin.Context, recovered any) {
	report := buildReport(c, recovered)
	path, err := writeReportToDisk(report)
	if err != nil {
		log.Printf("crash: disk write failed: %v", err)
	} else {
		report.DiskPath = path
		indexInDB(report)
	}
	log.Printf("crash %s: %s", report.ID, report.ErrorMessage)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
		"error":    "internal server error",
		"crash_id": report.ID,
	})
}

func buildReport(c *gin.Context, recovered any) *CrashReport {
	id := newID()
	r := &CrashReport{
		ID:           id,
		Service:      serviceName,
		Environment:  envOr("HELIOS_ENV", "production"),
		ErrorType:    "panic",
		ErrorMessage: fmt.Sprintf("%v", recovered),
		StackTrace:   string(debug.Stack()),
		OccurredAt:   time.Now(),
	}
	r.Request = map[string]any{
		"method":     c.Request.Method,
		"path":       c.Request.URL.Path,
		"full_path":  c.FullPath(),
		"remote_ip":  c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}
	if sess, ok := auth.CurrentSession(c); ok {
		uid := sess.UserID
		r.UserID = &uid
	}
	return r
}

func writeReportToDisk(r *CrashReport) (string, error) {
	if crashDir == "" {
		return "", fmt.Errorf("crash dir not initialized")
	}
	day := r.OccurredAt.UTC().Format("2006-01-02")
	dir := filepath.Join(crashDir, day)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d-%s.json", r.OccurredAt.UnixNano(), r.ID)
	path := filepath.Join(dir, name)
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o600)
}

func indexInDB(r *CrashReport) {
	ctx, _ := json.Marshal(map[string]any{
		"crash_id":  r.ID,
		"disk_path": r.DiskPath,
		"request":   r.Request,
	})
	var uid any = nil
	if r.UserID != nil {
		uid = *r.UserID
	}
	if _, err := db.DB.Exec(
		`INSERT INTO crash_reports
		   (service, environment, error_type, error_message, stack_trace, context, user_id, occurred_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Service, r.Environment, r.ErrorType, r.ErrorMessage, r.StackTrace,
		string(ctx), uid, r.OccurredAt,
	); err != nil {
		log.Printf("crash: db index failed: %v", err)
	}
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ReadReport loads a crash report by its disk path. The path is validated to
// be inside the configured crash directory.
func ReadReport(path string) (*CrashReport, error) {
	if crashDir == "" {
		return nil, fmt.Errorf("crash dir not initialized")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rootAbs, _ := filepath.Abs(crashDir)
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == "" || rel[0] == '.' {
		return nil, fmt.Errorf("path outside crash dir")
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var r CrashReport
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
