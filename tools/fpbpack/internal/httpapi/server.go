package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
)

type Loader func() (management.Snapshot, error)

type Server struct {
	loader  Loader
	version string
}

func NewHandler(loader Loader, version string) http.Handler {
	server := &Server{loader: loader, version: version}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /api/status", server.status)
	mux.HandleFunc("GET /api/inventory", server.inventory)
	mux.HandleFunc("GET /api/mods", server.mods)
	mux.HandleFunc("GET /api/diagnostics", server.diagnostics)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	snapshot, ok := s.load(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		management.Status
		Version string `json:"version"`
	}{Status: snapshot.Status, Version: s.version})
}

func (s *Server) inventory(w http.ResponseWriter, _ *http.Request) {
	snapshot, ok := s.load(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		GeneratedAt time.Time        `json:"generated_at"`
		Summary     inventorySummary `json:"summary"`
		Mods        []management.Mod `json:"mods"`
	}{
		GeneratedAt: snapshot.Inventory.GeneratedAt,
		Summary: inventorySummary{
			Total:  snapshot.Inventory.Summary.Total,
			Server: snapshot.Inventory.Summary.Server,
			Client: snapshot.Inventory.Summary.Client,
		},
		Mods: snapshot.Mods,
	})
}

type inventorySummary struct {
	Total  int `json:"total"`
	Server int `json:"server"`
	Client int `json:"client"`
}

func (s *Server) mods(w http.ResponseWriter, _ *http.Request) {
	snapshot, ok := s.load(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Mods []management.Mod `json:"mods"`
	}{Mods: snapshot.Mods})
}

func (s *Server) diagnostics(w http.ResponseWriter, _ *http.Request) {
	snapshot, ok := s.load(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, snapshot.Diagnostics)
}

func (s *Server) load(w http.ResponseWriter) (management.Snapshot, bool) {
	if s.loader == nil {
		writeError(w, http.StatusServiceUnavailable, "snapshot loader is not configured")
		return management.Snapshot{}, false
	}
	snapshot, err := s.loader()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return management.Snapshot{}, false
	}
	return snapshot, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
