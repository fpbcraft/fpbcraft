package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
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
type PlacementPlanCreator func(context.Context, string) (planning.Plan, error)
type PlansLoader func() ([]planning.Summary, error)
type PlanLoader func(string) (planning.Plan, error)
type HistoryLoader func() ([]planning.HistoryEvent, error)
type RetentionLoader func() service.RuntimeSettings
type RetentionUpdater func(service.RuntimeSettings) (service.RuntimeSettings, error)
type RulesLoader func() map[string]service.UpdateRule
type RuleSetter func(string, service.UpdateRule) (service.UpdateRule, error)
type RuleClearer func(string) error
type RefreshStatusLoader func() service.RefreshStatus
type ProvidersLoader func() []service.ProviderStatus
type ProviderCredentialSetter func(context.Context, string, string) (service.ProviderStatus, error)
type ProviderCredentialClearer func(string) (service.ProviderStatus, error)
type ModManager func(context.Context, service.ModManagementRequest) (service.ModManagementResult, error)
type ModRefresher func(context.Context, string) error
type CraftyStatusLoader func(context.Context) service.CraftyStatus
type CraftyConfigSetter func(context.Context, service.CraftyConfigRequest) (service.CraftyStatus, error)
type CraftyCredentialClearer func() (service.CraftyStatus, error)
type ServerControl func(context.Context) (service.CraftyStatus, error)
type PlanApplier func(context.Context, string) (service.ApplyResult, error)
type BackupRestorer func(context.Context, string) (service.RestoreResult, error)

type ServerOptions struct {
	Updates      UpdatesLoader
	Refresh      RefreshFunc
	CheckUpdates RefreshFunc
	CreatePlan   PlanCreator
	CreatePlacementPlan PlacementPlanCreator
	Plans        PlansLoader
	Plan         PlanLoader
	History      HistoryLoader
	Retention    RetentionLoader
	UpdateRetention RetentionUpdater
	Rules        RulesLoader
	SetRule      RuleSetter
	ClearRule    RuleClearer
	RefreshStatus RefreshStatusLoader
	Providers    ProvidersLoader
	SetProviderCredential   ProviderCredentialSetter
	ClearProviderCredential ProviderCredentialClearer
	ManageMod     ModManager
	RefreshMod    ModRefresher
	CraftyStatus  CraftyStatusLoader
	SetCraftyConfig CraftyConfigSetter
	ClearCraftyCredential CraftyCredentialClearer
	StartServer   ServerControl
	StopServer    ServerControl
	ApplyPlan     PlanApplier
	RestoreBackup BackupRestorer
	BackgroundContext context.Context
	Web          http.Handler
}

type Server struct {
	loader        Loader
	updatesLoader UpdatesLoader
	refresh       RefreshFunc
	checkUpdates  RefreshFunc
	createPlan    PlanCreator
	createPlacementPlan PlacementPlanCreator
	plansLoader   PlansLoader
	planLoader    PlanLoader
	historyLoader HistoryLoader
	retentionLoader RetentionLoader
	updateRetention RetentionUpdater
	rulesLoader    RulesLoader
	setRule        RuleSetter
	clearRule      RuleClearer
	refreshStatus  RefreshStatusLoader
	providersLoader ProvidersLoader
	setProviderCredential ProviderCredentialSetter
	clearProviderCredential ProviderCredentialClearer
	manageMod      ModManager
	refreshMod     ModRefresher
	craftyStatus   CraftyStatusLoader
	setCraftyConfig CraftyConfigSetter
	clearCraftyCredential CraftyCredentialClearer
	startServer    ServerControl
	stopServer     ServerControl
	applyPlan      PlanApplier
	restoreBackup  BackupRestorer
	backgroundCtx context.Context
	backgroundMu sync.Mutex
	backgroundRefresh bool
	version       string
}

func NewHandler(loader Loader, version string) http.Handler {
	return NewHandlerWithOptions(loader, version, ServerOptions{})
}

func NewHandlerWithOptions(loader Loader, version string, opts ServerOptions) http.Handler {
	backgroundCtx := opts.BackgroundContext
	if backgroundCtx == nil {
		backgroundCtx = context.Background()
	}
	server := &Server{
		loader: loader, updatesLoader: opts.Updates, refresh: opts.Refresh,
		checkUpdates: opts.CheckUpdates, createPlan: opts.CreatePlan,
		createPlacementPlan: opts.CreatePlacementPlan,
		plansLoader: opts.Plans, planLoader: opts.Plan, historyLoader: opts.History,
		retentionLoader: opts.Retention, updateRetention: opts.UpdateRetention,
		rulesLoader: opts.Rules, setRule: opts.SetRule, clearRule: opts.ClearRule,
		refreshStatus: opts.RefreshStatus,
		providersLoader: opts.Providers,
		setProviderCredential: opts.SetProviderCredential,
		clearProviderCredential: opts.ClearProviderCredential,
		manageMod: opts.ManageMod,
		refreshMod: opts.RefreshMod,
		craftyStatus: opts.CraftyStatus,
		setCraftyConfig: opts.SetCraftyConfig,
		clearCraftyCredential: opts.ClearCraftyCredential,
		startServer: opts.StartServer,
		stopServer: opts.StopServer,
		applyPlan: opts.ApplyPlan,
		restoreBackup: opts.RestoreBackup,
		backgroundCtx: backgroundCtx,
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
	mux.HandleFunc("POST /api/placement-plans", server.createPlacementPlanHandler)
	mux.HandleFunc("GET /api/plans/{id}", server.plan)
	mux.HandleFunc("GET /api/history", server.history)
	mux.HandleFunc("GET /api/settings", server.retentionSettings)
	mux.HandleFunc("PUT /api/settings", server.updateRetentionSettings)
	mux.HandleFunc("GET /api/update-rules", server.updateRules)
	mux.HandleFunc("PUT /api/update-rules", server.setUpdateRule)
	mux.HandleFunc("DELETE /api/update-rules", server.clearUpdateRule)
	mux.HandleFunc("GET /api/providers", server.providers)
	mux.HandleFunc("PUT /api/providers/{id}/credentials", server.setProviderCredentials)
	mux.HandleFunc("DELETE /api/providers/{id}/credentials", server.clearProviderCredentials)
	mux.HandleFunc("POST /api/mod-management", server.manageModHandler)
	mux.HandleFunc("POST /api/mod-metadata/refresh", server.refreshModMetadata)
	mux.HandleFunc("GET /api/crafty", server.crafty)
	mux.HandleFunc("PUT /api/crafty", server.updateCrafty)
	mux.HandleFunc("DELETE /api/crafty/credentials", server.clearCraftyCredentials)
	mux.HandleFunc("POST /api/server/start", server.startMinecraftServer)
	mux.HandleFunc("POST /api/server/stop", server.stopMinecraftServer)
	mux.HandleFunc("POST /api/plans/{id}/apply", server.applyPlanHandler)
	mux.HandleFunc("POST /api/backups/{id}/restore", server.restoreBackupHandler)
	if opts.Web != nil {
		mux.Handle("/", opts.Web)
	}
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.load(w)
	if !ok {
		return
	}
	response := struct {
		management.Status
		Version string                `json:"version"`
		Refresh service.RefreshStatus `json:"refresh"`
		Crafty  service.CraftyStatus  `json:"crafty"`
	}{
		Status: snapshot.Status,
		Version: s.version,
	}
	if s.refreshStatus != nil {
		response.Refresh = s.refreshStatus()
	}
	if s.craftyStatus != nil {
		response.Crafty = s.craftyStatus(r.Context())
		response.ServerState = response.Crafty.State
	}
	if s.applyPlan != nil {
		response.Mode = "managed"
		response.ReadOnly = false
	}
	writeJSON(w, http.StatusOK, response)
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

func (s *Server) refreshAll(w http.ResponseWriter, _ *http.Request) {
	if s.refresh == nil {
		writeError(w, http.StatusServiceUnavailable, "refresh is not configured")
		return
	}
	started := s.startBackgroundRefresh(s.refresh)
	status := "refreshing"
	if !started {
		status = "already_refreshing"
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": status})
}

func (s *Server) checkForUpdates(w http.ResponseWriter, _ *http.Request) {
	if s.checkUpdates == nil {
		writeError(w, http.StatusServiceUnavailable, "update refresh is not configured")
		return
	}
	started := s.startBackgroundRefresh(s.checkUpdates)
	status := "refreshing"
	if !started {
		status = "already_refreshing"
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": status})
}

func (s *Server) startBackgroundRefresh(refresh RefreshFunc) bool {
	if s.refreshStatus != nil && s.refreshStatus().Refreshing {
		return false
	}
	s.backgroundMu.Lock()
	if s.backgroundRefresh {
		s.backgroundMu.Unlock()
		return false
	}
	s.backgroundRefresh = true
	s.backgroundMu.Unlock()

	go func() {
		defer func() {
			s.backgroundMu.Lock()
			s.backgroundRefresh = false
			s.backgroundMu.Unlock()
		}()
		_ = refresh(s.backgroundCtx)
	}()
	return true
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

func (s *Server) createPlacementPlanHandler(w http.ResponseWriter, r *http.Request) {
	if s.createPlacementPlan == nil {
		writeError(w, http.StatusServiceUnavailable, "placement planning is not configured")
		return
	}
	var request struct {
		Path string `json:"path"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid placement plan request: "+err.Error())
		return
	}
	plan, err := s.createPlacementPlan(r.Context(), request.Path)
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

func (s *Server) updateRules(w http.ResponseWriter, _ *http.Request) {
	if s.rulesLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "update rules are not configured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": s.rulesLoader()})
}

func (s *Server) setUpdateRule(w http.ResponseWriter, r *http.Request) {
	if s.setRule == nil {
		writeError(w, http.StatusServiceUnavailable, "update rules are not configured")
		return
	}
	var request struct {
		Key  string             `json:"key"`
		Rule service.UpdateRule `json:"rule"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid update rule: "+err.Error())
		return
	}
	rule, err := s.setRule(request.Key, request.Rule)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": request.Key, "rule": rule})
}

func (s *Server) clearUpdateRule(w http.ResponseWriter, r *http.Request) {
	if s.clearRule == nil {
		writeError(w, http.StatusServiceUnavailable, "update rules are not configured")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "candidate key is required")
		return
	}
	if err := s.clearRule(key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared", "key": key})
}

func (s *Server) providers(w http.ResponseWriter, _ *http.Request) {
	if s.providersLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "provider status is not configured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.providersLoader()})
}

func (s *Server) setProviderCredentials(w http.ResponseWriter, r *http.Request) {
	if s.setProviderCredential == nil {
		writeError(w, http.StatusServiceUnavailable, "provider credential storage is not configured")
		return
	}
	var request struct {
		APIKey string `json:"api_key"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider credential: "+err.Error())
		return
	}
	status, err := s.setProviderCredential(r.Context(), r.PathValue("id"), request.APIKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) clearProviderCredentials(w http.ResponseWriter, r *http.Request) {
	if s.clearProviderCredential == nil {
		writeError(w, http.StatusServiceUnavailable, "provider credential storage is not configured")
		return
	}
	status, err := s.clearProviderCredential(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) manageModHandler(w http.ResponseWriter, r *http.Request) {
	if s.manageMod == nil {
		writeError(w, http.StatusServiceUnavailable, "mod management is not configured")
		return
	}
	var request service.ModManagementRequest
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid mod management request: "+err.Error())
		return
	}
	result, err := s.manageMod(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) refreshModMetadata(w http.ResponseWriter, r *http.Request) {
	if s.refreshMod == nil {
		writeError(w, http.StatusServiceUnavailable, "per-mod metadata refresh is not configured")
		return
	}
	var request struct {
		Path string `json:"path"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid metadata refresh request: "+err.Error())
		return
	}
	path := request.Path
	started := s.startBackgroundRefresh(func(ctx context.Context) error {
		return s.refreshMod(ctx, path)
	})
	status := "refreshing"
	if !started {
		status = "already_refreshing"
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": status})
}

func (s *Server) crafty(w http.ResponseWriter, r *http.Request) {
	if s.craftyStatus == nil {
		writeError(w, http.StatusServiceUnavailable, "Crafty integration is not configured")
		return
	}
	writeJSON(w, http.StatusOK, s.craftyStatus(r.Context()))
}

func (s *Server) updateCrafty(w http.ResponseWriter, r *http.Request) {
	if s.setCraftyConfig == nil {
		writeError(w, http.StatusServiceUnavailable, "Crafty configuration is not available")
		return
	}
	var request service.CraftyConfigRequest
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid Crafty configuration: "+err.Error())
		return
	}
	status, err := s.setCraftyConfig(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) clearCraftyCredentials(w http.ResponseWriter, _ *http.Request) {
	if s.clearCraftyCredential == nil {
		writeError(w, http.StatusServiceUnavailable, "Crafty credential storage is not available")
		return
	}
	status, err := s.clearCraftyCredential()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) startMinecraftServer(w http.ResponseWriter, _ *http.Request) {
	if s.startServer == nil {
		writeError(w, http.StatusServiceUnavailable, "Crafty start control is not configured")
		return
	}
	status, err := s.startServer(s.backgroundCtx)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func (s *Server) stopMinecraftServer(w http.ResponseWriter, _ *http.Request) {
	if s.stopServer == nil {
		writeError(w, http.StatusServiceUnavailable, "Crafty stop control is not configured")
		return
	}
	status, err := s.stopServer(s.backgroundCtx)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func (s *Server) applyPlanHandler(w http.ResponseWriter, r *http.Request) {
	if s.applyPlan == nil {
		writeError(w, http.StatusServiceUnavailable, "Apply is not configured")
		return
	}
	result, err := s.applyPlan(s.backgroundCtx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) restoreBackupHandler(w http.ResponseWriter, r *http.Request) {
	if s.restoreBackup == nil {
		writeError(w, http.StatusServiceUnavailable, "Restore is not configured")
		return
	}
	var request struct {
		Confirm bool `json:"confirm"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid restore request: "+err.Error())
		return
	}
	if !request.Confirm {
		writeError(w, http.StatusBadRequest, "restore requires explicit confirmation")
		return
	}
	result, err := s.restoreBackup(s.backgroundCtx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
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
