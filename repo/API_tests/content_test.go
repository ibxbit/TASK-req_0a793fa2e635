package apitests

import (
	"net/http"
	"testing"
)

func TestContent_DynastyCRUDRoundTrip(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	name := "Dyn_" + uniqSuffix()

	// Create
	code, body, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name":       name,
		"start_year": 100,
		"end_year":   200,
	}, nil)
	assertStatus(t, code, http.StatusCreated, "create dynasty")
	idF, ok := body["id"].(float64)
	if !ok {
		t.Fatalf("missing id in create response: %v", body)
	}
	id := int64(idF)

	// Get
	code, got, _ := doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "get dynasty")
	if got["name"] != name {
		t.Fatalf("name mismatch: %v vs %s", got["name"], name)
	}

	// Update
	code, got, _ = doJSON(t, c, "PUT", "/dynasties/"+itoa(id), map[string]any{
		"name": name + "_v2",
	}, nil)
	assertStatus(t, code, http.StatusOK, "update dynasty")
	if got["name"] != name+"_v2" {
		t.Fatalf("update did not take effect: %v", got["name"])
	}

	// Delete
	code, _, _ = doJSON(t, c, "DELETE", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusOK, "delete dynasty")

	// 404 after delete
	code, _, _ = doJSON(t, c, "GET", "/dynasties/"+itoa(id), nil, nil)
	assertStatus(t, code, http.StatusNotFound, "GET after delete")
}

func TestContent_MissingRequiredFieldReturns400(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"start_year": 100, // no name
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "dynasty without name")
}

func TestContent_SelfIntersectingGeometryRejected(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	code, _, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{
		"name": "Geom_" + uniqSuffix(),
		"geometry": map[string]any{
			"type":        "LineString",
			"coordinates": [][]float64{{0, 0}, {2, 2}, {0, 2}, {2, 0}},
		},
	}, nil)
	assertStatus(t, code, http.StatusBadRequest, "self-intersecting LineString")
}

func TestContent_BulkCreate(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	suffix := uniqSuffix()
	code, body, _ := doJSON(t, c, "POST", "/dynasties/bulk", map[string]any{
		"create": []map[string]any{
			{"name": "Bulk1_" + suffix},
			{"name": "Bulk2_" + suffix},
		},
	}, nil)
	assertStatus(t, code, http.StatusOK, "bulk create")

	created, ok := body["created"].([]any)
	if !ok || len(created) != 2 {
		t.Fatalf("expected 2 created rows, got %v", body["created"])
	}
	// Cleanup
	for _, it := range created {
		if row, ok := it.(map[string]any); ok {
			if idF, ok := row["id"].(float64); ok {
				doJSON(t, c, "DELETE", "/dynasties/"+itoa(int64(idF)), nil, nil)
			}
		}
	}
}

func TestContent_PoemReferencesDynasty(t *testing.T) {
	c := newClient(t)
	loginAdmin(t, c)
	suffix := uniqSuffix()

	_, dBody, _ := doJSON(t, c, "POST", "/dynasties", map[string]any{"name": "Tang_" + suffix}, nil)
	dID := int64(dBody["id"].(float64))
	defer doJSON(t, c, "DELETE", "/dynasties/"+itoa(dID), nil, nil)

	code, pBody, raw := doJSON(t, c, "POST", "/poems", map[string]any{
		"title":      "Poem_" + suffix,
		"body":       "春眠不觉晓",
		"dynasty_id": dID,
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create poem: %d %s", code, raw)
	}
	pID := int64(pBody["id"].(float64))
	defer doJSON(t, c, "DELETE", "/poems/"+itoa(pID), nil, nil)

	if pBody["dynasty_id"] == nil || int64(pBody["dynasty_id"].(float64)) != dID {
		t.Fatalf("dynasty_id did not round-trip: %v", pBody["dynasty_id"])
	}
}
