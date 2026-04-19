package validation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestValidate_NilPayload(t *testing.T) {
	if err := ValidateGeoJSON(nil); err != nil {
		t.Fatalf("expected nil for empty payload, got %v", err)
	}
}

func TestValidate_Point_Valid(t *testing.T) {
	in := json.RawMessage(`{"type":"Point","coordinates":[0,0]}`)
	if err := ValidateGeoJSON(in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_LineString_Valid(t *testing.T) {
	in := json.RawMessage(`{"type":"LineString","coordinates":[[0,0],[1,1],[2,0]]}`)
	if err := ValidateGeoJSON(in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_LineString_SelfIntersects(t *testing.T) {
	// Classic X: two segments crossing at (1,1)
	in := json.RawMessage(`{"type":"LineString","coordinates":[[0,0],[2,2],[0,2],[2,0]]}`)
	err := ValidateGeoJSON(in)
	if err != ErrSelfIntersect {
		t.Fatalf("expected self-intersect error, got %v", err)
	}
}

func TestValidate_Polygon_Valid(t *testing.T) {
	in := json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[4,0],[4,4],[0,4],[0,0]]]}`)
	if err := ValidateGeoJSON(in); err != nil {
		t.Fatalf("expected valid square, got %v", err)
	}
}

func TestValidate_VertexCapExceeded(t *testing.T) {
	// Build a LineString with MaxVertices + 1 points
	var coords []string
	for i := 0; i <= MaxVertices; i++ {
		coords = append(coords, fmt.Sprintf("[%d,0]", i))
	}
	raw := fmt.Sprintf(`{"type":"LineString","coordinates":[%s]}`, strings.Join(coords, ","))
	err := ValidateGeoJSON(json.RawMessage(raw))
	if err != ErrTooManyVertices {
		t.Fatalf("expected vertex cap error, got %v", err)
	}
}

func TestValidate_InvalidJSON(t *testing.T) {
	if err := ValidateGeoJSON(json.RawMessage(`not json`)); err != ErrBadGeometry {
		t.Fatalf("expected bad geometry, got %v", err)
	}
}

func TestValidate_UnknownType(t *testing.T) {
	in := json.RawMessage(`{"type":"Nonsense","coordinates":[[0,0]]}`)
	if err := ValidateGeoJSON(in); err != ErrBadGeometry {
		t.Fatalf("expected bad geometry, got %v", err)
	}
}
