package apitests

import (
	"net/http"
	"testing"
)

func TestMonitoring_RequiresAdmin(t *testing.T) {
	// reviewer is authenticated but not admin — must be denied.
	c := newClient(t)
	loginAs(t, c, userReviewer, passReviewer)
	code, _, _ := doJSON(t, c, "GET", "/monitoring/metrics", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot read metrics")

	code, _, _ = doJSON(t, c, "GET", "/monitoring/metrics/summary", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot read summary")

	code, _, _ = doJSON(t, c, "GET", "/monitoring/crashes", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot read crashes")
}

func TestMonitoring_MetricsEnvelope(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/monitoring/metrics?limit=10", nil, nil)
	assertStatus(t, code, http.StatusOK, "admin metrics")
	if _, ok := body["items"]; !ok {
		t.Fatalf("missing items: %v", body)
	}
	if v, _ := body["limit"].(float64); int(v) != 10 {
		t.Fatalf("limit not echoed: %v", body["limit"])
	}
}

func TestMonitoring_MetricsSummary(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/monitoring/metrics/summary", nil, nil)
	assertStatus(t, code, http.StatusOK, "admin summary")
	if _, ok := body["items"]; !ok {
		t.Fatalf("summary missing items: %v", body)
	}
}

func TestMonitoring_CrashesList(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/monitoring/crashes?limit=5", nil, nil)
	assertStatus(t, code, http.StatusOK, "admin crashes list")
	if _, ok := body["items"]; !ok {
		t.Fatalf("crashes missing items: %v", body)
	}
	if _, ok := body["crash_dir"]; !ok {
		t.Fatalf("expected crash_dir in response: %v", body)
	}
}

func TestMonitoring_CrashNotFoundClean(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/monitoring/crashes/999999999", nil, nil)
	assertStatus(t, code, http.StatusNotFound, "missing crash id")
	if body["error"] == nil {
		t.Fatalf("expected error in response: %v", body)
	}
}

func TestMonitoring_CrashInvalidIDReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/monitoring/crashes/not-a-number", nil, nil)
	assertStatus(t, code, http.StatusBadRequest, "non-numeric crash id")
	if body["error"] == nil {
		t.Fatalf("expected error: %v", body)
	}
}

func TestMonitoring_MetricsFilterByName(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	// The ?name= filter is passed through; regardless of whether any rows match,
	// the envelope must contain an items array (not a 500).
	code, body, _ := doJSON(t, c, "GET", "/monitoring/metrics?name=cpu_usage", nil, nil)
	assertStatus(t, code, http.StatusOK, "name filter")
	if _, ok := body["items"]; !ok {
		t.Fatalf("expected items array: %v", body)
	}
}

func TestMonitoring_MetricsFilterBySince(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	// Use a far-future since so result set is empty but the query must succeed.
	code, body, _ := doJSON(t, c, "GET", "/monitoring/metrics?since=2099-01-01T00:00:00Z", nil, nil)
	assertStatus(t, code, http.StatusOK, "since filter returns empty items")
	items, _ := body["items"].([]any)
	if items == nil {
		t.Fatalf("items should be an array (possibly empty): %v", body)
	}
}

func TestMonitoring_MetricsEnvelopeContainsOffsetField(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/monitoring/metrics?limit=5&offset=0", nil, nil)
	assertStatus(t, code, http.StatusOK, "metrics pagination envelope")
	if _, ok := body["offset"]; !ok {
		t.Fatalf("metrics response missing offset field: %v", body)
	}
}

func TestMonitoring_AnonBlocked(t *testing.T) {
	c := newClient(t)
	// No login — expect 401 on all monitoring endpoints.
	for _, path := range []string{
		"/monitoring/metrics",
		"/monitoring/metrics/summary",
		"/monitoring/crashes",
		"/monitoring/crashes/1",
	} {
		code, _, _ := doJSON(t, c, "GET", path, nil, nil)
		if code != http.StatusUnauthorized {
			t.Errorf("expected 401 for anon on %s, got %d", path, code)
		}
	}
}

func TestMonitoring_NonAdminRolesBlocked(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	for _, path := range []string{
		"/monitoring/metrics",
		"/monitoring/metrics/summary",
		"/monitoring/crashes",
	} {
		code, _, _ := doJSON(t, c, "GET", path, nil, nil)
		if code != http.StatusForbidden {
			t.Errorf("editor: expected 403 for %s, got %d", path, code)
		}
	}
}
