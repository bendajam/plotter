package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "category_list", cats)
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	color := strings.TrimSpace(r.FormValue("color"))
	catType := r.FormValue("type")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if color == "" {
		color = "#64748b"
	}

	if _, err := h.db.CreateCategory(name, color, catType); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "category_list", cats)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	color := strings.TrimSpace(r.FormValue("color"))
	catType := r.FormValue("type")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateCategory(id, name, color, catType); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "category_list", cats)
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteCategory(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "category_list", cats)
}
