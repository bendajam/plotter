package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"plotter/db"
)

// GET /plots/{id}/geo-refs
func (h *Handler) ListGeoRefs(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	refs, err := h.db.GetGeoRefs(plotID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if refs == nil {
		refs = []db.PlotGeoRef{}
	}
	writeJSON(w, http.StatusOK, refs)
}

// PUT /plots/{id}/geo-refs/{index}
// Body: { "image_x": float, "image_y": float, "lat": float, "lng": float }
func (h *Handler) UpsertGeoRef(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || index < 0 || index > 3 {
		http.Error(w, "index must be 0–3", http.StatusBadRequest)
		return
	}

	var body struct {
		ImageX float64 `json:"image_x"`
		ImageY float64 `json:"image_y"`
		Lat    float64 `json:"lat"`
		Lng    float64 `json:"lng"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.db.UpsertGeoRef(plotID, index, body.ImageX, body.ImageY, body.Lat, body.Lng); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	refs, err := h.db.GetGeoRefs(plotID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, refs)
}

// DELETE /plots/{id}/geo-refs/{index}
func (h *Handler) DeleteGeoRef(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil || index < 0 || index > 3 {
		http.Error(w, "index must be 0–3", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteGeoRef(plotID, index); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /plots/{id}/geo-refs
func (h *Handler) DeleteAllGeoRefs(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteAllGeoRefs(plotID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
