package auth

import "testing"

func TestHashAndVerify_RoundTrip(t *testing.T) {
	h, err := HashPassword("correcthorse")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if h == "correcthorse" {
		t.Fatal("hash returned plaintext")
	}
	if !VerifyPassword(h, "correcthorse") {
		t.Fatal("verify failed on correct password")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("verify passed on wrong password")
	}
}

func TestPasswordPolicy(t *testing.T) {
	if err := ValidatePasswordPolicy("short"); err == nil {
		t.Fatal("expected rejection of short password")
	}
	if err := ValidatePasswordPolicy("abcdefgh"); err != nil {
		t.Fatalf("expected 8-char password accepted, got %v", err)
	}
}

func TestHashProducesDifferentSaltsEachCall(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("expected different hashes (bcrypt salting)")
	}
	if !VerifyPassword(h1, "same") || !VerifyPassword(h2, "same") {
		t.Fatal("both hashes should verify against the same password")
	}
}

func TestLoginRateLimiter_LocksAfterFailures(t *testing.T) {
	uname := "ratelimit_test_user"
	// Clear state (in case earlier tests touched it)
	RecordLoginResult(uname, true)
	for i := 0; i < maxLoginFailures; i++ {
		RecordLoginResult(uname, false)
	}
	if !IsLoginLocked(uname) {
		t.Fatal("expected username to be locked after repeated failures")
	}
	// Success clears
	RecordLoginResult(uname, true)
	if IsLoginLocked(uname) {
		t.Fatal("success should clear lockout")
	}
}
