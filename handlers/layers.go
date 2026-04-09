package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListLayers(w http.ResponseWriter, r *http.Request) {
	layers, err := h.db.GetLayers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "layer_list", layers)
}

func (h *Handler) CreateLayer(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	color := strings.TrimSpace(r.FormValue("color"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if color == "" {
		color = "#64748b"
	}
	if _, err := h.db.CreateLayer(name, color); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	color := strings.TrimSpace(r.FormValue("color"))
	if err := h.db.UpdateLayer(id, name, color); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	layers, _ := h.db.GetLayers()
	h.renderPartial(w, r, "layer_list", layers)
}
