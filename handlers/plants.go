package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"plotter/db"
)

type plantGroupResponse struct {
	Group    *db.PlantGroup    `json:"group"`
	Members  []db.Marker       `json:"members"`
	Harvests []db.GroupHarvest  `json:"harvests"`
}

func (h *Handler) UpsertTaxonomy(w http.ResponseWriter, r *http.Request) {
	markerID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	genus := strings.TrimSpace(r.FormValue("genus"))
	species := strings.TrimSpace(r.FormValue("species"))
	cultivar := strings.TrimSpace(r.FormValue("cultivar"))

	tax, err := h.db.UpsertTaxonomy(markerID, genus, species, cultivar)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		writeJSON(w, http.StatusOK, tax)
		return
	}
	h.renderPartial(w, r, "taxonomy", map[string]interface{}{
		"Taxonomy": tax,
		"MarkerID": markerID,
	})
}

func (h *Handler) CreateHarvest(w http.ResponseWriter, r *http.Request) {
	markerID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	date := strings.TrimSpace(r.FormValue("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	weightGrams, err := strconv.ParseFloat(r.FormValue("weight_grams"), 64)
	if err != nil || weightGrams <= 0 {
		http.Error(w, "valid weight_grams required", http.StatusBadRequest)
		return
	}
	notes := strings.TrimSpace(r.FormValue("notes"))

	harvestID, err := h.db.CreateHarvest(markerID, date, weightGrams, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.isJSON(r) {
		harvest, err := h.db.GetHarvest(harvestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, harvest)
		return
	}

	harvests, err := h.db.GetHarvests(markerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "harvest_list", map[string]interface{}{
		"Harvests": harvests,
		"MarkerID": markerID,
	})
}

func (h *Handler) DeleteHarvest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	markerID, err := h.db.DeleteHarvest(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	harvests, _ := h.db.GetHarvests(markerID)
	h.renderPartial(w, r, "harvest_list", map[string]interface{}{
		"Harvests": harvests,
		"MarkerID": markerID,
	})
}

// ── Plant Groups ──────────────────────────────────────────────

func (h *Handler) CreatePlantGroup(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	// Parse comma-separated or multi-value marker_ids
	var markerIDs []int64
	for _, raw := range strings.Split(r.FormValue("marker_ids"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			markerIDs = append(markerIDs, id)
		}
	}

	groupID, err := h.db.CreatePlantGroup(plotID, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(markerIDs) > 0 {
		h.db.SetMarkersGroup(groupID, markerIDs)
	}

	h.loadAndRenderGroup(w, r, groupID)
}

// CreatePlantGroupFromBody handles POST /plant-groups with a JSON body.
// Used by the Android client which sends {name, marker_ids} without a plot ID
// in the URL; the plot is derived from the first marker.
func (h *Handler) CreatePlantGroupFromBody(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		MarkerIDs []int64 `json:"marker_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if len(req.MarkerIDs) == 0 {
		http.Error(w, "marker_ids required", http.StatusBadRequest)
		return
	}
	first, err := h.db.GetMarker(req.MarkerIDs[0])
	if err != nil {
		http.Error(w, "marker not found", http.StatusNotFound)
		return
	}
	groupID, err := h.db.CreatePlantGroup(first.PlotID, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.db.SetMarkersGroup(groupID, req.MarkerIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.loadAndRenderGroup(w, r, groupID)
}

func (h *Handler) ViewPlantGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.loadAndRenderGroup(w, r, groupID)
}

func (h *Handler) UpdatePlantGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if err := h.db.UpdatePlantGroup(groupID, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.loadAndRenderGroup(w, r, groupID)
}

func (h *Handler) DeletePlantGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeletePlantGroup(groupID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<p class="hint">Group deleted. Click a marker to get started.</p>`))
}

func (h *Handler) AddGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	markerID, err := strconv.ParseInt(r.FormValue("marker_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid marker_id", http.StatusBadRequest)
		return
	}
	if err := h.db.SetMarkersGroup(groupID, []int64{markerID}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.loadAndRenderGroup(w, r, groupID)
}

func (h *Handler) RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	markerID, err := strconv.ParseInt(chi.URLParam(r, "mid"), 10, 64)
	if err != nil {
		http.Error(w, "invalid marker id", http.StatusBadRequest)
		return
	}
	if err := h.db.RemoveGroupMember(markerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.loadAndRenderGroup(w, r, groupID)
}

func (h *Handler) CreateGroupHarvest(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	date := strings.TrimSpace(r.FormValue("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	weightGrams, err := strconv.ParseFloat(r.FormValue("weight_grams"), 64)
	if err != nil || weightGrams <= 0 {
		http.Error(w, "valid weight_grams required", http.StatusBadRequest)
		return
	}
	notes := strings.TrimSpace(r.FormValue("notes"))

	harvestID, err := h.db.CreateGroupHarvest(groupID, date, weightGrams, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		harvest, err := h.db.GetGroupHarvest(harvestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, harvest)
		return
	}
	harvests, _ := h.db.GetGroupHarvests(groupID)
	h.renderPartial(w, r, "group_harvest_list", map[string]interface{}{
		"Harvests": harvests,
		"GroupID":  groupID,
	})
}

func (h *Handler) DeleteGroupHarvest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	groupID, err := h.db.DeleteGroupHarvest(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.isJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	harvests, _ := h.db.GetGroupHarvests(groupID)
	h.renderPartial(w, r, "group_harvest_list", map[string]interface{}{
		"Harvests": harvests,
		"GroupID":  groupID,
	})
}

func (h *Handler) loadAndRenderGroup(w http.ResponseWriter, r *http.Request, groupID int64) {
	group, err := h.db.GetPlantGroup(groupID)
	if err != nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	members, _ := h.db.GetGroupMarkers(groupID)
	harvests, _ := h.db.GetGroupHarvests(groupID)
	if h.isJSON(r) {
		if members == nil {
			members = []db.Marker{}
		}
		if harvests == nil {
			harvests = []db.GroupHarvest{}
		}
		writeJSON(w, http.StatusOK, plantGroupResponse{Group: group, Members: members, Harvests: harvests})
		return
	}
	h.renderPartial(w, r, "plant_group", map[string]interface{}{
		"Group":    group,
		"Members":  members,
		"Harvests": harvests,
		"Today":    time.Now().Format("2006-01-02"),
	})
}

