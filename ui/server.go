package ui

import (
	"database/sql"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/focustrack/focustrack/internal/config"
	"github.com/focustrack/focustrack/internal/storage"
)

//go:embed frontend/*
var frontendFS embed.FS

func newRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	
	// API routes
	mux.HandleFunc("/api/day", handleDayStats(db))

	// DELETE /api/data — wipes all sessions from the DB
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, err := db.Exec(`DELETE FROM sessions`); err != nil {
			http.Error(w, "failed to clear data", http.StatusInternalServerError)
			log.Printf("warn: clear data: %v", err)
			return
		}
		if _, err := db.Exec(`VACUUM`); err != nil {
			log.Printf("warn: vacuum after clear: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	// /api/limits — GET list, POST add/update, DELETE remove
	mux.HandleFunc("/api/limits", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, err := config.Load()
			if err != nil || cfg == nil {
				cfg = &config.Config{AppLimits: make(map[string]int)}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cfg.AppLimits)

		case http.MethodPost:
			var body struct {
				App     string `json:"app"`
				Minutes int    `json:"minutes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.App == "" || body.Minutes <= 0 {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			cfg, _ := config.Load()
			if cfg == nil {
				cfg = &config.Config{}
			}
			if cfg.AppLimits == nil {
				cfg.AppLimits = make(map[string]int)
			}
			cfg.AppLimits[body.App] = body.Minutes
			config.Save(cfg)
			w.WriteHeader(http.StatusOK)

		case http.MethodDelete:
			var body struct {
				App string `json:"app"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.App == "" {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			cfg, _ := config.Load()
			if cfg != nil && cfg.AppLimits != nil {
				delete(cfg.AppLimits, body.App)
				config.Save(cfg)
			}
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			cfg, err := config.Load()
			if err != nil || cfg == nil {
				cfg = &config.Config{} // return empty defaults on error
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cfg)
			return
		}

		if r.Method == http.MethodPost {
			var patch map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			cfg, _ := config.Load()
			if cfg == nil {
				cfg = &config.Config{}
			}

			// Apply patch fields
			if v, ok := patch["startWithWindows"]; ok {
				cfg.StartWithWindows = v.(bool)
				// Apply registry change immediately
				if cfg.StartWithWindows {
					setStartup(true)
				} else {
					setStartup(false)
				}
			}
			if v, ok := patch["notifyDaily"]; ok {
				cfg.NotifyDaily = v.(bool)
			}
			if v, ok := patch["limitAlerts"]; ok {
				cfg.LimitAlerts = v.(bool)
			}

			config.Save(cfg)
			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Create an HTTP handler serving the embedded files under public path
	// The path strip removes the "frontend/" prefix so the roots match.
	fs := http.FS(frontendFS)
	fileServer := http.StripPrefix("/", http.FileServer(fs))

	mux.Handle("/", interceptRoot(fileServer))
	return mux
}

func handleDayStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dateStr := r.URL.Query().Get("date") // "2026-05-26"
		if dateStr == "" {
			http.Error(w, "missing date param", http.StatusBadRequest)
			return
		}

		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date format", http.StatusBadRequest)
			return
		}

		stats, err := storage.GetDailyStats(db, date)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		// Compute total seconds for that day
		var totalSec int64
		for _, s := range stats {
			totalSec += s.TotalSeconds
		}

		type AppRow struct {
			Name     string  `json:"name"`
			TotalSec int64   `json:"totalSec"`
			Sessions int     `json:"sessions"`
			PctOfTop float64 `json:"pctOfTop"`
		}

		topSec := int64(0)
		if len(stats) > 0 {
			topSec = stats[0].TotalSeconds
		}

		rows := make([]AppRow, 0, len(stats))
		for _, s := range stats {
			pct := 0.0
			if topSec > 0 {
				pct = float64(s.TotalSeconds) / float64(topSec) * 100
			}
			// Fallback session handling: old apps might use RunCount depending on storage version
			// Wait, the prompt specifically says `s.Sessions`. I will use what the prompt provided.
			// Let's assume storage.AppStat struct has Sessions, or we just map it out as in the prompt limit.
			// Re-reading prompt snippet... `Sessions: s.Sessions`
			rows = append(rows, AppRow{
				Name:     s.AppName,
				TotalSec: s.TotalSeconds,
				Sessions: s.Sessions, // User prompt says `Sessions: s.Sessions`
				PctOfTop: pct,
			})
		}

		resp := struct {
			Date     string   `json:"date"`
			TotalSec int64    `json:"totalSec"`
			Apps     []AppRow `json:"apps"`
		}{
			Date:     dateStr,
			TotalSec: totalSec,
			Apps:     rows,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func interceptRoot(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just adjusting the URL if needed so it serves from frontend/
		if r.URL.Path == "/" {
			r.URL.Path = "/frontend/"
		} else {
			r.URL.Path = "/frontend" + r.URL.Path
		}
		h.ServeHTTP(w, r)
	})
}
