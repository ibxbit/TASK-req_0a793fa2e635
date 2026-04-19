package apitests

import (
	"net/http"
	"testing"
)

func TestArbitration_StatusesRequireAuth(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/arbitration/statuses", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon arbitration/statuses")
}

func TestArbitration_StatusesReturnsSeededCodes(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/arbitration/statuses", nil, nil)
	assertStatus(t, code, http.StatusOK, "list arbitration statuses")
	items, _ := body["items"].([]any)
	if len(items) < 4 {
		t.Fatalf("expected at least 4 seeded statuses, got %v", items)
	}
	codes := map[string]bool{}
	for _, it := range items {
		if row, ok := it.(map[string]any); ok {
			if c, ok := row["code"].(string); ok {
				codes[c] = true
			}
		}
	}
	for _, want := range []string{"submitted", "under_review", "resolved_upheld"} {
		if !codes[want] {
			t.Fatalf("expected code %q in statuses; got %v", want, codes)
		}
	}
}
