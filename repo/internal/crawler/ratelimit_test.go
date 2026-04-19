package crawler

import (
	"testing"
	"time"
)

func TestHostLimiter_FirstCallNoSleep(t *testing.T) {
	l := NewHostLimiter(50 * time.Millisecond)
	slept := l.Wait("example.com")
	if slept != 0 {
		t.Fatalf("first call should not sleep, slept %v", slept)
	}
}

func TestHostLimiter_EnforcesMinGap(t *testing.T) {
	gap := 80 * time.Millisecond
	l := NewHostLimiter(gap)
	l.Wait("example.com")
	start := time.Now()
	l.Wait("example.com")
	elapsed := time.Since(start)
	if elapsed < gap-10*time.Millisecond {
		t.Fatalf("second call did not honor gap: %v < %v", elapsed, gap)
	}
}

func TestHostLimiter_DifferentHostsIndependent(t *testing.T) {
	l := NewHostLimiter(100 * time.Millisecond)
	l.Wait("a.example.com")
	slept := l.Wait("b.example.com")
	if slept != 0 {
		t.Fatalf("different host should not sleep, slept %v", slept)
	}
}

func TestHostOf_Parses(t *testing.T) {
	if got := HostOf("http://example.com/path"); got != "example.com" {
		t.Fatalf("HostOf: got %q", got)
	}
	if got := HostOf("https://host:8080/"); got != "host:8080" {
		t.Fatalf("HostOf with port: got %q", got)
	}
	if got := HostOf("not a url"); got == "" {
		t.Fatalf("HostOf on malformed should return the input, got %q", got)
	}
}
