package apitests

import (
	"net/http"
	"testing"
)

// Extra coverage specifically for the complaint assignment hardening:
// assigning to a non-staff account must be rejected even when the caller
// is an administrator.

func TestComplaints_AssignToNonArbitratorRejected(t *testing.T) {
	// Step 1: find the seeded `member` user's id via the admin-only API
	// over a 2-step lookup. There's no /users endpoint, so we rely on the
	// fact that audit entries or complaints by that user will surface the
	// id. Simpler: login as member, create a complaint, capture the id
	// from /complaints/mine (the complainant_id is the member's user id).
	memberClient := newClient(t)
	loginAs(t, memberClient, userMember, passMember)

	code, body, _ := doJSON(t, memberClient, "POST", "/complaints", map[string]any{
		"subject":     "test member complaint",
		"target_type": "other",
		"notes":       "hidden",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("member complaint submit: %d %v", code, body)
	}
	memberUserID := int64(body["complainant_id"].(float64))

	// Step 2: admin tries to assign that complaint to the member — must fail.
	adminClient := newClient(t)
	loginAdmin(t, adminClient)
	complaintID := int64(body["id"].(float64))
	code, forbidden, _ := doJSON(t, adminClient, "POST", "/complaints/"+itoa(complaintID)+"/assign",
		map[string]any{"arbitrator_id": memberUserID}, nil)
	assertStatus(t, code, http.StatusBadRequest, "member cannot be arbitrator")
	if forbidden["error"] == nil {
		t.Fatalf("400 missing error field: %v", forbidden)
	}

	// Step 3: but assigning to a reviewer works. We need the reviewer's id.
	// Login as reviewer and submit a dummy complaint so we can capture it.
	rv := newClient(t)
	loginAs(t, rv, userReviewer, passReviewer)
	_, myBody, _ := doJSON(t, rv, "POST", "/complaints", map[string]any{
		"subject": "rv test", "target_type": "other",
	}, nil)
	reviewerID := int64(myBody["complainant_id"].(float64))

	code, ok, _ := doJSON(t, adminClient, "POST", "/complaints/"+itoa(complaintID)+"/assign",
		map[string]any{"arbitrator_id": reviewerID}, nil)
	assertStatus(t, code, http.StatusOK, "admin can assign to reviewer")
	if ok["arbitrator_role"] != "reviewer" {
		t.Fatalf("expected arbitrator_role=reviewer in response, got %v", ok)
	}
}

func TestComplaints_AssignToUnknownUser(t *testing.T) {
	// Submit a complaint first so we have something to assign.
	memberClient := newClient(t)
	loginAs(t, memberClient, userMember, passMember)
	_, body, _ := doJSON(t, memberClient, "POST", "/complaints", map[string]any{
		"subject": "x", "target_type": "other",
	}, nil)
	complaintID := int64(body["id"].(float64))

	adminClient := newClient(t)
	loginAdmin(t, adminClient)
	code, resp, _ := doJSON(t, adminClient, "POST", "/complaints/"+itoa(complaintID)+"/assign",
		map[string]any{"arbitrator_id": 999999999}, nil)
	assertStatus(t, code, http.StatusBadRequest, "unknown user id")
	if resp["error"] == nil {
		t.Fatalf("400 missing error field")
	}
}
