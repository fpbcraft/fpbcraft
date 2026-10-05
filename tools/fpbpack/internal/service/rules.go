package service

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func (s *Service) Rules() map[string]UpdateRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneRules(s.state.UpdateRules)
}

func (s *Service) SetRule(key string, rule UpdateRule) (UpdateRule, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return UpdateRule{}, fmt.Errorf("candidate key is required")
	}
	rule.IgnoredVersions = normalizeStrings(rule.IgnoredVersions)

	s.mu.Lock()
	previousRules := cloneRules(s.state.UpdateRules)
	if s.state.UpdateRules == nil {
		s.state.UpdateRules = map[string]UpdateRule{}
	}
	if ruleEmpty(rule) {
		delete(s.state.UpdateRules, key)
	} else {
		s.state.UpdateRules[key] = rule
	}
	s.state.UpdatedAt = time.Now().UTC()
	nextState := s.state
	nextState.UpdateRules = cloneRules(s.state.UpdateRules)
	s.mu.Unlock()

	if err := writeJSONAtomic(s.stateFilePath(), nextState); err != nil {
		s.mu.Lock()
		s.state.UpdateRules = previousRules
		s.mu.Unlock()
		return UpdateRule{}, fmt.Errorf("persist update rule: %w", err)
	}

	s.mu.Lock()
	if s.hasUpdate {
		applyRulesToReport(&s.updates, s.state.UpdateRules, time.Now().UTC())
		report := s.updates
		s.mu.Unlock()
		if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), report); err != nil {
			return rule, fmt.Errorf("persist ruled update cache: %w", err)
		}
	} else {
		s.mu.Unlock()
	}
	return rule, nil
}

func (s *Service) ClearRule(key string) error {
	_, err := s.SetRule(key, UpdateRule{})
	return err
}

func (s *Service) applyUpdateRules(report *updatecheck.Report) {
	s.mu.RLock()
	rules := cloneRules(s.state.UpdateRules)
	s.mu.RUnlock()
	applyRulesToReport(report, rules, time.Now().UTC())
}

func applyRulesToReport(report *updatecheck.Report, rules map[string]UpdateRule, now time.Time) {
	for index := range report.Candidates {
		candidate := &report.Candidates[index]
		if candidate.BaseClassification != "" {
			candidate.Classification = candidate.BaseClassification
		}
		candidate.Reasons = filterRuleReasons(candidate.Reasons)

		rule, ok := rules[candidate.Key]
		if !ok || candidate.Classification == updatecheck.ClassificationUpToDate {
			continue
		}
		switch {
		case rule.IgnoreMod:
			candidate.Classification = updatecheck.ClassificationIgnored
			candidate.Reasons = append(candidate.Reasons, updatecheck.Reason{
				Code: "rule_ignore_mod",
				Message: "Updates for this mod are ignored.",
			})
		case rule.PinVersion != "" && candidate.Installed.ID == rule.PinVersion:
			candidate.Classification = updatecheck.ClassificationIgnored
			candidate.Reasons = append(candidate.Reasons, updatecheck.Reason{
				Code: "rule_pinned_version",
				Message: "The installed version is pinned.",
			})
		case candidate.Target != nil && containsString(rule.IgnoredVersions, candidate.Target.ID):
			candidate.Classification = updatecheck.ClassificationIgnored
			candidate.Reasons = append(candidate.Reasons, updatecheck.Reason{
				Code: "rule_ignore_version",
				Message: "This target version is ignored.",
			})
		case rule.ReviewAfter != nil && now.Before(rule.ReviewAfter.UTC()):
			candidate.Classification = updatecheck.ClassificationIgnored
			candidate.Reasons = append(candidate.Reasons, updatecheck.Reason{
				Code: "rule_review_later",
				Message: "Review deferred until " + rule.ReviewAfter.UTC().Format(time.RFC3339) + ".",
			})
		}
	}
	report.RecalculateSummary()
}

func cloneRules(input map[string]UpdateRule) map[string]UpdateRule {
	if len(input) == 0 {
		return map[string]UpdateRule{}
	}
	out := make(map[string]UpdateRule, len(input))
	for key, rule := range input {
		copy := rule
		copy.IgnoredVersions = append([]string(nil), rule.IgnoredVersions...)
		if rule.ReviewAfter != nil {
			when := *rule.ReviewAfter
			copy.ReviewAfter = &when
		}
		out[key] = copy
	}
	return out
}

func normalizeStrings(values []string) []string {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func filterRuleReasons(reasons []updatecheck.Reason) []updatecheck.Reason {
	result := reasons[:0]
	for _, reason := range reasons {
		if !strings.HasPrefix(reason.Code, "rule_") {
			result = append(result, reason)
		}
	}
	return result
}

func ruleEmpty(rule UpdateRule) bool {
	return rule.PinVersion == "" && !rule.IgnoreMod && len(rule.IgnoredVersions) == 0 && rule.ReviewAfter == nil
}
