package handlers

// Pricing management CRUD: campaigns, coupons, pricing_rules, member_tiers.
//
// Read endpoints (GET) are available to any authenticated user so UIs can
// render eligibility and tier information; write endpoints (POST/PUT/DELETE)
// are restricted to the administrator and marketing_manager roles, which are
// the two personas explicitly responsible for revenue configuration.
//
// Validation rules enforced here:
//   - campaign/coupon time window: starts_at <= ends_at when both provided
//   - percentage discount_value is within 0..100 (40% combined cap is applied
//     at quote time — we still want the individual input to be a real pct)
//   - fixed-amount discount_value is non-negative
//   - campaign_type `group_buy` requires min_group_size > 1
//   - coupon code uniqueness is enforced at the SQL layer (UNIQUE KEY)
//   - member_tier level uniqueness is enforced at the SQL layer (UNIQUE KEY)

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"helios-backend/internal/audit"
	"helios-backend/internal/auth"
	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

func RegisterPricingManagement(r *gin.RouterGroup) {
	write := auth.RequireRole("administrator", "marketing_manager")

	g := r.Group("", auth.AuthRequired())

	// --- campaigns ---
	g.GET("/campaigns", listCampaigns)
	g.GET("/campaigns/:id", getCampaign)
	g.POST("/campaigns", write, createCampaign)
	g.PUT("/campaigns/:id", write, updateCampaign)
	g.DELETE("/campaigns/:id", write, deleteCampaign)

	// --- coupons ---
	g.GET("/coupons", listCoupons)
	g.GET("/coupons/:id", getCoupon)
	g.POST("/coupons", write, createCoupon)
	g.PUT("/coupons/:id", write, updateCoupon)
	g.DELETE("/coupons/:id", write, deleteCoupon)

	// --- pricing-rules ---
	g.GET("/pricing-rules", listPricingRules)
	g.GET("/pricing-rules/:id", getPricingRule)
	g.POST("/pricing-rules", write, createPricingRule)
	g.PUT("/pricing-rules/:id", write, updatePricingRule)
	g.DELETE("/pricing-rules/:id", write, deletePricingRule)

	// --- member-tiers ---
	g.GET("/member-tiers", listMemberTiers)
	g.GET("/member-tiers/:id", getMemberTier)
	g.POST("/member-tiers", write, createMemberTier)
	g.PUT("/member-tiers/:id", write, updateMemberTier)
	g.DELETE("/member-tiers/:id", write, deleteMemberTier)
}

// =====================================================================
// CAMPAIGNS
// =====================================================================

type Campaign struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	CampaignType  string     `json:"campaign_type"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue float64    `json:"discount_value"`
	MinGroupSize  *int       `json:"min_group_size,omitempty"`
	Status        string     `json:"status"`
	StartsAt      *time.Time `json:"starts_at,omitempty"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	CreatedBy     *int64     `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

var validCampaignTypes = map[string]bool{"standard": true, "flash_sale": true, "group_buy": true}
var validCampaignStatuses = map[string]bool{"draft": true, "active": true, "paused": true, "ended": true}
var validDiscountTypes = map[string]bool{"percentage": true, "fixed": true}

func validateCampaign(c *Campaign) error {
	if c.Name == "" {
		return errors.New("name required")
	}
	if !validCampaignTypes[c.CampaignType] {
		return errors.New("campaign_type must be one of standard|flash_sale|group_buy")
	}
	if !validDiscountTypes[c.DiscountType] {
		return errors.New("discount_type must be percentage or fixed")
	}
	if c.DiscountValue < 0 {
		return errors.New("discount_value must be non-negative")
	}
	if c.DiscountType == "percentage" && c.DiscountValue > 100 {
		return errors.New("percentage discount_value must be <= 100")
	}
	if c.Status != "" && !validCampaignStatuses[c.Status] {
		return errors.New("status must be one of draft|active|paused|ended")
	}
	if c.StartsAt != nil && c.EndsAt != nil && c.StartsAt.After(*c.EndsAt) {
		return errors.New("starts_at must be <= ends_at")
	}
	if c.CampaignType == "group_buy" {
		if c.MinGroupSize == nil || *c.MinGroupSize < 2 {
			return errors.New("group_buy requires min_group_size >= 2")
		}
	}
	return nil
}

func scanCampaign(row interface{ Scan(...any) error }) (*Campaign, error) {
	var c Campaign
	var desc sql.NullString
	var mgs sql.NullInt32
	var starts, ends sql.NullTime
	var creator sql.NullInt64
	if err := row.Scan(&c.ID, &c.Name, &desc, &c.CampaignType, &c.DiscountType, &c.DiscountValue,
		&mgs, &c.Status, &starts, &ends, &creator, &c.CreatedAt); err != nil {
		return nil, err
	}
	if desc.Valid {
		c.Description = desc.String
	}
	if mgs.Valid {
		v := int(mgs.Int32)
		c.MinGroupSize = &v
	}
	if starts.Valid {
		t := starts.Time
		c.StartsAt = &t
	}
	if ends.Valid {
		t := ends.Time
		c.EndsAt = &t
	}
	if creator.Valid {
		v := creator.Int64
		c.CreatedBy = &v
	}
	return &c, nil
}

const campaignCols = `id, name, description, campaign_type, discount_type, discount_value,
		 min_group_size, status, starts_at, ends_at, created_by, created_at`

func listCampaigns(c *gin.Context) {
	p := readPaging(c)
	args := []any{}
	q := `SELECT ` + campaignCols + ` FROM campaigns`
	where := []string{}
	if v := c.Query("status"); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	if v := c.Query("campaign_type"); v != "" {
		where = append(where, "campaign_type = ?")
		args = append(args, v)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Campaign{}
	for rows.Next() {
		cm, err := scanCampaign(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *cm)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getCampaign(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cm, err := scanCampaign(db.DB.QueryRow(`SELECT `+campaignCols+` FROM campaigns WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, cm)
}

func createCampaign(c *gin.Context) {
	var in Campaign
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.DiscountType == "" {
		in.DiscountType = "percentage"
	}
	if in.CampaignType == "" {
		in.CampaignType = "standard"
	}
	if err := validateCampaign(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	sess, _ := auth.CurrentSession(c)
	var creator any = nil
	if sess != nil {
		creator = sess.UserID
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		`INSERT INTO campaigns (name, description, campaign_type, discount_type, discount_value,
			min_group_size, status, starts_at, ends_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, nullStrIf(in.Description != "", in.Description),
		in.CampaignType, in.DiscountType, in.DiscountValue,
		nullIntPtr(in.MinGroupSize), in.Status,
		nullTimePtr(in.StartsAt), nullTimePtr(in.EndsAt), creator,
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	in.ID = id
	in.CreatedAt = time.Now()
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, "campaign", id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateCampaign(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in Campaign
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	in.ID = id
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanCampaign(tx.QueryRow(`SELECT `+campaignCols+` FROM campaigns WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	// Preserve existing values for omitted fields
	if in.Name == "" {
		in.Name = before.Name
	}
	if in.CampaignType == "" {
		in.CampaignType = before.CampaignType
	}
	if in.DiscountType == "" {
		in.DiscountType = before.DiscountType
	}
	if in.Status == "" {
		in.Status = before.Status
	}
	if err := validateCampaign(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := tx.Exec(
		`UPDATE campaigns SET name=?, description=?, campaign_type=?, discount_type=?,
			discount_value=?, min_group_size=?, status=?, starts_at=?, ends_at=? WHERE id=?`,
		in.Name, nullStrIf(in.Description != "", in.Description),
		in.CampaignType, in.DiscountType, in.DiscountValue,
		nullIntPtr(in.MinGroupSize), in.Status,
		nullTimePtr(in.StartsAt), nullTimePtr(in.EndsAt), id,
	); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, "campaign", id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteCampaign(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanCampaign(tx.QueryRow(`SELECT `+campaignCols+` FROM campaigns WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM campaigns WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionDelete, "campaign", id, before, nil); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// =====================================================================
// COUPONS
// =====================================================================

type Coupon struct {
	ID            int64      `json:"id"`
	Code          string     `json:"code"`
	CampaignID    *int64     `json:"campaign_id,omitempty"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue float64    `json:"discount_value"`
	MinAmount     *float64   `json:"min_amount,omitempty"`
	UsageLimit    *int       `json:"usage_limit,omitempty"`
	UsedCount     int        `json:"used_count"`
	PerUserLimit  *int       `json:"per_user_limit,omitempty"`
	StartsAt      *time.Time `json:"starts_at,omitempty"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
}

var validCouponStatuses = map[string]bool{"active": true, "disabled": true, "expired": true}

func validateCoupon(c *Coupon) error {
	if c.Code == "" {
		return errors.New("code required")
	}
	if !validDiscountTypes[c.DiscountType] {
		return errors.New("discount_type must be percentage or fixed")
	}
	if c.DiscountValue < 0 {
		return errors.New("discount_value must be non-negative")
	}
	if c.DiscountType == "percentage" && c.DiscountValue > 100 {
		return errors.New("percentage discount_value must be <= 100")
	}
	if c.Status != "" && !validCouponStatuses[c.Status] {
		return errors.New("status must be one of active|disabled|expired")
	}
	if c.UsageLimit != nil && *c.UsageLimit < 0 {
		return errors.New("usage_limit must be non-negative")
	}
	if c.PerUserLimit != nil && *c.PerUserLimit < 0 {
		return errors.New("per_user_limit must be non-negative")
	}
	if c.StartsAt != nil && c.EndsAt != nil && c.StartsAt.After(*c.EndsAt) {
		return errors.New("starts_at must be <= ends_at")
	}
	return nil
}

func scanCoupon(row interface{ Scan(...any) error }) (*Coupon, error) {
	var cp Coupon
	var camp sql.NullInt64
	var minAmt sql.NullFloat64
	var usage, perUser sql.NullInt32
	var starts, ends sql.NullTime
	if err := row.Scan(&cp.ID, &cp.Code, &camp, &cp.DiscountType, &cp.DiscountValue,
		&minAmt, &usage, &cp.UsedCount, &perUser, &starts, &ends, &cp.Status, &cp.CreatedAt); err != nil {
		return nil, err
	}
	if camp.Valid {
		v := camp.Int64
		cp.CampaignID = &v
	}
	if minAmt.Valid {
		v := minAmt.Float64
		cp.MinAmount = &v
	}
	if usage.Valid {
		v := int(usage.Int32)
		cp.UsageLimit = &v
	}
	if perUser.Valid {
		v := int(perUser.Int32)
		cp.PerUserLimit = &v
	}
	if starts.Valid {
		t := starts.Time
		cp.StartsAt = &t
	}
	if ends.Valid {
		t := ends.Time
		cp.EndsAt = &t
	}
	return &cp, nil
}

const couponCols = `id, code, campaign_id, discount_type, discount_value, min_amount,
		 usage_limit, used_count, per_user_limit, starts_at, ends_at, status, created_at`

func listCoupons(c *gin.Context) {
	p := readPaging(c)
	args := []any{}
	q := `SELECT ` + couponCols + ` FROM coupons`
	where := []string{}
	if v := c.Query("status"); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)
	rows, err := db.DB.Query(q, args...)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []Coupon{}
	for rows.Next() {
		cp, err := scanCoupon(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *cp)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getCoupon(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cp, err := scanCoupon(db.DB.QueryRow(`SELECT `+couponCols+` FROM coupons WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, cp)
}

func createCoupon(c *gin.Context) {
	var in Coupon
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.DiscountType == "" {
		in.DiscountType = "percentage"
	}
	if err := validateCoupon(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	// Campaign FK validation
	if in.CampaignID != nil {
		var exists int
		if err := db.DB.QueryRow(`SELECT 1 FROM campaigns WHERE id = ?`, *in.CampaignID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				fail(c, http.StatusBadRequest, "campaign_id does not exist")
				return
			}
			dbFail(c, err)
			return
		}
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		`INSERT INTO coupons (code, campaign_id, discount_type, discount_value, min_amount,
			usage_limit, per_user_limit, starts_at, ends_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Code, nullInt64Ptr(in.CampaignID), in.DiscountType, in.DiscountValue,
		nullFloat64Ptr(in.MinAmount), nullIntPtr(in.UsageLimit), nullIntPtr(in.PerUserLimit),
		nullTimePtr(in.StartsAt), nullTimePtr(in.EndsAt), in.Status,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			fail(c, http.StatusConflict, "coupon code already exists")
			return
		}
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	in.ID = id
	in.CreatedAt = time.Now()
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, "coupon", id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateCoupon(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in Coupon
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	in.ID = id
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanCoupon(tx.QueryRow(`SELECT `+couponCols+` FROM coupons WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if in.Code == "" {
		in.Code = before.Code
	}
	if in.DiscountType == "" {
		in.DiscountType = before.DiscountType
	}
	if in.Status == "" {
		in.Status = before.Status
	}
	if err := validateCoupon(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := tx.Exec(
		`UPDATE coupons SET code=?, campaign_id=?, discount_type=?, discount_value=?,
			min_amount=?, usage_limit=?, per_user_limit=?, starts_at=?, ends_at=?, status=? WHERE id=?`,
		in.Code, nullInt64Ptr(in.CampaignID), in.DiscountType, in.DiscountValue,
		nullFloat64Ptr(in.MinAmount), nullIntPtr(in.UsageLimit), nullIntPtr(in.PerUserLimit),
		nullTimePtr(in.StartsAt), nullTimePtr(in.EndsAt), in.Status, id,
	); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, "coupon", id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteCoupon(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanCoupon(tx.QueryRow(`SELECT `+couponCols+` FROM coupons WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM coupons WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionDelete, "coupon", id, before, nil); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// =====================================================================
// PRICING RULES
// =====================================================================

type PricingRule struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	RuleType    string     `json:"rule_type"`
	TargetScope string     `json:"target_scope"`
	TargetID    *int64     `json:"target_id,omitempty"`
	Value       float64    `json:"value"`
	MinAmount   *float64   `json:"min_amount,omitempty"`
	MaxDiscount *float64   `json:"max_discount,omitempty"`
	Priority    int        `json:"priority"`
	CampaignID  *int64     `json:"campaign_id,omitempty"`
	Active      bool       `json:"active"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

var validRuleTypes = map[string]bool{"percentage": true, "fixed": true, "tiered": true, "bundle": true}
var validRuleScopes = map[string]bool{"all": true, "poem": true, "author": true, "dynasty": true, "genre": true, "tier": true}

func validatePricingRule(r *PricingRule) error {
	if r.Name == "" {
		return errors.New("name required")
	}
	if !validRuleTypes[r.RuleType] {
		return errors.New("rule_type must be one of percentage|fixed|tiered|bundle")
	}
	if !validRuleScopes[r.TargetScope] {
		return errors.New("target_scope must be one of all|poem|author|dynasty|genre|tier")
	}
	if r.Value < 0 {
		return errors.New("value must be non-negative")
	}
	if r.RuleType == "percentage" && r.Value > 100 {
		return errors.New("percentage rule value must be <= 100")
	}
	if r.StartsAt != nil && r.EndsAt != nil && r.StartsAt.After(*r.EndsAt) {
		return errors.New("starts_at must be <= ends_at")
	}
	return nil
}

func scanPricingRule(row interface{ Scan(...any) error }) (*PricingRule, error) {
	var pr PricingRule
	var target sql.NullInt64
	var minAmt, maxDisc sql.NullFloat64
	var camp sql.NullInt64
	var starts, ends sql.NullTime
	var active uint8
	if err := row.Scan(&pr.ID, &pr.Name, &pr.RuleType, &pr.TargetScope, &target, &pr.Value,
		&minAmt, &maxDisc, &pr.Priority, &camp, &active, &starts, &ends, &pr.CreatedAt); err != nil {
		return nil, err
	}
	if target.Valid {
		v := target.Int64
		pr.TargetID = &v
	}
	if minAmt.Valid {
		v := minAmt.Float64
		pr.MinAmount = &v
	}
	if maxDisc.Valid {
		v := maxDisc.Float64
		pr.MaxDiscount = &v
	}
	if camp.Valid {
		v := camp.Int64
		pr.CampaignID = &v
	}
	pr.Active = active == 1
	if starts.Valid {
		t := starts.Time
		pr.StartsAt = &t
	}
	if ends.Valid {
		t := ends.Time
		pr.EndsAt = &t
	}
	return &pr, nil
}

const ruleCols = `id, name, rule_type, target_scope, target_id, value,
		 min_amount, max_discount, priority, campaign_id, active, starts_at, ends_at, created_at`

func listPricingRules(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(`SELECT `+ruleCols+` FROM pricing_rules ORDER BY priority DESC, id DESC LIMIT ? OFFSET ?`,
		p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []PricingRule{}
	for rows.Next() {
		r, err := scanPricingRule(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *r)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getPricingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := scanPricingRule(db.DB.QueryRow(`SELECT `+ruleCols+` FROM pricing_rules WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

func createPricingRule(c *gin.Context) {
	var in PricingRule
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if in.RuleType == "" {
		in.RuleType = "percentage"
	}
	if in.TargetScope == "" {
		in.TargetScope = "all"
	}
	if err := validatePricingRule(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		`INSERT INTO pricing_rules (name, rule_type, target_scope, target_id, value,
			min_amount, max_discount, priority, campaign_id, active, starts_at, ends_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.RuleType, in.TargetScope, nullInt64Ptr(in.TargetID), in.Value,
		nullFloat64Ptr(in.MinAmount), nullFloat64Ptr(in.MaxDiscount), in.Priority,
		nullInt64Ptr(in.CampaignID), boolInt(in.Active),
		nullTimePtr(in.StartsAt), nullTimePtr(in.EndsAt),
	)
	if err != nil {
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	in.ID = id
	in.CreatedAt = time.Now()
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, "pricing_rule", id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updatePricingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in PricingRule
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	in.ID = id
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanPricingRule(tx.QueryRow(`SELECT `+ruleCols+` FROM pricing_rules WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if in.Name == "" {
		in.Name = before.Name
	}
	if in.RuleType == "" {
		in.RuleType = before.RuleType
	}
	if in.TargetScope == "" {
		in.TargetScope = before.TargetScope
	}
	if err := validatePricingRule(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := tx.Exec(
		`UPDATE pricing_rules SET name=?, rule_type=?, target_scope=?, target_id=?, value=?,
			min_amount=?, max_discount=?, priority=?, campaign_id=?, active=?, starts_at=?, ends_at=? WHERE id=?`,
		in.Name, in.RuleType, in.TargetScope, nullInt64Ptr(in.TargetID), in.Value,
		nullFloat64Ptr(in.MinAmount), nullFloat64Ptr(in.MaxDiscount), in.Priority,
		nullInt64Ptr(in.CampaignID), boolInt(in.Active),
		nullTimePtr(in.StartsAt), nullTimePtr(in.EndsAt), id,
	); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, "pricing_rule", id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deletePricingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanPricingRule(tx.QueryRow(`SELECT `+ruleCols+` FROM pricing_rules WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM pricing_rules WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionDelete, "pricing_rule", id, before, nil); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// =====================================================================
// MEMBER TIERS
// =====================================================================

type MemberTier struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Level        int       `json:"level"`
	MonthlyPrice float64   `json:"monthly_price"`
	YearlyPrice  float64   `json:"yearly_price"`
	Benefits     any       `json:"benefits,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func validateMemberTier(t *MemberTier) error {
	if t.Name == "" {
		return errors.New("name required")
	}
	if t.Level < 0 {
		return errors.New("level must be non-negative")
	}
	if t.MonthlyPrice < 0 || t.YearlyPrice < 0 {
		return errors.New("prices must be non-negative")
	}
	return nil
}

func scanMemberTier(row interface{ Scan(...any) error }) (*MemberTier, error) {
	var t MemberTier
	var benefits sql.NullString
	if err := row.Scan(&t.ID, &t.Name, &t.Level, &t.MonthlyPrice, &t.YearlyPrice, &benefits, &t.CreatedAt); err != nil {
		return nil, err
	}
	if benefits.Valid && benefits.String != "" {
		var parsed any
		if err := json.Unmarshal([]byte(benefits.String), &parsed); err == nil {
			t.Benefits = parsed
		} else {
			t.Benefits = benefits.String
		}
	}
	return &t, nil
}

const tierCols = `id, name, level, monthly_price, yearly_price, benefits, created_at`

func listMemberTiers(c *gin.Context) {
	p := readPaging(c)
	rows, err := db.DB.Query(`SELECT `+tierCols+` FROM member_tiers ORDER BY level ASC, id ASC LIMIT ? OFFSET ?`,
		p.Limit, p.Offset)
	if err != nil {
		dbFail(c, err)
		return
	}
	defer rows.Close()
	out := []MemberTier{}
	for rows.Next() {
		t, err := scanMemberTier(rows)
		if err != nil {
			dbFail(c, err)
			return
		}
		out = append(out, *t)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "limit": p.Limit, "offset": p.Offset})
}

func getMemberTier(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := scanMemberTier(db.DB.QueryRow(`SELECT `+tierCols+` FROM member_tiers WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func createMemberTier(c *gin.Context) {
	var in MemberTier
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateMemberTier(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var benefitsJSON any = nil
	if in.Benefits != nil {
		b, err := json.Marshal(in.Benefits)
		if err != nil {
			fail(c, http.StatusBadRequest, "invalid benefits json")
			return
		}
		benefitsJSON = string(b)
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		`INSERT INTO member_tiers (name, level, monthly_price, yearly_price, benefits)
		 VALUES (?, ?, ?, ?, ?)`,
		in.Name, in.Level, in.MonthlyPrice, in.YearlyPrice, benefitsJSON,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			fail(c, http.StatusConflict, "tier with this name or level already exists")
			return
		}
		dbFail(c, err)
		return
	}
	id, _ := res.LastInsertId()
	in.ID = id
	in.CreatedAt = time.Now()
	if err := audit.WriteCtx(c, tx, audit.ActionCreate, "member_tier", id, nil, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, in)
}

func updateMemberTier(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in MemberTier
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid json")
		return
	}
	in.ID = id
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanMemberTier(tx.QueryRow(`SELECT `+tierCols+` FROM member_tiers WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if in.Name == "" {
		in.Name = before.Name
	}
	if err := validateMemberTier(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var benefitsJSON any = nil
	if in.Benefits != nil {
		b, err := json.Marshal(in.Benefits)
		if err != nil {
			fail(c, http.StatusBadRequest, "invalid benefits json")
			return
		}
		benefitsJSON = string(b)
	}
	if _, err := tx.Exec(
		`UPDATE member_tiers SET name=?, level=?, monthly_price=?, yearly_price=?, benefits=? WHERE id=?`,
		in.Name, in.Level, in.MonthlyPrice, in.YearlyPrice, benefitsJSON, id,
	); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionUpdate, "member_tier", id, before, in); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func deleteMemberTier(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	tx, err := db.DB.Begin()
	if err != nil {
		dbFail(c, err)
		return
	}
	defer tx.Rollback()
	before, err := scanMemberTier(tx.QueryRow(`SELECT `+tierCols+` FROM member_tiers WHERE id = ? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM member_tiers WHERE id = ?`, id); err != nil {
		dbFail(c, err)
		return
	}
	if err := audit.WriteCtx(c, tx, audit.ActionDelete, "member_tier", id, before, nil); err != nil {
		dbFail(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// =====================================================================
// Shared nullable helpers
// =====================================================================

func nullStrIf(cond bool, s string) any {
	if !cond {
		return nil
	}
	return s
}

func nullInt64Ptr(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullFloat64Ptr(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullTimePtr(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return *t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
