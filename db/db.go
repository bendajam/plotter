package db

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

// ── Model types ───────────────────────────────────────────────

type Plot struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	ImagePath string    `json:"image_path"`
	CreatedAt time.Time `json:"created_at"`
}

type Category struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Type      string    `json:"type"` // "plant" | "other"
	CreatedAt time.Time `json:"created_at"`
}

type PlantTaxonomy struct {
	ID        int64     `json:"id"`
	MarkerID  int64     `json:"marker_id"`
	Genus     string    `json:"genus"`
	Species   string    `json:"species"`
	Cultivar  string    `json:"cultivar"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlantGroup struct {
	ID        int64     `json:"id"`
	PlotID    int64     `json:"plot_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type GroupHarvest struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"group_id"`
	Date        string    `json:"date"`
	WeightGrams float64   `json:"weight_grams"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

type Harvest struct {
	ID          int64     `json:"id"`
	MarkerID    int64     `json:"marker_id"`
	Date        string    `json:"date"`
	WeightGrams float64   `json:"weight_grams"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

type Layer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type Marker struct {
	ID            int64     `json:"id"`
	PlotID        int64     `json:"plot_id"`
	Shape         string    `json:"shape"`
	Coords        string    `json:"coords"`
	Label         string    `json:"label"`
	EndDate       string    `json:"end_date"`
	CategoryID    *int64    `json:"category_id"`
	CategoryName  string    `json:"category_name"`
	CategoryColor string    `json:"category_color"`
	CategoryType  string    `json:"category_type"` // "plant" | "other"
	LayerID       *int64    `json:"layer_id"`
	LayerName     string    `json:"layer_name"`
	LayerColor    string    `json:"layer_color"`
	GroupID       *int64    `json:"group_id"`
	GroupName     string    `json:"group_name"`
	PlantedDate   string    `json:"planted_date"`
	CreatedAt     time.Time `json:"created_at"`
}

type MarkerEntry struct {
	ID        int64        `json:"id"`
	MarkerID  int64        `json:"marker_id"`
	Date      string       `json:"date"`
	Notes     string       `json:"notes"`
	CreatedAt time.Time    `json:"created_at"`
	Images    []EntryImage `json:"images"` // populated by GetEntriesWithImages
}

type EntryImage struct {
	ID        int64     `json:"id"`
	EntryID   int64     `json:"entry_id"`
	ImagePath string    `json:"image_path"`
	Caption   string    `json:"caption"`
	CreatedAt time.Time `json:"created_at"`
}

type Transplant struct {
	ID               int64     `json:"id"`
	MarkerID         int64     `json:"marker_id"`
	OldCoords        string    `json:"old_coords"`
	NewCoords        string    `json:"new_coords"`
	TransplantedDate string    `json:"transplanted_date"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
}

type Weather struct {
	ID           int64     `json:"id"`
	PlotID       int64     `json:"plot_id"`
	Date         string    `json:"date"`
	RainfallMM   *float64  `json:"rainfall_mm"`
	TempHighC    *float64  `json:"temp_high_c"`
	TempLowC     *float64  `json:"temp_low_c"`
	WindSpeedKMH *float64  `json:"wind_speed_kmh"`
	WindDir      string    `json:"wind_dir"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
}

// ── Init & migrations ─────────────────────────────────────────

func Init(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := sqldb.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, err
	}

	// Version table
	sqldb.Exec(`CREATE TABLE IF NOT EXISTS _version (v INTEGER NOT NULL DEFAULT 0)`)
	sqldb.Exec(`INSERT OR IGNORE INTO _version (v) VALUES (0)`)
	var v int
	sqldb.QueryRow(`SELECT v FROM _version`).Scan(&v)

	if v < 1 {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS plots (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL, address TEXT NOT NULL, image_path TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			`CREATE TABLE IF NOT EXISTS categories (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE, color TEXT NOT NULL DEFAULT '#64748b',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			`CREATE TABLE IF NOT EXISTS markers (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				plot_id INTEGER NOT NULL REFERENCES plots(id) ON DELETE CASCADE,
				shape TEXT NOT NULL, coords TEXT NOT NULL, label TEXT NOT NULL DEFAULT '',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			`CREATE TABLE IF NOT EXISTS weather (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				plot_id INTEGER NOT NULL REFERENCES plots(id) ON DELETE CASCADE,
				date DATE NOT NULL, rainfall_mm REAL, temp_high_c REAL, temp_low_c REAL,
				wind_speed_kmh REAL, wind_dir TEXT NOT NULL DEFAULT '',
				notes TEXT NOT NULL DEFAULT '', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		}
		for _, s := range stmts {
			if _, err := sqldb.Exec(s); err != nil {
				return nil, err
			}
		}
		sqldb.Exec(`UPDATE _version SET v = 1`)
		v = 1
	}

	if v < 2 {
		// category_id on markers; date-based entries replacing year-based
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL`)
		sqldb.Exec(`DROP TABLE IF EXISTS marker_images`)
		sqldb.Exec(`DROP TABLE IF EXISTS marker_entries`)
		sqldb.Exec(`CREATE TABLE marker_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			marker_id INTEGER NOT NULL REFERENCES markers(id) ON DELETE CASCADE,
			date DATE NOT NULL DEFAULT (date('now')),
			notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`UPDATE _version SET v = 2`)
		v = 2
	}

	if v < 3 {
		// layers + entry_images + end_date/layer_id on markers
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS layers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE, color TEXT NOT NULL DEFAULT '#64748b',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS entry_images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL REFERENCES marker_entries(id) ON DELETE CASCADE,
			image_path TEXT NOT NULL, caption TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN end_date DATE`)
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN layer_id INTEGER REFERENCES layers(id) ON DELETE SET NULL`)
		// Recreate entries without the old single-image columns (drop is safe — data is dev only)
		sqldb.Exec(`DROP TABLE IF EXISTS marker_entries`)
		sqldb.Exec(`CREATE TABLE marker_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			marker_id INTEGER NOT NULL REFERENCES markers(id) ON DELETE CASCADE,
			date DATE NOT NULL DEFAULT (date('now')),
			notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`UPDATE _version SET v = 3`)
		v = 3
	}

	if v < 4 {
		sqldb.Exec(`ALTER TABLE categories ADD COLUMN type TEXT NOT NULL DEFAULT 'other'`)
		sqldb.Exec(`UPDATE categories SET type='plant' WHERE name IN ('Tree','Bush','Flower','Vegetable','Herb')`)
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS plant_taxonomy (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			marker_id INTEGER NOT NULL UNIQUE REFERENCES markers(id) ON DELETE CASCADE,
			genus TEXT NOT NULL DEFAULT '',
			species TEXT NOT NULL DEFAULT '',
			cultivar TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS harvests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			marker_id INTEGER NOT NULL REFERENCES markers(id) ON DELETE CASCADE,
			date DATE NOT NULL DEFAULT (date('now')),
			weight_grams REAL NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`UPDATE _version SET v = 4`)
		v = 4
	}

	if v < 5 {
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS plant_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plot_id INTEGER NOT NULL REFERENCES plots(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN group_id INTEGER REFERENCES plant_groups(id) ON DELETE SET NULL`)
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS group_harvests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL REFERENCES plant_groups(id) ON DELETE CASCADE,
			date DATE NOT NULL DEFAULT (date('now')),
			weight_grams REAL NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`UPDATE _version SET v = 5`)
		v = 5
	}

	if v < 6 {
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN planted_date DATE`)
		sqldb.Exec(`UPDATE _version SET v = 6`)
		v = 6
	}

	if v < 7 {
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN deleted_at DATETIME`)
		sqldb.Exec(`UPDATE _version SET v = 7`)
		v = 7
	}

	if v < 8 {
		sqldb.Exec(`CREATE TABLE IF NOT EXISTS transplants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			marker_id INTEGER NOT NULL REFERENCES markers(id) ON DELETE CASCADE,
			old_coords TEXT NOT NULL,
			new_coords TEXT NOT NULL,
			transplanted_date DATE NOT NULL DEFAULT (date('now')),
			notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		sqldb.Exec(`UPDATE _version SET v = 8`)
		v = 8
	}

	if v < 9 {
		// Soft-delete support — error is ignored on fresh DBs that already
		// have this column from the v7 block above.
		sqldb.Exec(`ALTER TABLE markers ADD COLUMN deleted_at DATETIME`)
		sqldb.Exec(`UPDATE _version SET v = 9`)
		v = 9
	}

	// Idempotent seeds
	for _, row := range []struct{ name, color, catType string }{
		{"Tree", "#166534", "plant"}, {"Bush", "#15803d", "plant"}, {"Flower", "#be185d", "plant"},
		{"Vegetable", "#b45309", "plant"}, {"Herb", "#0d9488", "plant"}, {"Path", "#64748b", "other"},
		{"Structure", "#1d4ed8", "other"}, {"Other", "#6b7280", "other"},
	} {
		sqldb.Exec(`INSERT OR IGNORE INTO categories (name, color, type) VALUES (?, ?, ?)`, row.name, row.color, row.catType)
	}
	for _, row := range []struct{ name, color string }{
		{"General", "#64748b"}, {"Water", "#0ea5e9"}, {"Electrical", "#eab308"},
		{"Irrigation", "#22d3ee"}, {"Lighting", "#f59e0b"}, {"Fencing", "#78716c"},
	} {
		sqldb.Exec(`INSERT OR IGNORE INTO layers (name, color) VALUES (?, ?)`, row.name, row.color)
	}

	return &DB{sqldb}, nil
}

// ── Plots ─────────────────────────────────────────────────────

func (d *DB) GetPlots() ([]Plot, error) {
	rows, err := d.Query(`SELECT id, name, address, image_path, created_at FROM plots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plot
	for rows.Next() {
		var p Plot
		rows.Scan(&p.ID, &p.Name, &p.Address, &p.ImagePath, &p.CreatedAt)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) GetPlot(id int64) (*Plot, error) {
	var p Plot
	err := d.QueryRow(`SELECT id, name, address, image_path, created_at FROM plots WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Address, &p.ImagePath, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) CreatePlot(name, address, imagePath string) (int64, error) {
	res, err := d.Exec(`INSERT INTO plots (name, address, image_path) VALUES (?, ?, ?)`, name, address, imagePath)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DeletePlot(id int64) error {
	_, err := d.Exec(`DELETE FROM plots WHERE id = ?`, id)
	return err
}

// ── Categories ────────────────────────────────────────────────

func (d *DB) GetCategory(id int64) (*Category, error) {
	var c Category
	err := d.QueryRow(`SELECT id, name, color, type, created_at FROM categories WHERE id=?`, id).
		Scan(&c.ID, &c.Name, &c.Color, &c.Type, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) GetCategories() ([]Category, error) {
	rows, err := d.Query(`SELECT id, name, color, type, created_at FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		rows.Scan(&c.ID, &c.Name, &c.Color, &c.Type, &c.CreatedAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) CreateCategory(name, color, catType string) (int64, error) {
	if catType != "plant" {
		catType = "other"
	}
	res, err := d.Exec(`INSERT INTO categories (name, color, type) VALUES (?, ?, ?)`, name, color, catType)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateCategory(id int64, name, color, catType string) error {
	if catType != "plant" {
		catType = "other"
	}
	_, err := d.Exec(`UPDATE categories SET name=?, color=?, type=? WHERE id=?`, name, color, catType, id)
	return err
}

func (d *DB) DeleteCategory(id int64) error {
	_, err := d.Exec(`DELETE FROM categories WHERE id=?`, id)
	return err
}

// ── Layers ────────────────────────────────────────────────────

func (d *DB) GetLayer(id int64) (*Layer, error) {
	var l Layer
	err := d.QueryRow(`SELECT id, name, color, created_at FROM layers WHERE id=?`, id).
		Scan(&l.ID, &l.Name, &l.Color, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (d *DB) GetLayers() ([]Layer, error) {
	rows, err := d.Query(`SELECT id, name, color, created_at FROM layers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Layer
	for rows.Next() {
		var l Layer
		rows.Scan(&l.ID, &l.Name, &l.Color, &l.CreatedAt)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (d *DB) CreateLayer(name, color string) (int64, error) {
	res, err := d.Exec(`INSERT INTO layers (name, color) VALUES (?, ?)`, name, color)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateLayer(id int64, name, color string) error {
	_, err := d.Exec(`UPDATE layers SET name=?, color=? WHERE id=?`, name, color, id)
	return err
}

func (d *DB) DeleteLayer(id int64) error {
	_, err := d.Exec(`DELETE FROM layers WHERE id=?`, id)
	return err
}

// ── Markers ───────────────────────────────────────────────────

const markerCols = `
	m.id, m.plot_id, m.shape, m.coords, m.label,
	COALESCE(m.end_date, ''),
	m.category_id, COALESCE(c.name,''), COALESCE(c.color,'#64748b'), COALESCE(c.type,'other'),
	m.layer_id,    COALESCE(l.name,''), COALESCE(l.color,'#64748b'),
	m.group_id,    COALESCE(g.name,''),
	COALESCE(m.planted_date, ''),
	m.created_at`

const markerJoin = `
	FROM markers m
	LEFT JOIN categories  c ON m.category_id = c.id
	LEFT JOIN layers      l ON m.layer_id     = l.id
	LEFT JOIN plant_groups g ON m.group_id    = g.id`

func scanMarker(row interface{ Scan(...any) error }) (*Marker, error) {
	var m Marker
	err := row.Scan(
		&m.ID, &m.PlotID, &m.Shape, &m.Coords, &m.Label, &m.EndDate,
		&m.CategoryID, &m.CategoryName, &m.CategoryColor, &m.CategoryType,
		&m.LayerID, &m.LayerName, &m.LayerColor,
		&m.GroupID, &m.GroupName,
		&m.PlantedDate,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (d *DB) GetMarkers(plotID int64) ([]Marker, error) {
	rows, err := d.Query(`SELECT `+markerCols+markerJoin+` WHERE m.plot_id=? AND m.deleted_at IS NULL ORDER BY m.created_at`, plotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Marker
	for rows.Next() {
		m, err := scanMarker(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (d *DB) GetMarker(id int64) (*Marker, error) {
	return scanMarker(d.QueryRow(`SELECT `+markerCols+markerJoin+` WHERE m.id=? AND m.deleted_at IS NULL`, id))
}

func (d *DB) CreateMarker(plotID int64, shape, coords, label, endDate, plantedDate string, categoryID, layerID *int64) (int64, error) {
	nullableDate := func(s string) interface{} {
		if s != "" {
			return s
		}
		return nil
	}
	res, err := d.Exec(
		`INSERT INTO markers (plot_id, shape, coords, label, end_date, planted_date, category_id, layer_id) VALUES (?,?,?,?,?,?,?,?)`,
		plotID, shape, coords, label, nullableDate(endDate), nullableDate(plantedDate), categoryID, layerID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateMarker(id int64, label, endDate, plantedDate string, categoryID, layerID *int64) error {
	nullableDate := func(s string) interface{} {
		if s != "" {
			return s
		}
		return nil
	}
	_, err := d.Exec(
		`UPDATE markers SET label=?, end_date=?, planted_date=?, category_id=?, layer_id=? WHERE id=?`,
		label, nullableDate(endDate), nullableDate(plantedDate), categoryID, layerID, id,
	)
	return err
}

func (d *DB) DeleteMarker(id int64) error {
	_, err := d.Exec(`UPDATE markers SET deleted_at = CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

// ── Marker Entries ────────────────────────────────────────────

func (d *DB) GetEntries(markerID int64) ([]MarkerEntry, error) {
	rows, err := d.Query(`SELECT id, marker_id, date, notes, created_at FROM marker_entries WHERE marker_id=? ORDER BY date DESC`, markerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MarkerEntry
	for rows.Next() {
		var e MarkerEntry
		rows.Scan(&e.ID, &e.MarkerID, &e.Date, &e.Notes, &e.CreatedAt)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) GetEntriesWithImages(markerID int64) ([]MarkerEntry, error) {
	entries, err := d.GetEntries(markerID)
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		imgs, err := d.GetEntryImages(e.ID)
		if err != nil {
			return nil, err
		}
		entries[i].Images = imgs
	}
	return entries, nil
}

func (d *DB) CreateEntry(markerID int64, date, notes string) (int64, error) {
	res, err := d.Exec(`INSERT INTO marker_entries (marker_id, date, notes) VALUES (?,?,?)`, markerID, date, notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetEntry(id int64) (*MarkerEntry, error) {
	var e MarkerEntry
	err := d.QueryRow(`SELECT id, marker_id, date, notes, created_at FROM marker_entries WHERE id=?`, id).
		Scan(&e.ID, &e.MarkerID, &e.Date, &e.Notes, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	imgs, err := d.GetEntryImages(e.ID)
	if err != nil {
		return nil, err
	}
	e.Images = imgs
	return &e, nil
}

// ── Entry Images ──────────────────────────────────────────────

func (d *DB) GetEntryImages(entryID int64) ([]EntryImage, error) {
	rows, err := d.Query(`SELECT id, entry_id, image_path, caption, created_at FROM entry_images WHERE entry_id=? ORDER BY created_at`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EntryImage
	for rows.Next() {
		var img EntryImage
		rows.Scan(&img.ID, &img.EntryID, &img.ImagePath, &img.Caption, &img.CreatedAt)
		out = append(out, img)
	}
	return out, rows.Err()
}

func (d *DB) AddEntryImage(entryID int64, imagePath, caption string) (int64, error) {
	res, err := d.Exec(`INSERT INTO entry_images (entry_id, image_path, caption) VALUES (?,?,?)`, entryID, imagePath, caption)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DeleteEntryImage(id int64) (int64, error) {
	var entryID int64
	d.QueryRow(`SELECT entry_id FROM entry_images WHERE id=?`, id).Scan(&entryID)
	_, err := d.Exec(`DELETE FROM entry_images WHERE id=?`, id)
	return entryID, err
}

// ── Plant Taxonomy ────────────────────────────────────────────

func (d *DB) GetTaxonomy(markerID int64) (*PlantTaxonomy, error) {
	var t PlantTaxonomy
	err := d.QueryRow(`SELECT id, marker_id, genus, species, cultivar, updated_at FROM plant_taxonomy WHERE marker_id=?`, markerID).
		Scan(&t.ID, &t.MarkerID, &t.Genus, &t.Species, &t.Cultivar, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (d *DB) UpsertTaxonomy(markerID int64, genus, species, cultivar string) (*PlantTaxonomy, error) {
	_, err := d.Exec(`
		INSERT INTO plant_taxonomy (marker_id, genus, species, cultivar, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(marker_id) DO UPDATE SET
			genus = excluded.genus,
			species = excluded.species,
			cultivar = excluded.cultivar,
			updated_at = CURRENT_TIMESTAMP`,
		markerID, genus, species, cultivar)
	if err != nil {
		return nil, err
	}
	return d.GetTaxonomy(markerID)
}

// ── Harvests ──────────────────────────────────────────────────

func (d *DB) GetHarvest(id int64) (*Harvest, error) {
	var h Harvest
	err := d.QueryRow(`SELECT id, marker_id, date, weight_grams, notes, created_at FROM harvests WHERE id=?`, id).
		Scan(&h.ID, &h.MarkerID, &h.Date, &h.WeightGrams, &h.Notes, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (d *DB) GetHarvests(markerID int64) ([]Harvest, error) {
	rows, err := d.Query(`SELECT id, marker_id, date, weight_grams, notes, created_at FROM harvests WHERE marker_id=? ORDER BY date DESC`, markerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Harvest
	for rows.Next() {
		var h Harvest
		rows.Scan(&h.ID, &h.MarkerID, &h.Date, &h.WeightGrams, &h.Notes, &h.CreatedAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (d *DB) CreateHarvest(markerID int64, date string, weightGrams float64, notes string) (int64, error) {
	res, err := d.Exec(`INSERT INTO harvests (marker_id, date, weight_grams, notes) VALUES (?,?,?,?)`, markerID, date, weightGrams, notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DeleteHarvest(id int64) (int64, error) {
	var markerID int64
	d.QueryRow(`SELECT marker_id FROM harvests WHERE id=?`, id).Scan(&markerID)
	_, err := d.Exec(`DELETE FROM harvests WHERE id=?`, id)
	return markerID, err
}

// ── Transplants ───────────────────────────────────────────────

func (d *DB) GetTransplant(id int64) (*Transplant, error) {
	var t Transplant
	err := d.QueryRow(`SELECT id, marker_id, old_coords, new_coords, transplanted_date, notes, created_at FROM transplants WHERE id=?`, id).
		Scan(&t.ID, &t.MarkerID, &t.OldCoords, &t.NewCoords, &t.TransplantedDate, &t.Notes, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (d *DB) GetTransplants(markerID int64) ([]Transplant, error) {
	rows, err := d.Query(
		`SELECT id, marker_id, old_coords, new_coords, transplanted_date, notes, created_at
		 FROM transplants WHERE marker_id=? ORDER BY transplanted_date DESC, created_at DESC`,
		markerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transplant
	for rows.Next() {
		var t Transplant
		rows.Scan(&t.ID, &t.MarkerID, &t.OldCoords, &t.NewCoords, &t.TransplantedDate, &t.Notes, &t.CreatedAt)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) CreateTransplant(markerID int64, oldCoords, newCoords, date, notes string) (int64, error) {
	res, err := d.Exec(
		`INSERT INTO transplants (marker_id, old_coords, new_coords, transplanted_date, notes) VALUES (?,?,?,?,?)`,
		markerID, oldCoords, newCoords, date, notes)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = d.Exec(`UPDATE markers SET coords=? WHERE id=?`, newCoords, markerID)
	return id, err
}

// ── Weather ───────────────────────────────────────────────────

func (d *DB) GetWeatherRecord(id int64) (*Weather, error) {
	var w Weather
	err := d.QueryRow(`SELECT id, plot_id, date, rainfall_mm, temp_high_c, temp_low_c, wind_speed_kmh, wind_dir, notes, created_at FROM weather WHERE id=?`, id).
		Scan(&w.ID, &w.PlotID, &w.Date, &w.RainfallMM, &w.TempHighC, &w.TempLowC, &w.WindSpeedKMH, &w.WindDir, &w.Notes, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (d *DB) GetWeather(plotID int64) ([]Weather, error) {
	rows, err := d.Query(`SELECT id, plot_id, date, rainfall_mm, temp_high_c, temp_low_c, wind_speed_kmh, wind_dir, notes, created_at FROM weather WHERE plot_id=? ORDER BY date DESC`, plotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Weather
	for rows.Next() {
		var w Weather
		rows.Scan(&w.ID, &w.PlotID, &w.Date, &w.RainfallMM, &w.TempHighC, &w.TempLowC, &w.WindSpeedKMH, &w.WindDir, &w.Notes, &w.CreatedAt)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (d *DB) CreateWeather(plotID int64, date string, rainfall, tempHigh, tempLow, windSpeed *float64, windDir, notes string) (int64, error) {
	res, err := d.Exec(`INSERT INTO weather (plot_id, date, rainfall_mm, temp_high_c, temp_low_c, wind_speed_kmh, wind_dir, notes) VALUES (?,?,?,?,?,?,?,?)`,
		plotID, date, rainfall, tempHigh, tempLow, windSpeed, windDir, notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DeleteWeather(id int64) error {
	_, err := d.Exec(`DELETE FROM weather WHERE id=?`, id)
	return err
}

// ── Plant Groups ──────────────────────────────────────────────

func (d *DB) CreatePlantGroup(plotID int64, name string) (int64, error) {
	res, err := d.Exec(`INSERT INTO plant_groups (plot_id, name) VALUES (?,?)`, plotID, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetPlantGroup(id int64) (*PlantGroup, error) {
	var g PlantGroup
	err := d.QueryRow(`SELECT id, plot_id, name, created_at FROM plant_groups WHERE id=?`, id).
		Scan(&g.ID, &g.PlotID, &g.Name, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (d *DB) UpdatePlantGroup(id int64, name string) error {
	_, err := d.Exec(`UPDATE plant_groups SET name=? WHERE id=?`, name, id)
	return err
}

func (d *DB) DeletePlantGroup(id int64) error {
	_, err := d.Exec(`DELETE FROM plant_groups WHERE id=?`, id)
	return err
}

func (d *DB) GetGroupMarkers(groupID int64) ([]Marker, error) {
	rows, err := d.Query(`SELECT `+markerCols+markerJoin+` WHERE m.group_id=? AND m.deleted_at IS NULL ORDER BY m.label`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Marker
	for rows.Next() {
		m, err := scanMarker(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (d *DB) SetMarkersGroup(groupID int64, markerIDs []int64) error {
	for _, mid := range markerIDs {
		if _, err := d.Exec(`UPDATE markers SET group_id=? WHERE id=?`, groupID, mid); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) RemoveGroupMember(markerID int64) error {
	_, err := d.Exec(`UPDATE markers SET group_id=NULL WHERE id=?`, markerID)
	return err
}

// ── Group Harvests ────────────────────────────────────────────

func (d *DB) GetGroupHarvest(id int64) (*GroupHarvest, error) {
	var h GroupHarvest
	err := d.QueryRow(`SELECT id, group_id, date, weight_grams, notes, created_at FROM group_harvests WHERE id=?`, id).
		Scan(&h.ID, &h.GroupID, &h.Date, &h.WeightGrams, &h.Notes, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (d *DB) GetGroupHarvests(groupID int64) ([]GroupHarvest, error) {
	rows, err := d.Query(`SELECT id, group_id, date, weight_grams, notes, created_at FROM group_harvests WHERE group_id=? ORDER BY date DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupHarvest
	for rows.Next() {
		var h GroupHarvest
		rows.Scan(&h.ID, &h.GroupID, &h.Date, &h.WeightGrams, &h.Notes, &h.CreatedAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (d *DB) CreateGroupHarvest(groupID int64, date string, weightGrams float64, notes string) (int64, error) {
	res, err := d.Exec(`INSERT INTO group_harvests (group_id, date, weight_grams, notes) VALUES (?,?,?,?)`, groupID, date, weightGrams, notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DeleteGroupHarvest(id int64) (int64, error) {
	var groupID int64
	d.QueryRow(`SELECT group_id FROM group_harvests WHERE id=?`, id).Scan(&groupID)
	_, err := d.Exec(`DELETE FROM group_harvests WHERE id=?`, id)
	return groupID, err
}
