package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── Open-Meteo types ──────────────────────────────────────────

type openMeteoResponse struct {
	Daily struct {
		Time            []string   `json:"time"`
		TempMax         []*float64 `json:"temperature_2m_max"`
		TempMin         []*float64 `json:"temperature_2m_min"`
		Precipitation   []*float64 `json:"precipitation_sum"`
		WindSpeedMax    []*float64 `json:"wind_speed_10m_max"`
		WindDirDominant []*float64 `json:"wind_direction_10m_dominant"`
		HumidityMax     []*float64 `json:"relative_humidity_2m_max"`
		HumidityMin     []*float64 `json:"relative_humidity_2m_min"`
	} `json:"daily"`
	Error  bool   `json:"error"`
	Reason string `json:"reason"`
}

type openMeteoCurrent struct {
	Current struct {
		Time          string  `json:"time"`
		Temperature   float64 `json:"temperature_2m"`
		Humidity      float64 `json:"relative_humidity_2m"`
		Precipitation float64 `json:"precipitation"`
		WindSpeed     float64 `json:"wind_speed_10m"`
		WindDir       float64 `json:"wind_direction_10m"`
		WeatherCode   int     `json:"weather_code"`
	} `json:"current"`
	Error  bool   `json:"error"`
	Reason string `json:"reason"`
}

// CurrentWeather is the response shape for GET /plots/{id}/weather/current.
type CurrentWeather struct {
	Time            string  `json:"time"`
	TemperatureC    float64 `json:"temperature_c"`
	HumidityPct     float64 `json:"humidity_pct"`
	PrecipitationMM float64 `json:"precipitation_mm"`
	WindSpeedKMH    float64 `json:"wind_speed_kmh"`
	WindDir         string  `json:"wind_dir"`
	Description     string  `json:"description"`
}

// ── Helpers ───────────────────────────────────────────────────

func degreesToCompass(deg float64) string {
	dirs := [16]string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	return dirs[int((deg+11.25)/22.5)%16]
}

func wmoDescription(code int) string {
	switch {
	case code == 0:
		return "Clear sky"
	case code == 1:
		return "Mainly clear"
	case code == 2:
		return "Partly cloudy"
	case code == 3:
		return "Overcast"
	case code == 45 || code == 48:
		return "Fog"
	case code >= 51 && code <= 55:
		return "Drizzle"
	case code >= 61 && code <= 65:
		return "Rain"
	case code >= 71 && code <= 77:
		return "Snow"
	case code >= 80 && code <= 82:
		return "Rain showers"
	case code == 85 || code == 86:
		return "Snow showers"
	case code == 95:
		return "Thunderstorm"
	case code == 96 || code == 99:
		return "Thunderstorm with hail"
	default:
		return "Unknown"
	}
}

// plotCentroid returns the average lat/lng of a plot's geo refs.
// Returns an error if no refs are set.
func (h *Handler) plotCentroid(plotID int64) (lat, lng float64, err error) {
	refs, err := h.db.GetGeoRefs(plotID)
	if err != nil {
		return 0, 0, err
	}
	if len(refs) == 0 {
		return 0, 0, fmt.Errorf("no geo refs set for plot %d", plotID)
	}
	for _, r := range refs {
		lat += r.Lat
		lng += r.Lng
	}
	n := float64(len(refs))
	return lat / n, lng / n, nil
}

// ── Daily fetch ───────────────────────────────────────────────

func fetchDailyWeather(lat, lng float64, date string) (*openMeteoResponse, error) {
	cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	base := "https://api.open-meteo.com/v1/forecast"
	if date < cutoff {
		base = "https://archive-api.open-meteo.com/v1/archive"
	}

	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%.6f", lat))
	params.Set("longitude", fmt.Sprintf("%.6f", lng))
	params.Set("start_date", date)
	params.Set("end_date", date)
	params.Set("daily", strings.Join([]string{
		"temperature_2m_max",
		"temperature_2m_min",
		"precipitation_sum",
		"wind_speed_10m_max",
		"wind_direction_10m_dominant",
		"relative_humidity_2m_max",
		"relative_humidity_2m_min",
	}, ","))
	params.Set("timezone", "auto")

	resp, err := http.Get(base + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("open-meteo request failed: %w", err)
	}
	defer resp.Body.Close()

	var omr openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&omr); err != nil {
		return nil, fmt.Errorf("invalid response from open-meteo: %w", err)
	}
	if omr.Error {
		return nil, fmt.Errorf("open-meteo error: %s", omr.Reason)
	}
	return &omr, nil
}

// syncPlotWeather fetches and stores one day of weather for a single plot.
// Skips silently if a record already exists for that date.
func (h *Handler) syncPlotWeather(plotID int64, date string) error {
	exists, err := h.db.HasWeatherForDate(plotID, date)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	lat, lng, err := h.plotCentroid(plotID)
	if err != nil {
		return err
	}

	omr, err := fetchDailyWeather(lat, lng, date)
	if err != nil {
		return err
	}
	if len(omr.Daily.Time) == 0 {
		return fmt.Errorf("no data returned for %s", date)
	}

	first := func(s []*float64) *float64 {
		if len(s) > 0 {
			return s[0]
		}
		return nil
	}

	windDir := ""
	if wd := first(omr.Daily.WindDirDominant); wd != nil {
		windDir = degreesToCompass(*wd)
	}

	_, err = h.db.CreateWeather(
		plotID, date,
		first(omr.Daily.Precipitation),
		first(omr.Daily.TempMax),
		first(omr.Daily.TempMin),
		first(omr.Daily.WindSpeedMax),
		windDir, "",
		first(omr.Daily.HumidityMax),
		first(omr.Daily.HumidityMin),
	)
	return err
}

// SyncAllPlots fetches yesterday's weather for every plot that has geo refs.
// Intended to be called by the scheduler and the sync-all HTTP endpoint.
func (h *Handler) SyncAllPlots(date string) {
	plots, err := h.db.GetPlots()
	if err != nil {
		log.Printf("weather sync: failed to list plots: %v", err)
		return
	}
	for _, p := range plots {
		if err := h.syncPlotWeather(p.ID, date); err != nil {
			log.Printf("weather sync: plot %d (%s): %v", p.ID, p.Name, err)
		}
	}
}

// ── HTTP handlers ─────────────────────────────────────────────

// POST /plots/{id}/weather/sync?date=YYYY-MM-DD
func (h *Handler) SyncWeather(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	if err := h.syncPlotWeather(plotID, date); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	records, err := h.db.GetWeather(plotID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Return the record for the synced date.
	for _, rec := range records {
		if rec.Date == date {
			writeJSON(w, http.StatusOK, rec)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent) // already existed, nothing new to return
}

// POST /weather/sync-all?date=YYYY-MM-DD
func (h *Handler) SyncAllWeatherHandler(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	h.SyncAllPlots(date)
	writeJSON(w, http.StatusOK, map[string]string{"synced_date": date})
}

// GET /plots/{id}/weather/current
//
// Returns live conditions from Open-Meteo without storing anything.
func (h *Handler) GetCurrentWeather(w http.ResponseWriter, r *http.Request) {
	plotID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid plot id", http.StatusBadRequest)
		return
	}

	lat, lng, err := h.plotCentroid(plotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%.6f", lat))
	params.Set("longitude", fmt.Sprintf("%.6f", lng))
	params.Set("current", strings.Join([]string{
		"temperature_2m",
		"relative_humidity_2m",
		"precipitation",
		"wind_speed_10m",
		"wind_direction_10m",
		"weather_code",
	}, ","))
	params.Set("timezone", "auto")

	resp, err := http.Get("https://api.open-meteo.com/v1/forecast?" + params.Encode())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "open-meteo request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var raw openMeteoCurrent
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid response from open-meteo"})
		return
	}
	if raw.Error {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": raw.Reason})
		return
	}

	c := raw.Current
	writeJSON(w, http.StatusOK, CurrentWeather{
		Time:            c.Time,
		TemperatureC:    c.Temperature,
		HumidityPct:     c.Humidity,
		PrecipitationMM: c.Precipitation,
		WindSpeedKMH:    c.WindSpeed,
		WindDir:         degreesToCompass(c.WindDir),
		Description:     wmoDescription(c.WeatherCode),
	})
}
