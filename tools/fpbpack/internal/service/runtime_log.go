package service

import (
	"fmt"
	"strings"
	"time"
)

const runtimeLogLimit = 500

type RuntimeLogEntry struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Area    string    `json:"area"`
	Message string    `json:"message"`
}

func (s *Service) logEvent(level, area, message string) {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		level = "info"
	}
	area = strings.TrimSpace(area)
	if area == "" {
		area = "service"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	s.mu.Lock()
	s.logSeq++
	entry := RuntimeLogEntry{
		ID: s.logSeq,
		Time: time.Now().UTC(),
		Level: level,
		Area: area,
		Message: message,
	}
	s.logs = append(s.logs, entry)
	if len(s.logs) > runtimeLogLimit {
		copy(s.logs, s.logs[len(s.logs)-runtimeLogLimit:])
		s.logs = s.logs[:runtimeLogLimit]
	}
	s.mu.Unlock()
}

func (s *Service) Logs(limit int) []RuntimeLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > runtimeLogLimit {
		limit = runtimeLogLimit
	}
	start := len(s.logs) - limit
	if start < 0 {
		start = 0
	}
	result := append([]RuntimeLogEntry(nil), s.logs[start:]...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func (s *Service) logRefreshFailure(kind string, err error) {
	if err != nil {
		s.logEvent("error", "refresh", fmt.Sprintf("%s refresh failed: %v", kind, err))
	}
}


func (s *Service) logModManagement(action string, result ModManagementResult, err error) {
	if err != nil {
		s.logEvent("error", "mods", fmt.Sprintf("%s failed: %v", action, err))
		return
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = action + " completed"
	}
	s.logEvent("info", "mods", message)
}
