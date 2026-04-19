package apitests

import (
	"net/http"
	"testing"
)

// seedPoem provisions a dynasty + poem that other tests can reference. It
// registers its own cleanup and returns the poem id.
func seedPoem(t *testing.T) int64 {
	t.Helper()
	hc := newClient(t)
	loginAdmin(t, hc)
	s := uniqSuffix()
	_, dBody, _ := doJSON(t, hc, "POST", "/dynasties", map[string]any{"name": "ExcerptDyn_" + s}, nil)
	dID := int64(dBody["id"].(float64))
	_, pBody, _ := doJSON(t, hc, "POST", "/poems", map[string]any{
		"title": "Host_" + s, "body": "Line one\nLine two", "dynasty_id": dID,
	}, nil)
	pID := int64(pBody["id"].(float64))
	t.Cleanup(func() {
		doJSON(t, hc, "DELETE", "/poems/"+itoa(pID), nil, nil)
		doJSON(t, hc, "DELETE", "/dynasties/"+itoa(dID), nil, nil)
	})
	return pID
}

func TestExcerpts_ListRequiresAuth(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/excerpts", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon list /excerpts")
}

func TestExcerpts_CRUDRoundTrip(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)

	code, body, raw := doJSON(t, c, "POST", "/excerpts", map[string]any{
		"poem_id":         pID,
		"start_offset":    0,
		"end_offset":      5,
		"excerpt_text":    "Hello",
		"annotation_type": "note",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create excerpt: %d %s", code, raw)
	}
	id := int64(body["id"].(float64))

	code, got, _ := doJSON(t, c, "GET", "/excerpts/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get excerpt")
	if got["excerpt_text"] != "Hello" {
		t.Fatalf("text mismatch: %v", got["excerpt_text"])
	}

	code, got, _ = doJSON(t, c, "PUT", "/excerpts/"+itoa(id), map[string]any{
		"poem_id":      pID,
		"start_offset": 0,
		"end_offset":   5,
		"excerpt_text": "Hello!",
	}, nil)
	assertStatus(t, code, http.StatusOK, "put excerpt")
	if got["excerpt_text"] != "Hello!" {
		t.Fatalf("updated text mismatch: %v", got["excerpt_text"])
	}

	code, _, _ = doJSON(t, c, "DELETE", "/excerpts/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "delete excerpt")
}

func TestExcerpts_ListFilterByPoemID(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)
	_, body, _ := doJSON(t, c, "POST", "/excerpts", map[string]any{
		"poem_id": pID, "start_offset": 0, "end_offset": 3, "excerpt_text": "Abc",
	}, nil)
	id := int64(body["id"].(float64))
	defer doJSON(t, c, "DELETE", "/excerpts/"+itoa(id), nil, nil)

	code, body, _ := doJSON(t, c, "GET", "/excerpts?poem_id="+itoa(pID), nil, nil)
	assertStatus(t, code, http.StatusOK, "list excerpts by poem")
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected at least 1 excerpt for poem %d: %v", pID, body)
	}
}

func TestExcerpts_InvalidAnnotationTypeReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)
	code, body, _ := doJSON(t, c, "POST", "/excerpts", map[string]any{
		"poem_id": pID, "start_offset": 0, "end_offset": 3,
		"excerpt_text": "Abc", "annotation_type": "bogus",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "bad annotation_type")
	if body["error"] == nil {
		t.Fatalf("expected error in body: %v", body)
	}
}

func TestExcerpts_BulkCreate(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	pID := seedPoem(t)

	code, body, _ := doJSON(t, c, "POST", "/excerpts/bulk", map[string]any{
		"create": []map[string]any{
			{"poem_id": pID, "start_offset": 0, "end_offset": 3, "excerpt_text": "Aaa"},
			{"poem_id": pID, "start_offset": 4, "end_offset": 7, "excerpt_text": "Bbb"},
		},
	}, nil)
	assertStatus(t, code, http.StatusOK, "bulk excerpts create")
	created, _ := body["created"].([]any)
	if len(created) != 2 {
		t.Fatalf("expected 2 created: %v", created)
	}
	ids := []int64{}
	for _, it := range created {
		row := it.(map[string]any)
		ids = append(ids, int64(row["id"].(float64)))
	}
	doJSON(t, c, "POST", "/excerpts/bulk", map[string]any{"delete": ids}, nil)
}
