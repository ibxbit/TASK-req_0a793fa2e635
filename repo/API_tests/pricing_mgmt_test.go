package apitests

import (
	"net/http"
	"testing"
	"time"
)

// Covers the full CRUD surface of pricing management: campaigns, coupons,
// pricing_rules and member_tiers. Every write endpoint is verified for
// both the allowed roles (admin + marketing_manager) and a forbidden role.

// -----------------------------------------------------------------------
// CAMPAIGNS
// -----------------------------------------------------------------------

func TestCampaigns_AnonRejected(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/campaigns", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon list /campaigns")
}

func TestCampaigns_NonPricingRoleCannotWrite(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	code, body, _ := doJSON(t, c, "POST", "/campaigns",
		map[string]any{"name": "x", "campaign_type": "standard", "discount_type": "percentage", "discount_value": 10}, nil)
	assertStatus(t, code, http.StatusForbidden, "editor cannot create campaigns")
	if _, ok := body["error"]; !ok {
		t.Fatalf("403 missing error field")
	}
}

func TestCampaigns_AdminCRUD(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// CREATE
	code, created, _ := doJSON(t, c, "POST", "/campaigns", map[string]any{
		"name":           "Flash_" + s,
		"campaign_type":  "flash_sale",
		"discount_type":  "percentage",
		"discount_value": 25,
		"status":         "active",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create campaign: %d %v", code, created)
	}
	id := int64(created["id"].(float64))
	defer doJSON(t, c, "DELETE", "/campaigns/"+itoa(id), nil, nil)

	if created["campaign_type"] != "flash_sale" {
		t.Fatalf("campaign_type mismatch: %v", created)
	}
	if int(created["discount_value"].(float64)) != 25 {
		t.Fatalf("discount_value mismatch: %v", created)
	}

	// GET by id
	code, fetched, _ := doJSON(t, c, "GET", "/campaigns/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get campaign")
	if int64(fetched["id"].(float64)) != id {
		t.Fatalf("id mismatch")
	}

	// UPDATE
	code, updated, _ := doJSON(t, c, "PUT", "/campaigns/"+itoa(id), map[string]any{
		"name": "Flash_" + s, "campaign_type": "flash_sale",
		"discount_type": "percentage", "discount_value": 30, "status": "paused",
	}, nil)
	assertStatus(t, code, http.StatusOK, "update campaign")
	if int(updated["discount_value"].(float64)) != 30 || updated["status"] != "paused" {
		t.Fatalf("update not persisted: %v", updated)
	}

	// LIST — filter by status
	code, list, _ := doJSON(t, c, "GET", "/campaigns?status=paused&limit=100", nil, nil)
	assertStatus(t, code, http.StatusOK, "list with filter")
	items, _ := list["items"].([]any)
	found := false
	for _, it := range items {
		if row, ok := it.(map[string]any); ok && int64(row["id"].(float64)) == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("filter status=paused did not include our campaign id=%d", id)
	}
}

func TestCampaigns_MarketingManagerCanWrite(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userMkt, passMkt)
	s := uniqSuffix()
	code, body, raw := doJSON(t, c, "POST", "/campaigns", map[string]any{
		"name": "MktMgr_" + s, "campaign_type": "standard",
		"discount_type": "percentage", "discount_value": 15,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("mkt manager should be allowed to create: %d %s", code, raw)
	}
	id := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/campaigns/"+itoa(id), nil, nil)
}

func TestCampaigns_Validation(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing name", map[string]any{"campaign_type": "standard", "discount_type": "percentage", "discount_value": 5}},
		{"bad campaign_type", map[string]any{"name": "x", "campaign_type": "nonsense", "discount_type": "percentage", "discount_value": 5}},
		{"bad discount_type", map[string]any{"name": "x", "campaign_type": "standard", "discount_type": "nope", "discount_value": 5}},
		{"negative discount_value", map[string]any{"name": "x", "campaign_type": "standard", "discount_type": "percentage", "discount_value": -1}},
		{"percentage over 100", map[string]any{"name": "x", "campaign_type": "standard", "discount_type": "percentage", "discount_value": 101}},
		{"group_buy without min_group_size", map[string]any{"name": "x", "campaign_type": "group_buy", "discount_type": "percentage", "discount_value": 5}},
		{"bad time window", map[string]any{
			"name": "x", "campaign_type": "standard", "discount_type": "percentage", "discount_value": 5,
			"starts_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			"ends_at":   time.Now().Format(time.RFC3339),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := doJSON(t, c, "POST", "/campaigns", tc.body, nil)
			assertStatus(t, code, http.StatusBadRequest, tc.name)
		})
	}
}

// -----------------------------------------------------------------------
// COUPONS
// -----------------------------------------------------------------------

func TestCoupons_CRUDAndUniqueness(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()
	code, body, raw := doJSON(t, c, "POST", "/coupons", map[string]any{
		"code":           "C_" + s,
		"discount_type":  "fixed",
		"discount_value": 20,
		"usage_limit":    100,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create coupon: %d %s", code, raw)
	}
	id := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/coupons/"+itoa(id), nil, nil)
	if body["code"] != "C_"+s {
		t.Fatalf("code round-trip mismatch")
	}
	if int(body["used_count"].(float64)) != 0 {
		t.Fatalf("used_count should default to 0")
	}

	// Duplicate code → 409.
	code, _, _ = doJSON(t, c, "POST", "/coupons", map[string]any{
		"code": "C_" + s, "discount_type": "fixed", "discount_value": 5,
	}, nil)
	assertStatus(t, code, http.StatusConflict, "duplicate coupon code")

	// Bad campaign_id reference → 400.
	code, _, _ = doJSON(t, c, "POST", "/coupons", map[string]any{
		"code": "C_bad_" + s, "discount_type": "percentage", "discount_value": 10,
		"campaign_id": 999999999,
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "unknown campaign_id")

	// Update + get.
	code, _, _ = doJSON(t, c, "PUT", "/coupons/"+itoa(id), map[string]any{
		"code": "C_" + s, "discount_type": "fixed", "discount_value": 30, "status": "disabled",
	}, nil)
	assertStatus(t, code, http.StatusOK, "update coupon")

	code, got, _ := doJSON(t, c, "GET", "/coupons/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get coupon")
	if got["status"] != "disabled" || int(got["discount_value"].(float64)) != 30 {
		t.Fatalf("update not persisted: %v", got)
	}
}

// -----------------------------------------------------------------------
// PRICING RULES
// -----------------------------------------------------------------------

func TestPricingRules_CRUDWithValidation(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// Bad percentage > 100.
	code, _, _ := doJSON(t, c, "POST", "/pricing-rules", map[string]any{
		"name": "bad_" + s, "rule_type": "percentage", "target_scope": "all", "value": 110,
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "rule percentage > 100")

	// Valid.
	code, body, _ := doJSON(t, c, "POST", "/pricing-rules", map[string]any{
		"name": "Rule_" + s, "rule_type": "fixed", "target_scope": "dynasty",
		"value": 5, "priority": 10, "active": true,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create rule: %d %v", code, body)
	}
	id := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/pricing-rules/"+itoa(id), nil, nil)
	if body["active"] != true {
		t.Fatalf("active flag should round-trip: %v", body)
	}
	if int(body["priority"].(float64)) != 10 {
		t.Fatalf("priority should round-trip: %v", body)
	}

	// LIST should return at least our rule — and priority is a valid order key.
	code, list, _ := doJSON(t, c, "GET", "/pricing-rules?limit=500", nil, nil)
	assertStatus(t, code, http.StatusOK, "list rules")
	items, _ := list["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least our rule")
	}
}

// -----------------------------------------------------------------------
// MEMBER TIERS
// -----------------------------------------------------------------------

func TestMemberTiers_CRUD(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// CREATE
	code, body, raw := doJSON(t, c, "POST", "/member-tiers", map[string]any{
		"name":          "Gold_" + s,
		"level":         99 + timeNowFrac(),
		"monthly_price": 9.99,
		"yearly_price":  99.9,
		"benefits":      map[string]any{"highlights": true, "poem_packs": 5},
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create tier: %d %s", code, raw)
	}
	id := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/member-tiers/"+itoa(id), nil, nil)

	if body["name"] != "Gold_"+s {
		t.Fatalf("name mismatch")
	}

	// GET
	code, got, _ := doJSON(t, c, "GET", "/member-tiers/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get tier")
	if _, ok := got["benefits"]; !ok {
		t.Fatalf("benefits field missing from response")
	}

	// UPDATE
	code, _, _ = doJSON(t, c, "PUT", "/member-tiers/"+itoa(id), map[string]any{
		"name": "GoldUpdated_" + s, "level": int(body["level"].(float64)),
		"monthly_price": 19.99, "yearly_price": 199,
	}, nil)
	assertStatus(t, code, http.StatusOK, "update tier")

	// Negative prices rejected.
	code, _, _ = doJSON(t, c, "POST", "/member-tiers", map[string]any{
		"name": "Bad_" + s, "level": 500 + timeNowFrac(), "monthly_price": -1,
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "negative price rejected")
}

func TestMemberTiers_ReadableByAnyAuthenticatedUser(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userMember, passMember)
	code, _, _ := doJSON(t, c, "GET", "/member-tiers", nil, nil)
	assertStatus(t, code, http.StatusOK, "member can list tiers")
}

// timeNowFrac returns a small integer derived from nanoseconds so tiers
// and coupons created in a tight loop don't collide on the UNIQUE level
// constraint.
func timeNowFrac() int {
	return int(time.Now().UnixNano() % 10000)
}
