package pricing

import "testing"

func TestQuote_NoDiscounts_TotalEqualsSubtotal(t *testing.T) {
	res, err := Quote(QuoteRequest{
		Items: []LineItem{
			{SKU: "poem:1", Price: 50, Quantity: 2},
			{SKU: "poem:2", Price: 30, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Subtotal != 130.00 {
		t.Errorf("subtotal: got %.2f", res.Subtotal)
	}
	if res.Total != 130.00 {
		t.Errorf("total: got %.2f", res.Total)
	}
	if res.TotalDiscount != 0 {
		t.Errorf("expected 0 discount, got %.2f", res.TotalDiscount)
	}
	if len(res.Applied) != 0 {
		t.Errorf("expected no applied, got %d", len(res.Applied))
	}
}

func TestQuote_MemberPricedExcluded(t *testing.T) {
	res, _ := Quote(QuoteRequest{
		Items: []LineItem{
			{SKU: "a", Price: 100, Quantity: 1, MemberPriced: false},
			{SKU: "b", Price: 40,  Quantity: 1, MemberPriced: true},
		},
	})
	if res.EligibleSubtotal != 100.00 {
		t.Errorf("eligible: got %.2f", res.EligibleSubtotal)
	}
	if res.MemberSubtotal != 40.00 {
		t.Errorf("member subtotal: got %.2f", res.MemberSubtotal)
	}
	if res.Subtotal != 140.00 {
		t.Errorf("subtotal: got %.2f", res.Subtotal)
	}
}

func TestQuote_EmptyItems_Errors(t *testing.T) {
	_, err := Quote(QuoteRequest{})
	if err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestComputeDiscount_Percentage(t *testing.T) {
	got := computeDiscount("percentage", 25, 200)
	if got != 50.00 {
		t.Errorf("expected 50, got %.2f", got)
	}
}

func TestComputeDiscount_FixedCapsAtBase(t *testing.T) {
	// fixed amount larger than base must clamp to base
	got := computeDiscount("fixed", 500, 120)
	if got != 120.00 {
		t.Errorf("expected clamp to 120, got %.2f", got)
	}
}

func TestComputeDiscount_UnknownKind(t *testing.T) {
	got := computeDiscount("mystery", 99, 100)
	if got != 0 {
		t.Errorf("expected 0 for unknown kind, got %.2f", got)
	}
}

func TestRound2(t *testing.T) {
	if round2(1.005) < 1.00 || round2(1.005) > 1.02 {
		t.Errorf("round2(1.005) unexpected: %.4f", round2(1.005))
	}
	if round2(1.994) != 1.99 {
		t.Errorf("round2(1.994) = %.4f, want 1.99", round2(1.994))
	}
}

func TestMaxDiscountPct_Constant(t *testing.T) {
	if MaxDiscountPct != 40.0 {
		t.Fatalf("cap constant drifted: %v", MaxDiscountPct)
	}
}
