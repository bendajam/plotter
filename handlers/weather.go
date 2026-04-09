package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListWeather(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	plot, err := h.db.GetPlot(plotID)
	if err != nil {
		http.Error(w, "plot not found", http.StatusNotFound)
		return
	}

	records, err := h.db.GetWeather(plotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, r, "weather_item", map[string]interface{}{
			"Records": records,
			"PlotID":  plotID,
		})
		return
	}

	h.render(w, r, "weather", map[string]interface{}{
		"Plot":    plot,
		"Records": records,
	})
}

func (h *Handler) CreateWeather(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	date := strings.TrimSpace(r.FormValue("date"))
	if date == "" {
		http.Error(w, "date required", http.StatusBadRequest)
		return
	}

	parseOptFloat := func(key string) *float64 {
		v := strings.TrimSpace(r.FormValue(key))
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return &f
	}

	rainfall := parseOptFloat("rainfall_mm")
	tempHigh := parseOptFloat("temp_high_c")
	tempLow := parseOptFloat("temp_low_c")
	windSpeed := parseOptFloat("wind_speed_kmh")
	windDir := strings.TrimSpace(r.FormValue("wind_dir"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	_, err = h.db.CreateWeather(plotID, date, rainfall, tempHigh, tempLow, windSpeed, windDir, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	records, err := h.db.GetWeather(plotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, r, "weather_item", map[string]interface{}{
		"Records": records,
		"PlotID":  plotID,
	})
}

func (h *Handler) DeleteWeather(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteWeather(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
