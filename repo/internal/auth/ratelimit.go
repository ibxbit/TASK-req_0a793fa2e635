package auth

import (
	"strings"
	"sync"
	"time"
)

const (
	maxLoginFailures = 5
	lockoutDuration  = 5 * time.Minute
	loginSweep       = 10 * time.Minute
)

type loginEntry struct {
	failures    int
	firstFailed time.Time
	lockedUntil time.Time
}

var (
	loginMu       sync.Mutex
	loginAttempts = make(map[string]*loginEntry)
)

func init() {
	go func() {
		t := time.NewTicker(loginSweep)
		defer t.Stop()
		for range t.C {
			sweepAttempts()
		}
	}()
}

func normalizeUsername(u string) string { return strings.ToLower(strings.TrimSpace(u)) }

// IsLoginLocked reports whether further login attempts for the username are
// currently being rejected outright because of too many recent failures.
func IsLoginLocked(username string) bool {
	u := normalizeUsername(username)
	if u == "" {
		return false
	}
	loginMu.Lock()
	defer loginMu.Unlock()
	e, ok := loginAttempts[u]
	if !ok {
		return false
	}
	return time.Now().Before(e.lockedUntil)
}

// RecordLoginResult records one login attempt. On success the entry is
// cleared; on failure, after maxLoginFailures the username is locked for
// lockoutDuration.
func RecordLoginResult(username string, success bool) {
	u := normalizeUsername(username)
	if u == "" {
		return
	}
	loginMu.Lock()
	defer loginMu.Unlock()

	if success {
		delete(loginAttempts, u)
		return
	}
	e, ok := loginAttempts[u]
	if !ok {
		e = &loginEntry{firstFailed: time.Now()}
		loginAttempts[u] = e
	}
	// Expire the failure window after lockoutDuration of no activity.
	if time.Since(e.firstFailed) > lockoutDuration {
		e.failures = 0
		e.firstFailed = time.Now()
	}
	e.failures++
	if e.failures >= maxLoginFailures {
		e.lockedUntil = time.Now().Add(lockoutDuration)
	}
}

func sweepAttempts() {
	now := time.Now()
	loginMu.Lock()
	defer loginMu.Unlock()
	for k, e := range loginAttempts {
		expired := now.After(e.lockedUntil) && now.Sub(e.firstFailed) > lockoutDuration
		if expired {
			delete(loginAttempts, k)
		}
	}
}
