package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/service"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

type Loader func() (management.Snapshot, error)
type UpdatesLoader func() (updatecheck.Report, error)
type RefreshFunc func(context.Context) error
type PlanCreator func(context.Context, []string) (planning.Plan, error)
type PlansLoader func() ([]planning.Summary, error)
type PlanLoader func(string) (planning.Plan, error)
type HistoryLoader func() ([]planning.HistoryEvent, error)
type RetentionLoader func() service.RuntimeSettings
type RetentionUpdater func(service.RuntimeSettings) (service.RuntimeSettings, error)

type ServerOptions struct {
	Updates      UpdatesLoader
	Refresh      RefreshFunc
	CheckUpdates RefreshFunc
	CreatePlan   PlanCreator
	Plans        PlansLoader
	Plan         PlanLoader
	History      HistoryLoader
	Retention    RetentionLoader
	UpdateRetention RetentionUpdater
	Web          http.Handler
}

type Server struct {
	loader        Loader
	updatesLoader UpdatesLoader
	refresh       RefreshFunc
	checkUpdates  RefreshFunc
	createPlan    PlanCreator
	plansLoader   PlansLoader
	planLoader    PlanLoader
	historyLoader HistoryLoader
	retentionLoader RetentionLoader
	updateRetention RetentionUpdater
	version       string
}

func NewHandler(loader Loader, version string) http.Handler {
	return NewHandlerWithOptions(loader, version, ServerOptions{})
}

func NewHandlerWithOptions(loader Loader, version string, opts ServerOptions) http.Handler {
	server := &Server{
		loader: loader, updatesLoader: opts.Updates, refresh: opts.Refresh,
		checkUpdates: opts.CheckUpdates, createPlan: opts.CreatePlan,
		plansLoader: opts.Plans, planLoader: opts.Plan, historyLoader: opts.History,
		retentionLoader: opts.Retention, updateRetention: opts.UpdateRetention,
		version: version,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /api/status", server.status)
	mux.HandleFunc("GET /api/inventory", server.inventory)
	mux.HandleFunc("GET /api/mods", server.mods)
	mux.HandleFunc("GET /api/diagnostics", server.diagnostics)
	mux.HandleFunc("GET /api/updates", server.updates)
	mux.HandleFunc("POST /api/refresh", server.refreshAll)
	mux.HandleFunc("POST /api/updates/check", server.checkForUpdates)
	mux.HandleFunc("GET /api/plans", server.plans)
	mux.HandleFunc("POST /api/plans", server.createPlanHandler)
	mux.HandleFunc("GET /api/plans/{id}", server.plan)
	mux.HandleFunc("GET /api/history", server.history)
	mux.HandleFunc("GET /api/settings", server.retentionSettings)
	mux.HandleFunc("PUT /api/settings", server.updateRetentionSettings)
	if opts.Web != nil {
		mux.Handle("/", opts.Web)
	}
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
			Total: snapshot.Inventory.Summary.Total,
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

func (s *Server) updates(w http.ResponseWriter, _ *http.Request) {
	if s.updatesLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "update discovery is not configured")
		return
	}
	report, err := s.updatesLoader()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) refreshAll(w http.ResponseWriter, r *http.Request) {
	if s.refresh == nil {
		writeError(w, http.StatusServiceUnavailable, "refresh is not configured")
		return
	}
	if err := s.refresh(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "refreshed"})
}

func (s *Server) checkForUpdates(w http.ResponseWriter, r *http.Request) {
	if s.checkUpdates == nil {
		writeError(w, http.StatusServiceUnavailable, "update refresh is not configured")
		return
	}
	if err := s.checkUpdates(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "refreshed"})
}

func (s *Server) plans(w http.ResponseWriter, _ *http.Request) {
	if s.plansLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "plan storage is not configured")
		return
	}
	plans, err := s.plansLoader()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func (s *Server) createPlanHandler(w http.ResponseWriter, r *http.Request) {
	if s.createPlan == nil {
		writeError(w, http.StatusServiceUnavailable, "planning is not configured")
		return
	}
	var request struct {
		CandidateKeys []string `json:"candidate_keys"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan request: "+err.Error())
		return
	}
	plan, err := s.createPlan(r.Context(), request.CandidateKeys)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) plan(w http.ResponseWriter, r *http.Request) {
	if s.planLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "plan storage is not configured")
		return
	}
	plan, err := s.planLoader(r.PathValue("id"))
	if errors.Is(err, planning.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) history(w http.ResponseWriter, _ *http.Request) {
	if s.historyLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "history is not configured")
		return
	}
	history, err := s.historyLoader()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": history})
}

func (s *Server) retentionSettings(w http.ResponseWriter, _ *http.Request) {
	if s.retentionLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "retention settings are not configured")
		return
	}
	writeJSON(w, http.StatusOK, s.retentionLoader())
}

func (s *Server) updateRetentionSettings(w http.ResponseWriter, r *http.Request) {
	if s.updateRetention == nil {
		writeError(w, http.StatusServiceUnavailable, "retention settings are not configured")
		return
	}
	var value service.RuntimeSettings
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid retention settings: "+err.Error())
		return
	}
	updated, err := s.updateRetention(value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
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
