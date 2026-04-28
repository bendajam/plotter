package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"plotter/db"
	"plotter/handlers"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dbPath   := env("PLOTTER_DB", "plotter.db")
	port     := env("PLOTTER_PORT", "8080")
	uploadDir := env("PLOTTER_UPLOAD_DIR", "uploads")

	os.MkdirAll(uploadDir+"/plots", 0755)
	os.MkdirAll(uploadDir+"/markers", 0755)

	database, err := db.Init(dbPath)
	if err != nil {
		log.Fatal("db init:", err)
	}
	defer database.Close()

	h, err := handlers.New(database, uploadDir)
	if err != nil {
		log.Fatal("handlers init:", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	r.Get("/", h.ListPlots)
	r.Get("/plots", h.ListPlotsJSON)
	r.Get("/plots/new", h.NewPlot)
	r.Post("/plots", h.CreatePlot)
	r.Get("/plots/{id}", h.ViewPlot)
	r.Delete("/plots/{id}", h.DeletePlot)

	r.Post("/markers/bulk", h.BulkUpdateMarkers)
	r.Post("/plots/{id}/markers", h.CreateMarker)
	r.Get("/plots/{id}/markers", h.ListPlotMarkers)
	r.Get("/markers/{id}", h.ViewMarker)
	r.Put("/markers/{id}", h.UpdateMarker)
	r.Post("/markers/{id}/entries", h.CreateEntry)
	r.Post("/markers/{id}/transplants", h.CreateTransplant)
	r.Delete("/markers/{id}", h.DeleteMarker)

	r.Post("/entries/{id}/images", h.AddEntryImages)
	r.Delete("/entry-images/{id}", h.DeleteEntryImage)

	r.Put("/markers/{id}/taxonomy", h.UpsertTaxonomy)
	r.Post("/markers/{id}/harvests", h.CreateHarvest)
	r.Delete("/harvests/{id}", h.DeleteHarvest)

	r.Post("/plots/{id}/plant-groups", h.CreatePlantGroup)
	r.Post("/plant-groups", h.CreatePlantGroupFromBody)
	r.Get("/plant-groups/{id}", h.ViewPlantGroup)
	r.Put("/plant-groups/{id}", h.UpdatePlantGroup)
	r.Delete("/plant-groups/{id}", h.DeletePlantGroup)
	r.Post("/plant-groups/{id}/members", h.AddGroupMember)
	r.Delete("/plant-groups/{id}/members/{mid}", h.RemoveGroupMember)
	r.Post("/plant-groups/{id}/harvests", h.CreateGroupHarvest)
	r.Delete("/group-harvests/{id}", h.DeleteGroupHarvest)

	r.Get("/plots/{id}/weather", h.ListWeather)
	r.Post("/plots/{id}/weather", h.CreateWeather)
	r.Delete("/weather/{id}", h.DeleteWeather)

	r.Get("/categories", h.ListCategories)
	r.Post("/categories", h.CreateCategory)
	r.Put("/categories/{id}", h.UpdateCategory)
	r.Delete("/categories/{id}", h.DeleteCategory)

	r.Get("/layers", h.ListLayers)
	r.Post("/layers", h.CreateLayer)
	r.Put("/layers/{id}", h.UpdateLayer)
	r.Delete("/layers/{id}", h.DeleteLayer)

	addr := ":" + port
	log.Println("Listening on http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
