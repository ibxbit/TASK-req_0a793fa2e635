package apitests

import (
	"net/http"
	"testing"
)

// registerMember registers a fresh member account and returns (username, password).
// The account is unique per call so tests don't collide.
func registerMember(t *testing.T) (string, string) {
	t.Helper()
	username := "m_" + uniqSuffix()
	password := "Password123"
	c := newClient(t)
	code, _, raw := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": username, "password": password,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("registerMember: %d %s", code, raw)
	}
	return username, password
}

// TestMember_SubmitReview_HappyPath proves a registered member can submit a
// review via POST /reviews and receives 201 with the correct shape.
func TestMember_SubmitReview_HappyPath(t *testing.T) {
	username, password := registerMember(t)
	pID := seedPoem(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	code, body, raw := doJSON(t, c, "POST", "/reviews", map[string]any{
		"poem_id":            pID,
		"rating_accuracy":    4,
		"rating_readability": 5,
		"rating_value":       4,
		"title":              "Member review",
		"content":            "Good poem",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("member POST /reviews: %d %s", code, raw)
	}
	if body["status"] != "pending" {
		t.Fatalf("expected status=pending, got %v", body["status"])
	}
	if body["id"] == nil {
		t.Fatalf("missing id in response: %v", body)
	}
}

// TestMember_SubmitReview_WithIdempotencyKey proves that duplicate review
// submissions carrying the same Idempotency-Key replay the cached response
// rather than creating a second record.
func TestMember_SubmitReview_WithIdempotencyKey(t *testing.T) {
	username, password := registerMember(t)
	pID := seedPoem(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	key := "rev-idem-" + uniqSuffix()
	payload := map[string]any{
		"poem_id":            pID,
		"rating_accuracy":    3,
		"rating_readability": 3,
		"rating_value":       3,
		"title":              "Idem review",
	}
	headers := map[string]string{"Idempotency-Key": key}

	code1, body1, raw1 := doJSON(t, c, "POST", "/reviews", payload, headers)
	if code1 != http.StatusCreated {
		t.Fatalf("first POST /reviews with key: %d %s", code1, raw1)
	}
	id1 := int64(body1["id"].(float64))

	// Replay — same key must return the same record, not a new one.
	code2, body2, raw2 := doJSON(t, c, "POST", "/reviews", payload, headers)
	if code2 != http.StatusCreated {
		t.Fatalf("replay POST /reviews with same key: %d %s", code2, raw2)
	}
	id2 := int64(body2["id"].(float64))
	if id1 != id2 {
		t.Fatalf("idempotency replay produced different id: first=%d second=%d", id1, id2)
	}
}

// TestMember_SubmitReview_InvalidRatingRejected validates that a member
// submitting an out-of-range rating gets 400.
func TestMember_SubmitReview_InvalidRatingRejected(t *testing.T) {
	username, password := registerMember(t)
	pID := seedPoem(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	code, body, _ := doJSON(t, c, "POST", "/reviews", map[string]any{
		"poem_id":            pID,
		"rating_accuracy":    9,
		"rating_readability": 5,
		"rating_value":       5,
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "member invalid rating")
	if body["error"] == nil {
		t.Fatalf("400 missing error field: %v", body)
	}
}

// TestMember_SubmitComplaint_HappyPath proves a registered member can submit a
// complaint via POST /complaints and receives 201.
func TestMember_SubmitComplaint_HappyPath(t *testing.T) {
	username, password := registerMember(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	code, body, raw := doJSON(t, c, "POST", "/complaints", map[string]any{
		"subject":     "Test complaint " + uniqSuffix(),
		"target_type": "other",
		"notes":       "Details here",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("member POST /complaints: %d %s", code, raw)
	}
	if body["arbitration_code"] != "submitted" {
		t.Fatalf("expected arbitration_code=submitted, got %v", body["arbitration_code"])
	}
	if body["id"] == nil {
		t.Fatalf("missing id in response: %v", body)
	}
}

// TestMember_SubmitComplaint_WithIdempotencyKey proves that a replayed
// complaint with the same Idempotency-Key returns the cached response.
func TestMember_SubmitComplaint_WithIdempotencyKey(t *testing.T) {
	username, password := registerMember(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	key := "cmp-idem-" + uniqSuffix()
	payload := map[string]any{
		"subject":     "Idem complaint " + uniqSuffix(),
		"target_type": "poem",
		"notes":       "Seen twice",
	}
	headers := map[string]string{"Idempotency-Key": key}

	code1, body1, raw1 := doJSON(t, c, "POST", "/complaints", payload, headers)
	if code1 != http.StatusCreated {
		t.Fatalf("first POST /complaints: %d %s", code1, raw1)
	}
	id1 := int64(body1["id"].(float64))

	code2, body2, raw2 := doJSON(t, c, "POST", "/complaints", payload, headers)
	if code2 != http.StatusCreated {
		t.Fatalf("replay POST /complaints: %d %s", code2, raw2)
	}
	id2 := int64(body2["id"].(float64))
	if id1 != id2 {
		t.Fatalf("idempotency replay produced different id: first=%d second=%d", id1, id2)
	}
}

// TestMember_SubmitComplaint_InvalidTargetType validates that an invalid
// target_type is rejected with 400.
func TestMember_SubmitComplaint_InvalidTargetType(t *testing.T) {
	username, password := registerMember(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	code, body, _ := doJSON(t, c, "POST", "/complaints", map[string]any{
		"subject":     "Bad target",
		"target_type": "galaxy",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "invalid target_type")
	if body["error"] == nil {
		t.Fatalf("400 missing error field: %v", body)
	}
}

// TestMember_BlockedFromStaffRoutes verifies a member account cannot access
// staff-only endpoints (crawl, complaints list, approvals, etc.).
func TestMember_BlockedFromStaffRoutes(t *testing.T) {
	username, password := registerMember(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	staffOnly := []string{
		"/crawl/nodes",
		"/crawl/jobs",
		"/complaints?limit=1",
		"/approvals?limit=1",
	}
	for _, ep := range staffOnly {
		code, _, _ := doJSON(t, c, "GET", ep, nil, nil)
		if code != http.StatusForbidden {
			t.Errorf("member GET %s: expected 403, got %d", ep, code)
		}
	}
}

// TestMember_CanListOwnComplaints ensures a member can access GET
// /complaints/mine after submitting.
func TestMember_CanListOwnComplaints(t *testing.T) {
	username, password := registerMember(t)

	c := newClient(t)
	loginAs(t, c, username, password)

	doJSON(t, c, "POST", "/complaints", map[string]any{
		"subject": "Mine_" + uniqSuffix(), "target_type": "other",
	}, nil)

	code, body, _ := doJSON(t, c, "GET", "/complaints/mine?limit=10", nil, nil)
	assertStatus(t, code, http.StatusOK, "member GET /complaints/mine")
	if body["items"] == nil {
		t.Fatalf("missing items in /complaints/mine: %v", body)
	}
}
