package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

const PendingChangesSchemaVersion = 1

type PendingChange struct {
	ID               string              `json:"id"`
	Action           string              `json:"action"`
	CandidateKey     string              `json:"candidate_key"`
	Name             string              `json:"name"`
	Provider         string              `json:"provider,omitempty"`
	ProjectID        string              `json:"project_id,omitempty"`
	Path             string              `json:"path,omitempty"`
	InstalledVersion string              `json:"installed_version,omitempty"`
	TargetVersion    string              `json:"target_version,omitempty"`
	Placement        inventory.Location  `json:"placement,omitempty"`
	AutoModpackGroup string              `json:"automodpack_group,omitempty"`
	CatalogRequest   *CatalogPlanRequest `json:"catalog_request,omitempty"`
}

type PendingChanges struct {
	SchemaVersion  int             `json:"schema_version"`
	Revision       uint64          `json:"revision"`
	UpdatedAt      *time.Time      `json:"updated_at,omitempty"`
	ReviewedPlanID string          `json:"reviewed_plan_id,omitempty"`
	Changes        []PendingChange `json:"changes"`
}

func (s *Service) PendingChanges() PendingChanges {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clonePendingChanges(s.state.PendingChanges)
}

func (s *Service) StagePendingUpdates(candidateKeys []string) (PendingChanges, error) {
	keys := normalizePendingKeys(candidateKeys)
	if len(keys) == 0 {
		return PendingChanges{}, fmt.Errorf("at least one update candidate is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasUpdate {
		return PendingChanges{}, fmt.Errorf("update discovery has not completed yet")
	}

	candidates := make(map[string]updatecheck.Candidate, len(s.updates.Candidates))
	for _, candidate := range s.updates.Candidates {
		candidates[candidate.Key] = candidate
	}
	for _, key := range keys {
		candidate, ok := candidates[key]
		if !ok {
			return PendingChanges{}, fmt.Errorf("update candidate %q is no longer available", key)
		}
		if candidate.Classification != updatecheck.ClassificationSafe &&
			candidate.Classification != updatecheck.ClassificationReview {
			return PendingChanges{}, fmt.Errorf("%s cannot be added to pending changes because it is %s", candidate.Name, candidate.Classification)
		}
		if candidate.Target == nil {
			return PendingChanges{}, fmt.Errorf("%s does not have a resolved target release", candidate.Name)
		}
		change := pendingChangeFromCandidate(candidate, "update", nil)
		s.state.PendingChanges.Changes = upsertPendingChange(s.state.PendingChanges.Changes, change)
	}
	if err := s.persistPendingChangesLocked(); err != nil {
		return PendingChanges{}, err
	}
	return clonePendingChanges(s.state.PendingChanges), nil
}

func (s *Service) StagePendingCatalogChange(
	ctx context.Context,
	request CatalogPlanRequest,
) (PendingChanges, error) {
	if !s.refreshMu.TryLock() {
		return PendingChanges{}, fmt.Errorf("provider refresh is in progress; retry after it finishes")
	}
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	s.mu.RLock()
	cat := s.state.Catalog
	s.mu.RUnlock()
	candidate, normalized, err := s.catalogCandidateForPlanRequest(ctx, request, cat)
	if err != nil {
		return PendingChanges{}, err
	}
	change := pendingChangeFromCandidate(candidate, normalized.Action, &normalized)
	if normalized.Path != "" {
		change.Path = normalized.Path
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.PendingChanges.Changes = upsertPendingChange(s.state.PendingChanges.Changes, change)
	if err := s.persistPendingChangesLocked(); err != nil {
		return PendingChanges{}, err
	}
	return clonePendingChanges(s.state.PendingChanges), nil
}

func (s *Service) RemovePendingChange(id string) (PendingChanges, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PendingChanges{}, fmt.Errorf("pending change ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]PendingChange, 0, len(s.state.PendingChanges.Changes))
	found := false
	for _, change := range s.state.PendingChanges.Changes {
		if change.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, change)
	}
	if !found {
		return PendingChanges{}, fmt.Errorf("pending change %q was not found", id)
	}
	s.state.PendingChanges.Changes = filtered
	if err := s.persistPendingChangesLocked(); err != nil {
		return PendingChanges{}, err
	}
	return clonePendingChanges(s.state.PendingChanges), nil
}

func (s *Service) DiscardPendingChanges() (PendingChanges, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision := s.state.PendingChanges.Revision + 1
	now := time.Now().UTC()
	s.state.PendingChanges = PendingChanges{
		SchemaVersion: PendingChangesSchemaVersion,
		Revision:      revision,
		UpdatedAt:     &now,
		Changes:       []PendingChange{},
	}
	s.state.UpdatedAt = now
	if err := s.persistState(); err != nil {
		return PendingChanges{}, fmt.Errorf("persist discarded pending changes: %w", err)
	}
	return clonePendingChanges(s.state.PendingChanges), nil
}

func (s *Service) ReviewPendingChanges(ctx context.Context) (planning.Plan, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	s.mu.RLock()
	pending := clonePendingChanges(s.state.PendingChanges)
	snapshot := s.snapshot
	cat := s.state.Catalog
	report := s.updates
	hasUpdate := s.hasUpdate
	s.mu.RUnlock()
	if len(pending.Changes) == 0 {
		return planning.Plan{}, fmt.Errorf("there are no pending changes to review")
	}

	cached := make(map[string]updatecheck.Candidate, len(report.Candidates))
	if hasUpdate {
		for _, candidate := range report.Candidates {
			cached[candidate.Key] = candidate
		}
	}

	selected := make([]string, 0, len(pending.Changes))
	candidates := make([]updatecheck.Candidate, 0, len(pending.Changes))
	for _, change := range pending.Changes {
		var candidate updatecheck.Candidate
		if change.Action == "update" {
			if !hasUpdate {
				return planning.Plan{}, fmt.Errorf("update discovery has not completed yet")
			}
			current, ok := cached[change.CandidateKey]
			if !ok {
				return planning.Plan{}, fmt.Errorf("%s is no longer present in the current update report; remove or restage that pending change", change.Name)
			}
			candidate = current
		} else {
			if change.CatalogRequest == nil {
				return planning.Plan{}, fmt.Errorf("pending %s change for %s is missing its catalog request", change.Action, change.Name)
			}
			resolved, _, err := s.catalogCandidateForPlanRequest(ctx, *change.CatalogRequest, cat)
			if err != nil {
				return planning.Plan{}, fmt.Errorf("resolve pending %s for %s: %w", change.Action, change.Name, err)
			}
			candidate = resolved
		}
		selected = append(selected, candidate.Key)
		candidates = append(candidates, candidate)
	}

	// Reverse-dependency checks are based on the live pack. When both a
	// dependency and its dependent are being removed in this same transaction,
	// that dependent no longer blocks the dependency removal.
	removedNames := map[string]struct{}{}
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate.Intent), "remove") {
			removedNames[strings.ToLower(strings.TrimSpace(candidate.Name))] = struct{}{}
		}
	}
	for index := range candidates {
		if !strings.EqualFold(strings.TrimSpace(candidates[index].Intent), "remove") {
			continue
		}
		filtered := candidates[index].RequiredBy[:0]
		for _, name := range candidates[index].RequiredBy {
			if _, removed := removedNames[strings.ToLower(strings.TrimSpace(name))]; removed {
				continue
			}
			filtered = append(filtered, name)
		}
		candidates[index].RequiredBy = filtered
	}

	now := time.Now().UTC()
	exactReport := updatecheck.Report{
		GeneratedAt: now,
		Minecraft:   s.options.Minecraft,
		Loader:      s.options.Loader,
		Candidates:  candidates,
	}
	exactReport.RecalculateSummary()

	// Refuse to freeze a review if the editable set changed while provider
	// metadata was being resolved.
	s.mu.RLock()
	currentRevision := s.state.PendingChanges.Revision
	s.mu.RUnlock()
	if currentRevision != pending.Revision {
		return planning.Plan{}, fmt.Errorf("pending changes changed while review was being prepared; review them again")
	}

	plan, err := planning.Build(selected, exactReport, snapshot, now)
	if err != nil {
		return planning.Plan{}, err
	}
	plan, err = s.persistPlannedChange(ctx, plan)
	if err != nil {
		return planning.Plan{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.PendingChanges.Revision != pending.Revision {
		return planning.Plan{}, fmt.Errorf("pending changes changed while the verified review was being created; review the current changes again")
	}
	s.state.PendingChanges.ReviewedPlanID = plan.ID
	s.state.PendingChanges.Revision++
	reviewedAt := time.Now().UTC()
	s.state.PendingChanges.UpdatedAt = &reviewedAt
	s.state.UpdatedAt = reviewedAt
	if err := s.persistState(); err != nil {
		return planning.Plan{}, fmt.Errorf("persist reviewed pending changes: %w", err)
	}
	return plan, nil
}

func pendingChangeFromCandidate(
	candidate updatecheck.Candidate,
	action string,
	request *CatalogPlanRequest,
) PendingChange {
	installed := candidate.Installed.Number
	if installed == "" {
		installed = candidate.Installed.Name
	}
	target := ""
	if candidate.Target != nil {
		target = candidate.Target.Number
		if target == "" {
			target = candidate.Target.Name
		}
	}
	if action == "remove" {
		target = "removed"
	}
	return PendingChange{
		ID:               candidate.Key,
		Action:           action,
		CandidateKey:     candidate.Key,
		Name:             candidate.Name,
		Provider:         candidate.Provider,
		ProjectID:        candidate.ProjectID,
		InstalledVersion: installed,
		TargetVersion:    target,
		Placement:        candidate.Deployment,
		AutoModpackGroup: candidate.AutoModpackGroup,
		CatalogRequest:   request,
	}
}

func upsertPendingChange(changes []PendingChange, replacement PendingChange) []PendingChange {
	result := make([]PendingChange, 0, len(changes)+1)
	for _, change := range changes {
		if change.ID == replacement.ID {
			continue
		}
		result = append(result, change)
	}
	result = append(result, replacement)
	sort.Slice(result, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(result[i].Name))
		right := strings.ToLower(strings.TrimSpace(result[j].Name))
		if left != right {
			return left < right
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func normalizePendingKeys(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *Service) persistPendingChangesLocked() error {
	now := time.Now().UTC()
	if s.state.PendingChanges.SchemaVersion == 0 {
		s.state.PendingChanges.SchemaVersion = PendingChangesSchemaVersion
	}
	s.state.PendingChanges.Revision++
	s.state.PendingChanges.UpdatedAt = &now
	s.state.PendingChanges.ReviewedPlanID = ""
	if s.state.PendingChanges.Changes == nil {
		s.state.PendingChanges.Changes = []PendingChange{}
	}
	s.state.UpdatedAt = now
	if err := s.persistState(); err != nil {
		return fmt.Errorf("persist pending changes: %w", err)
	}
	return nil
}

func clonePendingChanges(value PendingChanges) PendingChanges {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = PendingChangesSchemaVersion
	}
	value.Changes = append([]PendingChange(nil), value.Changes...)
	for index := range value.Changes {
		if value.Changes[index].CatalogRequest != nil {
			request := *value.Changes[index].CatalogRequest
			value.Changes[index].CatalogRequest = &request
		}
	}
	if value.Changes == nil {
		value.Changes = []PendingChange{}
	}
	return value
}

