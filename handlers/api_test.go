package handlers_test

// JSON API tests — covers every endpoint used by the Android client plus
// the JSON response path for all other routes that support Accept:
// application/json.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func postJSON(target string, v any) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, jsonBody(v))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	return r
}

func putJSON(target string, v any) *http.Request {
	r := httptest.NewRequest(http.MethodPut, target, jsonBody(v))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	return r
}

func getJSON(target string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.Header.Set("Accept", "application/json")
	return r
}

func deleteJSON(target string) *http.Request {
	r := httptest.NewRequest(http.MethodDelete, target, nil)
	r.Header.Set("Accept", "application/json")
	return r
}

func assertJSON(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// decodeJSON unmarshals the response body into v and fails on error.
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, rr.Body.String())
	}
}

// ── GET /plots ────────────────────────────────────────────────

func TestListPlotsJSON(t *testing.T) {
	h, d := newHandler(t)
	mustCreatePlot(t, d)

	r := getJSON("/plots")
	rr := httptest.NewRecorder()
	h.ListPlots(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var plots []map[string]any
	decodeJSON(t, rr, &plots)
	if len(plots) != 1 {
		t.Fatalf("expected 1 plot, got %d", len(plots))
	}
	if plots[0]["name"] != "Test Plot" {
		t.Errorf("plot name = %v, want Test Plot", plots[0]["name"])
	}
	if _, ok := plots[0]["image_url"]; !ok {
		t.Error("plot JSON should include image_url")
	}
	if _, ok := plots[0]["image_path"]; ok {
		t.Error("plot JSON should not expose image_path")
	}
}

func TestListPlotsJSON_Empty(t *testing.T) {
	h, _ := newHandler(t)

	r := getJSON("/plots")
	rr := httptest.NewRecorder()
	h.ListPlots(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "[]")
}

// ── GET /plots/{id} ───────────────────────────────────────────

func TestViewPlotJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	mustCreateMarker(t, d, plotID)

	r := getJSON(fmt.Sprintf("/plots/%d", plotID))
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.ViewPlot(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var body map[string]any
	decodeJSON(t, rr, &body)
	if body["plot"] == nil {
		t.Error("response should include plot")
	}
	if body["markers"] == nil {
		t.Error("response should include markers")
	}
	if body["categories"] == nil {
		t.Error("response should include categories")
	}
	if body["layers"] == nil {
		t.Error("response should include layers")
	}
}

// ── GET /plots/{id}/markers (Android: listMarkers) ────────────

func TestListPlotMarkersJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	mustCreateMarker(t, d, plotID)
	mustCreateMarker(t, d, plotID)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/plots/%d/markers", plotID), nil)
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.ListPlotMarkers(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var markers []map[string]any
	decodeJSON(t, rr, &markers)
	if len(markers) != 2 {
		t.Errorf("expected 2 markers, got %d", len(markers))
	}
}

func TestListPlotMarkersJSON_Empty(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/plots/%d/markers", plotID), nil)
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.ListPlotMarkers(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "[]")
}

func TestListPlotMarkersJSON_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/plots/bad/markers", nil), "id", "bad")
	rr := httptest.NewRecorder()
	h.ListPlotMarkers(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── POST /plots/{id}/markers (Android: createMarker) ─────────

func TestCreateMarkerJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := postJSON(fmt.Sprintf("/plots/%d/markers", plotID), map[string]any{
		"shape":  "circle",
		"coords": map[string]any{"x": 0.5, "y": 0.5, "r": 0.05},
		"label":  "Tomato",
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreateMarker(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "Tomato")
}

func TestCreateMarkerJSON_MissingShape(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := postJSON(fmt.Sprintf("/plots/%d/markers", plotID), map[string]any{
		"coords": map[string]any{"x": 0.5, "y": 0.5},
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreateMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateMarkerJSON_WithCategory(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	catID, _ := d.CreateCategory("Herbs", "#228B22", "plant")

	r := postJSON(fmt.Sprintf("/plots/%d/markers", plotID), map[string]any{
		"shape":       "circle",
		"coords":      map[string]any{"x": 0.3, "y": 0.3, "r": 0.04},
		"label":       "Basil",
		"category_id": catID,
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreateMarker(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertContains(t, rr.Body.String(), "Basil")
}

// ── GET /markers/{id} (Android: getMarker) ───────────────────

func TestGetMarkerJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := getJSON(fmt.Sprintf("/markers/%d", markerID))
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.ViewMarker(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var body map[string]any
	decodeJSON(t, rr, &body)
	if body["marker"] == nil {
		t.Error("response should include marker")
	}
}

// ── PUT /markers/{id} (Android: updateMarker) ────────────────

func TestUpdateMarkerJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := putJSON(fmt.Sprintf("/markers/%d", markerID), map[string]any{
		"label":        "Updated via JSON",
		"planted_date": "2026-03-01",
		"end_date":     "2026-10-01",
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.UpdateMarker(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "Updated via JSON")
}

func TestUpdateMarkerJSON_WithCategory(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)
	catID, _ := d.CreateCategory("Fruit", "#FF6347", "plant")

	r := putJSON(fmt.Sprintf("/markers/%d", markerID), map[string]any{
		"label":       "Cherry Tomato",
		"category_id": catID,
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.UpdateMarker(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Cherry Tomato")
}

// ── DELETE /markers/{id} (Android: deleteMarker) ─────────────

func TestDeleteMarkerJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := deleteJSON(fmt.Sprintf("/markers/%d", markerID))
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.DeleteMarker(rr, r)

	assertStatus(t, rr.Code, http.StatusNoContent)
}

// ── POST /markers/{id}/transplants (Android: transplantMarker) ──

func TestCreateTransplantJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := postJSON(fmt.Sprintf("/markers/%d/transplants", markerID), map[string]any{
		"coords": map[string]any{"x": 0.7, "y": 0.7, "r": 0.05},
		"date":   "2026-04-01",
		"notes":  "Moved to sunnier spot",
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateTransplant(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertJSON(t, rr)
}

func TestCreateTransplantJSON_MissingCoords(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := postJSON(fmt.Sprintf("/markers/%d/transplants", markerID), map[string]any{
		"notes": "no coords",
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateTransplant(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── POST /markers/{id}/entries (Android: createEntry) ────────

func TestCreateEntryJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := postJSON(fmt.Sprintf("/markers/%d/entries", markerID), map[string]any{
		"note": "Leaves look healthy",
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateEntry(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertJSON(t, rr)
}

func TestCreateEntryJSON_WithDate(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := postJSON(fmt.Sprintf("/markers/%d/entries", markerID), map[string]any{
		"note": "Flowering",
		"date": "2026-05-15",
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateEntry(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertContains(t, rr.Body.String(), "2026-05-15")
}

// ── GET /categories (Android: listCategories) ────────────────

func TestListCategoriesJSON(t *testing.T) {
	h, _ := newHandler(t)

	r := getJSON("/categories")
	rr := httptest.NewRecorder()
	h.ListCategories(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var cats []map[string]any
	decodeJSON(t, rr, &cats)
	// DB seeds some categories (e.g. "Tree")
	if len(cats) == 0 {
		t.Error("expected at least one seeded category")
	}
}

// ── POST /categories (Android: createCategory) ───────────────

func TestCreateCategoryJSON(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/categories", map[string]any{
		"name":  "Cucumber",
		"color": "#66BB6A",
		"type":  "plant",
	})
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "Cucumber")
}

func TestCreateCategoryJSON_DefaultColor(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/categories", map[string]any{"name": "Squash"})
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertContains(t, rr.Body.String(), "#64748b")
}

func TestCreateCategoryJSON_MissingName(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/categories", map[string]any{"color": "#aaa"})
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateCategoryJSON_InvalidBody(t *testing.T) {
	h, _ := newHandler(t)

	r := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── PUT /categories/{id} (Android: updateCategory) ───────────

func TestUpdateCategoryJSON(t *testing.T) {
	h, d := newHandler(t)
	catID, _ := d.CreateCategory("OldCat", "#000", "other")

	r := putJSON(fmt.Sprintf("/categories/%d", catID), map[string]any{
		"name":  "NewCat",
		"color": "#FFFFFF",
		"type":  "plant",
	})
	r = withParam(r, "id", fmt.Sprint(catID))
	rr := httptest.NewRecorder()
	h.UpdateCategory(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "NewCat")
}

func TestUpdateCategoryJSON_MissingName(t *testing.T) {
	h, d := newHandler(t)
	catID, _ := d.CreateCategory("Cat", "#000", "other")

	r := putJSON(fmt.Sprintf("/categories/%d", catID), map[string]any{"color": "#aaa"})
	r = withParam(r, "id", fmt.Sprint(catID))
	rr := httptest.NewRecorder()
	h.UpdateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── DELETE /categories/{id} (Android: deleteCategory) ────────

func TestDeleteCategoryJSON(t *testing.T) {
	h, d := newHandler(t)
	catID, _ := d.CreateCategory("ToDelete", "#aaa", "other")

	r := deleteJSON(fmt.Sprintf("/categories/%d", catID))
	r = withParam(r, "id", fmt.Sprint(catID))
	rr := httptest.NewRecorder()
	h.DeleteCategory(rr, r)

	assertStatus(t, rr.Code, http.StatusNoContent)
}

// ── GET /layers (Android: listLayers) ────────────────────────

func TestListLayersJSON(t *testing.T) {
	h, _ := newHandler(t)

	r := getJSON("/layers")
	rr := httptest.NewRecorder()
	h.ListLayers(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var layers []map[string]any
	decodeJSON(t, rr, &layers)
	if len(layers) == 0 {
		t.Error("expected at least one seeded layer")
	}
}

// ── POST /layers (Android: createLayer) ──────────────────────

func TestCreateLayerJSON(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/layers", map[string]any{
		"name":  "Mulch",
		"color": "#8B4513",
	})
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "Mulch")
}

func TestCreateLayerJSON_DefaultColor(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/layers", map[string]any{"name": "NoColor"})
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)

	assertStatus(t, rr.Code, http.StatusCreated)
	assertContains(t, rr.Body.String(), "#64748b")
}

func TestCreateLayerJSON_MissingName(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/layers", map[string]any{"color": "#aaa"})
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateLayerJSON_InvalidBody(t *testing.T) {
	h, _ := newHandler(t)

	r := httptest.NewRequest(http.MethodPost, "/layers", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── PUT /layers/{id} (Android: updateLayer) ──────────────────

func TestUpdateLayerJSON(t *testing.T) {
	h, d := newHandler(t)
	layerID, _ := d.CreateLayer("OldLayer", "#000")

	r := putJSON(fmt.Sprintf("/layers/%d", layerID), map[string]any{
		"name":  "NewLayer",
		"color": "#FFFFFF",
	})
	r = withParam(r, "id", fmt.Sprint(layerID))
	rr := httptest.NewRecorder()
	h.UpdateLayer(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)
	assertContains(t, rr.Body.String(), "NewLayer")
}

func TestUpdateLayerJSON_InvalidBody(t *testing.T) {
	h, d := newHandler(t)
	layerID, _ := d.CreateLayer("Layer", "#000")

	r := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/layers/%d", layerID), strings.NewReader("bad"))
	r.Header.Set("Content-Type", "application/json")
	r = withParam(r, "id", fmt.Sprint(layerID))
	rr := httptest.NewRecorder()
	h.UpdateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── DELETE /layers/{id} (Android: deleteLayer) ───────────────

func TestDeleteLayerJSON(t *testing.T) {
	h, d := newHandler(t)
	layerID, _ := d.CreateLayer("ToDelete", "#aaa")

	r := deleteJSON(fmt.Sprintf("/layers/%d", layerID))
	r = withParam(r, "id", fmt.Sprint(layerID))
	rr := httptest.NewRecorder()
	h.DeleteLayer(rr, r)

	assertStatus(t, rr.Code, http.StatusNoContent)
}

// ── GET /plant-groups/{id} (Android: getPlantGroup) ──────────

func TestGetPlantGroupJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	m1 := mustCreateMarker(t, d, plotID)
	m2 := mustCreateMarker(t, d, plotID)
	gID, _ := d.CreatePlantGroup(plotID, "Berry Patch")
	d.SetMarkersGroup(gID, []int64{m1, m2})

	r := getJSON(fmt.Sprintf("/plant-groups/%d", gID))
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.ViewPlantGroup(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var body map[string]any
	decodeJSON(t, rr, &body)
	if body["group"] == nil {
		t.Error("response should include group")
	}
	members, _ := body["members"].([]any)
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

// ── POST /plant-groups (Android: createPlantGroup) ───────────

func TestCreatePlantGroupFromBodyJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	m1 := mustCreateMarker(t, d, plotID)
	m2 := mustCreateMarker(t, d, plotID)

	r := postJSON("/plant-groups", map[string]any{
		"name":       "Salad Corner",
		"marker_ids": []int64{m1, m2},
	})
	rr := httptest.NewRecorder()
	h.CreatePlantGroupFromBody(rr, r)

	assertStatus(t, rr.Code, http.StatusOK)
	assertJSON(t, rr)

	var body map[string]any
	decodeJSON(t, rr, &body)
	grp, _ := body["group"].(map[string]any)
	if grp["name"] != "Salad Corner" {
		t.Errorf("group name = %v, want Salad Corner", grp["name"])
	}
}

func TestCreatePlantGroupFromBodyJSON_MissingName(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	m1 := mustCreateMarker(t, d, plotID)

	r := postJSON("/plant-groups", map[string]any{"marker_ids": []int64{m1}})
	rr := httptest.NewRecorder()
	h.CreatePlantGroupFromBody(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreatePlantGroupFromBodyJSON_MissingMarkerIDs(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/plant-groups", map[string]any{"name": "Empty"})
	rr := httptest.NewRecorder()
	h.CreatePlantGroupFromBody(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreatePlantGroupFromBodyJSON_MarkerNotFound(t *testing.T) {
	h, _ := newHandler(t)

	r := postJSON("/plant-groups", map[string]any{
		"name":       "Ghost Group",
		"marker_ids": []int64{99999},
	})
	rr := httptest.NewRecorder()
	h.CreatePlantGroupFromBody(rr, r)
	assertStatus(t, rr.Code, http.StatusNotFound)
}

func TestCreatePlantGroupFromBodyJSON_InvalidBody(t *testing.T) {
	h, _ := newHandler(t)

	r := httptest.NewRequest(http.MethodPost, "/plant-groups", strings.NewReader("bad json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreatePlantGroupFromBody(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── DELETE /plots/{id} JSON ───────────────────────────────────

func TestDeletePlotJSON(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := deleteJSON(fmt.Sprintf("/plots/%d", plotID))
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.DeletePlot(rr, r)

	assertStatus(t, rr.Code, http.StatusNoContent)
}
