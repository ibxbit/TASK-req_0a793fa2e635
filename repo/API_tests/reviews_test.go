package apitests

import (
	"net/http"
	"testing"
)

// Covers /reviews CRUD, moderate, and object-level auth (users can edit their
// own but not other users'). Also exercises GET /reviews/:id directly so the
// single-read handler is audit-visible.

func TestReviews_AnonRejected(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/reviews", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon list /reviews")

	code, _, _ = doJSON(t, c, "GET", "/reviews/1", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon GET /reviews/:id")
}

func TestReviews_CreateAndList(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)

	code, body, raw := doJSON(t, c, "POST", "/reviews", map[string]any{
		"poem_id":            pID,
		"rating_accuracy":    4,
		"rating_readability": 5,
		"rating_value":       4,
		"title":              "Great",
		"content":            "Very nice",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create /reviews: %d %s", code, raw)
	}
	rid := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/reviews/"+itoa(rid), nil, nil)

	if body["status"] != "pending" {
		t.Fatalf("new review status should be 'pending', got %v", body["status"])
	}
	if v, _ := body["rating"].(float64); int(v) < 1 || int(v) > 5 {
		t.Fatalf("rating out of bounds: %v", body["rating"])
	}
	if int64(body["poem_id"].(float64)) != pID {
		t.Fatalf("poem_id round-trip mismatch: got %v want %d", body["poem_id"], pID)
	}

	code, got, _ := doJSON(t, c, "GET", "/reviews?poem_id="+itoa(pID), nil, nil)
	assertStatus(t, code, http.StatusOK, "list reviews by poem")
	items, _ := got["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least one review for poem %d", pID)
	}
	// Echoed paging params prove the listing handler plumbed them through.
	if lim, _ := got["limit"].(float64); int(lim) <= 0 {
		t.Fatalf("missing/invalid limit echo: %v", got["limit"])
	}
}

// Direct GET /reviews/:id — covers the single-read branch explicitly.
func TestReviews_GetByID(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)

	_, created, _ := doJSON(t, c, "POST", "/reviews", map[string]any{
		"poem_id":            pID,
		"rating_accuracy":    3,
		"rating_readability": 4,
		"rating_value":       5,
		"title":              "GetByID",
		"content":            "body text",
	}, nil)
	rid := int64(created["id"].(float64))
	defer doJSON(t, c, "DELETE", "/reviews/"+itoa(rid), nil, nil)

	// Happy path: 200 + canonical shape.
	code, body, raw := doJSON(t, c, "GET", "/reviews/"+itoa(rid), nil, nil)
	if code != http.StatusOK {
		t.Fatalf("GET /reviews/%d: code=%d body=%s", rid, code, raw)
	}
	for _, k := range []string{"id", "poem_id", "user_id", "status", "rating", "rating_accuracy", "rating_readability", "rating_value", "title", "content"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing field %q in GET /reviews/:id response: %v", k, body)
		}
	}
	if int64(body["id"].(float64)) != rid {
		t.Fatalf("GET /reviews/:id returned wrong id: got %v want %d", body["id"], rid)
	}
	if int64(body["poem_id"].(float64)) != pID {
		t.Fatalf("poem_id mismatch: got %v want %d", body["poem_id"], pID)
	}
	if body["title"] != "GetByID" {
		t.Fatalf("title round-trip: got %v", body["title"])
	}
	if body["status"] != "pending" {
		t.Fatalf("expected status=pending on fresh review, got %v", body["status"])
	}
	if int(body["rating_accuracy"].(float64)) != 3 ||
		int(body["rating_readability"].(float64)) != 4 ||
		int(body["rating_value"].(float64)) != 5 {
		t.Fatalf("rating components round-trip mismatch: %v", body)
	}

	// 404 branch: unknown id.
	code, notFound, _ := doJSON(t, c, "GET", "/reviews/999999999", nil, nil)
	assertStatus(t, code, http.StatusNotFound, "GET unknown review id")
	if _, ok := notFound["error"]; !ok {
		t.Fatalf("404 response missing \"error\" field: %v", notFound)
	}

	// 400 branch: non-numeric id.
	code, bad, _ := doJSON(t, c, "GET", "/reviews/not-a-number", nil, nil)
	assertStatus(t, code, http.StatusBadRequest, "GET /reviews/not-a-number")
	if _, ok := bad["error"]; !ok {
		t.Fatalf("400 response missing \"error\" field: %v", bad)
	}
}

func TestReviews_InvalidRatingReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)
	code, body, _ := doJSON(t, c, "POST", "/reviews", map[string]any{
		"poem_id":            pID,
		"rating_accuracy":    9, // invalid
		"rating_readability": 5,
		"rating_value":       5,
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "rating > 5 rejected")
	if _, ok := body["error"]; !ok {
		t.Fatalf("400 response missing \"error\" field: %v", body)
	}
}

func TestReviews_ModeratorTransitions(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)
	_, body, _ := doJSON(t, c, "POST", "/reviews", map[string]any{
		"poem_id": pID, "rating_accuracy": 3, "rating_readability": 3, "rating_value": 3,
	}, nil)
	rid := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/reviews/"+itoa(rid), nil, nil)

	// reviewer role can moderate
	r := newClient(t)
	loginAs(t, r, userReviewer, passReviewer)
	code, got, _ := doJSON(t, r, "POST", "/reviews/"+itoa(rid)+"/moderate",
		map[string]any{"status": "approved"}, nil)
	assertStatus(t, code, http.StatusOK, "reviewer moderate approved")
	if got["status"] != "approved" {
		t.Fatalf("status not approved: %v", got["status"])
	}
	// A follow-up GET must reflect the moderated status (state transition persisted).
	_, afterMod, _ := doJSON(t, c, "GET", "/reviews/"+itoa(rid), nil, nil)
	if afterMod["status"] != "approved" {
		t.Fatalf("GET /reviews/:id after moderate: status=%v want approved", afterMod["status"])
	}

	// editor role cannot moderate
	ed := newClient(t)
	loginAs(t, ed, userEditor, passEditor)
	code, errBody, _ := doJSON(t, ed, "POST", "/reviews/"+itoa(rid)+"/moderate",
		map[string]any{"status": "hidden"}, nil)
	assertStatus(t, code, http.StatusForbidden, "editor cannot moderate")
	if _, ok := errBody["error"]; !ok {
		t.Fatalf("403 response missing \"error\" field: %v", errBody)
	}

	// invalid status rejected
	code, _, _ = doJSON(t, r, "POST", "/reviews/"+itoa(rid)+"/moderate",
		map[string]any{"status": "made_up"}, nil)
	assertStatus(t, code, http.StatusBadRequest, "invalid moderation status")
}

func TestReviews_OwnerCanUpdateOthersCannot(t *testing.T) {
	// editor creates a review
	ed := newClient(t)
	loginAs(t, ed, userEditor, passEditor)
	pID := seedPoem(t)
	_, body, _ := doJSON(t, ed, "POST", "/reviews", map[string]any{
		"poem_id": pID, "rating_accuracy": 4, "rating_readability": 4, "rating_value": 4,
		"title": "Mine",
	}, nil)
	rid := int64(body["id"].(float64))
	defer doJSON(t, newClientAsAdmin(t), "DELETE", "/reviews/"+itoa(rid), nil, nil)

	// owner (editor) can PUT — assert the update persisted.
	code, updated, _ := doJSON(t, ed, "PUT", "/reviews/"+itoa(rid), map[string]any{
		"poem_id": pID, "rating_accuracy": 5, "rating_readability": 5, "rating_value": 5, "title": "Mine2",
	}, nil)
	assertStatus(t, code, http.StatusOK, "owner PUT own review")
	if updated["title"] != "Mine2" {
		t.Fatalf("PUT did not persist title: %v", updated["title"])
	}
	if int(updated["rating_accuracy"].(float64)) != 5 {
		t.Fatalf("PUT did not persist rating_accuracy: %v", updated["rating_accuracy"])
	}

	// reviewer (non-owner, non-admin) cannot PUT someone else's review.
	rv := newClient(t)
	loginAs(t, rv, userReviewer, passReviewer)
	code, forbidden, _ := doJSON(t, rv, "PUT", "/reviews/"+itoa(rid), map[string]any{
		"poem_id": pID, "rating_accuracy": 1, "rating_readability": 1, "rating_value": 1,
	}, nil)
	assertStatus(t, code, http.StatusForbidden, "non-owner PUT blocked")
	if _, ok := forbidden["error"]; !ok {
		t.Fatalf("403 response missing \"error\" field: %v", forbidden)
	}

	// Confirm the review body wasn't clobbered by the forbidden attempt.
	_, afterGet, _ := doJSON(t, ed, "GET", "/reviews/"+itoa(rid), nil, nil)
	if int(afterGet["rating_accuracy"].(float64)) != 5 {
		t.Fatalf("rating_accuracy unexpectedly mutated by non-owner: %v", afterGet["rating_accuracy"])
	}
}

// newClientAsAdmin is a small convenience for cleanup in tests where the
// primary client was logged in as another role.
func newClientAsAdmin(t *testing.T) *http.Client {
	c := newClient(t)
	loginAdmin(t, c)
	return c
}
