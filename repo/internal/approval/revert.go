package approval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type reverter func(tx *sql.Tx, action string, before, after json.RawMessage) error

var reverters = map[string]reverter{
	"dynasty":      revertDynasty,
	"author":       revertAuthor,
	"poem":         revertPoem,
	"excerpt":      revertExcerpt,
	"tag":          revertTag,
	"campaign":     revertCampaign,
	"coupon":       revertCoupon,
	"pricing_rule": revertPricingRule,
	"member_tier":  revertMemberTier,
}

func revertOne(tx *sql.Tx, entityType, action string, before, after json.RawMessage) error {
	r, ok := reverters[entityType]
	if !ok {
		return fmt.Errorf("no reverter registered for entity type: %s", entityType)
	}
	return r(tx, action, before, after)
}

// RestoreRevision is the public entry point used by the /revisions restore
// endpoint. It re-applies the `before` state for an update (or re-inserts a
// deleted row, or deletes a created one) — mirroring the approval revert
// logic but usable outside the batch-approval flow.
func RestoreRevision(tx *sql.Tx, entityType, action string, before, after json.RawMessage) error {
	return revertOne(tx, entityType, action, before, after)
}

// SupportedEntityTypes lists entity_type values that the revision restore
// workflow can handle. Keep in sync with the reverters map above.
func SupportedEntityTypes() []string {
	out := make([]string, 0, len(reverters))
	for k := range reverters {
		out = append(out, k)
	}
	return out
}

// IsSupportedEntity reports whether a restore can be executed for the given
// audit entity_type. Callers should 400 when this is false.
func IsSupportedEntity(entityType string) bool {
	_, ok := reverters[entityType]
	return ok
}

// ---------- helpers ----------

func nStrBlank(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func extractID(raw json.RawMessage) (int64, error) {
	var s struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	return s.ID, nil
}

func nInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// ---------- dynasty ----------

type dynastyShape struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	StartYear   *int    `json:"start_year"`
	EndYear     *int    `json:"end_year"`
	Description *string `json:"description"`
}

func revertDynasty(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM dynasties WHERE id = ?`, id)
		return err
	case "update":
		var b dynastyShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE dynasties SET name=?, start_year=?, end_year=?, description=? WHERE id=?`,
			b.Name, nInt(b.StartYear), nInt(b.EndYear), nStr(b.Description), b.ID)
		return err
	case "delete":
		var b dynastyShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO dynasties (id, name, start_year, end_year, description) VALUES (?, ?, ?, ?, ?)`,
			b.ID, b.Name, nInt(b.StartYear), nInt(b.EndYear), nStr(b.Description))
		return err
	}
	return nil
}

// ---------- author ----------

type authorShape struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	AltNames  *string `json:"alt_names"`
	DynastyID *int64  `json:"dynasty_id"`
	BirthYear *int    `json:"birth_year"`
	DeathYear *int    `json:"death_year"`
	Biography *string `json:"biography"`
}

func revertAuthor(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM authors WHERE id = ?`, id)
		return err
	case "update":
		var b authorShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE authors SET name=?, alt_names=?, dynasty_id=?, birth_year=?, death_year=?, biography=? WHERE id=?`,
			b.Name, nStr(b.AltNames), nInt64(b.DynastyID), nInt(b.BirthYear), nInt(b.DeathYear), nStr(b.Biography), b.ID)
		return err
	case "delete":
		var b authorShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO authors (id, name, alt_names, dynasty_id, birth_year, death_year, biography) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.Name, nStr(b.AltNames), nInt64(b.DynastyID), nInt(b.BirthYear), nInt(b.DeathYear), nStr(b.Biography))
		return err
	}
	return nil
}

// ---------- poem ----------

type poemShape struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	AuthorID       *int64  `json:"author_id"`
	DynastyID      *int64  `json:"dynasty_id"`
	MeterPatternID *int64  `json:"meter_pattern_id"`
	Body           string  `json:"body"`
	Preface        *string `json:"preface"`
	Translation    *string `json:"translation"`
	Source         *string `json:"source"`
	Status         string  `json:"status"`
	Version        int     `json:"version"`
}

func revertPoem(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM poems WHERE id = ?`, id)
		return err
	case "update":
		var b poemShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE poems SET title=?, author_id=?, dynasty_id=?, meter_pattern_id=?, body=?, preface=?, translation=?, source=?, status=?, version=? WHERE id=?`,
			b.Title, nInt64(b.AuthorID), nInt64(b.DynastyID), nInt64(b.MeterPatternID),
			b.Body, nStr(b.Preface), nStr(b.Translation), nStr(b.Source), b.Status, b.Version, b.ID)
		return err
	case "delete":
		var b poemShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO poems (id, title, author_id, dynasty_id, meter_pattern_id, body, preface, translation, source, status, version)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.Title, nInt64(b.AuthorID), nInt64(b.DynastyID), nInt64(b.MeterPatternID),
			b.Body, nStr(b.Preface), nStr(b.Translation), nStr(b.Source), b.Status, b.Version)
		return err
	}
	return nil
}

// ---------- excerpt ----------

type excerptShape struct {
	ID             int64   `json:"id"`
	PoemID         int64   `json:"poem_id"`
	StartOffset    uint32  `json:"start_offset"`
	EndOffset      uint32  `json:"end_offset"`
	ExcerptText    string  `json:"excerpt_text"`
	Annotation     *string `json:"annotation"`
	AnnotationType string  `json:"annotation_type"`
	AuthorID       *int64  `json:"author_id"`
}

func revertExcerpt(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM excerpts WHERE id = ?`, id)
		return err
	case "update":
		var b excerptShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE excerpts SET poem_id=?, start_offset=?, end_offset=?, excerpt_text=?, annotation=?, annotation_type=?, author_id=? WHERE id=?`,
			b.PoemID, b.StartOffset, b.EndOffset, b.ExcerptText, nStr(b.Annotation), b.AnnotationType, nInt64(b.AuthorID), b.ID)
		return err
	case "delete":
		var b excerptShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO excerpts (id, poem_id, start_offset, end_offset, excerpt_text, annotation, annotation_type, author_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.PoemID, b.StartOffset, b.EndOffset, b.ExcerptText, nStr(b.Annotation), b.AnnotationType, nInt64(b.AuthorID))
		return err
	}
	return nil
}

// ---------- tag (genres.kind = 'tag') ----------

type tagShape struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	ParentID    *int64  `json:"parent_id"`
	Description *string `json:"description"`
}

func revertTag(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM genres WHERE id = ? AND kind = 'tag'`, id)
		return err
	case "update":
		var b tagShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE genres SET name=?, parent_id=?, description=? WHERE id=? AND kind='tag'`,
			b.Name, nInt64(b.ParentID), nStr(b.Description), b.ID)
		return err
	case "delete":
		var b tagShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO genres (id, name, kind, parent_id, description) VALUES (?, ?, 'tag', ?, ?)`,
			b.ID, b.Name, nInt64(b.ParentID), nStr(b.Description))
		return err
	}
	return nil
}

// ---------- campaign ----------

type campaignShape struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	CampaignType  string     `json:"campaign_type"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue float64    `json:"discount_value"`
	MinGroupSize  *int       `json:"min_group_size"`
	Status        string     `json:"status"`
	StartsAt      *time.Time `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
	CreatedBy     *int64     `json:"created_by"`
}

func revertCampaign(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM campaigns WHERE id = ?`, id)
		return err
	case "update":
		var b campaignShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE campaigns SET name=?, description=?, campaign_type=?, discount_type=?,
			 discount_value=?, min_group_size=?, status=?, starts_at=?, ends_at=? WHERE id=?`,
			b.Name, nStrBlank(b.Description), b.CampaignType, b.DiscountType, b.DiscountValue,
			nInt(b.MinGroupSize), b.Status, nTime(b.StartsAt), nTime(b.EndsAt), b.ID)
		return err
	case "delete":
		var b campaignShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO campaigns (id, name, description, campaign_type, discount_type,
			 discount_value, min_group_size, status, starts_at, ends_at, created_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.Name, nStrBlank(b.Description), b.CampaignType, b.DiscountType, b.DiscountValue,
			nInt(b.MinGroupSize), b.Status, nTime(b.StartsAt), nTime(b.EndsAt), nInt64(b.CreatedBy))
		return err
	}
	return nil
}

// ---------- coupon ----------

type couponShape struct {
	ID            int64      `json:"id"`
	Code          string     `json:"code"`
	CampaignID    *int64     `json:"campaign_id"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue float64    `json:"discount_value"`
	MinAmount     *float64   `json:"min_amount"`
	UsageLimit    *int       `json:"usage_limit"`
	UsedCount     int        `json:"used_count"`
	PerUserLimit  *int       `json:"per_user_limit"`
	StartsAt      *time.Time `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
	Status        string     `json:"status"`
}

func revertCoupon(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM coupons WHERE id = ?`, id)
		return err
	case "update":
		var b couponShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE coupons SET code=?, campaign_id=?, discount_type=?, discount_value=?,
			 min_amount=?, usage_limit=?, per_user_limit=?, starts_at=?, ends_at=?, status=? WHERE id=?`,
			b.Code, nInt64(b.CampaignID), b.DiscountType, b.DiscountValue,
			nFloat(b.MinAmount), nInt(b.UsageLimit), nInt(b.PerUserLimit),
			nTime(b.StartsAt), nTime(b.EndsAt), b.Status, b.ID)
		return err
	case "delete":
		var b couponShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO coupons (id, code, campaign_id, discount_type, discount_value,
			 min_amount, usage_limit, used_count, per_user_limit, starts_at, ends_at, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.Code, nInt64(b.CampaignID), b.DiscountType, b.DiscountValue,
			nFloat(b.MinAmount), nInt(b.UsageLimit), b.UsedCount, nInt(b.PerUserLimit),
			nTime(b.StartsAt), nTime(b.EndsAt), b.Status)
		return err
	}
	return nil
}

// ---------- pricing_rule ----------

type pricingRuleShape struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	RuleType    string     `json:"rule_type"`
	TargetScope string     `json:"target_scope"`
	TargetID    *int64     `json:"target_id"`
	Value       float64    `json:"value"`
	MinAmount   *float64   `json:"min_amount"`
	MaxDiscount *float64   `json:"max_discount"`
	Priority    int        `json:"priority"`
	CampaignID  *int64     `json:"campaign_id"`
	Active      bool       `json:"active"`
	StartsAt    *time.Time `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
}

func revertPricingRule(tx *sql.Tx, action string, before, after json.RawMessage) error {
	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM pricing_rules WHERE id = ?`, id)
		return err
	case "update":
		var b pricingRuleShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE pricing_rules SET name=?, rule_type=?, target_scope=?, target_id=?, value=?,
			 min_amount=?, max_discount=?, priority=?, campaign_id=?, active=?, starts_at=?, ends_at=? WHERE id=?`,
			b.Name, b.RuleType, b.TargetScope, nInt64(b.TargetID), b.Value,
			nFloat(b.MinAmount), nFloat(b.MaxDiscount), b.Priority,
			nInt64(b.CampaignID), boolToInt(b.Active), nTime(b.StartsAt), nTime(b.EndsAt), b.ID)
		return err
	case "delete":
		var b pricingRuleShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO pricing_rules (id, name, rule_type, target_scope, target_id, value,
			 min_amount, max_discount, priority, campaign_id, active, starts_at, ends_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			b.ID, b.Name, b.RuleType, b.TargetScope, nInt64(b.TargetID), b.Value,
			nFloat(b.MinAmount), nFloat(b.MaxDiscount), b.Priority,
			nInt64(b.CampaignID), boolToInt(b.Active), nTime(b.StartsAt), nTime(b.EndsAt))
		return err
	}
	return nil
}

// ---------- member_tier ----------

type memberTierShape struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Level        int     `json:"level"`
	MonthlyPrice float64 `json:"monthly_price"`
	YearlyPrice  float64 `json:"yearly_price"`
	Benefits     any     `json:"benefits"`
}

func revertMemberTier(tx *sql.Tx, action string, before, after json.RawMessage) error {
	marshalBenefits := func(v any) (any, error) {
		if v == nil {
			return nil, nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}

	switch action {
	case "create":
		id, err := extractID(after)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM member_tiers WHERE id = ?`, id)
		return err
	case "update":
		var b memberTierShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		benefitsJSON, err := marshalBenefits(b.Benefits)
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			`UPDATE member_tiers SET name=?, level=?, monthly_price=?, yearly_price=?, benefits=? WHERE id=?`,
			b.Name, b.Level, b.MonthlyPrice, b.YearlyPrice, benefitsJSON, b.ID)
		return err
	case "delete":
		var b memberTierShape
		if err := json.Unmarshal(before, &b); err != nil {
			return err
		}
		benefitsJSON, err := marshalBenefits(b.Benefits)
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			`INSERT INTO member_tiers (id, name, level, monthly_price, yearly_price, benefits)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			b.ID, b.Name, b.Level, b.MonthlyPrice, b.YearlyPrice, benefitsJSON)
		return err
	}
	return nil
}
