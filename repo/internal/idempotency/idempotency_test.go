package idempotency

import "testing"

// isMutating is an internal helper that decides whether the idempotency
// middleware should inspect a request. Safe methods (GET/HEAD/OPTIONS) are
// ignored so the middleware does no DB work for them.
func TestIsMutating(t *testing.T) {
	cases := []struct {
		method string
		want   bool
	}{
		{"POST", true},
		{"PUT", true},
		{"PATCH", true},
		{"DELETE", true},
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isMutating(tc.method); got != tc.want {
			t.Fatalf("isMutating(%q)=%v want %v", tc.method, got, tc.want)
		}
	}
}

func TestHeaderKeyIsStandard(t *testing.T) {
	if HeaderKey != "Idempotency-Key" {
		t.Fatalf("HeaderKey drift: %q", HeaderKey)
	}
}
