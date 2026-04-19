package apitests

import (
	"net/http"
	"testing"
)

func TestIdempotency_RetryReturnsCachedResponse(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)

	suffix := uniqSuffix()
	key := "idem-" + suffix
	body := map[string]any{"name": "Idem_" + suffix}

	// First POST creates the row
	code1, resp1, _ := doJSON(t, c, "POST", "/dynasties", body, map[string]string{
		"Idempotency-Key": key,
	})
	assertStatus(t, code1, http.StatusCreated, "first POST with key")

	id1, ok := resp1["id"].(float64)
	if !ok {
		t.Fatalf("first response missing id: %v", resp1)
	}

	// Second POST with same key must replay, NOT create a new row
	code2, resp2, _ := doJSON(t, c, "POST", "/dynasties", body, map[string]string{
		"Idempotency-Key": key,
	})
	assertStatus(t, code2, http.StatusCreated, "replay POST with same key")

	id2, ok := resp2["id"].(float64)
	if !ok {
		t.Fatalf("replay response missing id: %v", resp2)
	}
	if int64(id1) != int64(id2) {
		t.Fatalf("replay produced different id: first=%d second=%d", int64(id1), int64(id2))
	}

	// Cleanup
	doJSON(t, c, "DELETE", "/dynasties/"+itoa(int64(id1)), nil, nil)
}
