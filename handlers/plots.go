package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"plotter/db"
)

func (h *Handler) ListPlots(w http.ResponseWriter, r *http.Request) {
	plots, err := h.db.GetPlots()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		if plots == nil {
			plots = []db.Plot{}
		}
		writeJSON(w, http.StatusOK, plots)
		return
	}
	h.render(w, r, "index", map[string]interface{}{
		"Plots": plots,
	})
}

func (h *Handler) ListPlotsJSON(w http.ResponseWriter, r *http.Request) {
	plots, err := h.db.GetPlots()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if plots == nil {
		plots = []db.Plot{}
	}
	writeJSON(w, http.StatusOK, plots)
}

func (h *Handler) NewPlot(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "plot_new", nil)
}

func (h *Handler) CreatePlot(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	name := strings.TrimSpace(r.FormValue("name"))
	address := strings.TrimSpace(r.FormValue("address"))

	if name == "" || address == "" {
		http.Error(w, "name and address are required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		http.Error(w, "unsupported image format", http.StatusBadRequest)
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(h.uploadDir, "plots", filename)

	out, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "failed to save image: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(out, file)

	id, err := h.db.CreatePlot(name, address, "plots/"+filename)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.isJSON(r) {
		plot, err := h.db.GetPlot(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, plot)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/plots/%d", id), http.StatusSeeOther)
}

type plotViewData struct {
	Plot        interface{}
	Markers     interface{}
	MarkersJSON string
}

func (h *Handler) ViewPlot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	plot, err := h.db.GetPlot(id)
	if err != nil {
		http.Error(w, "plot not found", http.StatusNotFound)
		return
	}

	markers, err := h.db.GetMarkers(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categories, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	layers, err := h.db.GetLayers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type markerJSON struct {
		ID          int64  `json:"id"`
		Shape       string `json:"shape"`
		Coords      string `json:"coords"`
		Label       string `json:"label"`
		CatID       int64  `json:"catId"`
		LayerID     int64  `json:"layerId"`
		Color       string `json:"color"`
		EndDate     string `json:"endDate"`
		PlantedDate string `json:"plantedDate"`
	}
	jmarkers := make([]markerJSON, len(markers))
	for i, m := range markers {
		catID := int64(0)
		if m.CategoryID != nil {
			catID = *m.CategoryID
		}
		layerID := int64(0)
		if m.LayerID != nil {
			layerID = *m.LayerID
		}
		color := m.CategoryColor
		if color == "" {
			color = "#64748b"
		}
		jmarkers[i] = markerJSON{m.ID, m.Shape, m.Coords, m.Label, catID, layerID, color, m.EndDate, m.PlantedDate}
	}
	if h.isJSON(r) {
		if markers == nil {
			markers = []db.Marker{}
		}
		if categories == nil {
			categories = []db.Category{}
		}
		if layers == nil {
			layers = []db.Layer{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"plot":       plot,
			"markers":    markers,
			"categories": categories,
			"layers":     layers,
		})
		return
	}

	jbytes, _ := json.Marshal(jmarkers)

	h.render(w, r, "plot", map[string]interface{}{
		"Plot":        plot,
		"Markers":     markers,
		"Categories":  categories,
		"Layers":      layers,
		"Today":       time.Now().Format("2006-01-02"),
		"MarkersJSON": template.JS(jbytes),
	})
}

// ListPlotMarkers returns all non-deleted markers for a plot as JSON.
// Used by the Android client: GET /plots/{id}/markers
func (h *Handler) ListPlotMarkers(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	markers, err := h.db.GetMarkers(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if markers == nil {
		markers = []db.Marker{}
	}
	writeJSON(w, http.StatusOK, markers)
}

func (h *Handler) RemapPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	plot, err := h.db.GetPlot(id)
	if err != nil {
		http.Error(w, "plot not found", http.StatusNotFound)
		return
	}
	markers, err := h.db.GetMarkersForRemap(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hasBackup, _ := h.db.HasRemapBackup(id)

	type mj struct {
		ID     int64  `json:"id"`
		Shape  string `json:"shape"`
		Coords string `json:"coords"`
	}
	mjs := make([]mj, len(markers))
	for i, m := range markers {
		mjs[i] = mj{m.ID, m.Shape, m.Coords}
	}
	jbytes, _ := json.Marshal(mjs)

	h.render(w, r, "remap", map[string]interface{}{
		"Plot":        plot,
		"MarkersJSON": template.JS(jbytes),
		"HasBackup":   hasBackup,
	})
}

func (h *Handler) UploadPlotImage(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		http.Error(w, "unsupported image format", http.StatusBadRequest)
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(h.uploadDir, "plots", filename)
	out, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "failed to save image", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(out, file)

	writeJSON(w, http.StatusOK, map[string]string{
		"image_path": "plots/" + filename,
		"image_url":  "/uploads/plots/" + filename,
	})
}

func (h *Handler) RemapPlot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req struct {
		NewImagePath string `json:"new_image_path"`
		SrcPoints    [4]Pt  `json:"src_points"`
		DstPoints    [4]Pt  `json:"dst_points"`
		CapturedDate string `json:"captured_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.NewImagePath == "" {
		http.Error(w, "new_image_path required", http.StatusBadRequest)
		return
	}

	H, err := computeHomography(req.SrcPoints, req.DstPoints)
	if err != nil {
		http.Error(w, "homography error: "+err.Error(), http.StatusBadRequest)
		return
	}

	markers, err := h.db.GetMarkersForRemap(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newCoords := make(map[int64]string, len(markers))
	for _, m := range markers {
		nc, err := transformCoords(H, m.Shape, m.Coords)
		if err != nil {
			nc = m.Coords
		}
		newCoords[m.ID] = nc
	}

	if err := h.db.SaveRemapBackup(id); err != nil {
		http.Error(w, "backup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.db.ApplyRemap(id, req.NewImagePath, newCoords); err != nil {
		http.Error(w, "remap failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	capturedDate := req.CapturedDate
	if capturedDate == "" {
		capturedDate = time.Now().Format("2006-01-02")
	}
	h.db.AddPlotImage(id, req.NewImagePath, capturedDate)

	writeJSON(w, http.StatusOK, map[string]string{
		"redirect": fmt.Sprintf("/plots/%d", id),
	})
}

func (h *Handler) UndoRemap(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.UndoRemap(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/plots/%d", id), http.StatusSeeOther)
}

func (h *Handler) GetPlotImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	imagePath, err := h.db.GetImageForDate(id, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"image_url": "/uploads/" + imagePath,
	})
}

func (h *Handler) DeletePlot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeletePlot(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// ── Harvest report ────────────────────────────────────────────

type harvestCultivar struct {
	Cultivar string
	Total    float64
}

type harvestLabel struct {
	Label     string
	Total     float64
	Cultivars []harvestCultivar // non-nil only when at least one cultivar is named
}

type harvestPeriod struct {
	Period string
	Labels []harvestLabel
	Total  float64
}

func buildHarvestPeriods(rows []db.HarvestReportRow, key func(db.HarvestReportRow) string) []harvestPeriod {
	type lk struct{ period, label string }
	var periodOrder []string
	periodSeen := map[string]bool{}
	labelOrder := map[string][]string{}
	labelSeen  := map[lk]bool{}
	totals     := map[lk]map[string]float64{} // [period+label][cultivar] = sum

	for _, r := range rows {
		p := key(r)
		if !periodSeen[p] {
			periodOrder = append(periodOrder, p)
			periodSeen[p] = true
		}
		k := lk{p, r.Label}
		if !labelSeen[k] {
			labelOrder[p] = append(labelOrder[p], r.Label)
			labelSeen[k] = true
			totals[k] = map[string]float64{}
		}
		totals[k][r.Cultivar] += r.Total
	}

	var periods []harvestPeriod
	for _, p := range periodOrder {
		var pTotal float64
		var labels []harvestLabel
		for _, label := range labelOrder[p] {
			k := lk{p, label}
			cv := totals[k]
			var labelTotal float64
			hasCultivar := false
			for name, v := range cv {
				labelTotal += v
				if name != "" {
					hasCultivar = true
				}
			}
			pTotal += labelTotal

			var cultivars []harvestCultivar
			if hasCultivar {
				for name, v := range cv {
					cultivars = append(cultivars, harvestCultivar{Cultivar: name, Total: v})
				}
				sort.Slice(cultivars, func(i, j int) bool {
					if cultivars[i].Cultivar == "" {
						return false
					}
					if cultivars[j].Cultivar == "" {
						return true
					}
					return cultivars[i].Cultivar < cultivars[j].Cultivar
				})
			}
			labels = append(labels, harvestLabel{Label: label, Total: labelTotal, Cultivars: cultivars})
		}
		periods = append(periods, harvestPeriod{Period: p, Labels: labels, Total: pTotal})
	}
	return periods
}

func (h *Handler) HarvestReport(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	plot, err := h.db.GetPlot(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rows, err := h.db.GetHarvestReport(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	monthly := buildHarvestPeriods(rows, func(r db.HarvestReportRow) string { return r.Month })
	yearly  := buildHarvestPeriods(rows, func(r db.HarvestReportRow) string { return r.Year })

	h.render(w, r, "harvest_report", map[string]interface{}{
		"Plot":    plot,
		"Monthly": monthly,
		"Yearly":  yearly,
	})
}
