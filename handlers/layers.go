package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"plotter/db"
)

func (h *Handler) ListLayers(w http.ResponseWriter, r *http.Request) {
	layers, err := h.db.GetLayers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		if layers == nil {
			layers = []db.Layer{}
		}
		writeJSON(w, http.StatusOK, layers)
		return
	}
	h.renderPartial(w, r, "layer_list", layers)
}

func (h *Handler) CreateLayer(w http.ResponseWriter, r *http.Request) {
	var name, color string
	if isJSONBody(r) {
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		name, color = req.Name, req.Color
	} else {
		r.ParseForm()
		name = strings.TrimSpace(r.FormValue("name"))
		color = strings.TrimSpace(r.FormValue("color"))
	}
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if color == "" {
		color = "#64748b"
	}
	layerID, err := h.db.CreateLayer(name, color)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		layer, err := h.db.GetLayer(layerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, layer)
		return
	}
	layers, _ := h.db.GetLayers()
	h.renderPartial(w, r, "layer_list", layers)
}

func (h *Handler) UpdateLayer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var name, color string
	if isJSONBody(r) {
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		name, color = req.Name, req.Color
	} else {
		r.ParseForm()
		name = strings.TrimSpace(r.FormValue("name"))
		color = strings.TrimSpace(r.FormValue("color"))
	}
	if err := h.db.UpdateLayer(id, name, color); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		layer, err := h.db.GetLayer(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, layer)
		return
	}
	layers, _ := h.db.GetLayers()
	h.renderPartial(w, r, "layer_list", layers)
}

func (h *Handler) DeleteLayer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteLayer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	layers, _ := h.db.GetLayers()
	h.renderPartial(w, r, "layer_list", layers)
}
