package handlers

import (
	"encoding/json"
	"fmt"
	"math"
)

// Pt is a normalized coordinate pair in [0,1] space.
type Pt struct{ X, Y float64 }

// computeHomography returns the 3×3 homography matrix that maps src[i] → dst[i]
// using the Direct Linear Transform with H[2][2]=1.
func computeHomography(src, dst [4]Pt) ([3][3]float64, error) {
	var A [8][8]float64
	var b [8]float64

	for i := 0; i < 4; i++ {
		sx, sy := src[i].X, src[i].Y
		dx, dy := dst[i].X, dst[i].Y

		A[2*i][0] = sx; A[2*i][1] = sy; A[2*i][2] = 1
		A[2*i][6] = -sx * dx; A[2*i][7] = -sy * dx
		b[2*i] = dx

		A[2*i+1][3] = sx; A[2*i+1][4] = sy; A[2*i+1][5] = 1
		A[2*i+1][6] = -sx * dy; A[2*i+1][7] = -sy * dy
		b[2*i+1] = dy
	}

	h, err := gaussElim8(A, b)
	if err != nil {
		return [3][3]float64{}, err
	}
	return [3][3]float64{
		{h[0], h[1], h[2]},
		{h[3], h[4], h[5]},
		{h[6], h[7], 1.0},
	}, nil
}

func gaussElim8(A [8][8]float64, b [8]float64) ([8]float64, error) {
	const n = 8
	for col := 0; col < n; col++ {
		maxRow, maxVal := col, math.Abs(A[col][col])
		for row := col + 1; row < n; row++ {
			if v := math.Abs(A[row][col]); v > maxVal {
				maxVal, maxRow = v, row
			}
		}
		if maxVal < 1e-12 {
			return [8]float64{}, fmt.Errorf("singular system: control points may be collinear or duplicated")
		}
		A[col], A[maxRow] = A[maxRow], A[col]
		b[col], b[maxRow] = b[maxRow], b[col]
		for row := col + 1; row < n; row++ {
			f := A[row][col] / A[col][col]
			b[row] -= f * b[col]
			for c := col; c < n; c++ {
				A[row][c] -= f * A[col][c]
			}
		}
	}
	var x [8]float64
	for i := n - 1; i >= 0; i-- {
		x[i] = b[i]
		for j := i + 1; j < n; j++ {
			x[i] -= A[i][j] * x[j]
		}
		x[i] /= A[i][i]
	}
	return x, nil
}

func applyH(H [3][3]float64, x, y float64) (float64, float64) {
	w := H[2][0]*x + H[2][1]*y + H[2][2]
	if math.Abs(w) < 1e-12 {
		return x, y
	}
	return (H[0][0]*x + H[0][1]*y + H[0][2]) / w,
		(H[1][0]*x + H[1][1]*y + H[1][2]) / w
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// transformCoords applies H to a marker's JSON coords string for any shape.
func transformCoords(H [3][3]float64, shape, coordsJSON string) (string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(coordsJSON), &raw); err != nil {
		return coordsJSON, err
	}

	tf := func(x, y float64) (float64, float64) {
		return applyH(H, x, y)
	}

	switch shape {
	case "point":
		nx, ny := tf(jsonF(raw, "x"), jsonF(raw, "y"))
		raw["x"], raw["y"] = nx, ny

	case "circle":
		nx, ny := tf(jsonF(raw, "cx"), jsonF(raw, "cy"))
		raw["cx"], raw["cy"] = nx, ny

	case "line":
		x1, y1 := tf(jsonF(raw, "x1"), jsonF(raw, "y1"))
		x2, y2 := tf(jsonF(raw, "x2"), jsonF(raw, "y2"))
		raw["x1"], raw["y1"] = x1, y1
		raw["x2"], raw["y2"] = x2, y2

	case "rect":
		x, y := jsonF(raw, "x"), jsonF(raw, "y")
		w, h := jsonF(raw, "w"), jsonF(raw, "h")
		corners := [4][2]float64{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}}
		minX, minY, maxX, maxY := 1.0, 1.0, 0.0, 0.0
		for _, c := range corners {
			tx, ty := tf(c[0], c[1])
			if tx < minX {
				minX = tx
			}
			if ty < minY {
				minY = ty
			}
			if tx > maxX {
				maxX = tx
			}
			if ty > maxY {
				maxY = ty
			}
		}
		raw["x"], raw["y"] = minX, minY
		raw["w"], raw["h"] = maxX-minX, maxY-minY

	case "path", "area":
		pts, _ := raw["points"].([]interface{})
		out := make([]interface{}, len(pts))
		for i, p := range pts {
			pm, _ := p.(map[string]interface{})
			nx, ny := tf(jsonF(pm, "x"), jsonF(pm, "y"))
			out[i] = map[string]interface{}{"x": nx, "y": ny}
		}
		raw["points"] = out
	}

	result, err := json.Marshal(raw)
	return string(result), err
}

func jsonF(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}
