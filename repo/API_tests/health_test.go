package apitests

import (
	"net/http"
	"testing"
)

func TestHealth_Reports200AndDBUp(t *testing.T) {
	c := newClient(t)
	code, body, raw := doJSON(t, c, "GET", "/health", nil, nil)
	assertStatus(t, code, http.StatusOK, "GET /health")
	if body["status"] != "ok" {
		t.Fatalf("status field: got %v, body=%s", body["status"], raw)
	}
	if body["db"] != "up" {
		t.Fatalf("db field: got %v, body=%s", body["db"], raw)
	}
}
