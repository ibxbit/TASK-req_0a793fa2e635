package unittests

import (
	"testing"

	"helios-backend/internal/pricing"
)

// These tests import the backend's pricing package directly and exercise the
// pure logic paths (no DB, no coupon/campaign lookup).

func TestPricing_SubtotalsAndTotalNoDiscount(t *testing.T) {
	res, err := pricing.Quote(pricing.QuoteRequest{
		Items: []pricing.LineItem{
			{SKU: "poem:1", Price: 50, Quantity: 2},
			{SKU: "poem:2", Price: 30, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Subtotal != 130.00 {
		t.Errorf("subtotal=%.2f", res.Subtotal)
	}
	if res.Total != 130.00 {
		t.Errorf("total=%.2f", res.Total)
	}
	if res.TotalDiscount != 0 {
		t.Errorf("discount=%.2f", res.TotalDiscount)
	}
	if res.Currency != "CNY" {
		t.Errorf("currency=%s", res.Currency)
	}
}

func TestPricing_MemberPricedSplit(t *testing.T) {
	res, err := pricing.Quote(pricing.QuoteRequest{
		Items: []pricing.LineItem{
			{SKU: "regular", Price: 100, Quantity: 1, MemberPriced: false},
			{SKU: "member", Price: 40, Quantity: 1, MemberPriced: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EligibleSubtotal != 100.00 {
		t.Errorf("eligible=%.2f", res.EligibleSubtotal)
	}
	if res.MemberSubtotal != 40.00 {
		t.Errorf("member=%.2f", res.MemberSubtotal)
	}
	if res.Subtotal != 140.00 {
		t.Errorf("subtotal=%.2f", res.Subtotal)
	}
}

func TestPricing_EmptyItemsError(t *testing.T) {
	_, err := pricing.Quote(pricing.QuoteRequest{})
	if err == nil {
		t.Fatal("expected error on empty items")
	}
}

func TestPricing_MaxDiscountCapConstant(t *testing.T) {
	if pricing.MaxDiscountPct != 40.0 {
		t.Fatalf("discount cap drifted: %.1f", pricing.MaxDiscountPct)
	}
}

func TestPricing_QuantityDefaultsToOne(t *testing.T) {
	// Quantity 0 is treated as 1 inside the engine
	res, _ := pricing.Quote(pricing.QuoteRequest{
		Items: []pricing.LineItem{{SKU: "x", Price: 25, Quantity: 0}},
	})
	if res.Subtotal != 25.00 {
		t.Fatalf("expected subtotal 25, got %.2f", res.Subtotal)
	}
}
