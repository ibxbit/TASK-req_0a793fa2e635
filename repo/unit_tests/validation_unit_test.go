package unittests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"helios-backend/internal/validation"
)

func TestValidation_ValidSquarePolygon(t *testing.T) {
	in := json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[4,0],[4,4],[0,4],[0,0]]]}`)
	if err := validation.ValidateGeoJSON(in); err != nil {
		t.Fatalf("expected valid square, got %v", err)
	}
}

func TestValidation_SelfIntersectingRejected(t *testing.T) {
	in := json.RawMessage(`{"type":"LineString","coordinates":[[0,0],[2,2],[0,2],[2,0]]}`)
	err := validation.ValidateGeoJSON(in)
	if err != validation.ErrSelfIntersect {
		t.Fatalf("expected ErrSelfIntersect, got %v", err)
	}
}

func TestValidation_MaxVertexCap(t *testing.T) {
	parts := make([]string, 0, validation.MaxVertices+2)
	for i := 0; i <= validation.MaxVertices; i++ {
		parts = append(parts, fmt.Sprintf("[%d,0]", i))
	}
	raw := `{"type":"LineString","coordinates":[` + strings.Join(parts, ",") + `]}`
	if err := validation.ValidateGeoJSON(json.RawMessage(raw)); err != validation.ErrTooManyVertices {
		t.Fatalf("expected ErrTooManyVertices, got %v", err)
	}
}

func TestValidation_BadJSON(t *testing.T) {
	if err := validation.ValidateGeoJSON(json.RawMessage(`not-json`)); err != validation.ErrBadGeometry {
		t.Fatalf("expected ErrBadGeometry, got %v", err)
	}
}

func TestValidation_NilPayloadOK(t *testing.T) {
	if err := validation.ValidateGeoJSON(nil); err != nil {
		t.Fatalf("nil should be OK, got %v", err)
	}
}
