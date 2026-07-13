package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// heatCoords covers every field used across the marker shapes (point, circle,
// line, rect, path, area) so a single Unmarshal works regardless of shape.
type heatCoords struct {
	X      *float64          `json:"x"`
	Y      *float64          `json:"y"`
	CX     *float64          `json:"cx"`
	CY     *float64          `json:"cy"`
	R      *float64          `json:"r"`
	X1     *float64          `json:"x1"`
	Y1     *float64          `json:"y1"`
	X2     *float64          `json:"x2"`
	Y2     *float64          `json:"y2"`
	W      *float64          `json:"w"`
	H      *float64          `json:"h"`
	Points []heatCoordsPoint `json:"points"`
}

type heatCoordsPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// shapeCentroid returns the normalized (0-1) center point of a marker shape,
// used to place it on the harvest heat map.
func shapeCentroid(shape, coordsJSON string) (x, y float64, ok bool) {
	var c heatCoords
	if err := json.Unmarshal([]byte(coordsJSON), &c); err != nil {
		return 0, 0, false
	}
	switch shape {
	case "point":
		if c.X == nil || c.Y == nil {
			return 0, 0, false
		}
		return *c.X, *c.Y, true
	case "circle":
		if c.CX == nil || c.CY == nil {
			return 0, 0, false
		}
		return *c.CX, *c.CY, true
	case "line":
		if c.X1 == nil || c.Y1 == nil || c.X2 == nil || c.Y2 == nil {
			return 0, 0, false
		}
		return (*c.X1 + *c.X2) / 2, (*c.Y1 + *c.Y2) / 2, true
	case "rect":
		if c.X == nil || c.Y == nil || c.W == nil || c.H == nil {
			return 0, 0, false
		}
		return *c.X + *c.W/2, *c.Y + *c.H/2, true
	case "path", "area":
		if len(c.Points) == 0 {
			return 0, 0, false
		}
		var sx, sy float64
		for _, p := range c.Points {
			sx += p.X
			sy += p.Y
		}
		n := float64(len(c.Points))
		return sx / n, sy / n, true
	default:
		return 0, 0, false
	}
}

type heatmapPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label"`
	Grams float64 `json:"grams"`
}

// HeatmapData returns the harvest-weight heat map points for a plot as JSON,
// consumed by the "Heat Map" toggle on the plot view.
func (h *Handler) HeatmapData(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	rows, err := h.db.GetHarvestHeatmap(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	points := make([]heatmapPoint, 0, len(rows))
	for _, row := range rows {
		x, y, ok := shapeCentroid(row.Shape, row.Coords)
		if !ok {
			continue
		}
		points = append(points, heatmapPoint{X: x, Y: y, Label: row.Label, Grams: row.Total})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Grams > points[j].Grams })

	writeJSON(w, http.StatusOK, points)
}
