package apitests

import (
	"net/http"
	"testing"
)

// Direct coverage for PUT /poems/:id and POST /poems/bulk — the routes the
// static audit flagged as having no HTTP test.

func TestPoems_UpdateInPlace(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()

	_, dBody, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{"name": "PoemUpdDyn_" + s}, nil)
	dID := int64(dBody["id"].(float64))
	defer doJSON(t, c, "DELETE", "/dynasties/"+itoa(dID), nil, nil)

	_, pBody, _ := doJSON(t, c, "POST", "/poems", map[string]any{
		"title": "T_" + s, "body": "original body", "dynasty_id": dID,
	}, nil)
	pID := int64(pBody["id"].(float64))
	defer doJSON(t, c, "DELETE", "/poems/"+itoa(pID), nil, nil)

	code, got, _ := doJSON(t, c, "PUT", "/poems/"+itoa(pID), map[string]any{
		"title": "T_" + s + "_v2", "body": "updated body",
	}, nil)
	assertStatus(t, code, http.StatusOK, "put /poems/:id")
	if got["title"] != "T_"+s+"_v2" {
		t.Fatalf("poem title did not update: %v", got["title"])
	}
	if got["body"] != "updated body" {
		t.Fatalf("poem body did not update: %v", got["body"])
	}
	if v, _ := got["version"].(float64); v < 2 {
		t.Fatalf("expected version >= 2 after update, got %v", got["version"])
	}
}

func TestPoems_RejectsInvalidStatus(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "POST", "/poems", map[string]any{
		"title": "Bad_" + uniqSuffix(),
		"body":  "x",
		"status": "made_up",
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "invalid poem status")
}

func TestPoems_BulkCreate(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()
	code, body, raw := doJSON(t, c, "POST", "/poems/bulk", map[string]any{
		"create": []map[string]any{
			{"title": "Bp1_" + s, "body": "a"},
			{"title": "Bp2_" + s, "body": "b"},
		},
	}, nil)
	if code != http.StatusOK {
		t.Fatalf("poems bulk create: %d %s", code, raw)
	}
	created, _ := body["created"].([]any)
	if len(created) != 2 {
		t.Fatalf("expected 2 poems created: %v", body)
	}
	for _, it := range created {
		if row, ok := it.(map[string]any); ok {
			if idF, ok := row["id"].(float64); ok {
				doJSON(t, c, "DELETE", "/poems/"+itoa(int64(idF)), nil, nil)
			}
		}
	}
}

func TestPoems_BulkUpdateAndDelete(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	s := uniqSuffix()
	_, b, _ := doJSON(t, c, "POST", "/poems", map[string]any{
		"title": "U1_" + s, "body": "a",
	}, nil)
	id := int64(b["id"].(float64))

	code, body, _ := doJSON(t, c, "POST", "/poems/bulk", map[string]any{
		"update": []map[string]any{{"id": id, "title": "U1_" + s + "_v2", "body": "a"}},
		"delete": []int64{},
	}, nil)
	assertStatus(t, code, http.StatusOK, "bulk update poem")
	if v, _ := body["updated"].(float64); int(v) != 1 {
		t.Fatalf("expected updated=1 got %v", body["updated"])
	}

	code, body, _ = doJSON(t, c, "POST", "/poems/bulk", map[string]any{"delete": []int64{id}}, nil)
	assertStatus(t, code, http.StatusOK, "bulk delete poem")
	if v, _ := body["deleted"].(float64); int(v) != 1 {
		t.Fatalf("expected deleted=1 got %v", body["deleted"])
	}
}
