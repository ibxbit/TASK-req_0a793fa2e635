package apitests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// baseURL returns the root of the REST API under test. Defaults to the
// compose-local backend hostname so the test-api service can reach it.
func baseURL() string {
	if v := os.Getenv("HELIOS_API_BASE"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:8080/api/v1"
}

// newClient returns an http.Client with its own cookie jar so tests don't
// share session state.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}
}

// doJSON issues a JSON request and returns status, parsed body, raw body.
func doJSON(t *testing.T, c *http.Client, method, path string, body any, extra map[string]string) (int, map[string]any, string) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL()+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	return resp.StatusCode, parsed, string(raw)
}

// doRaw issues a request with a raw body (e.g. malformed JSON).
func doRaw(t *testing.T, c *http.Client, method, path, rawBody string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL()+path, strings.NewReader(rawBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func loginAdmin(t *testing.T, c *http.Client) {
	t.Helper()
	loginAs(t, c, "admin", "admin123")
}

// loginAs authenticates the client as any seeded demo user. Credentials for
// every RBAC role are documented in repo/README.md (§ Default credentials).
func loginAs(t *testing.T, c *http.Client, username, password string) {
	t.Helper()
	code, _, raw := doJSON(t, c, "POST", "/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, nil)
	if code != http.StatusOK {
		t.Fatalf("login %s failed: status=%d body=%s", username, code, raw)
	}
}

// Fixtures — these are the seeded demo accounts from internal/auth/bootstrap.go.
const (
	userEditor   = "editor"
	passEditor   = "editor123"
	userReviewer = "reviewer"
	passReviewer = "reviewer123"
	userMkt      = "marketer"
	passMkt      = "marketer123"
	userCrawler  = "crawler"
	passCrawler  = "crawler123"
	// Regular end-user persona (non-staff).
	userMember = "member"
	passMember = "member123"
)

// assertStatus fails the test unless status matches.
func assertStatus(t *testing.T, got, want int, desc string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: expected status %d, got %d", desc, want, got)
	}
}

// mustString extracts a string from a decoded JSON tree.
func mustString(t *testing.T, m map[string]any, path ...string) string {
	t.Helper()
	cur := any(m)
	for _, k := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("not an object at %q", strings.Join(path, "."))
		}
		cur = obj[k]
	}
	s, ok := cur.(string)
	if !ok {
		t.Fatalf("not a string at %q: %v", strings.Join(path, "."), cur)
	}
	return s
}

// uniqSuffix returns a per-test suffix for entity names so runs don't collide.
var (
	randMu  sync.Mutex
	randSrc = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func uniqSuffix() string {
	randMu.Lock()
	defer randMu.Unlock()
	return fmt.Sprintf("%d_%d", time.Now().UnixNano(), randSrc.Intn(1<<16))
}

// TestMain waits for the backend to become healthy before any test runs.
func TestMain(m *testing.M) {
	c := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(90 * time.Second)
	for {
		resp, err := c.Get(baseURL() + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(os.Stderr, "backend never reached healthy state at %s\n", baseURL())
			os.Exit(2)
		}
		time.Sleep(2 * time.Second)
	}
	os.Exit(m.Run())
}
