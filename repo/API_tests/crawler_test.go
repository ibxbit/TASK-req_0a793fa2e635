package apitests

import (
	"net/http"
	"testing"
)

// Covers crawler endpoints: nodes, jobs CRUD and lifecycle (pause/resume/cancel/reset),
// plus metrics + logs and role separation.

func TestCrawler_NodesListRequiresAuth(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/crawl/nodes", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon crawl/nodes")

	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/crawl/nodes", nil, nil)
	assertStatus(t, code, http.StatusOK, "admin crawl/nodes")
	if _, ok := body["items"]; !ok {
		t.Fatalf("missing items in /crawl/nodes: %v", body)
	}
}

func TestCrawler_JobLifecycleAsOperator(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userCrawler, passCrawler)

	name := "Job_" + uniqSuffix()
	code, body, raw := doJSON(t, c, "POST", "/crawl/jobs", map[string]any{
		"job_name":     name,
		"source_url":   "https://example.com/corpus",
		"priority":     5,
		"max_attempts": 3,
		"daily_quota":  100,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create crawl job: %d %s", code, raw)
	}
	id := int64(body["id"].(float64))
	if body["status"] != "queued" {
		t.Fatalf("new job should be queued, got %v", body["status"])
	}

	// get
	code, got, _ := doJSON(t, c, "GET", "/crawl/jobs/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "GET /crawl/jobs/:id")
	if got["job_name"] != name {
		t.Fatalf("job name mismatch: %v", got["job_name"])
	}

	// pause
	code, pause, _ := doJSON(t, c, "POST", "/crawl/jobs/"+itoa(id)+"/pause", nil, nil)
	assertStatus(t, code, http.StatusOK, "pause job")
	if pause["status"] != "paused" {
		t.Fatalf("expected paused, got %v", pause["status"])
	}

	// resume
	code, resume, _ := doJSON(t, c, "POST", "/crawl/jobs/"+itoa(id)+"/resume", nil, nil)
	assertStatus(t, code, http.StatusOK, "resume job")
	if resume["status"] != "queued" {
		t.Fatalf("expected queued after resume, got %v", resume["status"])
	}

	// cancel
	code, cancel, _ := doJSON(t, c, "POST", "/crawl/jobs/"+itoa(id)+"/cancel", nil, nil)
	assertStatus(t, code, http.StatusOK, "cancel job")
	if cancel["status"] != "cancelled" {
		t.Fatalf("expected cancelled, got %v", cancel["status"])
	}

	// reset (cancelled job can't transition to pause/resume but reset always works)
	code, reset, _ := doJSON(t, c, "POST", "/crawl/jobs/"+itoa(id)+"/reset", nil, nil)
	assertStatus(t, code, http.StatusOK, "reset job")
	if reset["status"] != "queued" {
		t.Fatalf("expected queued after reset, got %v", reset["status"])
	}
}

func TestCrawler_JobRequiresName(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userCrawler, passCrawler)
	code, body, _ := doJSON(t, c, "POST", "/crawl/jobs", map[string]any{
		"source_url": "https://example.com",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "create without job_name")
	if body["error"] == nil {
		t.Fatalf("expected error field: %v", body)
	}
}

func TestCrawler_NonOperatorCannotCreateJob(t *testing.T) {
	// content_editor — not a crawler operator or admin.
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	code, _, _ := doJSON(t, c, "POST", "/crawl/jobs", map[string]any{
		"job_name": "blocked", "source_url": "https://example.com",
	}, nil)
	assertStatus(t, code, http.StatusForbidden, "editor creating crawl job")
}

func TestCrawler_JobsListEchoesPaging(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/crawl/jobs?limit=5&offset=0", nil, nil)
	assertStatus(t, code, http.StatusOK, "list jobs")
	if v, _ := body["limit"].(float64); int(v) != 5 {
		t.Fatalf("limit not echoed: %v", body["limit"])
	}
}

func TestCrawler_MetricsAndLogsEndpoints(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userCrawler, passCrawler)
	// Create a job so /metrics and /logs have a valid id.
	_, body, _ := doJSON(t, c, "POST", "/crawl/jobs", map[string]any{
		"job_name": "ML_" + uniqSuffix(), "source_url": "https://example.com",
	}, nil)
	id := int64(body["id"].(float64))

	code, met, _ := doJSON(t, c, "GET", "/crawl/jobs/"+itoa(id)+"/metrics", nil, nil)
	assertStatus(t, code, http.StatusOK, "metrics endpoint")
	if v, _ := met["job_id"].(float64); int64(v) != id {
		t.Fatalf("metrics job_id mismatch: %v", met["job_id"])
	}
	if _, ok := met["metrics"]; !ok {
		t.Fatalf("missing metrics array in response: %v", met)
	}

	code, logs, _ := doJSON(t, c, "GET", "/crawl/jobs/"+itoa(id)+"/logs?limit=5", nil, nil)
	assertStatus(t, code, http.StatusOK, "logs endpoint")
	if _, ok := logs["items"]; !ok {
		t.Fatalf("missing items in logs response: %v", logs)
	}
}

func TestCrawler_TransitionConflict(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userCrawler, passCrawler)
	_, body, _ := doJSON(t, c, "POST", "/crawl/jobs", map[string]any{
		"job_name": "TX_" + uniqSuffix(), "source_url": "https://example.com",
	}, nil)
	id := int64(body["id"].(float64))

	// resume on a queued job is a no-op (not in paused/failed). Expect 409.
	code, _, _ := doJSON(t, c, "POST", "/crawl/jobs/"+itoa(id)+"/resume", nil, nil)
	assertStatus(t, code, http.StatusConflict, "resume on queued job")
}

// TestCrawler_ReadEndpoints_DenyNonStaff verifies that member, editor, reviewer,
// and marketing_manager all receive 403 on every crawler read endpoint.
func TestCrawler_ReadEndpoints_DenyNonStaff(t *testing.T) {
	// Create a job as crawler_operator so we have a valid job ID for path-param endpoints.
	setup := newClient(t)
	loginAs(t, setup, userCrawler, passCrawler)
	_, jbody, _ := doJSON(t, setup, "POST", "/crawl/jobs", map[string]any{
		"job_name": "RBAC_" + uniqSuffix(), "source_url": "https://example.com",
	}, nil)
	jobID := itoa(int64(jbody["id"].(float64)))

	endpoints := []string{
		"/crawl/nodes",
		"/crawl/jobs",
		"/crawl/jobs/" + jobID,
		"/crawl/jobs/" + jobID + "/metrics",
		"/crawl/jobs/" + jobID + "/logs",
	}

	denied := []struct{ user, pass string }{
		{userMember, passMember},
		{userEditor, passEditor},
		{userReviewer, passReviewer},
		{userMkt, passMkt},
	}

	for _, u := range denied {
		c := newClient(t)
		loginAs(t, c, u.user, u.pass)
		for _, ep := range endpoints {
			code, _, _ := doJSON(t, c, "GET", ep, nil, nil)
			if code != http.StatusForbidden {
				t.Errorf("user=%s GET %s: expected 403, got %d", u.user, ep, code)
			}
		}
	}
}

// TestCrawler_ReadEndpoints_AllowStaff verifies that administrator and
// crawler_operator both receive 200 on every crawler read endpoint.
func TestCrawler_ReadEndpoints_AllowStaff(t *testing.T) {
	// Create a job so the per-job endpoints have a valid ID.
	setup := newClient(t)
	loginAs(t, setup, userCrawler, passCrawler)
	_, jbody, _ := doJSON(t, setup, "POST", "/crawl/jobs", map[string]any{
		"job_name": "AllowRBAC_" + uniqSuffix(), "source_url": "https://example.com",
	}, nil)
	jobID := itoa(int64(jbody["id"].(float64)))

	endpoints := []string{
		"/crawl/nodes",
		"/crawl/jobs",
		"/crawl/jobs/" + jobID,
		"/crawl/jobs/" + jobID + "/metrics",
		"/crawl/jobs/" + jobID + "/logs",
	}

	allowed := []struct{ user, pass string }{
		{"admin", "admin123"},
		{userCrawler, passCrawler},
	}

	for _, u := range allowed {
		c := newClient(t)
		loginAs(t, c, u.user, u.pass)
		for _, ep := range endpoints {
			code, _, _ := doJSON(t, c, "GET", ep, nil, nil)
			if code != http.StatusOK {
				t.Errorf("user=%s GET %s: expected 200, got %d", u.user, ep, code)
			}
		}
	}
}

// TestCrawler_ReadEndpoints_AnonReturns401 verifies unauthenticated requests
// get 401 on all crawler read endpoints.
func TestCrawler_ReadEndpoints_AnonReturns401(t *testing.T) {
	endpoints := []string{
		"/crawl/nodes",
		"/crawl/jobs",
		"/crawl/jobs/1",
		"/crawl/jobs/1/metrics",
		"/crawl/jobs/1/logs",
	}
	c := newClient(t)
	for _, ep := range endpoints {
		code, _, _ := doJSON(t, c, "GET", ep, nil, nil)
		if code != http.StatusUnauthorized {
			t.Errorf("anon GET %s: expected 401, got %d", ep, code)
		}
	}
}
