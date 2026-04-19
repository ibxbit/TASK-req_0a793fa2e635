package apitests

import (
	"net/http"
	"testing"
)

// Verifies the security fix: /pricing/quote no longer trusts a client-supplied
// user_id. Only admins/marketing_managers may quote on behalf of another user.

func TestPricing_SpoofedUserIDRejected(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)

	// Fetch editor's real user id
	_, me, _ := doJSON(t, c, "GET", "/auth/me", nil, nil)
	selfID := int64(me["user"].(map[string]any)["id"].(float64))
	other := selfID + 9999

	code, body, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"user_id": other,
		"items":   []map[string]any{{"sku": "x", "price": 10.0, "quantity": 1}},
	}, nil)
	assertStatus(t, code, http.StatusForbidden, "editor quoting as another user")
	if body["error"] == nil {
		t.Fatalf("expected error in body: %v", body)
	}
}

func TestPricing_AdminMayQuoteForOther(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"user_id": 42,
		"items":   []map[string]any{{"sku": "x", "price": 10.0, "quantity": 1}},
	}, nil)
	assertStatus(t, code, http.StatusOK, "admin quoting for another user")
	if _, ok := body["subtotal"]; !ok {
		t.Fatalf("expected subtotal in response: %v", body)
	}
}

func TestPricing_MarketingManagerMayQuoteForOther(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userMkt, passMkt)
	code, _, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"user_id": 42,
		"items":   []map[string]any{{"sku": "x", "price": 10.0, "quantity": 1}},
	}, nil)
	assertStatus(t, code, http.StatusOK, "marketing_manager quoting for another user")
}
