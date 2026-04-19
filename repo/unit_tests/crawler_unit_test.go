package unittests

import (
	"testing"
	"time"

	"helios-backend/internal/crawler"
)

func TestCrawler_FirstCallToHostDoesNotSleep(t *testing.T) {
	l := crawler.NewHostLimiter(50 * time.Millisecond)
	slept := l.Wait("host.example")
	if slept != 0 {
		t.Fatalf("first Wait should be zero, got %v", slept)
	}
}

func TestCrawler_SecondCallEnforcesGap(t *testing.T) {
	gap := 80 * time.Millisecond
	l := crawler.NewHostLimiter(gap)
	l.Wait("host.example")
	start := time.Now()
	l.Wait("host.example")
	elapsed := time.Since(start)
	if elapsed < gap-10*time.Millisecond {
		t.Fatalf("second Wait did not enforce gap: %v < %v", elapsed, gap)
	}
}

func TestCrawler_SeparateHostsIndependent(t *testing.T) {
	l := crawler.NewHostLimiter(100 * time.Millisecond)
	l.Wait("a.example")
	if slept := l.Wait("b.example"); slept != 0 {
		t.Fatalf("different host should not sleep, got %v", slept)
	}
}

func TestCrawler_HostOfExtractsHost(t *testing.T) {
	if got := crawler.HostOf("http://example.com/x"); got != "example.com" {
		t.Fatalf("HostOf: got %q", got)
	}
	if got := crawler.HostOf("https://example.com:9090/y"); got != "example.com:9090" {
		t.Fatalf("HostOf with port: got %q", got)
	}
}
