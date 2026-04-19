package apitests

import (
	"net/http"
	"testing"
)

func TestRBAC_UnauthenticatedReadsAre401(t *testing.T) {
	c := newClient(t)
	for _, path := range []string{
		"/dynasties",
		"/poems",
		"/authors",
		"/approvals",
		"/audit-logs",
		"/monitoring/metrics",
		"/crawl/jobs",
		"/settings/approval",
	} {
		code, _, _ := doJSON(t, c, "GET", path, nil, nil)
		if code != http.StatusUnauthorized {
			t.Fatalf("GET %s expected 401, got %d", path, code)
		}
	}
}

func TestRBAC_UnauthenticatedWriteIs401(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "POST", "/dynasties", map[string]string{"name": "X"}, nil)
	assertStatus(t, code, http.StatusUnauthorized, "unauth POST /dynasties")
}

func TestRBAC_AdminCanReachAdminEndpoints(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	for _, path := range []string{
		"/approvals",
		"/audit-logs",
		"/monitoring/metrics",
		"/monitoring/metrics/summary",
		"/settings/approval",
	} {
		code, _, raw := doJSON(t, c, "GET", path, nil, nil)
		if code != http.StatusOK {
			t.Fatalf("admin GET %s expected 200, got %d (%s)", path, code, raw)
		}
	}
}

func TestRBAC_AdminCanCreateContent(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name": "RBAC_" + uniqSuffix(),
	}, nil)
	assertStatus(t, code, http.StatusCreated, "admin create dynasty")
	if _, ok := body["id"]; !ok {
		t.Fatalf("created dynasty missing id: %v", body)
	}
	// Clean up
	id := int64(body["id"].(float64))
	doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
}
