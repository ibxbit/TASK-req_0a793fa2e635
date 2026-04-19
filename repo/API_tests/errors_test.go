package apitests

import (
	"net/http"
	"testing"
)

func TestErrors_MalformedJSONReturns400WithErrorField(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body := doRaw(t, c, "POST", "/dynasties", "not-valid-json")
	assertStatus(t, code, http.StatusBadRequest, "malformed POST body")
	if !containsStr(body, `"error"`) {
		t.Fatalf("expected error field in response: %s", body)
	}
}

func TestErrors_MissingRequiredField400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "POST", "/poems", map[string]any{
		"title": "missing body",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "poem without body")
}

func TestErrors_UnknownIDReturns404(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "GET", "/poems/999999999", nil, nil)
	assertStatus(t, code, http.StatusNotFound, "unknown poem id")
}

func TestErrors_UnknownRouteReturns404Or405(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/totally-unknown-route", nil, nil)
	switch code {
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		// ok
	default:
		t.Fatalf("unknown route expected 404 or 405, got %d", code)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
