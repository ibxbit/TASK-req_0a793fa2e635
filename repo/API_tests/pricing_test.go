package apitests

import (
	"net/http"
	"testing"
)

func TestPricing_NoDiscountsTotalEqualsSubtotal(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, raw := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"items": []map[string]any{
			{"sku": "poem:1", "price": 50.0, "quantity": 2},
			{"sku": "poem:2", "price": 30.0, "quantity": 1},
		},
	}, nil)
	assertStatus(t, code, http.StatusOK, "basic quote")
	if v := body["subtotal"]; v != 130.00 {
		t.Fatalf("subtotal: got %v, body=%s", v, raw)
	}
	if v := body["total"]; v != 130.00 {
		t.Fatalf("total: got %v, body=%s", v, raw)
	}
	if v := body["total_discount"]; v != 0.0 && v != nil {
		t.Fatalf("expected 0 discount, got %v", v)
	}
}

func TestPricing_MemberPricedExcluded(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"items": []map[string]any{
			{"sku": "a", "price": 100.0, "quantity": 1, "member_priced": false},
			{"sku": "b", "price": 40.0, "quantity": 1, "member_priced": true},
		},
	}, nil)
	assertStatus(t, code, http.StatusOK, "member-priced mix")
	if v := body["discount_eligible_subtotal"]; v != 100.00 {
		t.Fatalf("eligible: got %v", v)
	}
	if v := body["member_priced_subtotal"]; v != 40.00 {
		t.Fatalf("member subtotal: got %v", v)
	}
}

func TestPricing_UnknownCouponReturnsRejected(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"items":       []map[string]any{{"sku": "x", "price": 50.0, "quantity": 1}},
		"coupon_code": "DOES_NOT_EXIST",
	}, nil)
	assertStatus(t, code, http.StatusOK, "unknown coupon")
	rej, ok := body["rejected"].([]any)
	if !ok || len(rej) == 0 {
		t.Fatalf("expected rejected entries, got %v", body["rejected"])
	}
}

func TestPricing_EmptyItemsReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"items": []any{},
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "empty items")
}

func TestPricing_RequiresAuth(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "POST", "/pricing/quote", map[string]any{
		"items": []map[string]any{{"sku": "x", "price": 1.0, "quantity": 1}},
	}, nil)
	assertStatus(t, code, http.StatusUnauthorized, "unauth quote")
}
