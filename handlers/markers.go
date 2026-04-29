package handlers

import (
	"encoding/json"
	"fmt"
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


func (h *Handler) CreateMarker(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}

	var shape, coords, label, endDate, plantedDate string
	var markerCatID, markerLayID *int64

	if isJSONBody(r) {
		var req struct {
			Shape      string          `json:"shape"`
			Coords     json.RawMessage `json:"coords"`
			Label      string          `json:"label"`
			CategoryID *int64          `json:"category_id"`
			LayerID    *int64          `json:"layer_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		shape = req.Shape
		coords = string(req.Coords)
		label = req.Label
		markerCatID = req.CategoryID
		markerLayID = req.LayerID
	} else {
		r.ParseForm()
		shape = r.FormValue("shape")
		coords = r.FormValue("coords")
		label = strings.TrimSpace(r.FormValue("label"))
		endDate = strings.TrimSpace(r.FormValue("end_date"))
		plantedDate = strings.TrimSpace(r.FormValue("planted_date"))
		parseOptID := func(key string) *int64 {
			if s := r.FormValue(key); s != "" {
				if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
					return &id
				}
			}
			return nil
		}
		markerCatID = parseOptID("category_id")
		markerLayID = parseOptID("layer_id")
	}

	if shape == "" || coords == "" {
		http.Error(w, "shape and coords required", http.StatusBadRequest)
		return
	}

	markerID, err := h.db.CreateMarker(plotID, shape, coords, label, endDate, plantedDate, markerCatID, markerLayID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	marker, err := h.db.GetMarker(markerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catID := int64(0)
	if marker.CategoryID != nil {
		catID = *marker.CategoryID
	}
	layerID := int64(0)
	if marker.LayerID != nil {
		layerID = *marker.LayerID
	}
	color := marker.CategoryColor
	if color == "" {
		color = "#64748b"
	}

	if h.isJSON(r) {
		writeJSON(w, http.StatusCreated, marker)
		return
	}

	w.Header().Set("HX-Trigger", fmt.Sprintf(
		`{"markerCreated":{"id":%d,"shape":%q,"coords":%s,"label":%q,"catId":%d,"layerId":%d,"color":%q,"endDate":%q}}`,
		marker.ID, marker.Shape, marker.Coords, marker.Label, catID, layerID, color, marker.EndDate,
	))
	h.renderPartial(w, r, "marker_item", marker)
}

func (h *Handler) ViewMarker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	marker, err := h.db.GetMarker(id)
	if err != nil {
		http.Error(w, "marker not found", http.StatusNotFound)
		return
	}

	entries, err := h.db.GetEntriesWithImages(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	layers, err := h.db.GetLayers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	transplants, err := h.db.GetTransplants(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	today := time.Now().Format("2006-01-02")
	data := map[string]interface{}{
		"Marker":      marker,
		"Entries":     entries,
		"Today":       today,
		"Categories":  cats,
		"Layers":      layers,
		"Taxonomy":    (*db.PlantTaxonomy)(nil),
		"Harvests":    []db.Harvest{},
		"Group":       (*db.PlantGroup)(nil),
		"Transplants": transplants,
	}

	if marker.CategoryType == "plant" {
		if tax, err := h.db.GetTaxonomy(id); err == nil {
			data["Taxonomy"] = tax
		}
		if harvests, err := h.db.GetHarvests(id); err == nil {
			data["Harvests"] = harvests
		}
	}

	if marker.GroupID != nil {
		if grp, err := h.db.GetPlantGroup(*marker.GroupID); err == nil {
			data["Group"] = grp
		}
	}

	if h.isJSON(r) {
		entries, _ := data["Entries"].([]db.MarkerEntry)
		transplants, _ := data["Transplants"].([]db.Transplant)
		harvests, _ := data["Harvests"].([]db.Harvest)
		if entries == nil {
			entries = []db.MarkerEntry{}
		}
		if transplants == nil {
			transplants = []db.Transplant{}
		}
		if harvests == nil {
			harvests = []db.Harvest{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"marker":      marker,
			"entries":     entries,
			"transplants": transplants,
			"taxonomy":    data["Taxonomy"],
			"harvests":    harvests,
			"group":       data["Group"],
		})
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, r, "marker_detail", data)
		return
	}
	h.render(w, r, "marker", data)
}

func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	markerID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var date, notes string
	if isJSONBody(r) {
		var req struct {
			Note string `json:"note"`
			Date string `json:"date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		notes = req.Note
		date = req.Date
	} else {
		r.ParseMultipartForm(64 << 20)
		date = strings.TrimSpace(r.FormValue("date"))
		notes = strings.TrimSpace(r.FormValue("notes"))
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	} else if len(date) > 10 {
		date = date[:10]
	}

	entryID, err := h.db.CreateEntry(markerID, date, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Attach any photos uploaded with the entry form.
	if r.MultipartForm != nil {
		caption := strings.TrimSpace(r.FormValue("caption"))
		for _, fh := range r.MultipartForm.File["images"] {
			file, err := fh.Open()
			if err != nil {
				continue
			}
			ext := strings.ToLower(filepath.Ext(fh.Filename))
			filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(h.uploadDir, "markers", filename)
			if out, err := os.Create(savePath); err == nil {
				io.Copy(out, file)
				out.Close()
				h.db.AddEntryImage(entryID, "markers/"+filename, caption)
			}
			file.Close()
		}
	}

	if h.isJSON(r) {
		entry, err := h.db.GetEntry(entryID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, entry)
		return
	}

	entries, err := h.db.GetEntriesWithImages(markerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, r, "entry_item", map[string]interface{}{
		"Entries":  entries,
		"MarkerID": markerID,
		"Today":    time.Now().Format("2006-01-02"),
	})
}

func (h *Handler) AddEntryImages(w http.ResponseWriter, r *http.Request) {
	entryID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseMultipartForm(64 << 20)

	caption := strings.TrimSpace(r.FormValue("caption"))
	files := r.MultipartForm.File["images"]
	for _, fh := range files {
		file, err := fh.Open()
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(h.uploadDir, "markers", filename)
		if out, err := os.Create(savePath); err == nil {
			io.Copy(out, file)
			out.Close()
			h.db.AddEntryImage(entryID, "markers/"+filename, caption)
		}
		file.Close()
	}

	images, err := h.db.GetEntryImages(entryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "entry_images", map[string]interface{}{
		"Images":  images,
		"EntryID": entryID,
	})
}

func (h *Handler) DeleteEntryImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	entryID, err := h.db.DeleteEntryImage(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	images, _ := h.db.GetEntryImages(entryID)
	h.renderPartial(w, r, "entry_images", map[string]interface{}{
		"Images":  images,
		"EntryID": entryID,
	})
}

func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	markerID, err := h.db.DeleteEntry(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entries, err := h.db.GetEntriesWithImages(markerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "entry_item", map[string]interface{}{
		"Entries":  entries,
		"MarkerID": markerID,
		"Today":    time.Now().Format("2006-01-02"),
	})
}

func (h *Handler) UpdateMarker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var label, endDate, plantedDate string
	var catID, layID *int64

	if isJSONBody(r) {
		var req struct {
			Label      string `json:"label"`
			CategoryID *int64 `json:"category_id"`
			LayerID    *int64 `json:"layer_id"`
			PlantedDate string `json:"planted_date"`
			EndDate    string `json:"end_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		label = req.Label
		endDate = req.EndDate
		plantedDate = req.PlantedDate
		catID = req.CategoryID
		layID = req.LayerID
	} else {
		r.ParseForm()
		label = strings.TrimSpace(r.FormValue("label"))
		endDate = strings.TrimSpace(r.FormValue("end_date"))
		plantedDate = strings.TrimSpace(r.FormValue("planted_date"))
		parseOptID := func(key string) *int64 {
			if s := r.FormValue(key); s != "" {
				if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
					return &v
				}
			}
			return nil
		}
		catID = parseOptID("category_id")
		layID = parseOptID("layer_id")
	}

	if err := h.db.UpdateMarker(id, label, endDate, plantedDate, catID, layID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.isJSON(r) {
		marker, err := h.db.GetMarker(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, marker)
		return
	}

	// Full marker page: redirect the browser so the complete layout reloads.
	if r.FormValue("context") == "full-page" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/markers/%d", id))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Sidebar panel: return the updated partial in-place.
	r.Method = "GET"
	h.ViewMarker(w, r)
}

func (h *Handler) BulkUpdateMarkers(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var markerIDs []int64
	for _, raw := range strings.Split(r.FormValue("marker_ids"), ",") {
		raw = strings.TrimSpace(raw)
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			markerIDs = append(markerIDs, id)
		}
	}
	if len(markerIDs) == 0 {
		http.Error(w, "no marker_ids", http.StatusBadRequest)
		return
	}

	// "" = no change, "clear" = set NULL, any number = set to that ID
	applyID := func(val string, current *int64) *int64 {
		if val == "clear" {
			return nil
		}
		if val != "" {
			if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
				return &id
			}
		}
		return current
	}
	// "" = no change, anything else = set (empty string clears the field)
	applyDate := func(val, current string) string {
		if val == "" {
			return current
		}
		return val // caller sends "clear" as an explicit empty — we just use val directly
	}

	categoryVal := r.FormValue("category_id")
	layerVal := r.FormValue("layer_id")
	endDate := strings.TrimSpace(r.FormValue("end_date"))
	plantedDate := strings.TrimSpace(r.FormValue("planted_date"))

	for _, mid := range markerIDs {
		m, err := h.db.GetMarker(mid)
		if err != nil {
			continue
		}
		h.db.UpdateMarker(mid,
			m.Label,
			applyDate(endDate, m.EndDate),
			applyDate(plantedDate, m.PlantedDate),
			applyID(categoryVal, m.CategoryID),
			applyID(layerVal, m.LayerID),
		)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CreateTransplant(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var newCoords, date, notes string

	if isJSONBody(r) {
		var req struct {
			Coords json.RawMessage `json:"coords"`
			Date   string          `json:"date"`
			Notes  string          `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		newCoords = string(req.Coords)
		date = req.Date
		notes = req.Notes
	} else {
		r.ParseForm()
		newCoords = strings.TrimSpace(r.FormValue("coords"))
		date = strings.TrimSpace(r.FormValue("date"))
		notes = strings.TrimSpace(r.FormValue("notes"))
	}

	if newCoords == "" {
		http.Error(w, "coords required", http.StatusBadRequest)
		return
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	marker, err := h.db.GetMarker(id)
	if err != nil {
		http.Error(w, "marker not found", http.StatusNotFound)
		return
	}

	transplantID, err := h.db.CreateTransplant(id, marker.Coords, newCoords, date, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.isJSON(r) {
		transplant, err := h.db.GetTransplant(transplantID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, transplant)
		return
	}

	transplants, err := h.db.GetTransplants(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"markerTransplanted":{"id":%d,"coords":%s}}`, id, newCoords))
	h.renderPartial(w, r, "transplant_list", map[string]interface{}{
		"Transplants": transplants,
		"MarkerID":    id,
	})
}

func (h *Handler) DeleteMarker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if _, err := h.db.GetMarker(id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.db.DeleteMarker(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
}
