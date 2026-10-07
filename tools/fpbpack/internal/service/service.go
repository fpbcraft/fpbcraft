package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

const (
	StateSchemaVersion = 1
	DefaultRetentionCount = 20
)

type Options struct {
	ServerRoot      string
	StateDir        string
	ServerModsPath  string
	ClientModsPath  string
	Minecraft       string
	Loader          string
	ModrinthBaseURL  string
	CurseForgeBaseURL string
	CurseForgeAPIKey string
	GitHubBaseURL     string
	GitHubToken       string
	NeoForgeBaseURL   string
	JavaExecutable    string
	CraftyURL         string
	CraftyServerID    string
	CraftyToken       string
	CraftyAllowInsecure bool
	BootstrapReport  string
}

type RuntimeSettings struct {
	RetentionCount int `json:"retention_count"`
}

type UpdateRule struct {
	PinVersion      string     `json:"pin_version,omitempty"`
	IgnoreMod       bool       `json:"ignore_mod,omitempty"`
	IgnoredVersions []string   `json:"ignored_versions,omitempty"`
	ReviewAfter     *time.Time `json:"review_after,omitempty"`
}

type CraftySettings struct {
	URL           string `json:"url,omitempty"`
	ServerID      string `json:"server_id,omitempty"`
	AllowInsecure bool   `json:"allow_insecure,omitempty"`
}

type AutoModpackManagedState struct {
	PendingPublish              bool       `json:"pending_publish,omitempty"`
	LastChangedAt               *time.Time `json:"last_changed_at,omitempty"`
	LastPublishRequestedAt      *time.Time `json:"last_publish_requested_at,omitempty"`
	PublishRequestedJournalHead int64      `json:"publish_requested_journal_head,omitempty"`
}

type State struct {
	SchemaVersion int             `json:"schema_version"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ImportedFrom  string          `json:"imported_from,omitempty"`
	Settings      RuntimeSettings       `json:"settings"`
	Crafty        CraftySettings        `json:"crafty,omitempty"`
	AutoModpack   AutoModpackManagedState `json:"automodpack,omitempty"`
	PendingChanges PendingChanges            `json:"pending_changes,omitempty"`
	UpdateRules   map[string]UpdateRule `json:"update_rules,omitempty"`
	Catalog       catalog.Report        `json:"catalog"`
}

type RefreshStatus struct {
	Refreshing  bool       `json:"refreshing"`
	Kind        string     `json:"kind,omitempty"`
	Phase       string     `json:"phase,omitempty"`
	Message     string     `json:"message,omitempty"`
	Current     int        `json:"current,omitempty"`
	Total       int        `json:"total,omitempty"`
	Percent     int        `json:"percent,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

type Service struct {
	mu            sync.RWMutex
	refreshMu     sync.Mutex
	catalogMu     sync.Mutex
	options       Options
	state         State
	snapshot      management.Snapshot
	updates       updatecheck.Report
	hasUpdate     bool
	refreshStatus RefreshStatus
	secrets       ProviderSecrets
	logSeq        uint64
	logs          []RuntimeLogEntry
}

func New(ctx context.Context, options Options) (*Service, error) {
	normalizeOptions(&options)
	if strings.TrimSpace(options.ServerRoot) == "" {
		return nil, fmt.Errorf("server root is required")
	}
	if strings.TrimSpace(options.StateDir) == "" {
		return nil, fmt.Errorf("state directory is required")
	}
	if err := os.MkdirAll(options.StateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}

	service := &Service{options: options}
	if err := service.loadOrBootstrapState(ctx); err != nil {
		return nil, err
	}
	artifactIDsChanged := catalog.EnsureManagedArtifactIDs(service.state.Catalog.Managed)
	autoModpackGroupsChanged := normalizeCatalogAutoModpackGroups(service.state.Catalog.Managed)
	placementWarningsBefore, _ := json.Marshal(service.state.Catalog.Placement)
	service.state.Catalog.RecalculateSummary()
	placementWarningsAfter, _ := json.Marshal(service.state.Catalog.Placement)
	if artifactIDsChanged || autoModpackGroupsChanged || string(placementWarningsBefore) != string(placementWarningsAfter) {
		service.state.UpdatedAt = time.Now().UTC()
		if err := service.persistState(); err != nil {
			return nil, fmt.Errorf("persist catalog state normalization: %w", err)
		}
	}
	if err := service.loadProviderSecrets(); err != nil {
		return nil, err
	}
	if service.state.Settings.RetentionCount == 0 {
		service.state.Settings.RetentionCount = DefaultRetentionCount
		service.state.UpdatedAt = time.Now().UTC()
		if err := service.persistState(); err != nil {
			return nil, fmt.Errorf("persist default settings: %w", err)
		}
	}
	service.loadRuntimeCaches()
	service.logEvent("info", "service", "FPBPack started from persisted state; automatic refresh waits for its configured interval")
	return service, nil
}

func (s *Service) loadRuntimeCaches() {
	var inv inventory.Inventory
	if err := readJSON(filepath.Join(s.options.StateDir, "inventory.json"), &inv); err == nil &&
		inv.SchemaVersion == inventory.SchemaVersion {
		s.snapshot = management.BuildSnapshot(inv, s.state.Catalog)
	}

	var report updatecheck.Report
	if err := readJSON(filepath.Join(s.options.StateDir, "updates.json"), &report); err == nil {
		s.applyUpdateRules(&report)
		s.updates = report
		s.hasUpdate = true
	}
}

func normalizeOptions(options *Options) {
	if options.ServerModsPath == "" {
		options.ServerModsPath = inventory.DefaultServerModsPath
	}
	if options.ClientModsPath == "" {
		options.ClientModsPath = inventory.DefaultClientModsPath
	}
	if options.Minecraft == "" {
		options.Minecraft = "1.21.1"
	}
	if options.Loader == "" {
		options.Loader = "neoforge"
	}
	if options.ModrinthBaseURL == "" {
		options.ModrinthBaseURL = inventory.DefaultModrinthAPI
	}
	if options.NeoForgeBaseURL == "" {
		options.NeoForgeBaseURL = DefaultNeoForgeMavenBaseURL
	}
	if options.JavaExecutable == "" {
		options.JavaExecutable = "java"
	}
}

func (s *Service) Snapshot() (management.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := s.snapshot
	snapshot.Mods = append([]management.Mod(nil), s.snapshot.Mods...)
	if s.hasUpdate && len(s.updates.Candidates) > 0 {
		icons := make(map[string]string, len(s.updates.Candidates))
		for _, candidate := range s.updates.Candidates {
			if strings.TrimSpace(candidate.IconURL) != "" {
				icons[candidate.Key] = candidate.IconURL
			}
		}
		for index := range snapshot.Mods {
			if icon := icons[snapshot.Mods[index].ID]; icon != "" {
				snapshot.Mods[index].IconURL = icon
			}
		}
	}
	return snapshot, nil
}

func (s *Service) Catalog() (catalog.Report, error) {
	return s.catalogSnapshot()
}

func (s *Service) CatalogPreview() (catalog.Report, error) {
	s.mu.RLock()
	bytes, err := json.Marshal(s.snapshot.Inventory)
	s.mu.RUnlock()
	if err != nil {
		return catalog.Report{}, err
	}
	var inv inventory.Inventory
	if err := json.Unmarshal(bytes, &inv); err != nil {
		return catalog.Report{}, err
	}
	result, err := catalog.Build(inv)
	if err != nil {
		return catalog.Report{}, err
	}
	return result.Report, nil
}

func (s *Service) RefreshStatus() RefreshStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.refreshStatus
}

func (s *Service) beginRefresh(kind, message string) {
	now := time.Now().UTC()
	s.mu.Lock()
	s.refreshStatus.Refreshing = true
	s.refreshStatus.Kind = kind
	s.refreshStatus.Phase = "starting"
	s.refreshStatus.Message = message
	s.refreshStatus.Current = 0
	s.refreshStatus.Total = 0
	s.refreshStatus.Percent = 0
	s.refreshStatus.StartedAt = &now
	s.refreshStatus.LastError = ""
	s.mu.Unlock()
	s.logEvent("info", "refresh", message)
}

func (s *Service) setRefreshProgress(phase, message string, current, total, percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	s.mu.Lock()
	if s.refreshStatus.Refreshing {
		s.refreshStatus.Phase = phase
		s.refreshStatus.Message = message
		s.refreshStatus.Current = current
		s.refreshStatus.Total = total
		s.refreshStatus.Percent = percent
	}
	s.mu.Unlock()
}

func (s *Service) finishRefresh(err error) {
	s.mu.Lock()
	s.refreshStatus.Refreshing = false
	s.refreshStatus.Phase = ""
	s.refreshStatus.Current = 0
	s.refreshStatus.Total = 0
	s.refreshStatus.StartedAt = nil
	if err != nil {
		kind := s.refreshStatus.Kind
		s.refreshStatus.Message = "Refresh failed"
		s.refreshStatus.LastError = err.Error()
		s.mu.Unlock()
		s.logRefreshFailure(kind, err)
		return
	}
	now := time.Now().UTC()
	kind := s.refreshStatus.Kind
	s.refreshStatus.Message = "Refresh complete"
	s.refreshStatus.Percent = 100
	s.refreshStatus.LastSuccess = &now
	s.refreshStatus.LastError = ""
	s.mu.Unlock()
	s.logEvent("info", "refresh", kind+" refresh completed")
}

func (s *Service) Updates() (updatecheck.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasUpdate {
		return updatecheck.Report{}, fmt.Errorf("update discovery has not completed yet")
	}
	return s.updates, nil
}

func (s *Service) catalogSnapshot() (catalog.Report, error) {
	s.mu.RLock()
	bytes, err := json.Marshal(s.state.Catalog)
	s.mu.RUnlock()
	if err != nil {
		return catalog.Report{}, err
	}
	var result catalog.Report
	if err := json.Unmarshal(bytes, &result); err != nil {
		return catalog.Report{}, err
	}
	return result, nil
}

func (s *Service) updateReportSnapshot() (updatecheck.Report, bool, error) {
	s.mu.RLock()
	if !s.hasUpdate {
		s.mu.RUnlock()
		return updatecheck.Report{}, false, nil
	}
	bytes, err := json.Marshal(s.updates)
	s.mu.RUnlock()
	if err != nil {
		return updatecheck.Report{}, false, err
	}
	var result updatecheck.Report
	if err := json.Unmarshal(bytes, &result); err != nil {
		return updatecheck.Report{}, false, err
	}
	return result, true, nil
}

func reconcileUpdateReportToCatalog(report *updatecheck.Report, current catalog.Report) {
	managed := make(map[string]catalog.Entry, len(current.Managed))
	for _, entry := range current.Managed {
		managed[catalog.EntryKey(entry)] = entry
	}
	filtered := report.Candidates[:0]
	for _, candidate := range report.Candidates {
		entry, ok := managed[candidate.Key]
		if !ok {
			continue
		}
		candidate.Deployment = entry.Deployment
		candidate.AutoModpackGroup = normalizeAutoModpackGroup(entry.Deployment, entry.AutoModpackGroup)
		inheritClientDependencyGroups(candidate.Dependencies, candidate.AutoModpackGroup)
		candidate.Side = entry.Side
		if strings.TrimSpace(entry.Name) != "" {
			candidate.Name = entry.Name
		}
		filtered = append(filtered, candidate)
	}
	report.Candidates = filtered
	report.RecalculateSummary()
}

func (s *Service) Refresh(ctx context.Context) (err error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh("full", "Refreshing inventory and provider metadata")
	defer func() { s.finishRefresh(err) }()

	s.setRefreshProgress("inventory", "Scanning installed JARs", 0, 0, 5)
	inv, err := s.scanInventory(ctx)
	if err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), inv); err != nil {
		return fmt.Errorf("write inventory cache: %w", err)
	}
	if _, err := s.reconcileCatalogWithInventory(inv); err != nil {
		return fmt.Errorf("reconcile catalog locations: %w", err)
	}

	acceptedCatalog, err := s.catalogSnapshot()
	if err != nil {
		return fmt.Errorf("snapshot accepted catalog: %w", err)
	}
	s.setRefreshProgress("providers", "Refreshing provider metadata", 0, len(acceptedCatalog.Managed), 20)
	updateCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	report := updatecheck.Discover(updateCtx, acceptedCatalog, updatecheck.Options{
		Minecraft:       s.options.Minecraft,
		Mode:            updatecheck.RefreshModeBackground,
		Loader:          s.options.Loader,
		ModrinthBaseURL: s.options.ModrinthBaseURL,
		CurseForgeBaseURL: s.options.CurseForgeBaseURL,
		CurseForgeAPIKey: func() string {
			key, _ := s.effectiveCurseForgeAPIKey()
			return key
		}(),
		GitHubBaseURL: s.options.GitHubBaseURL,
		GitHubToken: s.options.GitHubToken,
		Progress: func(current, total int, name string) {
			percent := 20
			if total > 0 {
				percent += (current * 70) / total
			}
			message := "Refreshing provider metadata"
			if strings.TrimSpace(name) != "" {
				message = "Checked " + name
			}
			s.setRefreshProgress("providers", message, current, total, percent)
		},
	})
	if err := updateCtx.Err(); err != nil {
		return fmt.Errorf("update discovery: %w", err)
	}

	s.setRefreshProgress("finalizing", "Reconciling refreshed data", 0, 0, 95)
	previousReport, hasPrevious, snapshotErr := s.updateReportSnapshot()
	if snapshotErr != nil {
		return fmt.Errorf("snapshot previous update report: %w", snapshotErr)
	}
	currentCatalog, snapshotErr := s.catalogSnapshot()
	if snapshotErr != nil {
		return fmt.Errorf("snapshot current catalog: %w", snapshotErr)
	}
	if hasPrevious {
		preserveFailedMetadata(previousReport, &report)
	}
	reconcileUpdateReportToCatalog(&report, currentCatalog)
	s.applyUpdateRules(&report)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), report); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	snapshot := management.BuildSnapshot(inv, currentCatalog)

	s.mu.Lock()
	s.snapshot = snapshot
	s.updates = report
	s.hasUpdate = true
	s.mu.Unlock()
	s.setRefreshProgress("finalizing", "Refresh complete", 0, 0, 100)
	return nil
}

func (s *Service) RefreshInventory(ctx context.Context) (err error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh("inventory", "Refreshing inventory")
	defer func() { s.finishRefresh(err) }()
	s.setRefreshProgress("inventory", "Scanning installed JARs", 0, 0, 10)

	inv, err := s.scanInventory(ctx)
	if err != nil {
		return err
	}
	if _, err := s.reconcileCatalogWithInventory(inv); err != nil {
		return fmt.Errorf("reconcile catalog locations: %w", err)
	}
	currentCatalog, snapshotErr := s.catalogSnapshot()
	if snapshotErr != nil {
		return fmt.Errorf("snapshot current catalog: %w", snapshotErr)
	}
	snapshot := management.BuildSnapshot(inv, currentCatalog)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), inv); err != nil {
		return fmt.Errorf("write inventory cache: %w", err)
	}
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
	s.setRefreshProgress("inventory", "Inventory refresh complete", 0, 0, 100)
	return nil
}

func (s *Service) CheckUpdates(ctx context.Context) (err error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh("updates", "Checking providers for updates")
	defer func() { s.finishRefresh(err) }()

	acceptedCatalog, err := s.catalogSnapshot()
	if err != nil {
		return fmt.Errorf("snapshot accepted catalog: %w", err)
	}
	s.setRefreshProgress("providers", "Checking providers for updates", 0, len(acceptedCatalog.Managed), 5)
	updateCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	report := updatecheck.Discover(updateCtx, acceptedCatalog, updatecheck.Options{
		Minecraft:       s.options.Minecraft,
		Mode:            updatecheck.RefreshModeInteractive,
		Loader:          s.options.Loader,
		ModrinthBaseURL: s.options.ModrinthBaseURL,
		CurseForgeBaseURL: s.options.CurseForgeBaseURL,
		CurseForgeAPIKey: func() string {
			key, _ := s.effectiveCurseForgeAPIKey()
			return key
		}(),
		GitHubBaseURL: s.options.GitHubBaseURL,
		GitHubToken: s.options.GitHubToken,
		Progress: func(current, total int, name string) {
			percent := 5
			if total > 0 {
				percent += (current * 90) / total
			}
			message := "Checking providers for updates"
			if strings.TrimSpace(name) != "" {
				message = "Checked " + name
			}
			s.setRefreshProgress("providers", message, current, total, percent)
		},
	})
	if err := updateCtx.Err(); err != nil {
		return fmt.Errorf("update discovery: %w", err)
	}
	previousReport, hasPrevious, snapshotErr := s.updateReportSnapshot()
	if snapshotErr != nil {
		return fmt.Errorf("snapshot previous update report: %w", snapshotErr)
	}
	if hasPrevious {
		preserveFailedMetadata(previousReport, &report)
	}
	currentCatalog, snapshotErr := s.catalogSnapshot()
	if snapshotErr != nil {
		return fmt.Errorf("snapshot current catalog: %w", snapshotErr)
	}
	reconcileUpdateReportToCatalog(&report, currentCatalog)
	s.applyUpdateRules(&report)
	s.setRefreshProgress("finalizing", "Saving update metadata", 0, 0, 97)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), report); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	s.mu.Lock()
	s.updates = report
	s.hasUpdate = true
	s.mu.Unlock()
	s.setRefreshProgress("finalizing", "Update check complete", 0, 0, 100)
	return nil
}

func (s *Service) scanInventory(ctx context.Context) (inventory.Inventory, error) {
	inv, err := inventory.Scan(inventory.ScanOptions{
		ServerRoot:     s.options.ServerRoot,
		ServerModsPath: s.options.ServerModsPath,
		ClientModsPath: s.options.ClientModsPath,
	})
	if err != nil {
		return inventory.Inventory{}, err
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	matches, lookupErr := (inventory.ModrinthClient{BaseURL: s.options.ModrinthBaseURL}).Match(lookupCtx, inv.Mods)
	if lookupErr != nil {
		inv.ModrinthError = lookupErr.Error()
		inv.RecalculateSummary()
		return inv, nil
	}
	inventory.ApplyModrinthMatches(&inv, matches)
	return inv, nil
}

func (s *Service) loadOrBootstrapState(ctx context.Context) error {
	statePath := filepath.Join(s.options.StateDir, "state.json")
	if err := readJSON(statePath, &s.state); err == nil {
		if s.state.SchemaVersion != StateSchemaVersion {
			return fmt.Errorf("unsupported state schema %d (expected %d)", s.state.SchemaVersion, StateSchemaVersion)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load state: %w", err)
	}

	if reportPath, ok, err := s.findBootstrapReport(); err != nil {
		return err
	} else if ok {
		var report catalog.Report
		if err := readJSON(reportPath, &report); err != nil {
			return fmt.Errorf("load bootstrap migration report: %w", err)
		}
		now := time.Now().UTC()
		s.state = State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt:     now,
			UpdatedAt:     now,
			ImportedFrom:  reportPath,
			Settings:      RuntimeSettings{RetentionCount: DefaultRetentionCount},
			Catalog:       report,
		}
		return s.persistState()
	}

	inv, err := s.scanInventory(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap inventory: %w", err)
	}
	if !inv.ModrinthChecked {
		return fmt.Errorf("cannot bootstrap management state without an existing migration report because exact provider lookup failed: %s", inv.ModrinthError)
	}
	result, err := catalog.Build(inv)
	if err != nil {
		return fmt.Errorf("bootstrap management state: %w", err)
	}
	now := time.Now().UTC()
	s.state = State{
		SchemaVersion: StateSchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
		Settings:      RuntimeSettings{RetentionCount: DefaultRetentionCount},
		Catalog:       result.Report,
	}
	return s.persistState()
}

func (s *Service) findBootstrapReport() (string, bool, error) {
	if strings.TrimSpace(s.options.BootstrapReport) != "" {
		path, err := filepath.Abs(s.options.BootstrapReport)
		if err != nil {
			return "", false, err
		}
		if _, err := os.Stat(path); err != nil {
			return "", false, fmt.Errorf("bootstrap migration report: %w", err)
		}
		return path, true, nil
	}

	candidates := []string{
		filepath.Join(s.options.StateDir, "migration-report.json"),
		filepath.Join(s.options.ServerRoot, "modpack", "migration-report.json"),
		filepath.Join("modpack", "migration-report.json"),
	}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, exists := seen[abs]; exists {
			continue
		}
		seen[abs] = struct{}{}
		if _, err := os.Stat(abs); err == nil {
			return abs, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, err
		}
	}
	return "", false, nil
}

func (s *Service) persistState() error {
	return writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), s.state)
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(target)
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".fpbpack-*.json")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
