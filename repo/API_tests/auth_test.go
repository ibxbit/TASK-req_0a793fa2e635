package apitests

import (
	"net/http"
	"testing"
)

func TestAuth_MeWithoutSessionReturns401(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/auth/me", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "GET /auth/me without session")
}

func TestAuth_LoginWithBadPasswordReturns401(t *testing.T) {
	c := newClient(t)
	code, body, _ := doJSON(t, c, "POST", "/auth/login", map[string]string{
		"username": "admin",
		"password": "wrong-password",
	}, nil)
	assertStatus(t, code, http.StatusUnauthorized, "login with bad password")
	if _, ok := body["error"]; !ok {
		t.Fatalf("response missing error field: %v", body)
	}
}

func TestAuth_FullRoundTrip(t *testing.T) {
	c := newClient(t)

	// 1. unauthenticated /me
	code, _, _ := doJSON(t, c, "GET", "/auth/me", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "unauth /me")

	// 2. login
	code, body, _ := doJSON(t, c, "POST", "/auth/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, nil)
	assertStatus(t, code, http.StatusOK, "login")
	if got := mustString(t, body, "user", "role"); got != "administrator" {
		t.Fatalf("unexpected role: %s", got)
	}

	// 3. authenticated /me
	code, body, _ = doJSON(t, c, "GET", "/auth/me", nil, nil)
	assertStatus(t, code, http.StatusOK, "authed /me")
	if got := mustString(t, body, "user", "username"); got != "admin" {
		t.Fatalf("unexpected username: %s", got)
	}

	// 4. logout
	code, _, _ = doJSON(t, c, "POST", "/auth/logout", nil, nil)
	assertStatus(t, code, http.StatusOK, "logout")

	// 5. /me after logout
	code, _, _ = doJSON(t, c, "GET", "/auth/me", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "/me after logout")
}

func TestAuth_MalformedBodyReturns400(t *testing.T) {
	c := newClient(t)
	code, _ := doRaw(t, c, "POST", "/auth/login", "not json")
	assertStatus(t, code, http.StatusBadRequest, "malformed login body")
}
