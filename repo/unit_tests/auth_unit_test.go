package unittests

import (
	"testing"

	"helios-backend/internal/auth"
)

func TestAuth_HashVerifyRoundTrip(t *testing.T) {
	h, err := auth.HashPassword("correcthorse")
	if err != nil {
		t.Fatal(err)
	}
	if h == "correcthorse" {
		t.Fatal("hash returned plaintext")
	}
	if !auth.VerifyPassword(h, "correcthorse") {
		t.Fatal("verify failed on correct password")
	}
	if auth.VerifyPassword(h, "wrong") {
		t.Fatal("verify passed on wrong password")
	}
}

func TestAuth_HashSaltIndependence(t *testing.T) {
	h1, _ := auth.HashPassword("same")
	h2, _ := auth.HashPassword("same")
	if h1 == h2 {
		t.Fatal("bcrypt hashes should differ due to fresh salt")
	}
}

func TestAuth_PasswordPolicyEnforced(t *testing.T) {
	if err := auth.ValidatePasswordPolicy("short"); err == nil {
		t.Fatal("policy must reject < 8-char password")
	}
	if err := auth.ValidatePasswordPolicy("12345678"); err != nil {
		t.Fatalf("policy rejected 8-char password: %v", err)
	}
}

func TestAuth_LoginLockoutRoundTrip(t *testing.T) {
	u := "unittests_user_" + t.Name()
	// Start clean
	auth.RecordLoginResult(u, true)
	if auth.IsLoginLocked(u) {
		t.Fatal("fresh user should not be locked")
	}
	for i := 0; i < 5; i++ {
		auth.RecordLoginResult(u, false)
	}
	if !auth.IsLoginLocked(u) {
		t.Fatal("user should be locked after 5 failures")
	}
	// Successful login clears
	auth.RecordLoginResult(u, true)
	if auth.IsLoginLocked(u) {
		t.Fatal("success should clear lockout")
	}
}
