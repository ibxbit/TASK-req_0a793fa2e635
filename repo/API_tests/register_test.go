package apitests

import (
	"net/http"
	"testing"
)

// TestRegister_HappyPath registers a fresh member account and verifies the
// returned user view has role=member, then confirms the new account can log in.
func TestRegister_HappyPath(t *testing.T) {
	username := "reg_" + uniqSuffix()
	password := "Password123"

	c := newClient(t)
	code, body, raw := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": username,
		"password": password,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d body=%s", code, raw)
	}
	userObj, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatalf("register: missing user object: %v", body)
	}
	if userObj["username"] != username {
		t.Fatalf("register: username mismatch: got %v want %s", userObj["username"], username)
	}
	if userObj["role"] != "member" {
		t.Fatalf("register: role should be member, got %v", userObj["role"])
	}
	if _, ok := userObj["id"].(float64); !ok {
		t.Fatalf("register: missing numeric id: %v", userObj)
	}

	// Newly registered account must be able to log in immediately.
	login := newClient(t)
	code, _, raw = doJSON(t, login, "POST", "/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, nil)
	if code != http.StatusOK {
		t.Fatalf("login after register: expected 200, got %d body=%s", code, raw)
	}
}

// TestRegister_DuplicateUsernameReturns409 ensures that registering with an
// already-taken username is rejected with 409 Conflict.
func TestRegister_DuplicateUsernameReturns409(t *testing.T) {
	username := "dup_" + uniqSuffix()
	c := newClient(t)
	code, _, _ := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": username, "password": "Password123",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("first register failed: %d", code)
	}

	code, body, _ := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": username, "password": "Password123",
	}, nil)
	assertStatus(t, code, http.StatusConflict, "duplicate username")
	if body["error"] == nil {
		t.Fatalf("409 missing error field: %v", body)
	}
}

// TestRegister_WeakPasswordRejected ensures the password policy is enforced
// (minimum 8 characters).
func TestRegister_WeakPasswordRejected(t *testing.T) {
	c := newClient(t)
	code, body, _ := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": "weakpw_" + uniqSuffix(), "password": "short",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "weak password rejected")
	if body["error"] == nil {
		t.Fatalf("400 missing error field: %v", body)
	}
}

// TestRegister_MissingFieldsReturns400 exercises required-field validation.
func TestRegister_MissingFieldsReturns400(t *testing.T) {
	c := newClient(t)

	// missing password
	code, _, _ := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": "nopw_" + uniqSuffix(),
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "missing password")

	// missing username
	code, _, _ = doJSON(t, c, "POST", "/auth/register", map[string]any{
		"password": "Password123",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "missing username")
}

// TestRegister_WithEmail registers with an optional email field and verifies
// that the response still has the expected shape.
func TestRegister_WithEmail(t *testing.T) {
	username := "emailreg_" + uniqSuffix()
	c := newClient(t)
	code, body, raw := doJSON(t, c, "POST", "/auth/register", map[string]any{
		"username": username,
		"password": "Password123",
		"email":    username + "@example.com",
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("register with email: expected 201, got %d body=%s", code, raw)
	}
	if body["user"] == nil {
		t.Fatalf("register with email: missing user field: %v", body)
	}
}
