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

func main() {
	os.MkdirAll("uploads/plots", 0755)
	os.MkdirAll("uploads/markers", 0755)

	database, err := db.Init("plotter.db")
	if err != nil {
		log.Fatal("db init:", err)
	}
	defer database.Close()

	h, err := handlers.New(database)
	if err != nil {
		log.Fatal("handlers init:", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	r.Get("/", h.ListPlots)
	r.Get("/plots/new", h.NewPlot)
	r.Post("/plots", h.CreatePlot)
	r.Get("/plots/{id}", h.ViewPlot)
	r.Delete("/plots/{id}", h.DeletePlot)

	r.Post("/markers/bulk", h.BulkUpdateMarkers)
	r.Post("/plots/{id}/markers", h.CreateMarker)
	r.Get("/markers/{id}", h.ViewMarker)
	r.Put("/markers/{id}", h.UpdateMarker)
	r.Post("/markers/{id}/entries", h.CreateEntry)
	r.Delete("/markers/{id}", h.DeleteMarker)

	r.Post("/entries/{id}/images", h.AddEntryImages)
	r.Delete("/entry-images/{id}", h.DeleteEntryImage)

	r.Put("/markers/{id}/taxonomy", h.UpsertTaxonomy)
	r.Post("/markers/{id}/harvests", h.CreateHarvest)
	r.Delete("/harvests/{id}", h.DeleteHarvest)

	r.Post("/plots/{id}/plant-groups", h.CreatePlantGroup)
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

	log.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
