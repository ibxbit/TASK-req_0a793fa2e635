package apitests

import (
	"net/http"
	"testing"
)

// rolematrix_test.go broadens the RBAC coverage beyond admin+anonymous.
// For each role we verify a representative allowed and denied action.

type roleCase struct {
	name, user, pass string
}

var allRoles = []roleCase{
	{"admin", "admin", "admin123"},
	{"content_editor", userEditor, passEditor},
	{"reviewer", userReviewer, passReviewer},
	{"marketing_manager", userMkt, passMkt},
	{"crawler_operator", userCrawler, passCrawler},
	{"member", userMember, passMember},
}

func TestRoleMatrix_AuditLogsAdminOnly(t *testing.T) {
	for _, rc := range allRoles {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			c := newClient(t)
			loginAs(t, c, rc.user, rc.pass)
			code, _, _ := doJSON(t, c, "GET", "/audit-logs", nil, nil)
			if rc.user == "admin" {
				assertStatus(t, code, http.StatusOK, "admin GET audit-logs")
			} else {
				assertStatus(t, code, http.StatusForbidden, rc.name+" GET audit-logs")
			}
		})
	}
}

func TestRoleMatrix_CrawlJobsCreate(t *testing.T) {
	for _, rc := range allRoles {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			c := newClient(t)
			loginAs(t, c, rc.user, rc.pass)
			code, _, _ := doJSON(t, c, "POST", "/crawl/jobs", map[string]any{
				"job_name": "RM_" + rc.name + "_" + uniqSuffix(), "source_url": "https://example.com",
			}, nil)
			allowed := rc.user == "admin" || rc.user == userCrawler
			if allowed {
				if code != http.StatusCreated {
					t.Fatalf("%s create job: expected 201, got %d", rc.name, code)
				}
			} else {
				assertStatus(t, code, http.StatusForbidden, rc.name+" create job")
			}
		})
	}
}

func TestRoleMatrix_ContentWrite(t *testing.T) {
	for _, rc := range allRoles {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			c := newClient(t)
			loginAs(t, c, rc.user, rc.pass)
			code, body, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
				"name": "RMD_" + rc.name + "_" + uniqSuffix(),
			}, nil)
			allowed := rc.user == "admin" || rc.user == userEditor
			if allowed {
				if code != http.StatusCreated {
					t.Fatalf("%s create dynasty: expected 201, got %d", rc.name, code)
				}
				id := int64(body["id"].(float64))
				doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
			} else {
				assertStatus(t, code, http.StatusForbidden, rc.name+" create dynasty")
			}
		})
	}
}

// TestRoleMatrix_PricingManagementWrite — only admin + marketing_manager can
// mutate pricing resources; all other authenticated roles get 403.
func TestRoleMatrix_PricingManagementWrite(t *testing.T) {
	for _, rc := range allRoles {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			c := newClient(t)
			loginAs(t, c, rc.user, rc.pass)
			code, body, _ := doJSON(t, c, "POST", "/campaigns", map[string]any{
				"name":           "RM_" + rc.name + "_" + uniqSuffix(),
				"campaign_type":  "standard",
				"discount_type":  "percentage",
				"discount_value": 10,
			}, nil)
			allowed := rc.user == "admin" || rc.user == userMkt
			if allowed {
				if code != http.StatusCreated {
					t.Fatalf("%s create campaign: expected 201, got %d", rc.name, code)
				}
				if id, ok := body["id"].(float64); ok {
					doJSON(t, c, "DELETE", "/campaigns/"+itoa(int64(id)), nil, nil)
				}
			} else {
				assertStatus(t, code, http.StatusForbidden, rc.name+" create campaign")
			}
		})
	}
}

// TestRoleMatrix_RevisionsAdminOnly — revision history exposes before/after
// snapshots; only administrator may list or restore.
func TestRoleMatrix_RevisionsAdminOnly(t *testing.T) {
	for _, rc := range allRoles {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			c := newClient(t)
			loginAs(t, c, rc.user, rc.pass)
			code, _, _ := doJSON(t, c, "GET", "/revisions/supported-entities", nil, nil)
			if rc.user == "admin" {
				assertStatus(t, code, http.StatusOK, "admin allowed")
			} else {
				assertStatus(t, code, http.StatusForbidden, rc.name+" blocked")
			}
		})
	}
}

// TestRoleMatrix_MemberCannotAccessConsole — the member role must not
// access any console-backed write endpoints.
func TestRoleMatrix_MemberCannotAccessConsole(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userMember, passMember)

	forbidden := []struct {
		method, path string
		body         map[string]any
	}{
		{"GET", "/audit-logs", nil},
		{"GET", "/approvals", nil},
		{"GET", "/monitoring/metrics", nil},
		{"POST", "/dynasties", map[string]any{"name": "nope"}},
		{"POST", "/crawl/jobs", map[string]any{"job_name": "no", "source_url": "https://x"}},
		{"POST", "/campaigns", map[string]any{"name": "n", "campaign_type": "standard", "discount_type": "percentage", "discount_value": 5}},
	}
	for _, f := range forbidden {
		code, _, _ := doJSON(t, c, f.method, f.path, f.body, nil)
		if code != http.StatusForbidden {
			t.Fatalf("member %s %s expected 403, got %d", f.method, f.path, code)
		}
	}

	// Reads that are fine.
	allowed := []string{"/dynasties", "/authors", "/poems", "/search?q=test", "/member-tiers"}
	for _, path := range allowed {
		code, _, _ := doJSON(t, c, "GET", path, nil, nil)
		if code != http.StatusOK {
			t.Fatalf("member GET %s expected 200, got %d", path, code)
		}
	}
}

func TestRoleMatrix_ReadsAreOpenToAllAuthedRoles(t *testing.T) {
	for _, rc := range allRoles {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			c := newClient(t)
			loginAs(t, c, rc.user, rc.pass)
			for _, path := range []string{"/dynasties", "/poems", "/authors", "/tags"} {
				code, _, _ := doJSON(t, c, "GET", path, nil, nil)
				if code != http.StatusOK {
					t.Fatalf("%s GET %s expected 200, got %d", rc.name, path, code)
				}
			}
		})
	}
}
