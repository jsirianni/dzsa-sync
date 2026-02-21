// Package api provides the HTTP API server (metrics and synced-servers endpoints).
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jsirianni/dzsa-sync/internal/servers"
)

// MetricsPath is the path for the Prometheus metrics handler.
const MetricsPath = "/metrics"

// NewServer returns an HTTP server that serves metrics at MetricsPath and JSON API at /api/v1/servers and /api/v1/servers/<port>.
func NewServer(addr string, metricsHandler http.Handler, store *servers.Store) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(MetricsPath, metricsHandler)
	mux.HandleFunc("GET /api/v1/servers", listHandler(store))
	mux.HandleFunc("GET /api/v1/servers/", singleHandler(store))

	return &http.Server{
		Addr:              addr,
		Handler:            mux,
		ReadHeaderTimeout:  10 * time.Second,
		ReadTimeout:        10 * time.Second,
		WriteTimeout:       10 * time.Second,
		IdleTimeout:        60 * time.Second,
	}
}

func listHandler(store *servers.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		entries := store.GetAll()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"servers": entries})
	}
}

func singleHandler(store *servers.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/servers/")
		if suffix == "" || strings.Contains(suffix, "/") {
			http.NotFound(w, r)
			return
		}
		port, err := strconv.Atoi(suffix)
		if err != nil {
			http.Error(w, "invalid port", http.StatusBadRequest)
			return
		}
		result, ok := store.Get(port)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}
