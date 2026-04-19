package apitests

import (
	"net/http"
	"testing"
)

// Covers /complaints full staff + end-user flow.

func TestComplaints_AnonSubmitRejected(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "POST", "/complaints", map[string]any{
		"subject": "x", "target_type": "other",
	}, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon cannot submit complaint")
}

func TestComplaints_SubmitListMineWorkflow(t *testing.T) {
	// Regular authenticated user (editor here — any active user works)
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	subj := "Subj_" + uniqSuffix()
	code, body, raw := doJSON(t, c, "POST", "/complaints", map[string]any{
		"subject": subj, "target_type": "poem", "notes": "encrypt me",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("submit complaint: %d %s", code, raw)
	}
	if body["arbitration_code"] != "submitted" {
		t.Fatalf("expected initial arbitration_code='submitted', got %v", body["arbitration_code"])
	}
	if body["encryption_scheme"] == nil {
		t.Fatalf("notes should be encrypted and scheme echoed: %v", body)
	}

	code, listed, _ := doJSON(t, c, "GET", "/complaints/mine?limit=10", nil, nil)
	assertStatus(t, code, http.StatusOK, "GET /complaints/mine")
	items, _ := listed["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least one of my complaints")
	}
}

func TestComplaints_InvalidTargetTypeRejected(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	code, _, _ := doJSON(t, c, "POST", "/complaints", map[string]any{
		"subject": "s", "target_type": "planet",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "invalid target_type")
}

func TestComplaints_MissingSubjectRejected(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	code, _, _ := doJSON(t, c, "POST", "/complaints", map[string]any{
		"target_type": "other",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "missing subject")
}

func TestComplaints_StaffListGetAssignResolve(t *testing.T) {
	// A regular user submits a complaint first.
	user := newClient(t)
	loginAs(t, user, userEditor, passEditor)
	subj := "Staff_" + uniqSuffix()
	_, sub, _ := doJSON(t, user, "POST", "/complaints", map[string]any{
		"subject": subj, "target_type": "review", "notes": "secret",
	}, nil)
	cid := int64(sub["id"].(float64))

	// Staff (reviewer) lists and fetches.
	st := newClient(t)
	loginAs(t, st, userReviewer, passReviewer)

	code, body, _ := doJSON(t, st, "GET", "/complaints?limit=50", nil, nil)
	assertStatus(t, code, http.StatusOK, "reviewer list complaints")
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected complaints list to contain our submission")
	}

	code, got, _ := doJSON(t, st, "GET", "/complaints/"+itoa(cid), nil, nil)
	assertStatus(t, code, http.StatusOK, "reviewer get complaint")
	if got["subject"] != subj {
		t.Fatalf("subject mismatch: %v", got["subject"])
	}
	if got["notes"] != "secret" {
		t.Fatalf("notes did not round-trip after decrypt: %v", got["notes"])
	}

	// Assign to self — we need the reviewer's user id. Use /auth/me.
	_, me, _ := doJSON(t, st, "GET", "/auth/me", nil, nil)
	meUser := me["user"].(map[string]any)
	arbID := int64(meUser["id"].(float64))

	code, ass, _ := doJSON(t, st, "POST", "/complaints/"+itoa(cid)+"/assign",
		map[string]any{"arbitrator_id": arbID}, nil)
	assertStatus(t, code, http.StatusOK, "assign complaint")
	if v, _ := ass["arbitrator_id"].(float64); int64(v) != arbID {
		t.Fatalf("assign arbitrator_id mismatch: %v", ass["arbitrator_id"])
	}

	// Resolve — terminal code should populate resolved_at
	code, res, _ := doJSON(t, st, "POST", "/complaints/"+itoa(cid)+"/resolve",
		map[string]any{"arbitration_code": "resolved_upheld", "resolution": "upheld"}, nil)
	assertStatus(t, code, http.StatusOK, "resolve complaint")
	if res["arbitration_code"] != "resolved_upheld" {
		t.Fatalf("resolve code mismatch: %v", res["arbitration_code"])
	}
	if v, _ := res["is_terminal"].(bool); !v {
		t.Fatalf("expected is_terminal=true: %v", res)
	}
}

func TestComplaints_NonStaffBlockedFromStaffRoutes(t *testing.T) {
	// Regular authenticated user hitting staff-only listing/assign/resolve.
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	code, _, _ := doJSON(t, c, "GET", "/complaints?limit=1", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "non-staff list all")

	code, _, _ = doJSON(t, c, "POST", "/complaints/1/assign", map[string]any{"arbitrator_id": 1}, nil)
	if code != http.StatusForbidden {
		t.Fatalf("non-staff assign expected 403, got %d", code)
	}
}

func TestComplaints_ResolveUnknownStatusRejected(t *testing.T) {
	// Submit one first
	u := newClient(t)
	loginAs(t, u, userEditor, passEditor)
	_, sub, _ := doJSON(t, u, "POST", "/complaints", map[string]any{
		"subject": "Rej_" + uniqSuffix(), "target_type": "other",
	}, nil)
	cid := int64(sub["id"].(float64))

	st := newClient(t)
	loginAs(t, st, userReviewer, passReviewer)
	code, body, _ := doJSON(t, st, "POST", "/complaints/"+itoa(cid)+"/resolve",
		map[string]any{"arbitration_code": "no_such_code", "resolution": ""}, nil)
	assertStatus(t, code, http.StatusBadRequest, "unknown arbitration_code")
	if body["error"] == nil {
		t.Fatalf("expected error in body: %v", body)
	}
}
