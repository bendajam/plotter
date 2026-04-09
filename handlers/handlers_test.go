package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"plotter/db"
	"plotter/handlers"
)

// ── Test infrastructure ───────────────────────────────────────

func TestMain(m *testing.M) {
	// Handler.New resolves templates relative to its source file, but file
	// upload paths (uploads/plots, uploads/markers) are relative to CWD.
	// Move CWD to the project root so both work.
	if err := os.Chdir(".."); err != nil {
		fmt.Fprintf(os.Stderr, "chdir to project root: %v\n", err)
		os.Exit(1)
	}
	os.MkdirAll("uploads/plots", 0755)
	os.MkdirAll("uploads/markers", 0755)
	os.Exit(m.Run())
}

func newHandler(t *testing.T) (*handlers.Handler, *db.DB) {
	t.Helper()
	database, err := db.Init(":memory:")
	if err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	h, err := handlers.New(database)
	if err != nil {
		t.Fatalf("handlers.New: %v", err)
	}
	return h, database
}

// withParam attaches chi URL params to a request.
func withParam(r *http.Request, pairs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(pairs); i += 2 {
		rctx.URLParams.Add(pairs[i], pairs[i+1])
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// postForm builds a POST request with application/x-www-form-urlencoded body.
func postForm(target string, vals url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// putForm builds a PUT request with application/x-www-form-urlencoded body.
func putForm(target string, vals url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPut, target, strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// deleteReq builds a DELETE request.
func deleteReq(target string) *http.Request {
	return httptest.NewRequest(http.MethodDelete, target, nil)
}

// mustCreatePlot creates a plot in the DB and returns its ID.
func mustCreatePlot(t *testing.T, d *db.DB) int64 {
	t.Helper()
	id, err := d.CreatePlot("Test Plot", "1 Garden Lane", "plots/test.jpg")
	if err != nil {
		t.Fatalf("CreatePlot: %v", err)
	}
	return id
}

// mustCreateMarker creates a marker and returns its ID.
func mustCreateMarker(t *testing.T, d *db.DB, plotID int64) int64 {
	t.Helper()
	id, err := d.CreateMarker(plotID, "circle", "[0.5,0.5,0.05]", "Test Plant", "", "", nil, nil)
	if err != nil {
		t.Fatalf("CreateMarker: %v", err)
	}
	return id
}

func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}

func assertContains(t *testing.T, body, substr string) {
	t.Helper()
	if !strings.Contains(body, substr) {
		t.Errorf("body does not contain %q\nbody: %s", substr, body)
	}
}

func assertHeader(t *testing.T, rr *httptest.ResponseRecorder, key, want string) {
	t.Helper()
	if got := rr.Header().Get(key); got != want {
		t.Errorf("header %q = %q, want %q", key, got, want)
	}
}

// ── Plots ─────────────────────────────────────────────────────

func TestListPlots(t *testing.T) {
	h, _ := newHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ListPlots(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Garden Plotter")
}

func TestNewPlot(t *testing.T) {
	h, _ := newHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/plots/new", nil)
	rr := httptest.NewRecorder()
	h.NewPlot(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestCreatePlot_MissingFields(t *testing.T) {
	h, _ := newHandler(t)

	tests := []struct {
		name string
		vals url.Values
	}{
		{"missing name", url.Values{"address": {"x"}}},
		{"missing address", url.Values{"name": {"x"}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			w := multipart.NewWriter(body)
			for k, vs := range tc.vals {
				w.WriteField(k, vs[0])
			}
			w.Close()
			r := httptest.NewRequest(http.MethodPost, "/plots", body)
			r.Header.Set("Content-Type", w.FormDataContentType())
			rr := httptest.NewRecorder()
			h.CreatePlot(rr, r)
			assertStatus(t, rr.Code, http.StatusBadRequest)
		})
	}
}

func TestCreatePlot_BadImageExtension(t *testing.T) {
	h, _ := newHandler(t)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("name", "My Plot")
	w.WriteField("address", "123 Main")
	fw, _ := w.CreateFormFile("image", "photo.bmp")
	fw.Write([]byte("fake image data"))
	w.Close()

	r := httptest.NewRequest(http.MethodPost, "/plots", body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	h.CreatePlot(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreatePlot_Success(t *testing.T) {
	h, _ := newHandler(t)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("name", "My Garden")
	w.WriteField("address", "456 Oak St")
	fw, _ := w.CreateFormFile("image", "photo.jpg")
	fw.Write([]byte("fake jpeg data"))
	w.Close()

	r := httptest.NewRequest(http.MethodPost, "/plots", body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	h.CreatePlot(rr, r)
	assertStatus(t, rr.Code, http.StatusSeeOther)
	if loc := rr.Header().Get("Location"); !strings.HasPrefix(loc, "/plots/") {
		t.Errorf("Location = %q, want /plots/<id>", loc)
	}
}

func TestViewPlot_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/plots/%d", plotID), nil)
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.ViewPlot(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Test Plot")
}

func TestViewPlot_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/plots/9999", nil), "id", "9999")
	rr := httptest.NewRecorder()
	h.ViewPlot(rr, r)
	assertStatus(t, rr.Code, http.StatusNotFound)
}

func TestViewPlot_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/plots/abc", nil), "id", "abc")
	rr := httptest.NewRecorder()
	h.ViewPlot(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeletePlot_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := withParam(deleteReq(fmt.Sprintf("/plots/%d", plotID)), "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.DeletePlot(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertHeader(t, rr, "HX-Redirect", "/")
}

func TestDeletePlot_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/plots/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeletePlot(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Markers ───────────────────────────────────────────────────

func TestCreateMarker_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := postForm("/plots/1/markers", url.Values{
		"shape":  {"circle"},
		"coords": {"[0.5,0.5,0.05]"},
		"label":  {"Rose"},
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreateMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	if rr.Header().Get("HX-Trigger") == "" {
		t.Error("HX-Trigger header should be set after marker creation")
	}
	assertContains(t, rr.Body.String(), "Rose")
}

func TestCreateMarker_MissingFields(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	tests := []struct {
		name string
		vals url.Values
	}{
		{"missing shape", url.Values{"coords": {"[0.5,0.5]"}}},
		{"missing coords", url.Values{"shape": {"circle"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := postForm("/markers", tc.vals)
			r = withParam(r, "id", fmt.Sprint(plotID))
			rr := httptest.NewRecorder()
			h.CreateMarker(rr, r)
			assertStatus(t, rr.Code, http.StatusBadRequest)
		})
	}
}

func TestCreateMarker_BadPlotID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/plots/bad/markers", url.Values{"shape": {"circle"}, "coords": {"[0.5,0.5]"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.CreateMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestViewMarker_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/markers/%d", markerID), nil)
	r = withParam(r, "id", fmt.Sprint(markerID))
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	h.ViewMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Test Plant")
}

func TestViewMarker_FullPage(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/markers/%d", markerID), nil)
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.ViewMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Test Plant")
}

func TestViewMarker_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/markers/9999", nil), "id", "9999")
	rr := httptest.NewRecorder()
	h.ViewMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusNotFound)
}

func TestUpdateMarker_Sidebar(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := putForm(fmt.Sprintf("/markers/%d", markerID), url.Values{
		"label":    {"Updated Label"},
		"end_date": {"2026-12-01"},
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	h.UpdateMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Updated Label")
}

func TestUpdateMarker_FullPage(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := putForm(fmt.Sprintf("/markers/%d", markerID), url.Values{
		"label":   {"Updated"},
		"context": {"full-page"},
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.UpdateMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	if got := rr.Header().Get("HX-Redirect"); !strings.HasPrefix(got, "/markers/") {
		t.Errorf("HX-Redirect = %q, want /markers/<id>", got)
	}
}

func TestUpdateMarker_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := putForm("/markers/bad", url.Values{"label": {"x"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.UpdateMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteMarker_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := withParam(deleteReq(fmt.Sprintf("/markers/%d", markerID)), "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.DeleteMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestDeleteMarker_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/markers/9999"), "id", "9999")
	rr := httptest.NewRecorder()
	h.DeleteMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusNotFound)
}

func TestDeleteMarker_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/markers/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteMarker(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestBulkUpdateMarkers_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	m1 := mustCreateMarker(t, d, plotID)
	m2 := mustCreateMarker(t, d, plotID)

	r := postForm("/markers/bulk", url.Values{
		"marker_ids":  {fmt.Sprintf("%d,%d", m1, m2)},
		"end_date":    {"2026-11-01"},
		"category_id": {""},
		"layer_id":    {""},
	})
	rr := httptest.NewRecorder()
	h.BulkUpdateMarkers(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertHeader(t, rr, "HX-Refresh", "true")
}

func TestBulkUpdateMarkers_NoIDs(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/markers/bulk", url.Values{"marker_ids": {""}})
	rr := httptest.NewRecorder()
	h.BulkUpdateMarkers(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Entries ───────────────────────────────────────────────────

func TestCreateEntry_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := postForm(fmt.Sprintf("/markers/%d/entries", markerID), url.Values{
		"date":  {"2025-06-01"},
		"notes": {"Looking great!"},
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateEntry(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Looking great!")
}

func TestCreateEntry_DefaultsDate(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	// No date supplied — handler should use today's date without error.
	r := postForm(fmt.Sprintf("/markers/%d/entries", markerID), url.Values{
		"notes": {"no date supplied"},
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateEntry(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestCreateEntry_BadMarkerID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/markers/bad/entries", url.Values{"notes": {"x"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.CreateEntry(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteEntryImage_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/entry-images/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteEntryImage(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteEntryImage_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)
	entryID, _ := d.CreateEntry(markerID, "2025-06-01", "notes")
	imgID, _ := d.AddEntryImage(entryID, "markers/test.jpg", "caption")

	r := withParam(deleteReq(fmt.Sprintf("/entry-images/%d", imgID)), "id", fmt.Sprint(imgID))
	rr := httptest.NewRecorder()
	h.DeleteEntryImage(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestAddEntryImages_BadID(t *testing.T) {
	h, _ := newHandler(t)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.Close()
	r := httptest.NewRequest(http.MethodPost, "/entries/bad/images", body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.AddEntryImages(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestAddEntryImages_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)
	entryID, _ := d.CreateEntry(markerID, "2025-06-01", "notes")

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("caption", "My photo")
	fw, _ := w.CreateFormFile("images", "photo.jpg")
	io.WriteString(fw, "fake jpeg bytes")
	w.Close()

	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/entries/%d/images", entryID), body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r = withParam(r, "id", fmt.Sprint(entryID))
	rr := httptest.NewRecorder()
	h.AddEntryImages(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

// ── Categories ────────────────────────────────────────────────

func TestListCategories(t *testing.T) {
	h, _ := newHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rr := httptest.NewRecorder()
	h.ListCategories(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	// Seeded categories should appear
	assertContains(t, rr.Body.String(), "Tree")
}

func TestCreateCategory_Success(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/categories", url.Values{
		"name":  {"Moss"},
		"color": {"#228822"},
		"type":  {"plant"},
	})
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Moss")
}

func TestCreateCategory_DefaultColor(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/categories", url.Values{"name": {"NoColor"}})
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "#64748b")
}

func TestCreateCategory_MissingName(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/categories", url.Values{"color": {"#aaa"}})
	rr := httptest.NewRecorder()
	h.CreateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestUpdateCategory_Success(t *testing.T) {
	h, d := newHandler(t)
	catID, _ := d.CreateCategory("OldName", "#000", "other")

	r := putForm(fmt.Sprintf("/categories/%d", catID), url.Values{
		"name":  {"NewName"},
		"color": {"#ffffff"},
		"type":  {"plant"},
	})
	r = withParam(r, "id", fmt.Sprint(catID))
	rr := httptest.NewRecorder()
	h.UpdateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "NewName")
}

func TestUpdateCategory_MissingName(t *testing.T) {
	h, d := newHandler(t)
	catID, _ := d.CreateCategory("Name", "#000", "other")

	r := putForm(fmt.Sprintf("/categories/%d", catID), url.Values{"color": {"#aaa"}})
	r = withParam(r, "id", fmt.Sprint(catID))
	rr := httptest.NewRecorder()
	h.UpdateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestUpdateCategory_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := putForm("/categories/bad", url.Values{"name": {"x"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.UpdateCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteCategory_Success(t *testing.T) {
	h, d := newHandler(t)
	catID, _ := d.CreateCategory("Temporary", "#aaa", "other")

	r := withParam(deleteReq(fmt.Sprintf("/categories/%d", catID)), "id", fmt.Sprint(catID))
	rr := httptest.NewRecorder()
	h.DeleteCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	if strings.Contains(rr.Body.String(), "Temporary") {
		t.Error("deleted category should not appear in response")
	}
}

func TestDeleteCategory_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/categories/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteCategory(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Layers ────────────────────────────────────────────────────

func TestListLayers(t *testing.T) {
	h, _ := newHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/layers", nil)
	rr := httptest.NewRecorder()
	h.ListLayers(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Water")
}

func TestCreateLayer_Success(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/layers", url.Values{
		"name":  {"Compost"},
		"color": {"#8B4513"},
	})
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Compost")
}

func TestCreateLayer_DefaultColor(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/layers", url.Values{"name": {"NoColor"}})
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "#64748b")
}

func TestCreateLayer_MissingName(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/layers", url.Values{"color": {"#aaa"}})
	rr := httptest.NewRecorder()
	h.CreateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestUpdateLayer_Success(t *testing.T) {
	h, d := newHandler(t)
	layerID, _ := d.CreateLayer("Old", "#000")

	r := putForm(fmt.Sprintf("/layers/%d", layerID), url.Values{
		"name":  {"New"},
		"color": {"#ffffff"},
	})
	r = withParam(r, "id", fmt.Sprint(layerID))
	rr := httptest.NewRecorder()
	h.UpdateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "New")
}

func TestUpdateLayer_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := putForm("/layers/bad", url.Values{"name": {"x"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.UpdateLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteLayer_Success(t *testing.T) {
	h, d := newHandler(t)
	layerID, _ := d.CreateLayer("Temp", "#aaa")

	r := withParam(deleteReq(fmt.Sprintf("/layers/%d", layerID)), "id", fmt.Sprint(layerID))
	rr := httptest.NewRecorder()
	h.DeleteLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	if strings.Contains(rr.Body.String(), "Temp") {
		t.Error("deleted layer should not appear in response")
	}
}

func TestDeleteLayer_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/layers/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteLayer(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Taxonomy ──────────────────────────────────────────────────

func TestUpsertTaxonomy_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := putForm(fmt.Sprintf("/markers/%d/taxonomy", markerID), url.Values{
		"genus":   {"Solanum"},
		"species": {"lycopersicum"},
		"cultivar": {"Cherry"},
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.UpsertTaxonomy(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Solanum")
}

func TestUpsertTaxonomy_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := putForm("/markers/bad/taxonomy", url.Values{"genus": {"x"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.UpsertTaxonomy(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Harvests ──────────────────────────────────────────────────

func TestCreateHarvest_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	r := postForm(fmt.Sprintf("/markers/%d/harvests", markerID), url.Values{
		"date":         {"2025-09-01"},
		"weight_grams": {"250"},
		"notes":        {"First batch"},
	})
	r = withParam(r, "id", fmt.Sprint(markerID))
	rr := httptest.NewRecorder()
	h.CreateHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "250")
}

func TestCreateHarvest_InvalidWeight(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)

	tests := []struct {
		name   string
		weight string
	}{
		{"zero weight", "0"},
		{"negative weight", "-10"},
		{"non-numeric", "abc"},
		{"missing", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := postForm(fmt.Sprintf("/markers/%d/harvests", markerID), url.Values{
				"date":         {"2025-09-01"},
				"weight_grams": {tc.weight},
			})
			r = withParam(r, "id", fmt.Sprint(markerID))
			rr := httptest.NewRecorder()
			h.CreateHarvest(rr, r)
			assertStatus(t, rr.Code, http.StatusBadRequest)
		})
	}
}

func TestCreateHarvest_BadMarkerID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/markers/bad/harvests", url.Values{"weight_grams": {"100"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.CreateHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteHarvest_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	markerID := mustCreateMarker(t, d, plotID)
	hID, _ := d.CreateHarvest(markerID, "2025-09-01", 100.0, "")

	r := withParam(deleteReq(fmt.Sprintf("/harvests/%d", hID)), "id", fmt.Sprint(hID))
	rr := httptest.NewRecorder()
	h.DeleteHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestDeleteHarvest_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/harvests/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Plant Groups ──────────────────────────────────────────────

func TestCreatePlantGroup_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	m1 := mustCreateMarker(t, d, plotID)
	m2 := mustCreateMarker(t, d, plotID)

	r := postForm(fmt.Sprintf("/plots/%d/plant-groups", plotID), url.Values{
		"name":       {"Herb Corner"},
		"marker_ids": {fmt.Sprintf("%d,%d", m1, m2)},
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreatePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Herb Corner")
}

func TestCreatePlantGroup_MissingName(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := postForm(fmt.Sprintf("/plots/%d/plant-groups", plotID), url.Values{
		"marker_ids": {"1,2"},
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreatePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreatePlantGroup_BadPlotID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/plots/bad/plant-groups", url.Values{"name": {"x"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.CreatePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestViewPlantGroup_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Veggie Bed")

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/plant-groups/%d", gID), nil)
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.ViewPlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Veggie Bed")
}

func TestViewPlantGroup_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/plant-groups/9999", nil), "id", "9999")
	rr := httptest.NewRecorder()
	h.ViewPlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusNotFound)
}

func TestUpdatePlantGroup_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Old Name")

	r := putForm(fmt.Sprintf("/plant-groups/%d", gID), url.Values{"name": {"New Name"}})
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.UpdatePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "New Name")
}

func TestUpdatePlantGroup_MissingName(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Name")

	r := putForm(fmt.Sprintf("/plant-groups/%d", gID), url.Values{"name": {""}})
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.UpdatePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeletePlantGroup_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Temp Group")

	r := withParam(deleteReq(fmt.Sprintf("/plant-groups/%d", gID)), "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.DeletePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Group deleted")
}

func TestDeletePlantGroup_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/plant-groups/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeletePlantGroup(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestAddGroupMember_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")
	markerID := mustCreateMarker(t, d, plotID)

	r := postForm(fmt.Sprintf("/plant-groups/%d/members", gID), url.Values{
		"marker_id": {fmt.Sprint(markerID)},
	})
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.AddGroupMember(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestAddGroupMember_BadMarkerID(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")

	r := postForm(fmt.Sprintf("/plant-groups/%d/members", gID), url.Values{
		"marker_id": {"bad"},
	})
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.AddGroupMember(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestAddGroupMember_BadGroupID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/plant-groups/bad/members", url.Values{"marker_id": {"1"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.AddGroupMember(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestRemoveGroupMember_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")
	markerID := mustCreateMarker(t, d, plotID)
	d.SetMarkersGroup(gID, []int64{markerID})

	r := withParam(deleteReq(fmt.Sprintf("/plant-groups/%d/members/%d", gID, markerID)),
		"id", fmt.Sprint(gID),
		"mid", fmt.Sprint(markerID),
	)
	rr := httptest.NewRecorder()
	h.RemoveGroupMember(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestRemoveGroupMember_BadGroupID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/plant-groups/bad/members/1"), "id", "bad", "mid", "1")
	rr := httptest.NewRecorder()
	h.RemoveGroupMember(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestRemoveGroupMember_BadMemberID(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")

	r := withParam(deleteReq(fmt.Sprintf("/plant-groups/%d/members/bad", gID)),
		"id", fmt.Sprint(gID), "mid", "bad")
	rr := httptest.NewRecorder()
	h.RemoveGroupMember(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateGroupHarvest_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")

	r := postForm(fmt.Sprintf("/plant-groups/%d/harvests", gID), url.Values{
		"date":         {"2025-09-15"},
		"weight_grams": {"1500"},
		"notes":        {"Bumper crop"},
	})
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.CreateGroupHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "1500")
}

func TestCreateGroupHarvest_InvalidWeight(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")

	r := postForm(fmt.Sprintf("/plant-groups/%d/harvests", gID), url.Values{
		"date":         {"2025-09-15"},
		"weight_grams": {"0"},
	})
	r = withParam(r, "id", fmt.Sprint(gID))
	rr := httptest.NewRecorder()
	h.CreateGroupHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateGroupHarvest_BadGroupID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/plant-groups/bad/harvests", url.Values{"weight_grams": {"100"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.CreateGroupHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteGroupHarvest_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Group")
	hID, _ := d.CreateGroupHarvest(gID, "2025-09-01", 500.0, "")

	r := withParam(deleteReq(fmt.Sprintf("/group-harvests/%d", hID)), "id", fmt.Sprint(hID))
	rr := httptest.NewRecorder()
	h.DeleteGroupHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestDeleteGroupHarvest_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/group-harvests/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteGroupHarvest(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

// ── Weather ───────────────────────────────────────────────────

func TestListWeather_FullPage(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/plots/%d/weather", plotID), nil)
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.ListWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
	assertContains(t, rr.Body.String(), "Test Plot")
}

func TestListWeather_HTMXPartial(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/plots/%d/weather", plotID), nil)
	r = withParam(r, "id", fmt.Sprint(plotID))
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	h.ListWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestListWeather_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/plots/9999/weather", nil), "id", "9999")
	rr := httptest.NewRecorder()
	h.ListWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusNotFound)
}

func TestListWeather_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(httptest.NewRequest(http.MethodGet, "/plots/bad/weather", nil), "id", "bad")
	rr := httptest.NewRecorder()
	h.ListWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateWeather_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := postForm(fmt.Sprintf("/plots/%d/weather", plotID), url.Values{
		"date":        {"2025-07-15"},
		"rainfall_mm": {"8.5"},
		"temp_high_c": {"29"},
		"wind_dir":    {"SW"},
		"notes":       {"Warm and humid"},
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreateWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestCreateWeather_MissingDate(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)

	r := postForm(fmt.Sprintf("/plots/%d/weather", plotID), url.Values{
		"rainfall_mm": {"5"},
	})
	r = withParam(r, "id", fmt.Sprint(plotID))
	rr := httptest.NewRecorder()
	h.CreateWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestCreateWeather_BadPlotID(t *testing.T) {
	h, _ := newHandler(t)
	r := postForm("/plots/bad/weather", url.Values{"date": {"2025-07-15"}})
	r = withParam(r, "id", "bad")
	rr := httptest.NewRecorder()
	h.CreateWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}

func TestDeleteWeather_Success(t *testing.T) {
	h, d := newHandler(t)
	plotID := mustCreatePlot(t, d)
	wID, _ := d.CreateWeather(plotID, "2025-07-15", nil, nil, nil, nil, "", "")

	r := withParam(deleteReq(fmt.Sprintf("/weather/%d", wID)), "id", fmt.Sprint(wID))
	rr := httptest.NewRecorder()
	h.DeleteWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusOK)
}

func TestDeleteWeather_BadID(t *testing.T) {
	h, _ := newHandler(t)
	r := withParam(deleteReq("/weather/bad"), "id", "bad")
	rr := httptest.NewRecorder()
	h.DeleteWeather(rr, r)
	assertStatus(t, rr.Code, http.StatusBadRequest)
}
