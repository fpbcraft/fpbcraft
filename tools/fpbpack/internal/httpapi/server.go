package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/service"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

type Loader func() (management.Snapshot, error)
type UpdatesLoader func() (updatecheck.Report, error)
type CatalogLoader func() (catalog.Report, error)
type CatalogSearcher func(context.Context, string, string) ([]updatecheck.CatalogProject, error)
type CatalogVersionsLoader func(context.Context, string, string) ([]updatecheck.CatalogVersion, error)
type CatalogPlanCreator func(context.Context, service.CatalogPlanRequest) (planning.Plan, error)
type PendingChangesLoader func() service.PendingChanges
type PendingUpdatesStager func([]string) (service.PendingChanges, error)
type PendingCatalogStager func(context.Context, service.CatalogPlanRequest) (service.PendingChanges, error)
type PendingChangeRemover func(string) (service.PendingChanges, error)
type PendingChangesDiscarder func() (service.PendingChanges, error)
type PendingChangesReviewer func(context.Context) (planning.Plan, error)
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
type LogsLoader func(int) []service.RuntimeLogEntry
type ProvidersLoader func() []service.ProviderStatus
type ProviderCredentialSetter func(context.Context, string, string) (service.ProviderStatus, error)
type ProviderCredentialClearer func(string) (service.ProviderStatus, error)
type ModManager func(context.Context, service.ModManagementRequest) (service.ModManagementResult, error)
type ModRefresher func(context.Context, string) error
type CraftyStatusLoader func(context.Context) service.CraftyStatus
type CraftyConfigSetter func(context.Context, service.CraftyConfigRequest) (service.CraftyStatus, error)
type CraftyCredentialClearer func() (service.CraftyStatus, error)
type ServerControl func(context.Context) (service.CraftyStatus, error)
type NeoForgeStatusLoader func(context.Context) (service.NeoForgeStatus, error)
type NeoForgeChanger func(context.Context, string) (service.NeoForgeChangeResult, error)
type PlanApplier func(context.Context, string) (service.ApplyResult, error)
type BackupRestorer func(context.Context, string) (service.RestoreResult, error)
type ManualArtifactAccepter func(context.Context, string, string, io.Reader) (planning.Plan, error)
type AutoModpackStatusLoader func() (service.AutoModpackStatus, error)
type AutoModpackConfigSetter func(context.Context, service.AutoModpackConfigRequest) (service.AutoModpackStatus, error)
type AutoModpackRawConfigSetter func(context.Context, service.AutoModpackRawConfigRequest) (service.AutoModpackStatus, error)
type AutoModpackGroupFilesLoader func(string, int, int, string) (service.AutoModpackPublishedFilesPage, error)
type AutoModpackGenerationDiffLoader func(int64) (service.AutoModpackGenerationDiff, error)
type AutoModpackGroupMigrator func(context.Context, service.AutoModpackGroupMigrationRequest) (service.AutoModpackStatus, error)
type AutoModpackActionRunner func(context.Context, service.AutoModpackActionRequest) (service.AutoModpackActionResult, error)

type ServerOptions struct {
	Updates      UpdatesLoader
	Catalog      CatalogLoader
	CatalogPreview CatalogLoader
	SearchCatalog CatalogSearcher
	CatalogVersions CatalogVersionsLoader
	CreateCatalogPlan CatalogPlanCreator
	PendingChanges PendingChangesLoader
	StagePendingUpdates PendingUpdatesStager
	StagePendingCatalog PendingCatalogStager
	RemovePendingChange PendingChangeRemover
	DiscardPendingChanges PendingChangesDiscarder
	ReviewPendingChanges PendingChangesReviewer
	Refresh      RefreshFunc
	RefreshInventory RefreshFunc
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
	Logs          LogsLoader
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
	NeoForgeStatus NeoForgeStatusLoader
	ChangeNeoForge NeoForgeChanger
	ApplyPlan     PlanApplier
	RestoreBackup BackupRestorer
	AcceptManualArtifact ManualArtifactAccepter
	AutoModpackStatus AutoModpackStatusLoader
	UpdateAutoModpackConfig AutoModpackConfigSetter
	UpdateAutoModpackRawConfig AutoModpackRawConfigSetter
	AutoModpackGroupFiles AutoModpackGroupFilesLoader
	AutoModpackGenerationDiff AutoModpackGenerationDiffLoader
	MigrateAutoModpackGroup AutoModpackGroupMigrator
	RunAutoModpackAction AutoModpackActionRunner
	BackgroundContext context.Context
	Web          http.Handler
}

type Server struct {
	loader        Loader
	updatesLoader UpdatesLoader
	catalogLoader CatalogLoader
	catalogPreview CatalogLoader
	searchCatalog CatalogSearcher
	catalogVersions CatalogVersionsLoader
	createCatalogPlan CatalogPlanCreator
	pendingChanges PendingChangesLoader
	stagePendingUpdates PendingUpdatesStager
	stagePendingCatalog PendingCatalogStager
	removePendingChange PendingChangeRemover
	discardPendingChanges PendingChangesDiscarder
	reviewPendingChanges PendingChangesReviewer
	refresh       RefreshFunc
	refreshInventory RefreshFunc
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
	logsLoader     LogsLoader
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
	neoForgeStatus NeoForgeStatusLoader
	changeNeoForge NeoForgeChanger
	applyPlan      PlanApplier
	restoreBackup  BackupRestorer
	acceptManualArtifact ManualArtifactAccepter
	autoModpackStatus AutoModpackStatusLoader
	updateAutoModpackConfig AutoModpackConfigSetter
	updateAutoModpackRawConfig AutoModpackRawConfigSetter
	autoModpackGroupFiles AutoModpackGroupFilesLoader
	autoModpackGenerationDiff AutoModpackGenerationDiffLoader
	migrateAutoModpackGroup AutoModpackGroupMigrator
	runAutoModpackAction AutoModpackActionRunner
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
		loader: loader, updatesLoader: opts.Updates, catalogLoader: opts.Catalog,
		catalogPreview: opts.CatalogPreview,
		searchCatalog: opts.SearchCatalog,
		catalogVersions: opts.CatalogVersions,
		createCatalogPlan: opts.CreateCatalogPlan,
		pendingChanges: opts.PendingChanges,
		stagePendingUpdates: opts.StagePendingUpdates,
		stagePendingCatalog: opts.StagePendingCatalog,
		removePendingChange: opts.RemovePendingChange,
		discardPendingChanges: opts.DiscardPendingChanges,
		reviewPendingChanges: opts.ReviewPendingChanges,
		refresh: opts.Refresh, refreshInventory: opts.RefreshInventory,
		checkUpdates: opts.CheckUpdates, createPlan: opts.CreatePlan,
		createPlacementPlan: opts.CreatePlacementPlan,
		plansLoader: opts.Plans, planLoader: opts.Plan, historyLoader: opts.History,
		retentionLoader: opts.Retention, updateRetention: opts.UpdateRetention,
		rulesLoader: opts.Rules, setRule: opts.SetRule, clearRule: opts.ClearRule,
		refreshStatus: opts.RefreshStatus,
		logsLoader: opts.Logs,
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
		neoForgeStatus: opts.NeoForgeStatus,
		changeNeoForge: opts.ChangeNeoForge,
		applyPlan: opts.ApplyPlan,
		restoreBackup: opts.RestoreBackup,
		acceptManualArtifact: opts.AcceptManualArtifact,
		autoModpackStatus: opts.AutoModpackStatus,
		updateAutoModpackConfig: opts.UpdateAutoModpackConfig,
		updateAutoModpackRawConfig: opts.UpdateAutoModpackRawConfig,
		autoModpackGroupFiles: opts.AutoModpackGroupFiles,
		autoModpackGenerationDiff: opts.AutoModpackGenerationDiff,
		migrateAutoModpackGroup: opts.MigrateAutoModpackGroup,
		runAutoModpackAction: opts.RunAutoModpackAction,
		backgroundCtx: backgroundCtx,
		version: version,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /api/status", server.status)
	mux.HandleFunc("GET /api/inventory", server.inventory)
	mux.HandleFunc("POST /api/inventory/refresh", server.refreshInventoryHandler)
	mux.HandleFunc("GET /api/catalog", server.catalog)
	mux.HandleFunc("POST /api/catalog/preview", server.catalogPreviewHandler)
	mux.HandleFunc("GET /api/catalog/search", server.catalogSearch)
	mux.HandleFunc("GET /api/catalog/projects/{provider}/{id}/versions", server.catalogProjectVersions)
	mux.HandleFunc("POST /api/catalog/plans", server.createCatalogPlanHandler)
	mux.HandleFunc("GET /api/pending-changes", server.pendingChangesHandler)
	mux.HandleFunc("POST /api/pending-changes/updates", server.stagePendingUpdatesHandler)
	mux.HandleFunc("POST /api/pending-changes/catalog", server.stagePendingCatalogHandler)
	mux.HandleFunc("DELETE /api/pending-changes/{id}", server.removePendingChangeHandler)
	mux.HandleFunc("DELETE /api/pending-changes", server.discardPendingChangesHandler)
	mux.HandleFunc("POST /api/pending-changes/review", server.reviewPendingChangesHandler)
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
	mux.HandleFunc("GET /api/logs", server.logs)
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
	mux.HandleFunc("GET /api/neoforge", server.neoForge)
	mux.HandleFunc("POST /api/neoforge/change", server.changeNeoForgeVersion)
	mux.HandleFunc("GET /api/automodpack", server.autoModpack)
	mux.HandleFunc("PUT /api/automodpack/config", server.updateAutoModpack)
	mux.HandleFunc("PUT /api/automodpack/config/raw", server.updateAutoModpackRaw)
	mux.HandleFunc("GET /api/automodpack/groups/{id}/files", server.autoModpackGroupFilesHandler)
	mux.HandleFunc("GET /api/automodpack/generations/{sequence}/diff", server.autoModpackGenerationDiffHandler)
	mux.HandleFunc("POST /api/automodpack/groups/migrate", server.migrateAutoModpackGroupHandler)
	mux.HandleFunc("POST /api/automodpack/action", server.runAutoModpackActionHandler)
	mux.HandleFunc("POST /api/plans/{id}/apply", server.applyPlanHandler)
	mux.HandleFunc("POST /api/plans/{id}/manual-artifact", server.acceptManualArtifactHandler)
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
			ClientGroups: snapshot.Inventory.Summary.ClientGroups,
		},
		Mods: snapshot.Mods,
	})
}

type inventorySummary struct {
	Total        int `json:"total"`
	Server       int `json:"server"`
	Client       int `json:"client"`
	ClientGroups int `json:"client_groups,omitempty"`
}

func (s *Server) catalog(w http.ResponseWriter, _ *http.Request) {
	if s.catalogLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog state is not configured")
		return
	}
	report, err := s.catalogLoader()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) catalogPreviewHandler(w http.ResponseWriter, _ *http.Request) {
	if s.catalogPreview == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog preview is not configured")
		return
	}
	report, err := s.catalogPreview()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) catalogSearch(w http.ResponseWriter, r *http.Request) {
	if s.searchCatalog == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog search is not configured")
		return
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if provider == "" || query == "" {
		writeError(w, http.StatusBadRequest, "provider and q are required")
		return
	}
	projects, err := s.searchCatalog(r.Context(), provider, query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *Server) catalogProjectVersions(w http.ResponseWriter, r *http.Request) {
	if s.catalogVersions == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog version browsing is not configured")
		return
	}
	versions, err := s.catalogVersions(
		r.Context(),
		r.PathValue("provider"),
		r.PathValue("id"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (s *Server) createCatalogPlanHandler(w http.ResponseWriter, r *http.Request) {
	if s.createCatalogPlan == nil {
		writeError(w, http.StatusServiceUnavailable, "catalog planning is not configured")
		return
	}
	var request service.CatalogPlanRequest
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid catalog plan request: "+err.Error())
		return
	}
	plan, err := s.createCatalogPlan(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) pendingChangesHandler(w http.ResponseWriter, _ *http.Request) {
	if s.pendingChanges == nil {
		writeError(w, http.StatusServiceUnavailable, "pending changes are not configured")
		return
	}
	writeJSON(w, http.StatusOK, s.pendingChanges())
}

func (s *Server) stagePendingUpdatesHandler(w http.ResponseWriter, r *http.Request) {
	if s.stagePendingUpdates == nil {
		writeError(w, http.StatusServiceUnavailable, "pending update staging is not configured")
		return
	}
	var request struct {
		CandidateKeys []string `json:"candidate_keys"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid pending update request: "+err.Error())
		return
	}
	pending, err := s.stagePendingUpdates(request.CandidateKeys)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pending)
}

func (s *Server) stagePendingCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if s.stagePendingCatalog == nil {
		writeError(w, http.StatusServiceUnavailable, "pending catalog staging is not configured")
		return
	}
	var request service.CatalogPlanRequest
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid pending catalog request: "+err.Error())
		return
	}
	pending, err := s.stagePendingCatalog(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pending)
}

func (s *Server) removePendingChangeHandler(w http.ResponseWriter, r *http.Request) {
	if s.removePendingChange == nil {
		writeError(w, http.StatusServiceUnavailable, "pending changes are not configured")
		return
	}
	pending, err := s.removePendingChange(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pending)
}

func (s *Server) discardPendingChangesHandler(w http.ResponseWriter, _ *http.Request) {
	if s.discardPendingChanges == nil {
		writeError(w, http.StatusServiceUnavailable, "pending changes are not configured")
		return
	}
	pending, err := s.discardPendingChanges()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pending)
}

func (s *Server) reviewPendingChangesHandler(w http.ResponseWriter, r *http.Request) {
	if s.reviewPendingChanges == nil {
		writeError(w, http.StatusServiceUnavailable, "pending changes review is not configured")
		return
	}
	plan, err := s.reviewPendingChanges(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) refreshInventoryHandler(w http.ResponseWriter, _ *http.Request) {
	if s.refreshInventory == nil {
		writeError(w, http.StatusServiceUnavailable, "inventory refresh is not configured")
		return
	}
	started := s.startBackgroundRefresh(s.refreshInventory)
	status := "refreshing"
	if !started {
		status = "already_refreshing"
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": status})
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

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	if s.logsLoader == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime logs are not configured")
		return
	}
	limit := 500
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if value > 1000 {
			value = 1000
		}
		limit = value
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": s.logsLoader(limit)})
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

func (s *Server) neoForge(w http.ResponseWriter, r *http.Request) {
	if s.neoForgeStatus == nil {
		writeError(w, http.StatusServiceUnavailable, "NeoForge version management is not configured")
		return
	}
	status, err := s.neoForgeStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) changeNeoForgeVersion(w http.ResponseWriter, r *http.Request) {
	if s.changeNeoForge == nil {
		writeError(w, http.StatusServiceUnavailable, "NeoForge version management is not configured")
		return
	}
	var request struct {
		Version string `json:"version"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid NeoForge change request: "+err.Error())
		return
	}
	if strings.TrimSpace(request.Version) == "" {
		writeError(w, http.StatusBadRequest, "version is required")
		return
	}
	result, err := s.changeNeoForge(r.Context(), request.Version)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "must be stopped") {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
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

func (s *Server) acceptManualArtifactHandler(w http.ResponseWriter, r *http.Request) {
	if s.acceptManualArtifact == nil {
		writeError(w, http.StatusServiceUnavailable, "manual artifact verification is not configured")
		return
	}
	candidateKey := strings.TrimSpace(r.URL.Query().Get("candidate_key"))
	if candidateKey == "" {
		writeError(w, http.StatusBadRequest, "candidate_key is required")
		return
	}
	plan, err := s.acceptManualArtifact(r.Context(), r.PathValue("id"), candidateKey, r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
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


func (s *Server) autoModpack(w http.ResponseWriter, _ *http.Request) {
	if s.autoModpackStatus == nil {
		writeError(w, http.StatusServiceUnavailable, "AutoModpack integration is not configured")
		return
	}
	status, err := s.autoModpackStatus()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) updateAutoModpack(w http.ResponseWriter, r *http.Request) {
	if s.updateAutoModpackConfig == nil {
		writeError(w, http.StatusServiceUnavailable, "AutoModpack configuration management is not configured")
		return
	}
	var request service.AutoModpackConfigRequest
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid AutoModpack configuration: "+err.Error())
		return
	}
	status, err := s.updateAutoModpackConfig(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) migrateAutoModpackGroupHandler(w http.ResponseWriter, r *http.Request) {
	if s.migrateAutoModpackGroup == nil {
		writeError(w, http.StatusServiceUnavailable, "AutoModpack group migration is not configured")
		return
	}
	var request service.AutoModpackGroupMigrationRequest
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid AutoModpack group migration: "+err.Error())
		return
	}
	status, err := s.migrateAutoModpackGroup(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) runAutoModpackActionHandler(w http.ResponseWriter, r *http.Request) {
	if s.runAutoModpackAction == nil {
		writeError(w, http.StatusServiceUnavailable, "AutoModpack operations are not configured")
		return
	}
	var request service.AutoModpackActionRequest
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid AutoModpack action: "+err.Error())
		return
	}
	result, err := s.runAutoModpackAction(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}


func (s *Server) updateAutoModpackRaw(w http.ResponseWriter, r *http.Request) {
	if s.updateAutoModpackRawConfig == nil {
		writeError(w, http.StatusServiceUnavailable, "raw AutoModpack configuration management is not configured")
		return
	}
	var request service.AutoModpackRawConfigRequest
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid raw AutoModpack configuration request: "+err.Error())
		return
	}
	status, err := s.updateAutoModpackRawConfig(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) autoModpackGroupFilesHandler(w http.ResponseWriter, r *http.Request) {
	if s.autoModpackGroupFiles == nil {
		writeError(w, http.StatusServiceUnavailable, "AutoModpack content browsing is not configured")
		return
	}
	offset, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	if err != nil && strings.TrimSpace(r.URL.Query().Get("offset")) != "" {
		writeError(w, http.StatusBadRequest, "offset must be an integer")
		return
	}
	limit, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if err != nil && strings.TrimSpace(r.URL.Query().Get("limit")) != "" {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	page, err := s.autoModpackGroupFiles(r.PathValue("id"), offset, limit, r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) autoModpackGenerationDiffHandler(w http.ResponseWriter, r *http.Request) {
	if s.autoModpackGenerationDiff == nil {
		writeError(w, http.StatusServiceUnavailable, "AutoModpack generation diff is not configured")
		return
	}
	sequence, err := strconv.ParseInt(r.PathValue("sequence"), 10, 64)
	if err != nil || sequence < 1 {
		writeError(w, http.StatusBadRequest, "generation sequence must be a positive integer")
		return
	}
	diff, err := s.autoModpackGenerationDiff(sequence)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diff)
}
