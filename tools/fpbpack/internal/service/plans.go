package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

func (s *Service) CreatePlan(ctx context.Context, candidateKeys []string) (planning.Plan, error) {
	s.mu.RLock()
	snapshot := s.snapshot
	report := s.updates
	hasUpdate := s.hasUpdate
	s.mu.RUnlock()
	if !hasUpdate {
		return planning.Plan{}, fmt.Errorf("update discovery has not completed yet")
	}

	plan, err := planning.Build(candidateKeys, report, snapshot, time.Now().UTC())
	if err != nil {
		return planning.Plan{}, err
	}
	planPath := filepath.Join(s.options.StateDir, "plans", plan.ID+".json")
	var existing planning.Plan
	if err := readJSON(planPath, &existing); err == nil && existing.Verified {
		return existing, nil
	} else if err != nil && !os.IsNotExist(err) {
		return planning.Plan{}, fmt.Errorf("read existing plan: %w", err)
	}

	s.verifyPlanArtifacts(ctx, &plan)
	if err := s.createRestorePoint(&plan); err != nil {
		plan.Status = planning.StatusBlocked
		plan.Blockers = append(plan.Blockers, planning.Finding{
			Code: "backup_creation_failed",
			Message: err.Error(),
		})
	}
	if err := writeJSONAtomic(planPath, plan); err != nil {
		return planning.Plan{}, fmt.Errorf("persist plan: %w", err)
	}
	event := plan.HistoryEvent()
	eventPath := filepath.Join(
		s.options.StateDir,
		"history",
		plan.CreatedAt.UTC().Format("20060102T150405.000000000Z")+"-"+plan.ID+".json",
	)
	if err := writeJSONAtomic(eventPath, event); err != nil {
		return planning.Plan{}, fmt.Errorf("persist plan history: %w", err)
	}
	return plan, nil
}

func (s *Service) Plans() ([]planning.Summary, error) {
	dir := filepath.Join(s.options.StateDir, "plans")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []planning.Summary{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]planning.Summary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var plan planning.Plan
		if err := readJSON(filepath.Join(dir, entry.Name()), &plan); err != nil {
			return nil, fmt.Errorf("read plan %s: %w", entry.Name(), err)
		}
		result = append(result, plan.Summary())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (s *Service) Plan(id string) (planning.Plan, error) {
	if !validPlanID(id) {
		return planning.Plan{}, planning.ErrNotFound
	}
	var plan planning.Plan
	if err := readJSON(filepath.Join(s.options.StateDir, "plans", id+".json"), &plan); err != nil {
		if os.IsNotExist(err) {
			return planning.Plan{}, planning.ErrNotFound
		}
		return planning.Plan{}, err
	}
	return plan, nil
}

func (s *Service) History() ([]planning.HistoryEvent, error) {
	dir := filepath.Join(s.options.StateDir, "history")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []planning.HistoryEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]planning.HistoryEvent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var event planning.HistoryEvent
		if err := readJSON(filepath.Join(dir, entry.Name()), &event); err != nil {
			return nil, fmt.Errorf("read history event %s: %w", entry.Name(), err)
		}
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func validPlanID(id string) bool {
	if !strings.HasPrefix(id, "plan-") || len(id) != len("plan-")+16 {
		return false
	}
	for _, char := range strings.TrimPrefix(id, "plan-") {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
