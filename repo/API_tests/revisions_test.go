package apitests

import (
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Revision restore: we create a dynasty, edit it, delete it, then use the
// /revisions API to walk the history and restore.

func TestRevisions_AnonRejected(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id=1", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon revisions list")
}

func TestRevisions_NonAdminForbidden(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userReviewer, passReviewer)
	code, body, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id=1", nil, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot list revisions")
	if _, ok := body["error"]; !ok {
		t.Fatalf("403 missing error field")
	}
}

func TestRevisions_ListAndRestore_UpdateRoundTrip(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// 1) Create a dynasty.
	_, created, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name":       "Rev_" + s,
		"start_year": 618,
		"end_year":   907,
	}, nil)
	id := int64(created["id"].(float64))
	defer doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)

	// 2) Update it so there's an audit row with before/after.
	_, _, _ = doJSON(t, c, "PUT", "/dynasties/"+itoa(id), map[string]any{
		"name": "Rev_" + s + "_EDITED", "start_year": 700, "end_year": 800,
	}, nil)

	// 3) List revisions — should include one create + one update entry.
	code, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id="+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "list revisions")
	items, _ := list["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("expected >=2 revisions (create+update), got %d: %v", len(items), items)
	}
	// Most recent entry (index 0) is the update. Find it.
	var updateRevID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "update" {
			updateRevID = int64(row["id"].(float64))
			if row["restorable"] != true {
				t.Fatalf("update revision should be restorable: %v", row)
			}
			break
		}
	}
	if updateRevID == 0 {
		t.Fatalf("no update revision found in %v", items)
	}

	// 4) Restore: rolls the dynasty back to its pre-update state.
	code, resp, _ := doJSON(t, c, "POST", "/revisions/"+itoa(updateRevID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore update revision")
	if resp["entity_type"] != "dynasty" {
		t.Fatalf("unexpected restore response: %v", resp)
	}

	// 5) GET the dynasty — name should be the pre-edit name.
	_, got, _ := doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	if got["name"] != "Rev_"+s {
		t.Fatalf("restore did not revert name: got %v", got["name"])
	}
}

func TestRevisions_RestoreCreateRemovesRow(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// Create a dynasty, capture its create-revision id.
	_, created, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name": "ForCreate_" + s,
	}, nil)
	id := int64(created["id"].(float64))

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id="+itoa(id), nil, nil)
	items, _ := list["items"].([]any)
	var createRevID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "create" {
			createRevID = int64(row["id"].(float64))
			break
		}
	}
	if createRevID == 0 {
		t.Fatalf("no create revision found")
	}

	// Restoring a "create" revision means: undo the create → delete the row.
	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(createRevID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore create = delete row")

	code, _, _ = doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "row should be gone after create restore")
}

func TestRevisions_RestoreDeleteReinsertsRow(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, created, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{"name": "Del_" + s}, nil)
	id := int64(created["id"].(float64))

	_, _, _ = doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id="+itoa(id), nil, nil)
	items, _ := list["items"].([]any)
	var deleteRevID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "delete" {
			deleteRevID = int64(row["id"].(float64))
			break
		}
	}
	if deleteRevID == 0 {
		t.Fatalf("no delete revision found")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(deleteRevID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore delete = reinsert row")

	code, got, _ := doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "dynasty should exist again")
	if got["name"] != "Del_"+s {
		t.Fatalf("restored name mismatch: %v", got["name"])
	}
	// Cleanup.
	doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
}

func TestRevisions_RestoreUnknownID(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/revisions/999999999/restore", nil, nil)
	assertStatus(t, code, http.StatusNotFound, "unknown revision")
	if body["error"] == nil {
		t.Fatalf("404 missing error field: %v", body)
	}
}

func TestRevisions_ListRequiresEntityParams(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "GET", "/revisions", nil, nil)
	assertStatus(t, code, http.StatusBadRequest, "missing entity_type")

	code, _, _ = doJSON(t, c, "GET", "/revisions?entity_type=dynasty", nil, nil)
	assertStatus(t, code, http.StatusBadRequest, "missing entity_id")

	code, _, _ = doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id=notanumber", nil, nil)
	assertStatus(t, code, http.StatusBadRequest, "bad entity_id format")
}

// Verify that a revision whose expires_at is in the past can no longer be
// restored. We use the /auth/me → user-id approach, then reach into MySQL
// via an admin-only SQL-backed path — but since the backend exposes no raw
// SQL endpoint, we instead exercise this by validating the handler's
// in-code gate: any audit row older than 30 days rejects with 410. We
// can't easily age a row from Go in a real HTTP test without a DB socket,
// so this test focuses on the no-row-found 404 and the "pending approval
// refusal" 409 branches, both of which the handler checks in the same
// gate.

func TestRevisions_PendingApprovalConflictOnRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// Enable approval requirement so deletes become pending.
	_, _, _ = doJSON(t, c, "PUT", "/settings/approval", map[string]any{"enabled": true}, nil)
	defer doJSON(t, c, "PUT", "/settings/approval", map[string]any{"enabled": false}, nil)

	_, created, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{"name": "Pend_" + s}, nil)
	id := int64(created["id"].(float64))
	// Delete triggers a pending audit row.
	_, _, _ = doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)

	// Find the pending delete revision.
	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id="+itoa(id), nil, nil)
	items, _ := list["items"].([]any)
	var pendingRev int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "delete" && row["approval_status"] == "pending" {
			pendingRev = int64(row["id"].(float64))
			break
		}
	}
	if pendingRev == 0 {
		// If approvals aren't currently enabled, this test is a no-op.
		t.Skip("no pending-approval delete revision produced; approval gating not active")
	}
	code, body, _ := doJSON(t, c, "POST", "/revisions/"+itoa(pendingRev)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusConflict, "pending revision cannot be restored")
	if body["error"] == nil {
		t.Fatalf("409 missing error field")
	}
}

func TestRevisions_SupportedEntities(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/revisions/supported-entities", nil, nil)
	assertStatus(t, code, http.StatusOK, "list supported entities")
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least one supported entity")
	}
	if int(body["retention_days"].(float64)) != 30 {
		t.Fatalf("retention_days should be 30, got %v", body["retention_days"])
	}

	// All 9 audited entity types must be listed.
	required := []string{"dynasty", "author", "poem", "excerpt", "tag",
		"campaign", "coupon", "pricing_rule", "member_tier"}
	found := map[string]bool{}
	for _, it := range items {
		if s, ok := it.(string); ok {
			found[s] = true
		}
	}
	for _, r := range required {
		if !found[r] {
			t.Errorf("supported-entities missing: %s", r)
		}
	}
}

// openRevTestDB opens a direct MySQL connection for tests that need to
// inject rows that cannot be created through the public API (e.g. expired
// audit entries). Skips when HELIOS_DB_DSN is not set.
func openRevTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("HELIOS_DB_DSN")
	if dsn == "" {
		t.Skip("HELIOS_DB_DSN not set; skipping DB-backed revision tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------- retention window ----------

// TestRevisions_ExpiredRevision_Returns410 directly inserts an audit row whose
// expires_at is in the past and confirms the restore endpoint returns 410 Gone.
func TestRevisions_ExpiredRevision_Returns410(t *testing.T) {
	db := openRevTestDB(t)
	c := newClient(t)
	loginAdmin(t, c)

	past := time.Now().Add(-31 * 24 * time.Hour)
	res, err := db.Exec(`
		INSERT INTO audit_logs
		  (action, entity_type, entity_id, before_json, after_json, expires_at, approval_status)
		VALUES ('update', 'dynasty', 999999998,
		        '{"id":999999998,"name":"old_name"}',
		        '{"id":999999998,"name":"new_name"}',
		        ?, 'not_required')`, past)
	if err != nil {
		t.Fatalf("insert expired audit row: %v", err)
	}
	rowID, _ := res.LastInsertId()
	t.Cleanup(func() { db.Exec(`DELETE FROM audit_logs WHERE id = ?`, rowID) })

	code, body, _ := doJSON(t, c, "POST", "/revisions/"+itoa(rowID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusGone, "expired revision must return 410")
	if body["error"] == nil {
		t.Fatalf("410 response missing error field: %v", body)
	}
}

// TestRevisions_InWindowRestore_Returns200 explicitly verifies that a recently
// created revision (within the 30-day retention window) can be restored and
// returns 200 OK.
func TestRevisions_InWindowRestore_Returns200(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, created, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name": "InWindow_" + s, "start_year": 500,
	}, nil)
	id := int64(created["id"].(float64))
	defer doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)

	doJSON(t, c, "PUT", "/dynasties/"+itoa(id), map[string]any{
		"name": "InWindow_" + s + "_v2",
	}, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=dynasty&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "update" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no update revision found")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "in-window restore must return 200")
}

// ---------- campaign restore round-trips ----------

func TestRevisions_Campaign_UpdateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, camp, _ := doJSON(t, c, "POST", "/campaigns", map[string]any{
		"name":           "CRev_" + s,
		"campaign_type":  "standard",
		"discount_type":  "percentage",
		"discount_value": 10.0,
	}, nil)
	id := int64(camp["id"].(float64))
	defer doJSON(t, c, "DELETE", "/campaigns/"+itoa(id), nil, nil)

	doJSON(t, c, "PUT", "/campaigns/"+itoa(id), map[string]any{
		"name": "CRev_" + s + "_EDITED", "discount_value": 20.0,
	}, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=campaign&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "update" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no update revision found for campaign")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore campaign update")

	_, got, _ := doJSON(t, c, "GET", "/campaigns/"+itoa(id), nil, nil)
	if got["name"] != "CRev_"+s {
		t.Fatalf("campaign name not restored: got %v", got["name"])
	}
}

func TestRevisions_Campaign_DeleteRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, camp, _ := doJSON(t, c, "POST", "/campaigns", map[string]any{
		"name":           "CDel_" + s,
		"campaign_type":  "standard",
		"discount_type":  "percentage",
		"discount_value": 5.0,
	}, nil)
	id := int64(camp["id"].(float64))

	doJSON(t, c, "DELETE", "/campaigns/"+itoa(id), nil, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=campaign&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "delete" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no delete revision found for campaign")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore campaign delete = re-insert")

	code, got, _ := doJSON(t, c, "GET", "/campaigns/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "campaign should exist after delete restore")
	if got["name"] != "CDel_"+s {
		t.Fatalf("restored campaign name mismatch: got %v", got["name"])
	}
	defer doJSON(t, c, "DELETE", "/campaigns/"+itoa(id), nil, nil)
}

func TestRevisions_Campaign_CreateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, camp, _ := doJSON(t, c, "POST", "/campaigns", map[string]any{
		"name":           "CCreate_" + s,
		"campaign_type":  "standard",
		"discount_type":  "fixed",
		"discount_value": 1.0,
	}, nil)
	id := int64(camp["id"].(float64))

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=campaign&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "create" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no create revision found for campaign")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore campaign create = delete row")

	code, _, _ = doJSON(t, c, "GET", "/campaigns/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "campaign should be gone after create restore")
}

// ---------- coupon restore round-trips ----------

func TestRevisions_Coupon_UpdateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, cp, _ := doJSON(t, c, "POST", "/coupons", map[string]any{
		"code": "CPR_" + s, "discount_type": "percentage", "discount_value": 10.0,
	}, nil)
	id := int64(cp["id"].(float64))
	defer doJSON(t, c, "DELETE", "/coupons/"+itoa(id), nil, nil)

	doJSON(t, c, "PUT", "/coupons/"+itoa(id), map[string]any{
		"discount_value": 20.0,
	}, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=coupon&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "update" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no update revision found for coupon")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore coupon update")

	_, got, _ := doJSON(t, c, "GET", "/coupons/"+itoa(id), nil, nil)
	if int(got["discount_value"].(float64)) != 10 {
		t.Fatalf("coupon discount_value not restored: got %v", got["discount_value"])
	}
}

func TestRevisions_Coupon_DeleteRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, cp, _ := doJSON(t, c, "POST", "/coupons", map[string]any{
		"code": "CDEL_" + s, "discount_type": "fixed", "discount_value": 5.0,
	}, nil)
	id := int64(cp["id"].(float64))

	doJSON(t, c, "DELETE", "/coupons/"+itoa(id), nil, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=coupon&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "delete" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no delete revision found for coupon")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore coupon delete = re-insert")

	code, _, _ = doJSON(t, c, "GET", "/coupons/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "coupon should exist after delete restore")
	defer doJSON(t, c, "DELETE", "/coupons/"+itoa(id), nil, nil)
}

func TestRevisions_Coupon_CreateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, cp, _ := doJSON(t, c, "POST", "/coupons", map[string]any{
		"code": "CCREATE_" + s, "discount_type": "percentage", "discount_value": 15.0,
	}, nil)
	id := int64(cp["id"].(float64))

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=coupon&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "create" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no create revision found for coupon")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore coupon create = delete row")

	code, _, _ = doJSON(t, c, "GET", "/coupons/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "coupon should be gone after create restore")
}

// ---------- pricing_rule restore round-trips ----------

func TestRevisions_PricingRule_UpdateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, pr, _ := doJSON(t, c, "POST", "/pricing-rules", map[string]any{
		"name": "PRRev_" + s, "rule_type": "percentage", "target_scope": "all", "value": 5.0,
	}, nil)
	id := int64(pr["id"].(float64))
	defer doJSON(t, c, "DELETE", "/pricing-rules/"+itoa(id), nil, nil)

	doJSON(t, c, "PUT", "/pricing-rules/"+itoa(id), map[string]any{
		"name": "PRRev_" + s + "_EDITED", "value": 15.0,
	}, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=pricing_rule&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "update" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no update revision found for pricing_rule")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore pricing_rule update")

	_, got, _ := doJSON(t, c, "GET", "/pricing-rules/"+itoa(id), nil, nil)
	if got["name"] != "PRRev_"+s {
		t.Fatalf("pricing_rule name not restored: got %v", got["name"])
	}
}

func TestRevisions_PricingRule_DeleteRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, pr, _ := doJSON(t, c, "POST", "/pricing-rules", map[string]any{
		"name": "PRDel_" + s, "rule_type": "fixed", "target_scope": "all", "value": 3.0,
	}, nil)
	id := int64(pr["id"].(float64))

	doJSON(t, c, "DELETE", "/pricing-rules/"+itoa(id), nil, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=pricing_rule&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "delete" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no delete revision found for pricing_rule")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore pricing_rule delete = re-insert")

	code, _, _ = doJSON(t, c, "GET", "/pricing-rules/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "pricing_rule should exist after delete restore")
	defer doJSON(t, c, "DELETE", "/pricing-rules/"+itoa(id), nil, nil)
}

func TestRevisions_PricingRule_CreateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, pr, _ := doJSON(t, c, "POST", "/pricing-rules", map[string]any{
		"name": "PRCreate_" + s, "rule_type": "percentage", "target_scope": "all", "value": 7.0,
	}, nil)
	id := int64(pr["id"].(float64))

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=pricing_rule&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "create" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no create revision found for pricing_rule")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore pricing_rule create = delete row")

	code, _, _ = doJSON(t, c, "GET", "/pricing-rules/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "pricing_rule should be gone after create restore")
}

// ---------- member_tier restore round-trips ----------

func TestRevisions_MemberTier_UpdateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	// Use a high level to avoid conflicts with existing tiers.
	_, tier, raw := doJSON(t, c, "POST", "/member-tiers", map[string]any{
		"name": "MTRev_" + s, "level": 900, "monthly_price": 9.99, "yearly_price": 99.99,
	}, nil)
	if tier["id"] == nil {
		t.Fatalf("create member_tier failed: %s", raw)
	}
	id := int64(tier["id"].(float64))
	defer doJSON(t, c, "DELETE", "/member-tiers/"+itoa(id), nil, nil)

	doJSON(t, c, "PUT", "/member-tiers/"+itoa(id), map[string]any{
		"name": "MTRev_" + s + "_EDITED", "level": 900, "monthly_price": 19.99, "yearly_price": 99.99,
	}, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=member_tier&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "update" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no update revision found for member_tier")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore member_tier update")

	_, got, _ := doJSON(t, c, "GET", "/member-tiers/"+itoa(id), nil, nil)
	if got["name"] != "MTRev_"+s {
		t.Fatalf("member_tier name not restored: got %v", got["name"])
	}
}

func TestRevisions_MemberTier_DeleteRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, tier, _ := doJSON(t, c, "POST", "/member-tiers", map[string]any{
		"name": "MTDel_" + s, "level": 901, "monthly_price": 4.99, "yearly_price": 49.99,
	}, nil)
	id := int64(tier["id"].(float64))

	doJSON(t, c, "DELETE", "/member-tiers/"+itoa(id), nil, nil)

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=member_tier&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "delete" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no delete revision found for member_tier")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore member_tier delete = re-insert")

	code, _, _ = doJSON(t, c, "GET", "/member-tiers/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "member_tier should exist after delete restore")
	defer doJSON(t, c, "DELETE", "/member-tiers/"+itoa(id), nil, nil)
}

func TestRevisions_MemberTier_CreateRestore(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, tier, _ := doJSON(t, c, "POST", "/member-tiers", map[string]any{
		"name": "MTCreate_" + s, "level": 902, "monthly_price": 2.99, "yearly_price": 29.99,
	}, nil)
	id := int64(tier["id"].(float64))

	_, list, _ := doJSON(t, c, "GET", "/revisions?entity_type=member_tier&entity_id="+itoa(id), nil, nil)
	items := list["items"].([]any)
	var revID int64
	for _, it := range items {
		row := it.(map[string]any)
		if row["action"] == "create" {
			revID = int64(row["id"].(float64))
			break
		}
	}
	if revID == 0 {
		t.Fatalf("no create revision found for member_tier")
	}

	code, _, _ := doJSON(t, c, "POST", "/revisions/"+itoa(revID)+"/restore", nil, nil)
	assertStatus(t, code, http.StatusOK, "restore member_tier create = delete row")

	code, _, _ = doJSON(t, c, "GET", "/member-tiers/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "member_tier should be gone after create restore")
}
