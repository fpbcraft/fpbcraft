package service

import (
	"context"
	"fmt"
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
	Action     string    `json:"action"`
	Command    string    `json:"command"`
	Status     string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
	Message    string    `json:"message"`
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
	s.logEvent("info", "automodpack", "Sent server command: "+command)
	message := "Crafty accepted the AutoModpack command."
	if action == "publish" || action == "revert_confirm" {
		message += " FPBPack will keep publication pending until AutoModpack's published journal advances; the server console remains authoritative if generation is rejected."
	}
	return AutoModpackActionResult{
		Action: action,
		Command: command,
		Status: "accepted",
		RequestedAt: now,
		Message: message,
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
