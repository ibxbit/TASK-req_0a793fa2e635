package apitests

import (
	"net/http"
	"testing"
)

// Covers PUT /settings/approval and approval round-trip via deletion of a dynasty.

func TestSettings_PutApprovalAdminRoundTrip(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)

	// Turn it on
	code, body, _ := doJSON(t, c, "PUT", "/settings/approval",
		map[string]any{"enabled": true}, nil)
	assertStatus(t, code, http.StatusOK, "enable approval_required")
	if body["approval_required"] != true {
		t.Fatalf("expected approval_required=true, got %v", body["approval_required"])
	}

	// GET should reflect it
	code, got, _ := doJSON(t, c, "GET", "/settings/approval", nil, nil)
	assertStatus(t, code, http.StatusOK, "get approval")
	if got["approval_required"] != true {
		t.Fatalf("expected true on read-back, got %v", got["approval_required"])
	}

	// Now delete a freshly created dynasty — should return a pending batch.
	_, dBody, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name": "ApprDyn_" + uniqSuffix(),
	}, nil)
	dID := int64(dBody["id"].(float64))
	code, delResp, _ := doJSON(t, c, "DELETE", "/dynasties/"+itoa(dID), nil, nil)
	assertStatus(t, code, http.StatusOK, "delete while approval enabled")
	appr, _ := delResp["approval"].(map[string]any)
	if appr == nil {
		t.Fatalf("expected 'approval' metadata on delete response: %v", delResp)
	}
	if appr["status"] != "pending" {
		t.Fatalf("approval.status: %v", appr["status"])
	}
	if appr["batch_id"] == "" || appr["batch_id"] == nil {
		t.Fatalf("approval.batch_id missing: %v", appr)
	}

	// Turn it off for subsequent tests.
	code, _, _ = doJSON(t, c, "PUT", "/settings/approval",
		map[string]any{"enabled": false}, nil)
	assertStatus(t, code, http.StatusOK, "disable approval_required")
}

func TestSettings_PutApprovalRequiresAdmin(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	code, _, _ := doJSON(t, c, "PUT", "/settings/approval",
		map[string]any{"enabled": true}, nil)
	assertStatus(t, code, http.StatusForbidden, "editor cannot toggle approval")
}

func TestSettings_InvalidBodyReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body := doRaw(t, c, "PUT", "/settings/approval", "not-json")
	assertStatus(t, code, http.StatusBadRequest, "invalid body")
	if !containsStr(body, `"error"`) {
		t.Fatalf("expected error field: %s", body)
	}
}
