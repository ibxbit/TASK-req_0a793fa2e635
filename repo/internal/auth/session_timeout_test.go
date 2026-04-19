package auth

import (
	"testing"
	"time"
)

// IdleTimeout is documented at 30 minutes. These tests pin the behaviour
// so it can't silently regress, and exercise both the on-access expiry
// (GetSession) and the background sweeper (SweepForTest).

func TestSession_ActiveRightNow_IsRetrievable(t *testing.T) {
	sess, err := CreateSession(1, "alice", "administrator")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer DestroySession(sess.ID)

	got, ok := GetSession(sess.ID)
	if !ok || got == nil {
		t.Fatalf("fresh session should be retrievable")
	}
	if got.UserID != 1 || got.RoleName != "administrator" {
		t.Fatalf("unexpected session contents: %+v", got)
	}
}

func TestSession_IdleTimeoutBoundary_RejectsOnlyBeyondThirtyMinutes(t *testing.T) {
	// Just-inside the window should still be valid — give the test a 10s
	// buffer so we don't flake on slow CI hosts.
	sess, err := CreateSession(2, "bob", "reviewer")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer DestroySession(sess.ID)

	BackdateForTest(sess.ID, IdleTimeout-10*time.Second)
	if _, ok := GetSession(sess.ID); !ok {
		t.Fatalf("session within idle window should still be valid")
	}

	// Push it past the 30-minute threshold.
	BackdateForTest(sess.ID, 11*time.Second) // now (IdleTimeout+1s)
	if _, ok := GetSession(sess.ID); ok {
		t.Fatalf("session beyond idle window should be rejected by GetSession")
	}

	// And the record should have been removed inline (GetSession destroys it).
	if _, ok := GetSession(sess.ID); ok {
		t.Fatalf("expired session should no longer exist in the store")
	}
}

func TestSession_TouchResetsIdleCounter(t *testing.T) {
	sess, err := CreateSession(3, "carol", "content_editor")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer DestroySession(sess.ID)

	BackdateForTest(sess.ID, 25*time.Minute)
	TouchSession(sess.ID) // should bring LastActiveAt back to now-ish
	BackdateForTest(sess.ID, 10*time.Minute)
	// Combined elapsed is 10m < 30m → still live.
	if _, ok := GetSession(sess.ID); !ok {
		t.Fatalf("touching a session should reset the idle counter")
	}
}

func TestSession_SweeperRemovesExpiredSessions(t *testing.T) {
	sess, err := CreateSession(4, "dave", "marketing_manager")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	BackdateForTest(sess.ID, IdleTimeout+5*time.Second)

	SweepForTest()

	// After the sweep the in-memory store should no longer contain the id.
	store.mu.RLock()
	_, stillThere := store.sessions[sess.ID]
	store.mu.RUnlock()
	if stillThere {
		t.Fatalf("sweeper should have evicted the expired session")
	}
}

func TestSession_DestroyIsIdempotent(t *testing.T) {
	sess, err := CreateSession(5, "erin", "crawler_operator")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	DestroySession(sess.ID)
	DestroySession(sess.ID) // must not panic or error
	if _, ok := GetSession(sess.ID); ok {
		t.Fatalf("destroyed session should not be retrievable")
	}
}

func TestSession_IdleTimeoutConstantMatchesSpec(t *testing.T) {
	if IdleTimeout != 30*time.Minute {
		t.Fatalf("IdleTimeout drift: got %v, spec says 30m", IdleTimeout)
	}
}
