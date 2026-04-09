package db

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// ── Plots ─────────────────────────────────────────────────────

func TestPlotCRUD(t *testing.T) {
	d := newTestDB(t)

	id, err := d.CreatePlot("Backyard", "123 Main St", "")
	if err != nil {
		t.Fatalf("CreatePlot: %v", err)
	}

	plot, err := d.GetPlot(id)
	if err != nil {
		t.Fatalf("GetPlot: %v", err)
	}
	if plot.Name != "Backyard" {
		t.Errorf("Name = %q, want %q", plot.Name, "Backyard")
	}
	if plot.Address != "123 Main St" {
		t.Errorf("Address = %q, want %q", plot.Address, "123 Main St")
	}

	plots, err := d.GetPlots()
	if err != nil {
		t.Fatalf("GetPlots: %v", err)
	}
	if len(plots) != 1 {
		t.Fatalf("GetPlots len = %d, want 1", len(plots))
	}

	if err := d.DeletePlot(id); err != nil {
		t.Fatalf("DeletePlot: %v", err)
	}

	plots, err = d.GetPlots()
	if err != nil {
		t.Fatalf("GetPlots after delete: %v", err)
	}
	if len(plots) != 0 {
		t.Errorf("GetPlots after delete len = %d, want 0", len(plots))
	}
}

func TestGetPlotNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetPlot(9999)
	if err == nil {
		t.Error("GetPlot with invalid id should return error")
	}
}

// ── Categories ────────────────────────────────────────────────

func TestCategoryCRUD(t *testing.T) {
	d := newTestDB(t)

	id, err := d.CreateCategory("Fruit Tree", "#166534", "plant")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	cats, err := d.GetCategories()
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	// seeded categories + the one we created
	var found *Category
	for i := range cats {
		if cats[i].ID == id {
			found = &cats[i]
			break
		}
	}
	if found == nil {
		t.Fatal("created category not returned by GetCategories")
	}
	if found.Name != "Fruit Tree" || found.Color != "#166534" || found.Type != "plant" {
		t.Errorf("unexpected category fields: %+v", found)
	}

	if err := d.UpdateCategory(id, "Fruit Tree", "#15803d", "plant"); err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}

	cats, _ = d.GetCategories()
	for _, c := range cats {
		if c.ID == id && c.Color != "#15803d" {
			t.Errorf("UpdateCategory color not persisted: got %q", c.Color)
		}
	}

	if err := d.DeleteCategory(id); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	cats, _ = d.GetCategories()
	for _, c := range cats {
		if c.ID == id {
			t.Error("DeleteCategory: category still present")
		}
	}
}

func TestCategoryTypeCoercion(t *testing.T) {
	d := newTestDB(t)

	id, err := d.CreateCategory("Misc", "#000000", "invalid")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	cats, _ := d.GetCategories()
	for _, c := range cats {
		if c.ID == id && c.Type != "other" {
			t.Errorf("invalid type should be coerced to 'other', got %q", c.Type)
		}
	}
}

// ── Layers ────────────────────────────────────────────────────

func TestLayerCRUD(t *testing.T) {
	d := newTestDB(t)

	id, err := d.CreateLayer("Drainage", "#a0522d")
	if err != nil {
		t.Fatalf("CreateLayer: %v", err)
	}

	layers, err := d.GetLayers()
	if err != nil {
		t.Fatalf("GetLayers: %v", err)
	}
	var found *Layer
	for i := range layers {
		if layers[i].ID == id {
			found = &layers[i]
			break
		}
	}
	if found == nil {
		t.Fatal("created layer not in GetLayers")
	}
	if found.Name != "Drainage" || found.Color != "#a0522d" {
		t.Errorf("unexpected layer fields: %+v", found)
	}

	if err := d.UpdateLayer(id, "Drainage", "#8b4513"); err != nil {
		t.Fatalf("UpdateLayer: %v", err)
	}

	if err := d.DeleteLayer(id); err != nil {
		t.Fatalf("DeleteLayer: %v", err)
	}

	layers, _ = d.GetLayers()
	for _, l := range layers {
		if l.ID == id {
			t.Error("DeleteLayer: layer still present")
		}
	}
}

// ── Markers ───────────────────────────────────────────────────

func setupPlot(t *testing.T, d *DB) int64 {
	t.Helper()
	id, err := d.CreatePlot("Test Plot", "1 Garden Lane", "")
	if err != nil {
		t.Fatalf("CreatePlot: %v", err)
	}
	return id
}

func TestMarkerCRUD(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	markerID, err := d.CreateMarker(plotID, "circle", "[0.5,0.5,0.05]", "Rose Bush", "", "", nil, nil)
	if err != nil {
		t.Fatalf("CreateMarker: %v", err)
	}

	marker, err := d.GetMarker(markerID)
	if err != nil {
		t.Fatalf("GetMarker: %v", err)
	}
	if marker.Label != "Rose Bush" {
		t.Errorf("Label = %q, want %q", marker.Label, "Rose Bush")
	}
	if marker.Shape != "circle" {
		t.Errorf("Shape = %q, want %q", marker.Shape, "circle")
	}
	if marker.PlotID != plotID {
		t.Errorf("PlotID = %d, want %d", marker.PlotID, plotID)
	}
	if marker.CategoryID != nil {
		t.Errorf("CategoryID should be nil, got %v", marker.CategoryID)
	}

	markers, err := d.GetMarkers(plotID)
	if err != nil {
		t.Fatalf("GetMarkers: %v", err)
	}
	if len(markers) != 1 {
		t.Fatalf("GetMarkers len = %d, want 1", len(markers))
	}

	if err := d.UpdateMarker(markerID, "Updated Rose", "2025-12-01", "2024-03-15", nil, nil); err != nil {
		t.Fatalf("UpdateMarker: %v", err)
	}

	marker, _ = d.GetMarker(markerID)
	if marker.Label != "Updated Rose" {
		t.Errorf("Label after update = %q", marker.Label)
	}
	if marker.EndDate != "2025-12-01" {
		t.Errorf("EndDate after update = %q", marker.EndDate)
	}
	if marker.PlantedDate != "2024-03-15" {
		t.Errorf("PlantedDate after update = %q", marker.PlantedDate)
	}

	if err := d.DeleteMarker(markerID); err != nil {
		t.Fatalf("DeleteMarker: %v", err)
	}

	_, err = d.GetMarker(markerID)
	if err == nil {
		t.Error("GetMarker after delete should return error")
	}
}

func TestMarkerWithCategoryAndLayer(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	catID, err := d.CreateCategory("Pumpkin Patch", "#b45309", "plant")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	layerID, err := d.CreateLayer("Drip Line", "#22d3ee")
	if err != nil {
		t.Fatalf("CreateLayer: %v", err)
	}

	markerID, err := d.CreateMarker(plotID, "rect", "[0.1,0.1,0.3,0.3]", "Tomato Bed", "2025-10-01", "2025-04-15", &catID, &layerID)
	if err != nil {
		t.Fatalf("CreateMarker with cat/layer: %v", err)
	}

	m, err := d.GetMarker(markerID)
	if err != nil {
		t.Fatalf("GetMarker: %v", err)
	}
	if m.CategoryID == nil || *m.CategoryID != catID {
		t.Errorf("CategoryID = %v, want %d", m.CategoryID, catID)
	}
	if m.LayerID == nil || *m.LayerID != layerID {
		t.Errorf("LayerID = %v, want %d", m.LayerID, layerID)
	}
	if m.EndDate != "2025-10-01" {
		t.Errorf("EndDate = %q", m.EndDate)
	}
	if m.PlantedDate != "2025-04-15" {
		t.Errorf("PlantedDate = %q", m.PlantedDate)
	}
}

func TestMarkerCascadeDeleteWithPlot(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	markerID, _ := d.CreateMarker(plotID, "circle", "[0.5,0.5,0.05]", "Marker", "", "", nil, nil)
	d.DeletePlot(plotID)

	_, err := d.GetMarker(markerID)
	if err == nil {
		t.Error("marker should be deleted when plot is deleted (CASCADE)")
	}
}

func TestMarkerNullableDates(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	markerID, _ := d.CreateMarker(plotID, "circle", "[0.5,0.5]", "No Dates", "", "", nil, nil)
	m, _ := d.GetMarker(markerID)

	if m.EndDate != "" {
		t.Errorf("EndDate should be empty string for NULL, got %q", m.EndDate)
	}
	if m.PlantedDate != "" {
		t.Errorf("PlantedDate should be empty string for NULL, got %q", m.PlantedDate)
	}
}

// ── Marker Entries ────────────────────────────────────────────

func TestMarkerEntryCRUD(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)
	markerID, _ := d.CreateMarker(plotID, "circle", "[0.5,0.5]", "Plant", "", "", nil, nil)

	entryID, err := d.CreateEntry(markerID, "2025-06-01", "Looking healthy")
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	entries, err := d.GetEntries(markerID)
	if err != nil {
		t.Fatalf("GetEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("GetEntries len = %d, want 1", len(entries))
	}
	if entries[0].ID != entryID {
		t.Errorf("entry ID mismatch")
	}
	if entries[0].Notes != "Looking healthy" {
		t.Errorf("Notes = %q", entries[0].Notes)
	}

	// GetEntriesWithImages should return entries with Images slice initialized
	entriesWithImages, err := d.GetEntriesWithImages(markerID)
	if err != nil {
		t.Fatalf("GetEntriesWithImages: %v", err)
	}
	if len(entriesWithImages) != 1 {
		t.Fatalf("GetEntriesWithImages len = %d, want 1", len(entriesWithImages))
	}
}

func TestEntryImages(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)
	markerID, _ := d.CreateMarker(plotID, "circle", "[0.5,0.5]", "Plant", "", "", nil, nil)
	entryID, _ := d.CreateEntry(markerID, "2025-06-01", "notes")

	imgID, err := d.AddEntryImage(entryID, "markers/img1.jpg", "First image")
	if err != nil {
		t.Fatalf("AddEntryImage: %v", err)
	}

	imgs, err := d.GetEntryImages(entryID)
	if err != nil {
		t.Fatalf("GetEntryImages: %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("GetEntryImages len = %d, want 1", len(imgs))
	}
	if imgs[0].ImagePath != "markers/img1.jpg" || imgs[0].Caption != "First image" {
		t.Errorf("unexpected image fields: %+v", imgs[0])
	}

	returnedEntryID, err := d.DeleteEntryImage(imgID)
	if err != nil {
		t.Fatalf("DeleteEntryImage: %v", err)
	}
	if returnedEntryID != entryID {
		t.Errorf("DeleteEntryImage returned entryID %d, want %d", returnedEntryID, entryID)
	}

	imgs, _ = d.GetEntryImages(entryID)
	if len(imgs) != 0 {
		t.Errorf("image should be deleted, still have %d", len(imgs))
	}
}

// ── Plant Taxonomy ────────────────────────────────────────────

func TestTaxonomyUpsert(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)
	markerID, _ := d.CreateMarker(plotID, "circle", "[0.5,0.5]", "Tomato", "", "", nil, nil)

	// Get before upsert should return nil, nil
	tax, err := d.GetTaxonomy(markerID)
	if err != nil {
		t.Fatalf("GetTaxonomy (empty): %v", err)
	}
	if tax != nil {
		t.Errorf("GetTaxonomy before upsert should return nil, got %+v", tax)
	}

	tax, err = d.UpsertTaxonomy(markerID, "Solanum", "lycopersicum", "Cherry")
	if err != nil {
		t.Fatalf("UpsertTaxonomy: %v", err)
	}
	if tax.Genus != "Solanum" || tax.Species != "lycopersicum" || tax.Cultivar != "Cherry" {
		t.Errorf("unexpected taxonomy: %+v", tax)
	}

	// Update via second upsert
	tax, err = d.UpsertTaxonomy(markerID, "Solanum", "lycopersicum", "Beefsteak")
	if err != nil {
		t.Fatalf("UpsertTaxonomy (update): %v", err)
	}
	if tax.Cultivar != "Beefsteak" {
		t.Errorf("cultivar after update = %q, want Beefsteak", tax.Cultivar)
	}
}

// ── Harvests ──────────────────────────────────────────────────

func TestHarvestCRUD(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)
	markerID, _ := d.CreateMarker(plotID, "circle", "[0.5,0.5]", "Tomato", "", "", nil, nil)

	hID, err := d.CreateHarvest(markerID, "2025-08-15", 350.5, "First harvest")
	if err != nil {
		t.Fatalf("CreateHarvest: %v", err)
	}

	harvests, err := d.GetHarvests(markerID)
	if err != nil {
		t.Fatalf("GetHarvests: %v", err)
	}
	if len(harvests) != 1 {
		t.Fatalf("GetHarvests len = %d, want 1", len(harvests))
	}
	if harvests[0].WeightGrams != 350.5 {
		t.Errorf("WeightGrams = %f, want 350.5", harvests[0].WeightGrams)
	}

	retMarkerID, err := d.DeleteHarvest(hID)
	if err != nil {
		t.Fatalf("DeleteHarvest: %v", err)
	}
	if retMarkerID != markerID {
		t.Errorf("DeleteHarvest returned markerID %d, want %d", retMarkerID, markerID)
	}

	harvests, _ = d.GetHarvests(markerID)
	if len(harvests) != 0 {
		t.Errorf("harvest should be deleted, still have %d", len(harvests))
	}
}

// ── Weather ───────────────────────────────────────────────────

func TestWeatherCRUD(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	rainfall := 12.5
	tempHigh := 28.0
	tempLow := 15.0

	wID, err := d.CreateWeather(plotID, "2025-07-10", &rainfall, &tempHigh, &tempLow, nil, "NW", "Sunny day")
	if err != nil {
		t.Fatalf("CreateWeather: %v", err)
	}

	weather, err := d.GetWeather(plotID)
	if err != nil {
		t.Fatalf("GetWeather: %v", err)
	}
	if len(weather) != 1 {
		t.Fatalf("GetWeather len = %d, want 1", len(weather))
	}
	w := weather[0]
	if w.RainfallMM == nil || *w.RainfallMM != rainfall {
		t.Errorf("RainfallMM = %v, want %f", w.RainfallMM, rainfall)
	}
	if w.WindDir != "NW" {
		t.Errorf("WindDir = %q, want NW", w.WindDir)
	}
	if w.Notes != "Sunny day" {
		t.Errorf("Notes = %q", w.Notes)
	}
	if w.WindSpeedKMH != nil {
		t.Errorf("WindSpeedKMH should be nil, got %v", w.WindSpeedKMH)
	}

	if err := d.DeleteWeather(wID); err != nil {
		t.Fatalf("DeleteWeather: %v", err)
	}

	weather, _ = d.GetWeather(plotID)
	if len(weather) != 0 {
		t.Errorf("weather should be deleted, still have %d", len(weather))
	}
}

// ── Plant Groups ──────────────────────────────────────────────

func TestPlantGroupCRUD(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	gID, err := d.CreatePlantGroup(plotID, "Berry Patch")
	if err != nil {
		t.Fatalf("CreatePlantGroup: %v", err)
	}

	group, err := d.GetPlantGroup(gID)
	if err != nil {
		t.Fatalf("GetPlantGroup: %v", err)
	}
	if group.Name != "Berry Patch" {
		t.Errorf("Name = %q, want Berry Patch", group.Name)
	}
	if group.PlotID != plotID {
		t.Errorf("PlotID = %d, want %d", group.PlotID, plotID)
	}

	if err := d.UpdatePlantGroup(gID, "Fruit Patch"); err != nil {
		t.Fatalf("UpdatePlantGroup: %v", err)
	}

	group, _ = d.GetPlantGroup(gID)
	if group.Name != "Fruit Patch" {
		t.Errorf("Name after update = %q", group.Name)
	}

	if err := d.DeletePlantGroup(gID); err != nil {
		t.Fatalf("DeletePlantGroup: %v", err)
	}

	_, err = d.GetPlantGroup(gID)
	if err == nil {
		t.Error("GetPlantGroup after delete should return error")
	}
}

func TestGroupMembers(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	gID, _ := d.CreatePlantGroup(plotID, "Herb Garden")
	m1, _ := d.CreateMarker(plotID, "circle", "[0.1,0.1]", "Basil", "", "", nil, nil)
	m2, _ := d.CreateMarker(plotID, "circle", "[0.2,0.2]", "Thyme", "", "", nil, nil)
	m3, _ := d.CreateMarker(plotID, "circle", "[0.3,0.3]", "Rosemary", "", "", nil, nil)

	if err := d.SetMarkersGroup(gID, []int64{m1, m2, m3}); err != nil {
		t.Fatalf("SetMarkersGroup: %v", err)
	}

	members, err := d.GetGroupMarkers(gID)
	if err != nil {
		t.Fatalf("GetGroupMarkers: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("GetGroupMarkers len = %d, want 3", len(members))
	}
	for _, m := range members {
		if m.GroupID == nil || *m.GroupID != gID {
			t.Errorf("marker %d GroupID = %v, want %d", m.ID, m.GroupID, gID)
		}
	}

	if err := d.RemoveGroupMember(m2); err != nil {
		t.Fatalf("RemoveGroupMember: %v", err)
	}

	members, _ = d.GetGroupMarkers(gID)
	if len(members) != 2 {
		t.Errorf("after RemoveGroupMember, len = %d, want 2", len(members))
	}
	for _, m := range members {
		if m.ID == m2 {
			t.Error("removed member still in group")
		}
	}
}

func TestGroupCascadeDeleteWithPlot(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)

	gID, _ := d.CreatePlantGroup(plotID, "Test Group")
	d.DeletePlot(plotID)

	_, err := d.GetPlantGroup(gID)
	if err == nil {
		t.Error("plant group should be deleted when plot is deleted (CASCADE)")
	}
}

// ── Group Harvests ────────────────────────────────────────────

func TestGroupHarvestCRUD(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Veggie Bed")

	hID, err := d.CreateGroupHarvest(gID, "2025-09-01", 1200.0, "Big batch")
	if err != nil {
		t.Fatalf("CreateGroupHarvest: %v", err)
	}

	harvests, err := d.GetGroupHarvests(gID)
	if err != nil {
		t.Fatalf("GetGroupHarvests: %v", err)
	}
	if len(harvests) != 1 {
		t.Fatalf("GetGroupHarvests len = %d, want 1", len(harvests))
	}
	if harvests[0].WeightGrams != 1200.0 {
		t.Errorf("WeightGrams = %f, want 1200.0", harvests[0].WeightGrams)
	}
	if harvests[0].Notes != "Big batch" {
		t.Errorf("Notes = %q", harvests[0].Notes)
	}

	retGroupID, err := d.DeleteGroupHarvest(hID)
	if err != nil {
		t.Fatalf("DeleteGroupHarvest: %v", err)
	}
	if retGroupID != gID {
		t.Errorf("DeleteGroupHarvest returned groupID %d, want %d", retGroupID, gID)
	}

	harvests, _ = d.GetGroupHarvests(gID)
	if len(harvests) != 0 {
		t.Errorf("group harvest should be deleted, still have %d", len(harvests))
	}
}

func TestGroupHarvestCascadeDelete(t *testing.T) {
	d := newTestDB(t)
	plotID := setupPlot(t, d)
	gID, _ := d.CreatePlantGroup(plotID, "Test Group")
	hID, _ := d.CreateGroupHarvest(gID, "2025-09-01", 500.0, "")

	d.DeletePlantGroup(gID)

	harvests, _ := d.GetGroupHarvests(hID)
	if len(harvests) != 0 {
		t.Error("group harvests should cascade-delete when group is deleted")
	}
}

// ── Seed data ─────────────────────────────────────────────────

func TestSeedCategories(t *testing.T) {
	d := newTestDB(t)

	cats, err := d.GetCategories()
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}

	byName := make(map[string]Category)
	for _, c := range cats {
		byName[c.Name] = c
	}

	expectedPlant := []string{"Tree", "Bush", "Flower", "Vegetable", "Herb"}
	for _, name := range expectedPlant {
		c, ok := byName[name]
		if !ok {
			t.Errorf("seed category %q not found", name)
			continue
		}
		if c.Type != "plant" {
			t.Errorf("seed category %q type = %q, want plant", name, c.Type)
		}
	}

	expectedOther := []string{"Path", "Structure", "Other"}
	for _, name := range expectedOther {
		if _, ok := byName[name]; !ok {
			t.Errorf("seed category %q not found", name)
		}
	}
}

func TestSeedLayers(t *testing.T) {
	d := newTestDB(t)

	layers, err := d.GetLayers()
	if err != nil {
		t.Fatalf("GetLayers: %v", err)
	}

	expected := []string{"General", "Water", "Electrical", "Irrigation", "Lighting", "Fencing"}
	byName := make(map[string]bool)
	for _, l := range layers {
		byName[l.Name] = true
	}
	for _, name := range expected {
		if !byName[name] {
			t.Errorf("seed layer %q not found", name)
		}
	}
}

// ── Migration idempotency ─────────────────────────────────────

func TestInitIdempotent(t *testing.T) {
	// Init on the same in-memory path isn't reusable, but we can verify
	// that calling Init twice on a file path doesn't fail.
	// Use a temp file for this.
	t.TempDir() // just ensure cleanup works

	dir := t.TempDir()
	path := dir + "/test.db"

	d1, err := Init(path)
	if err != nil {
		t.Fatalf("first Init: %v", err)
	}
	d1.Close()

	d2, err := Init(path)
	if err != nil {
		t.Fatalf("second Init (idempotency): %v", err)
	}
	d2.Close()
}
