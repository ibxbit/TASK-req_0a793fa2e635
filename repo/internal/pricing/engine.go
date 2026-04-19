package pricing

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// MaxDiscountPct is the hard cap on combined coupon + campaign discounts,
// expressed against the discount-eligible (non member-priced) subtotal.
const MaxDiscountPct = 40.0

type LineItem struct {
	SKU          string  `json:"sku"`
	Price        float64 `json:"price"`
	Quantity     int     `json:"quantity"`
	MemberPriced bool    `json:"member_priced"`
}

type QuoteRequest struct {
	UserID     *int64     `json:"user_id,omitempty"`
	Items      []LineItem `json:"items"`
	CouponCode string     `json:"coupon_code,omitempty"`
	CampaignID *int64     `json:"campaign_id,omitempty"`
	GroupSize  int        `json:"group_size,omitempty"`
	At         *time.Time `json:"at,omitempty"`
}

type AppliedDiscount struct {
	Type   string  `json:"type"` // "campaign" | "coupon"
	Name   string  `json:"name,omitempty"`
	Code   string  `json:"code,omitempty"`
	Kind   string  `json:"kind,omitempty"` // "percentage" | "fixed"
	Value  float64 `json:"value,omitempty"`
	Amount float64 `json:"amount"`
	Note   string  `json:"note,omitempty"`
}

type RejectedDiscount struct {
	Type   string `json:"type"`
	Name   string `json:"name,omitempty"`
	Code   string `json:"code,omitempty"`
	Reason string `json:"reason"`
}

type QuoteResult struct {
	Currency          string             `json:"currency"`
	At                time.Time          `json:"at"`
	Subtotal          float64            `json:"subtotal"`
	MemberSubtotal    float64            `json:"member_priced_subtotal"`
	EligibleSubtotal  float64            `json:"discount_eligible_subtotal"`
	TotalDiscount     float64            `json:"total_discount"`
	DiscountPercent   float64            `json:"discount_percent"`
	Total             float64            `json:"total"`
	CapApplied        bool               `json:"cap_applied"`
	CapNote           string             `json:"cap_note,omitempty"`
	Applied           []AppliedDiscount  `json:"applied"`
	Rejected          []RejectedDiscount `json:"rejected,omitempty"`
	StackingViolation string             `json:"stacking_violation,omitempty"`
}

// Quote computes the final price and discount breakdown. It enforces:
//   - max 1 coupon + 1 campaign (stacking cap)
//   - max 40% combined discount against the discount-eligible subtotal
//   - member-priced items are fully excluded from discounts
func Quote(req QuoteRequest) (*QuoteResult, error) {
	now := time.Now()
	if req.At != nil && !req.At.IsZero() {
		now = *req.At
	}

	result := &QuoteResult{
		Currency: "CNY",
		At:       now,
		Applied:  []AppliedDiscount{},
	}

	if len(req.Items) == 0 {
		return result, fmt.Errorf("items required")
	}

	var subtotal, memberSub, eligible float64
	for _, it := range req.Items {
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		line := it.Price * float64(qty)
		subtotal += line
		if it.MemberPriced {
			memberSub += line
		} else {
			eligible += line
		}
	}
	result.Subtotal = round2(subtotal)
	result.MemberSubtotal = round2(memberSub)
	result.EligibleSubtotal = round2(eligible)

	groupSize := req.GroupSize
	if groupSize < 1 {
		groupSize = 1
	}

	// Campaign (0 or 1)
	var campaignAmt float64
	if req.CampaignID != nil {
		amt, applied, rejected := applyCampaign(*req.CampaignID, eligible, groupSize, now)
		campaignAmt = amt
		if applied != nil {
			result.Applied = append(result.Applied, *applied)
		}
		if rejected != nil {
			result.Rejected = append(result.Rejected, *rejected)
		}
	}

	// Coupon (0 or 1)
	var couponAmt float64
	if code := strings.TrimSpace(req.CouponCode); code != "" {
		amt, applied, rejected := applyCoupon(code, eligible, req.UserID, now)
		couponAmt = amt
		if applied != nil {
			result.Applied = append(result.Applied, *applied)
		}
		if rejected != nil {
			result.Rejected = append(result.Rejected, *rejected)
		}
	}

	// Cap at 40% of eligible
	total := campaignAmt + couponAmt
	if eligible > 0 {
		cap := eligible * (MaxDiscountPct / 100.0)
		if total > cap && total > 0 {
			scale := cap / total
			for i := range result.Applied {
				result.Applied[i].Amount = round2(result.Applied[i].Amount * scale)
			}
			total = cap
			result.CapApplied = true
			result.CapNote = fmt.Sprintf("combined discounts scaled down to %.0f%% cap", MaxDiscountPct)
		}
	}

	result.TotalDiscount = round2(total)
	if eligible > 0 {
		result.DiscountPercent = round2(total / eligible * 100)
	}
	result.Total = round2(subtotal - total)

	return result, nil
}

func round2(x float64) float64 {
	return math.Round(x*100) / 100
}

func computeDiscount(kind string, value, base float64) float64 {
	switch kind {
	case "percentage":
		return round2(base * value / 100.0)
	case "fixed":
		if value > base {
			value = base
		}
		return round2(value)
	}
	return 0
}
