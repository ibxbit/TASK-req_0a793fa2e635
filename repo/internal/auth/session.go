package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"time"
)

const (
	SessionCookieName = "helios_session"
	IdleTimeout       = 30 * time.Minute
	sweepInterval     = 5 * time.Minute
	MinPasswordLength = 8
	EnvCookieSecure   = "HELIOS_COOKIE_SECURE"
)

// CookieSecure reports whether session cookies should carry the Secure flag.
// Disabled by default so the local/offline stack works over HTTP; enable by
// setting HELIOS_COOKIE_SECURE=1 when serving the app behind HTTPS.
func CookieSecure() bool {
	v := os.Getenv(EnvCookieSecure)
	return v == "1" || v == "true" || v == "yes"
}

// ValidatePasswordPolicy enforces the password minimum length. Called from any
// code path that sets a password (bootstrap, future password change endpoints).
func ValidatePasswordPolicy(pw string) error {
	if len(pw) < MinPasswordLength {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

type Session struct {
	ID           string
	UserID       int64
	Username     string
	RoleName     string
	CreatedAt    time.Time
	LastActiveAt time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

var store = newStore()

func newStore() *Store {
	s := &Store{sessions: make(map[string]*Session)}
	go s.sweepLoop()
	return s
}

func (s *Store) sweepLoop() {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for range t.C {
		s.sweep()
	}
}

func (s *Store) sweep() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if now.Sub(sess.LastActiveAt) > IdleTimeout {
			delete(s.sessions, id)
		}
	}
}

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateSession(userID int64, username, role string) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &Session{
		ID:           id,
		UserID:       userID,
		Username:     username,
		RoleName:     role,
		CreatedAt:    now,
		LastActiveAt: now,
	}
	store.mu.Lock()
	store.sessions[id] = sess
	store.mu.Unlock()
	return sess, nil
}

func GetSession(id string) (*Session, bool) {
	store.mu.RLock()
	sess, ok := store.sessions[id]
	store.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(sess.LastActiveAt) > IdleTimeout {
		DestroySession(id)
		return nil, false
	}
	return sess, true
}

func TouchSession(id string) {
	store.mu.Lock()
	if sess, ok := store.sessions[id]; ok {
		sess.LastActiveAt = time.Now()
	}
	store.mu.Unlock()
}

func DestroySession(id string) {
	store.mu.Lock()
	delete(store.sessions, id)
	store.mu.Unlock()
}

// BackdateForTest mutates LastActiveAt on a session so that callers of
// GetSession/TouchSession observe the configured idle-timeout behaviour
// without waiting 30 real minutes. Only used by tests.
func BackdateForTest(id string, by time.Duration) {
	store.mu.Lock()
	if sess, ok := store.sessions[id]; ok {
		sess.LastActiveAt = sess.LastActiveAt.Add(-by)
	}
	store.mu.Unlock()
}

// SweepForTest runs the internal sweep loop once synchronously. Only used
// by tests that need to observe the cleanup-by-sweeper code path without
// waiting for the background ticker.
func SweepForTest() {
	store.sweep()
}
