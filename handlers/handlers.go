package handlers

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"plotter/db"
)

// templateRoot returns the directory that contains the "templates" folder.
// It prefers a path relative to the running binary (works when deployed),
// falling back to the source-file path (works during `go test` and `go run`).
func templateRoot() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "templates")); err == nil {
			return dir
		}
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}

type Handler struct {
	db        *db.DB
	templates map[string]*template.Template
}

func New(database *db.DB) (*Handler, error) {
	tmplDir := filepath.Join(templateRoot(), "templates")

	layout := filepath.Join(tmplDir, "layout.html")

	pages := []string{"index", "plot_new", "plot", "weather"}
	templates := make(map[string]*template.Template)

	for _, name := range pages {
		page := filepath.Join(tmplDir, name+".html")
		t, err := template.New("layout.html").Funcs(funcMap()).ParseFiles(layout, page)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		templates[name] = t
	}

	partials := []string{"marker_detail", "marker_item", "weather_item", "entry_item", "marker_image_item", "category_list", "layer_list", "entry_images", "taxonomy", "harvest_list", "group_harvest_list", "plant_group", "transplant_list"}
	// These partials compose sub-templates
	withPlantSubs      := map[string]bool{"marker_detail": true}
	withGroupSubs      := map[string]bool{"plant_group": true}
	withTransplantSubs := map[string]bool{"marker_detail": true}
	for _, name := range partials {
		page := filepath.Join(tmplDir, "partials", name+".html")
		files := []string{page}
		if withPlantSubs[name] {
			files = append(files,
				filepath.Join(tmplDir, "partials", "taxonomy.html"),
				filepath.Join(tmplDir, "partials", "harvest_list.html"),
			)
		}
		if withGroupSubs[name] {
			files = append(files,
				filepath.Join(tmplDir, "partials", "group_harvest_list.html"),
			)
		}
		if withTransplantSubs[name] {
			files = append(files,
				filepath.Join(tmplDir, "partials", "transplant_list.html"),
			)
		}
		t, err := template.New(name+".html").Funcs(funcMap()).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("partial %s: %w", name, err)
		}
		templates["partial/"+name] = t
	}

	// marker page also needs plant sub-templates
	for _, name := range []string{"marker"} {
		page := filepath.Join(tmplDir, name+".html")
		t, err := template.New("layout.html").Funcs(funcMap()).ParseFiles(
			layout, page,
			filepath.Join(tmplDir, "partials", "taxonomy.html"),
			filepath.Join(tmplDir, "partials", "harvest_list.html"),
			filepath.Join(tmplDir, "partials", "transplant_list.html"),
		)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		templates[name] = t
	}

	return &Handler{db: database, templates: templates}, nil
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		"deref64": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
		"derefF": func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p
		},
		"notNil": func(p interface{}) bool {
			return p != nil
		},
		"dict": func(kvs ...interface{}) map[string]interface{} {
			m := make(map[string]interface{}, len(kvs)/2)
			for i := 0; i+1 < len(kvs); i += 2 {
				if k, ok := kvs[i].(string); ok {
					m[k] = kvs[i+1]
				}
			}
			return m
		},
		"harvestTotal": func(harvests []db.Harvest) string {
			var total float64
			for _, h := range harvests {
				total += h.WeightGrams
			}
			if total >= 1000 {
				kg := math.Round(total/1000*100) / 100
				return fmt.Sprintf("%.2f kg", kg)
			}
			return fmt.Sprintf("%.0f g", total)
		},
		"groupHarvestTotal": func(harvests []db.GroupHarvest) string {
			var total float64
			for _, h := range harvests {
				total += h.WeightGrams
			}
			if total >= 1000 {
				kg := math.Round(total/1000*100) / 100
				return fmt.Sprintf("%.2f kg", kg)
			}
			return fmt.Sprintf("%.0f g", total)
		},
	}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	t, ok := h.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	t, ok := h.templates["partial/"+name]
	if !ok {
		http.Error(w, "partial not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
