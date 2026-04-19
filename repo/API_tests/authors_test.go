package apitests

import (
	"net/http"
	"testing"
)

// Covers GET/POST/PUT/DELETE /authors, /authors/:id, /authors/bulk.

func TestAuthors_ListRequiresAuthAndReturnsEnvelope(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/authors", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon list /authors")

	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/authors?limit=5", nil, nil)
	assertStatus(t, code, http.StatusOK, "admin list /authors")
	if _, ok := body["items"]; !ok {
		t.Fatalf("missing items in list response: %v", body)
	}
	if v, _ := body["limit"].(float64); int(v) != 5 {
		t.Fatalf("limit not echoed: %v", body["limit"])
	}
}

func TestAuthors_CRUDRoundTrip(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	name := "Author_" + uniqSuffix()

	code, body, raw := doJSON(t, c, "POST", "/authors", map[string]any{"name": name}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create /authors: %d %s", code, raw)
	}
	id := int64(body["id"].(float64))
	if body["name"] != name {
		t.Fatalf("author name did not round-trip: %v", body["name"])
	}

	code, got, _ := doJSON(t, c, "GET", "/authors/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get /authors/:id")
	if got["name"] != name {
		t.Fatalf("get name mismatch: %v", got["name"])
	}

	code, got, _ = doJSON(t, c, "PUT", "/authors/"+itoa(id), map[string]any{"name": name + "_v2"}, nil)
	assertStatus(t, code, http.StatusOK, "put /authors/:id")
	if got["name"] != name+"_v2" {
		t.Fatalf("update did not take: %v", got["name"])
	}

	code, _, _ = doJSON(t, c, "DELETE", "/authors/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "delete /authors/:id")

	code, _, _ = doJSON(t, c, "GET", "/authors/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "get after delete")
}

func TestAuthors_CreateRejectsMissingName(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/authors", map[string]any{}, nil)
	assertStatus(t, code, http.StatusBadRequest, "create without name")
	if body["error"] == nil {
		t.Fatalf("expected error field: %v", body)
	}
}

func TestAuthors_BulkCreateAndDelete(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()
	code, body, _ := doJSON(t, c, "POST", "/authors/bulk", map[string]any{
		"create": []map[string]any{
			{"name": "BA1_" + s},
			{"name": "BA2_" + s},
		},
	}, nil)
	assertStatus(t, code, http.StatusOK, "bulk create /authors")
	created, _ := body["created"].([]any)
	if len(created) != 2 {
		t.Fatalf("expected 2 created, got %v", body["created"])
	}

	ids := []int64{}
	for _, it := range created {
		if row, ok := it.(map[string]any); ok {
			if idF, ok := row["id"].(float64); ok {
				ids = append(ids, int64(idF))
			}
		}
	}
	if len(ids) != 2 {
		t.Fatalf("failed to extract ids: %v", created)
	}
	code, body, _ = doJSON(t, c, "POST", "/authors/bulk", map[string]any{"delete": ids}, nil)
	assertStatus(t, code, http.StatusOK, "bulk delete /authors")
	if n, _ := body["deleted"].(float64); int(n) != 2 {
		t.Fatalf("expected deleted=2, got %v", body["deleted"])
	}
}

func TestAuthors_ContentEditorRoleCanWrite(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userEditor, passEditor)
	name := "EditorAuthor_" + uniqSuffix()
	code, body, raw := doJSON(t, c, "POST", "/authors", map[string]any{"name": name}, nil)
	if code != http.StatusCreated {
		t.Fatalf("editor create /authors: %d %s", code, raw)
	}
	doJSON(t, c, "DELETE", "/authors/"+itoa(int64(body["id"].(float64))), nil, nil)
}

func TestAuthors_ReviewerRoleCannotWrite(t *testing.T) {
	c := newClient(t)
	loginAs(t, c, userReviewer, passReviewer)
	code, _, _ := doJSON(t, c, "POST", "/authors", map[string]any{"name": "Blocked"}, nil)
	assertStatus(t, code, http.StatusForbidden, "reviewer cannot create author")
}
