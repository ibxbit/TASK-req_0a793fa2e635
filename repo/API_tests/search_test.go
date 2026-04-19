package apitests

import (
	"net/http"
	"testing"
)

func TestSearch_OptionsEchoDefaultsAreDeterministic(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/search?q=test", nil, nil)
	assertStatus(t, code, http.StatusOK, "search with q")
	opts, ok := body["options"].(map[string]any)
	if !ok {
		t.Fatalf("search response missing options: %v", body)
	}
	if opts["expand_synonyms"] != false {
		t.Fatalf("expected synonyms default false, got %v", opts["expand_synonyms"])
	}
	if opts["convert_cjk"] != false {
		t.Fatalf("expected cjk default false, got %v", opts["convert_cjk"])
	}
}

func TestSearch_HighlightFlagRespected(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/search?q=test&highlight=1", nil, nil)
	assertStatus(t, code, http.StatusOK, "highlight query")
	opts := body["options"].(map[string]any)
	if opts["highlight"] != true {
		t.Fatalf("expected highlight=true, got %v", opts["highlight"])
	}
}

func TestSearch_SuggestEndpoint(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/search/suggest?q=test&limit=3", nil, nil)
	assertStatus(t, code, http.StatusOK, "GET /search/suggest")
	if _, ok := body["suggestions"]; !ok {
		t.Fatalf("suggest response missing 'suggestions': %v", body)
	}
}

func TestSearch_ReindexIsAdminOnly(t *testing.T) {
	c := newClient(t)

	// anonymous → 401
	code, _, _ := doJSON(t, c, "POST", "/search/reindex", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon reindex")

	// admin → 200
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "POST", "/search/reindex", nil, nil)
	assertStatus(t, code, http.StatusOK, "admin reindex")
	if body["reindexed"] != true {
		t.Fatalf("expected reindexed=true, got %v", body["reindexed"])
	}
}
