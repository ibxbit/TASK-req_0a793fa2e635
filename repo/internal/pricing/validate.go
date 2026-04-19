package pricing

import (
	"database/sql"
	"fmt"
	"time"

	"helios-backend/internal/db"
)

func applyCampaign(id int64, eligible float64, groupSize int, now time.Time) (float64, *AppliedDiscount, *RejectedDiscount) {
	var (
		name, campType, discType, status string
		discValue                        float64
		minGroup                         sql.NullInt32
		starts, ends                     sql.NullTime
	)
	err := db.DB.QueryRow(`
		SELECT name, campaign_type, discount_type, discount_value, status,
		       min_group_size, starts_at, ends_at
		FROM campaigns WHERE id = ?`, id).Scan(
		&name, &campType, &discType, &discValue, &status,
		&minGroup, &starts, &ends,
	)
	if err == sql.ErrNoRows {
		return 0, nil, &RejectedDiscount{Type: "campaign", Reason: "campaign not found"}
	}
	if err != nil {
		return 0, nil, &RejectedDiscount{Type: "campaign", Reason: "campaign lookup failed"}
	}

	if status != "active" {
		return 0, nil, &RejectedDiscount{Type: "campaign", Name: name,
			Reason: fmt.Sprintf("campaign status is %q", status)}
	}
	if starts.Valid && now.Before(starts.Time) {
		return 0, nil, &RejectedDiscount{Type: "campaign", Name: name, Reason: "campaign has not started"}
	}
	if ends.Valid && now.After(ends.Time) {
		return 0, nil, &RejectedDiscount{Type: "campaign", Name: name, Reason: "campaign has ended"}
	}
	if campType == "group_buy" {
		if !minGroup.Valid || int(minGroup.Int32) < 1 {
			return 0, nil, &RejectedDiscount{Type: "campaign", Name: name,
				Reason: "group_buy campaign missing min_group_size"}
		}
		if groupSize < int(minGroup.Int32) {
			return 0, nil, &RejectedDiscount{Type: "campaign", Name: name,
				Reason: fmt.Sprintf("group_buy requires %d participants, got %d",
					minGroup.Int32, groupSize)}
		}
	}

	if eligible <= 0 {
		return 0, nil, &RejectedDiscount{Type: "campaign", Name: name,
			Reason: "no discount-eligible items"}
	}

	amt := computeDiscount(discType, discValue, eligible)
	if amt <= 0 {
		return 0, nil, &RejectedDiscount{Type: "campaign", Name: name,
			Reason: "computed discount is zero"}
	}

	note := campType
	if campType == "group_buy" && minGroup.Valid {
		note = fmt.Sprintf("group_buy (min %d, actual %d)", minGroup.Int32, groupSize)
	}
	return amt, &AppliedDiscount{
		Type:   "campaign",
		Name:   name,
		Kind:   discType,
		Value:  discValue,
		Amount: amt,
		Note:   note,
	}, nil
}

func applyCoupon(code string, eligible float64, userID *int64, now time.Time) (float64, *AppliedDiscount, *RejectedDiscount) {
	var (
		id                    int64
		discType              string
		discValue             float64
		minAmount             sql.NullFloat64
		usageLimit            sql.NullInt32
		usedCount             int
		perUserLimit          sql.NullInt32
		starts, ends          sql.NullTime
		status                string
	)
	err := db.DB.QueryRow(`
		SELECT id, discount_type, discount_value, min_amount, usage_limit,
		       used_count, per_user_limit, starts_at, ends_at, status
		FROM coupons WHERE code = ?`, code).Scan(
		&id, &discType, &discValue, &minAmount, &usageLimit,
		&usedCount, &perUserLimit, &starts, &ends, &status,
	)
	if err == sql.ErrNoRows {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code, Reason: "coupon not found"}
	}
	if err != nil {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code, Reason: "coupon lookup failed"}
	}

	if status != "active" {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code,
			Reason: fmt.Sprintf("coupon status is %q", status)}
	}
	if starts.Valid && now.Before(starts.Time) {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code, Reason: "coupon has not started"}
	}
	if ends.Valid && now.After(ends.Time) {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code, Reason: "coupon has expired"}
	}
	if usageLimit.Valid && usedCount >= int(usageLimit.Int32) {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code,
			Reason: "global usage limit reached"}
	}
	if minAmount.Valid && eligible < minAmount.Float64 {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code,
			Reason: fmt.Sprintf("requires minimum amount %.2f", minAmount.Float64)}
	}

	if perUserLimit.Valid && userID != nil && *userID > 0 {
		var n int
		if err := db.DB.QueryRow(`
			SELECT COUNT(*) FROM orders
			WHERE user_id = ? AND coupon_id = ? AND status IN ('pending','paid')`,
			*userID, id).Scan(&n); err == nil {
			if n >= int(perUserLimit.Int32) {
				return 0, nil, &RejectedDiscount{Type: "coupon", Code: code,
					Reason: "per-user usage limit reached"}
			}
		}
	}

	if eligible <= 0 {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code,
			Reason: "no discount-eligible items"}
	}

	amt := computeDiscount(discType, discValue, eligible)
	if amt <= 0 {
		return 0, nil, &RejectedDiscount{Type: "coupon", Code: code,
			Reason: "computed discount is zero"}
	}

	note := discType
	if usageLimit.Valid && usageLimit.Int32 == 1 {
		note = "single-use " + note
	}
	return amt, &AppliedDiscount{
		Type:   "coupon",
		Code:   code,
		Kind:   discType,
		Value:  discValue,
		Amount: amt,
		Note:   note,
	}, nil
}
