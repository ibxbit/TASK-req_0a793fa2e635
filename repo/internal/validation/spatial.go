package validation

import (
	"encoding/json"
	"errors"
	"fmt"
)

const MaxVertices = 10000

var (
	ErrTooManyVertices = fmt.Errorf("geometry exceeds max vertices (%d)", MaxVertices)
	ErrSelfIntersect   = errors.New("geometry has self-intersecting edges")
	ErrBadGeometry     = errors.New("invalid geometry payload")
)

type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// ValidateGeoJSON validates a GeoJSON geometry object: enforces vertex cap
// and rejects self-intersections for LineString / Polygon rings.
func ValidateGeoJSON(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var g Geometry
	if err := json.Unmarshal(raw, &g); err != nil {
		return ErrBadGeometry
	}
	switch g.Type {
	case "Point":
		return nil
	case "MultiPoint":
		var pts [][]float64
		if err := json.Unmarshal(g.Coordinates, &pts); err != nil {
			return ErrBadGeometry
		}
		return checkVertices(len(pts))
	case "LineString":
		var line [][]float64
		if err := json.Unmarshal(g.Coordinates, &line); err != nil {
			return ErrBadGeometry
		}
		if err := checkVertices(len(line)); err != nil {
			return err
		}
		if hasSelfIntersection(line, false) {
			return ErrSelfIntersect
		}
		return nil
	case "MultiLineString":
		var lines [][][]float64
		if err := json.Unmarshal(g.Coordinates, &lines); err != nil {
			return ErrBadGeometry
		}
		total := 0
		for _, line := range lines {
			total += len(line)
			if hasSelfIntersection(line, false) {
				return ErrSelfIntersect
			}
		}
		return checkVertices(total)
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			return ErrBadGeometry
		}
		total := 0
		for _, ring := range rings {
			total += len(ring)
			if hasSelfIntersection(ring, true) {
				return ErrSelfIntersect
			}
		}
		return checkVertices(total)
	case "MultiPolygon":
		var polys [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
			return ErrBadGeometry
		}
		total := 0
		for _, poly := range polys {
			for _, ring := range poly {
				total += len(ring)
				if hasSelfIntersection(ring, true) {
					return ErrSelfIntersect
				}
			}
		}
		return checkVertices(total)
	default:
		return ErrBadGeometry
	}
}

func checkVertices(n int) error {
	if n > MaxVertices {
		return ErrTooManyVertices
	}
	return nil
}

// hasSelfIntersection does O(n^2) pairwise segment-intersection check.
// For a closed ring, the first and last vertex are identical.
func hasSelfIntersection(pts [][]float64, closed bool) bool {
	n := len(pts)
	if n < 4 {
		return false
	}
	// Build segments
	last := n - 1
	if !closed {
		last = n - 1
	}
	for i := 0; i < last-1; i++ {
		a1, a2 := pts[i], pts[i+1]
		for j := i + 1; j < last; j++ {
			// Adjacent segments share an endpoint — skip
			if j == i+1 {
				continue
			}
			// For closed rings, last segment shares endpoint with first
			if closed && i == 0 && j == last-1 {
				continue
			}
			b1, b2 := pts[j], pts[j+1]
			if segmentsIntersect(a1, a2, b1, b2) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(p1, p2, p3, p4 []float64) bool {
	if len(p1) < 2 || len(p2) < 2 || len(p3) < 2 || len(p4) < 2 {
		return false
	}
	d1 := orient(p3, p4, p1)
	d2 := orient(p3, p4, p2)
	d3 := orient(p1, p2, p3)
	d4 := orient(p1, p2, p4)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	return false
}

func orient(a, b, c []float64) float64 {
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}
