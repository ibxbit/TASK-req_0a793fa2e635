package apitests

import (
	"io"
	"net/http"
	"testing"
)

func TestContentPack_RequiresAuth(t *testing.T) {
	c := newClient(t)
	code, _, _ := doJSON(t, c, "GET", "/content-packs/current", nil, nil)
	assertStatus(t, code, http.StatusUnauthorized, "anon content pack")
}

func TestContentPack_GetReturnsJSONWithPoems(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, body, _ := doJSON(t, c, "GET", "/content-packs/current", nil, nil)
	assertStatus(t, code, http.StatusOK, "get current pack")
	for _, k := range []string{"version", "built_at", "poems", "authors", "dynasties", "tags"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing %q in pack: %v", k, body)
		}
	}
}

func TestContentPack_HeadReturnsETag(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	req, _ := http.NewRequest("HEAD", baseURL()+"/content-packs/current", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("HEAD: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HEAD status: %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Fatalf("HEAD missing ETag header: %v", resp.Header)
	}
}
