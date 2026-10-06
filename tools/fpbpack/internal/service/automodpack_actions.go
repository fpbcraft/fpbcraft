package service

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AutoModpackActionRequest struct {
	Action   string `json:"action"`
	Notes    string `json:"notes,omitempty"`
	Sequence int64  `json:"sequence,omitempty"`
}

type AutoModpackActionResult struct {
	Action      string    `json:"action"`
	Command     string    `json:"command"`
	Status      string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
	Message     string    `json:"message"`
	Output      []string  `json:"output"`
	OutputError string    `json:"output_error,omitempty"`
}

func (s *Service) RunAutoModpackAction(ctx context.Context, request AutoModpackActionRequest) (AutoModpackActionResult, error) {
	action := strings.TrimSpace(strings.ToLower(request.Action))
	command := ""
	notes := sanitizeAutoModpackCommandText(request.Notes)
	switch action {
	case "reload":
		command = "automodpack config reload"
	case "host_restart":
		command = "automodpack host restart"
	case "preview":
		command = "automodpack generate preview"
		if notes != "" {
			command += " notes " + notes
		}
	case "publish":
		command = "automodpack generate"
		if notes != "" {
			command += " notes " + notes
		}
	case "revert_preview":
		if request.Sequence < 1 {
			return AutoModpackActionResult{}, fmt.Errorf("generation sequence must be positive")
		}
		command = fmt.Sprintf("automodpack generate revert %d", request.Sequence)
	case "revert_confirm":
		if request.Sequence < 1 {
			return AutoModpackActionResult{}, fmt.Errorf("generation sequence must be positive")
		}
		command = fmt.Sprintf("automodpack generate revert %d confirm", request.Sequence)
		if notes != "" {
			command += " notes " + notes
		}
	case "groups":
		command = "automodpack groups"
	case "host_activity":
		command = "automodpack host activity"
	default:
		return AutoModpackActionResult{}, fmt.Errorf("unsupported AutoModpack action %q", request.Action)
	}

	publishBaseline := int64(0)
	if action == "publish" || action == "revert_confirm" {
		_, _, publishBaseline, _ = s.autoModpackPublishedContent()
	}

	status := s.CraftyStatus(ctx)
	if !status.Configured {
		return AutoModpackActionResult{}, fmt.Errorf("Crafty is not configured")
	}
	if !status.Connected {
		return AutoModpackActionResult{}, fmt.Errorf("Crafty server state is unavailable: %s", status.Detail)
	}
	if status.State != "running" {
		return AutoModpackActionResult{}, fmt.Errorf("Minecraft server must be running to execute AutoModpack console commands")
	}

	config, token, _ := s.effectiveCraftyConfig()
	beforeLines, beforeErr := s.craftyTerminalLines(ctx, config, token)
	if err := s.craftyRequestWithBody(
		ctx,
		config,
		token,
		http.MethodPost,
		"/servers/"+url.PathEscape(config.ServerID)+"/stdin",
		map[string]string{"command": command},
		nil,
	); err != nil {
		return AutoModpackActionResult{}, err
	}

	now := time.Now().UTC()
	if action == "publish" || action == "revert_confirm" {
		s.mu.Lock()
		s.state.AutoModpack.LastPublishRequestedAt = &now
		s.state.AutoModpack.PublishRequestedJournalHead = publishBaseline
		s.state.UpdatedAt = now
		err := s.persistState()
		s.mu.Unlock()
		if err != nil {
			return AutoModpackActionResult{}, fmt.Errorf("persist AutoModpack publish request: %w", err)
		}
	}
	output := []string{}
	outputErr := ""
	if beforeErr != nil {
		outputErr = beforeErr.Error()
	} else {
		captured, captureErr := s.captureCraftyCommandOutput(ctx, config, token, beforeLines)
		if captureErr != nil {
			outputErr = captureErr.Error()
		} else {
			output = captured
		}
	}
	s.logEvent("info", "automodpack", "Sent server command: "+command)
	message := "AutoModpack command submitted through Crafty."
	if action == "publish" || action == "revert_confirm" {
		message += " FPBPack will keep publication pending until AutoModpack's published journal advances; the server console remains authoritative if generation is rejected."
	}
	return AutoModpackActionResult{
		Action: action,
		Command: command,
		Status: "accepted",
		RequestedAt: now,
		Message: message,
		Output: output,
		OutputError: outputErr,
	}, nil
}

func sanitizeAutoModpackCommandText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.TrimSpace(value)
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}


func (s *Service) craftyTerminalLines(ctx context.Context, config CraftySettings, token string) ([]string, error) {
	var lines []string
	if err := s.craftyRequest(
		ctx,
		config,
		token,
		http.MethodGet,
		"/servers/"+url.PathEscape(config.ServerID)+"/logs",
		&lines,
	); err != nil {
		return nil, fmt.Errorf("Crafty terminal output unavailable: %w", err)
	}
	for index := range lines {
		lines[index] = html.UnescapeString(lines[index])
	}
	return lines, nil
}

func (s *Service) captureCraftyCommandOutput(
	ctx context.Context,
	config CraftySettings,
	token string,
	before []string,
) ([]string, error) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(4 * time.Second)
	defer timeout.Stop()

	latest := []string{}
	stablePolls := 0
	for {
		select {
		case <-ctx.Done():
			return latest, ctx.Err()
		case <-timeout.C:
			return capAutoModpackOutput(latest), nil
		case <-ticker.C:
			after, err := s.craftyTerminalLines(ctx, config, token)
			if err != nil {
				return latest, err
			}
			current := terminalLinesAfter(before, after)
			if len(current) == 0 {
				continue
			}
			if stringSlicesEqual(current, latest) {
				stablePolls++
			} else {
				latest = current
				stablePolls = 0
			}
			if stablePolls >= 2 {
				return capAutoModpackOutput(latest), nil
			}
		}
	}
}

func terminalLinesAfter(before, after []string) []string {
	maxOverlap := len(before)
	if len(after) < maxOverlap {
		maxOverlap = len(after)
	}
	for overlap := maxOverlap; overlap >= 0; overlap-- {
		start := len(before) - overlap
		matched := true
		for index := 0; index < overlap; index++ {
			if before[start+index] != after[index] {
				matched = false
				break
			}
		}
		if matched {
			return append([]string{}, after[overlap:]...)
		}
	}
	return append([]string{}, after...)
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func capAutoModpackOutput(lines []string) []string {
	const maxLines = 100
	if len(lines) <= maxLines {
		return append([]string{}, lines...)
	}
	return append([]string{}, lines[len(lines)-maxLines:]...)
}
