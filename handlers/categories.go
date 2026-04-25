package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"plotter/db"
)

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		if cats == nil {
			cats = []db.Category{}
		}
		writeJSON(w, http.StatusOK, cats)
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

	catID, err := h.db.CreateCategory(name, color, catType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.isJSON(r) {
		cat, err := h.db.GetCategory(catID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, cat)
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

	if h.isJSON(r) {
		cat, err := h.db.GetCategory(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, cat)
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

	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cats, err := h.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "category_list", cats)
}
