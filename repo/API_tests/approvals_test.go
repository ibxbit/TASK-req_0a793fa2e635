package apitests

import (
	"net/http"
	"testing"
)

// Covers the approvals workflow end-to-end: enable → delete → approve/reject.

func toggleApproval(t *testing.T, c *http.Client, enabled bool) {
	t.Helper()
	code, _, _ := doJSON(t, c, "PUT", "/settings/approval",
		map[string]any{"enabled": enabled}, nil)
	if code != http.StatusOK {
		t.Fatalf("toggle approval_required=%v: status=%d", enabled, code)
	}
}

func TestApprovals_RejectBatchRevertsDelete(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	toggleApproval(t, c, true)
	defer toggleApproval(t, c, false)

	s := uniqSuffix()
	_, body, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{"name": "Apr_" + s}, nil)
	id := int64(body["id"].(float64))
	_, delResp, _ := doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
	batch, _ := delResp["approval"].(map[string]any)
	if batch == nil {
		t.Fatalf("expected approval meta after delete: %v", delResp)
	}
	batchID, _ := batch["batch_id"].(string)
	if batchID == "" {
		t.Fatalf("batch_id missing: %v", batch)
	}

	// GET /approvals should include this batch.
	code, listed, _ := doJSON(t, c, "GET", "/approvals", nil, nil)
	assertStatus(t, code, http.StatusOK, "list approvals")
	items, _ := listed["items"].([]any)
	found := false
	for _, it := range items {
		if row, ok := it.(map[string]any); ok {
			if row["batch_id"] == batchID {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("batch %s not in pending approvals list", batchID)
	}

	// Reject: should revert the delete (row back).
	code, rej, _ := doJSON(t, c, "POST", "/approvals/"+batchID+"/reject", nil, nil)
	assertStatus(t, code, http.StatusOK, "reject batch")
	if v, _ := rej["reverted"].(float64); int(v) < 1 {
		t.Fatalf("expected reverted >= 1: %v", rej)
	}

	// The original row should exist again.
	code, _, _ = doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "row restored after rejection")

	// Cleanup: disable approval to delete cleanly.
	toggleApproval(t, c, false)
	doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
}

func TestApprovals_ApproveBatchMakesDeletePermanent(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	toggleApproval(t, c, true)
	defer toggleApproval(t, c, false)

	s := uniqSuffix()
	_, body, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{"name": "Apr2_" + s}, nil)
	id := int64(body["id"].(float64))
	_, delResp, _ := doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
	appr, _ := delResp["approval"].(map[string]any)
	batchID, _ := appr["batch_id"].(string)

	code, apr, _ := doJSON(t, c, "POST", "/approvals/"+batchID+"/approve", nil, nil)
	assertStatus(t, code, http.StatusOK, "approve batch")
	if v, _ := apr["approved"].(float64); int(v) < 1 {
		t.Fatalf("expected approved >= 1: %v", apr)
	}

	// Row should stay gone.
	code, _, _ = doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "row stays deleted after approval")
}

func TestApprovals_UnknownBatchIDReturns404(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "POST", "/approvals/does-not-exist/approve", nil, nil)
	assertStatus(t, code, http.StatusNotFound, "approve unknown batch")
	code, _, _ = doJSON(t, c, "POST", "/approvals/does-not-exist/reject", nil, nil)
	assertStatus(t, code, http.StatusNotFound, "reject unknown batch")
}

func TestApprovals_NonAdminForbidden(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userReviewer, passReviewer)
	code, _, _ := doJSON(t, c, "GET", "/approvals", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot list approvals")
	code, _, _ = doJSON(t, c, "POST", "/approvals/x/approve", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot approve")
}
