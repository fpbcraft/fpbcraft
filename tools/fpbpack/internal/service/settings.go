package service

import (
	"fmt"
	"path/filepath"
	"time"
)

func (s *Service) Settings() RuntimeSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	settings := s.state.Settings
	if settings.RetentionCount == 0 {
		settings.RetentionCount = DefaultRetentionCount
	}
	return settings
}

func (s *Service) UpdateSettings(settings RuntimeSettings) (RuntimeSettings, error) {
	if settings.RetentionCount < 1 || settings.RetentionCount > 100 {
		return RuntimeSettings{}, fmt.Errorf("retention_count must be between 1 and 100")
	}

	s.mu.Lock()
	previous := s.state
	s.state.Settings = settings
	s.state.UpdatedAt = time.Now().UTC()
	next := s.state
	s.mu.Unlock()

	if err := writeJSONAtomic(s.stateFilePath(), next); err != nil {
		s.mu.Lock()
		s.state = previous
		s.mu.Unlock()
		return RuntimeSettings{}, fmt.Errorf("persist settings: %w", err)
	}
	if err := s.pruneHistory(settings.RetentionCount); err != nil {
		return settings, fmt.Errorf("apply retention: %w", err)
	}
	return settings, nil
}

func (s *Service) stateFilePath() string {
	return filepath.Join(s.options.StateDir, "state.json")
}
