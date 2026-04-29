package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
