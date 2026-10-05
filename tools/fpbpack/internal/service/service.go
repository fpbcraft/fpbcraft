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

type State struct {
	SchemaVersion int             `json:"schema_version"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ImportedFrom  string          `json:"imported_from,omitempty"`
	Settings      RuntimeSettings       `json:"settings"`
	Crafty        CraftySettings        `json:"crafty,omitempty"`
	UpdateRules   map[string]UpdateRule `json:"update_rules,omitempty"`
	Catalog       catalog.Report        `json:"catalog"`
}

type RefreshStatus struct {
	Refreshing  bool       `json:"refreshing"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

type Service struct {
	mu            sync.RWMutex
	refreshMu     sync.Mutex
	options       Options
	state         State
	snapshot      management.Snapshot
	updates       updatecheck.Report
	hasUpdate     bool
	refreshStatus RefreshStatus
	secrets       ProviderSecrets
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
	placementWarningsBefore := len(service.state.Catalog.Placement)
	service.state.Catalog.RecalculateSummary()
	if artifactIDsChanged || len(service.state.Catalog.Placement) != placementWarningsBefore {
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
}

func (s *Service) Snapshot() (management.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot, nil
}

func (s *Service) RefreshStatus() RefreshStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.refreshStatus
}

func (s *Service) beginRefresh() {
	s.mu.Lock()
	s.refreshStatus.Refreshing = true
	s.refreshStatus.LastError = ""
	s.mu.Unlock()
}

func (s *Service) finishRefresh(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshStatus.Refreshing = false
	if err != nil {
		s.refreshStatus.LastError = err.Error()
		return
	}
	now := time.Now().UTC()
	s.refreshStatus.LastSuccess = &now
	s.refreshStatus.LastError = ""
}

func (s *Service) Updates() (updatecheck.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasUpdate {
		return updatecheck.Report{}, fmt.Errorf("update discovery has not completed yet")
	}
	return s.updates, nil
}

func (s *Service) Refresh(ctx context.Context) (err error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh()
	defer func() { s.finishRefresh(err) }()

	inv, err := s.scanInventory(ctx)
	if err != nil {
		return err
	}
	snapshot := management.BuildSnapshot(inv, s.state.Catalog)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), inv); err != nil {
		return fmt.Errorf("write inventory cache: %w", err)
	}

	updateCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	report := updatecheck.Discover(updateCtx, s.state.Catalog, updatecheck.Options{
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
	})
	if err := updateCtx.Err(); err != nil {
		return fmt.Errorf("update discovery: %w", err)
	}
	s.mu.RLock()
	previousReport := s.updates
	hasPrevious := s.hasUpdate
	s.mu.RUnlock()
	if hasPrevious {
		preserveFailedMetadata(previousReport, &report)
	}
	s.applyUpdateRules(&report)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), report); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}

	s.mu.Lock()
	s.snapshot = snapshot
	s.updates = report
	s.hasUpdate = true
	s.mu.Unlock()
	return nil
}

func (s *Service) RefreshInventory(ctx context.Context) (err error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh()
	defer func() { s.finishRefresh(err) }()

	inv, err := s.scanInventory(ctx)
	if err != nil {
		return err
	}
	snapshot := management.BuildSnapshot(inv, s.state.Catalog)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), inv); err != nil {
		return fmt.Errorf("write inventory cache: %w", err)
	}
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
	return nil
}

func (s *Service) CheckUpdates(ctx context.Context) (err error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh()
	defer func() { s.finishRefresh(err) }()

	updateCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	report := updatecheck.Discover(updateCtx, s.state.Catalog, updatecheck.Options{
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
	})
	if err := updateCtx.Err(); err != nil {
		return fmt.Errorf("update discovery: %w", err)
	}
	s.mu.RLock()
	previousReport := s.updates
	hasPrevious := s.hasUpdate
	s.mu.RUnlock()
	if hasPrevious {
		preserveFailedMetadata(previousReport, &report)
	}
	s.applyUpdateRules(&report)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), report); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	s.mu.Lock()
	s.updates = report
	s.hasUpdate = true
	s.mu.Unlock()
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
