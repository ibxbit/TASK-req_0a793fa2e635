package search

import "testing"

func contains(list []string, needle string) bool {
	for _, s := range list {
		if s == needle {
			return true
		}
	}
	return false
}

func TestExpandCJK_SC_to_TC(t *testing.T) {
	got := ExpandCJKToken("国")
	if !contains(got, "国") || !contains(got, "國") {
		t.Fatalf("expected both 国 and 國, got %v", got)
	}
}

func TestExpandCJK_TC_to_SC(t *testing.T) {
	got := ExpandCJKToken("國")
	if !contains(got, "國") || !contains(got, "国") {
		t.Fatalf("expected both 國 and 国, got %v", got)
	}
}

func TestExpandCJK_NonCJK_PassThrough(t *testing.T) {
	got := ExpandCJKToken("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("expected [hello], got %v", got)
	}
}

func TestExpandCJK_Empty(t *testing.T) {
	if got := ExpandCJKToken(""); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestCJKVariants_Identity(t *testing.T) {
	vs := CJKVariants('a')
	if len(vs) != 1 || vs[0] != 'a' {
		t.Fatalf("expected [a], got %v", vs)
	}
}
