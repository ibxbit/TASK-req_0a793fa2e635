package apitests

import (
	"net/http"
	"testing"
)

// Covers GET/POST/PUT/DELETE /tags, /tags/:id, /tags/bulk.

func TestTags_ListRequiresAuth(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/tags", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon list /tags")
}

func TestTags_CRUDRoundTrip(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	name := "Tag_" + uniqSuffix()

	code, body, _ := doJSON(t, c, "POST", "/tags", map[string]any{"name": name}, nil)
	assertStatus(t, code, http.StatusCreated, "create /tags")
	id := int64(body["id"].(float64))

	code, got, _ := doJSON(t, c, "GET", "/tags/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get /tags/:id")
	if got["name"] != name {
		t.Fatalf("tag name mismatch: %v", got["name"])
	}

	code, got, _ = doJSON(t, c, "PUT", "/tags/"+itoa(id), map[string]any{"name": name + "_v2"}, nil)
	assertStatus(t, code, http.StatusOK, "put /tags/:id")
	if got["name"] != name+"_v2" {
		t.Fatalf("update did not take: %v", got["name"])
	}

	code, _, _ = doJSON(t, c, "DELETE", "/tags/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "delete /tags/:id")
}

func TestTags_CreateMissingNameReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/tags", map[string]any{}, nil)
	assertStatus(t, code, http.StatusBadRequest, "create tag without name")
	if body["error"] == nil {
		t.Fatalf("expected error in response: %v", body)
	}
}

func TestTags_BulkCreateUpdateDelete(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()
	code, body, _ := doJSON(t, c, "POST", "/tags/bulk", map[string]any{
		"create": []map[string]any{{"name": "T1_" + s}, {"name": "T2_" + s}},
	}, nil)
	assertStatus(t, code, http.StatusOK, "bulk create tags")
	created, _ := body["created"].([]any)
	if len(created) != 2 {
		t.Fatalf("expected 2 created tags, got %v", created)
	}
	ids := []int64{}
	for _, it := range created {
		row := it.(map[string]any)
		ids = append(ids, int64(row["id"].(float64)))
	}

	// Bulk update name of first
	code, _, _ = doJSON(t, c, "POST", "/tags/bulk", map[string]any{
		"update": []map[string]any{{"id": ids[0], "name": "T1updated_" + s}},
	}, nil)
	assertStatus(t, code, http.StatusOK, "bulk update")

	// Cleanup
	code, body, _ = doJSON(t, c, "POST", "/tags/bulk", map[string]any{"delete": ids}, nil)
	assertStatus(t, code, http.StatusOK, "bulk delete")
	if n, _ := body["deleted"].(float64); int(n) != 2 {
		t.Fatalf("expected deleted=2, got %v", body["deleted"])
	}
}
