package crawler

import (
	"net/url"
	"sync"
	"time"
)

// HostLimiter enforces a minimum interval between requests per host.
// Default: 1 req/sec per host.
type HostLimiter struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
	minGap   time.Duration
}

func NewHostLimiter(minGap time.Duration) *HostLimiter {
	if minGap <= 0 {
		minGap = 1 * time.Second
	}
	return &HostLimiter{
		lastSeen: make(map[string]time.Time),
		minGap:   minGap,
	}
}

// Wait blocks until another request against `host` is permitted. Returns the
// amount of time it slept.
func (l *HostLimiter) Wait(host string) time.Duration {
	if host == "" {
		return 0
	}
	l.mu.Lock()
	now := time.Now()
	var sleep time.Duration
	if last, ok := l.lastSeen[host]; ok {
		if delta := now.Sub(last); delta < l.minGap {
			sleep = l.minGap - delta
		}
	}
	nextAllowed := now.Add(sleep)
	l.lastSeen[host] = nextAllowed
	l.mu.Unlock()
	if sleep > 0 {
		time.Sleep(sleep)
	}
	return sleep
}

// HostOf returns the host portion of a URL (with port if present) for keying.
func HostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}
