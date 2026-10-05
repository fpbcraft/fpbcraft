package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type CraftyStatus struct {
	Configured       bool   `json:"configured"`
	Connected        bool   `json:"connected"`
	State            string `json:"state"`
	Detail           string `json:"detail,omitempty"`
	URL              string `json:"url,omitempty"`
	ServerID         string `json:"server_id,omitempty"`
	CredentialSource string `json:"credential_source,omitempty"`
	AllowInsecure    bool   `json:"allow_insecure,omitempty"`
}

type CraftyConfigRequest struct {
	URL           string `json:"url"`
	ServerID      string `json:"server_id"`
	APIToken      string `json:"api_token,omitempty"`
	AllowInsecure bool   `json:"allow_insecure,omitempty"`
}

type craftyEnvelope struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
	ErrorData any             `json:"error_data"`
}

func (s *Service) effectiveCraftyConfig() (CraftySettings, string, string) {
	s.mu.RLock()
	savedConfig := s.state.Crafty
	savedToken := strings.TrimSpace(s.secrets.CraftyAPIToken)
	s.mu.RUnlock()

	config := savedConfig
	if strings.TrimSpace(config.URL) == "" || strings.TrimSpace(config.ServerID) == "" {
		config = CraftySettings{
			URL: strings.TrimSpace(s.options.CraftyURL),
			ServerID: strings.TrimSpace(s.options.CraftyServerID),
			AllowInsecure: s.options.CraftyAllowInsecure,
		}
	}
	if savedToken != "" {
		return config, savedToken, "saved"
	}
	if token := strings.TrimSpace(s.options.CraftyToken); token != "" {
		return config, token, "environment"
	}
	return config, "", ""
}

func (s *Service) CraftyStatus(ctx context.Context) CraftyStatus {
	config, token, source := s.effectiveCraftyConfig()
	return s.craftyStatusWith(ctx, config, token, source)
}

func (s *Service) craftyStatusWith(
	ctx context.Context,
	config CraftySettings,
	token string,
	credentialSource string,
) CraftyStatus {
	status := CraftyStatus{
		Configured: strings.TrimSpace(config.URL) != "" &&
			strings.TrimSpace(config.ServerID) != "" &&
			strings.TrimSpace(token) != "",
		State: "unknown",
		URL: strings.TrimSpace(config.URL),
		ServerID: strings.TrimSpace(config.ServerID),
		CredentialSource: credentialSource,
		AllowInsecure: config.AllowInsecure,
	}
	if !status.Configured {
		status.Detail = "Configure Crafty URL, server ID, and API token before Apply/Restore."
		return status
	}

	var stats map[string]any
	if err := s.craftyRequest(ctx, config, token, http.MethodGet, "/servers/"+url.PathEscape(config.ServerID)+"/stats", &stats); err != nil {
		status.Detail = err.Error()
		return status
	}
	running, ok := craftyRunning(stats)
	if !ok {
		status.Detail = "Crafty stats response did not include a recognizable running state."
		return status
	}
	status.Connected = true
	if running {
		status.State = "running"
		status.Detail = "Minecraft server is running."
	} else {
		status.State = "stopped"
		status.Detail = "Minecraft server is stopped."
	}
	return status
}

func craftyRunning(stats map[string]any) (bool, bool) {
	for _, key := range []string{"running", "server_running", "is_running"} {
		value, exists := stats[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case float64:
			return typed != 0, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true", "running", "online", "1":
				return true, true
			case "false", "stopped", "offline", "0":
				return false, true
			}
		}
	}
	return false, false
}

func (s *Service) SetCraftyConfig(ctx context.Context, request CraftyConfigRequest) (CraftyStatus, error) {
	config := CraftySettings{
		URL: strings.TrimRight(strings.TrimSpace(request.URL), "/"),
		ServerID: strings.TrimSpace(request.ServerID),
		AllowInsecure: request.AllowInsecure,
	}
	if config.URL == "" || config.ServerID == "" {
		return CraftyStatus{}, fmt.Errorf("Crafty URL and server ID are required")
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return CraftyStatus{}, fmt.Errorf("Crafty URL must be an absolute http(s) URL")
	}

	s.mu.RLock()
	currentToken := strings.TrimSpace(s.secrets.CraftyAPIToken)
	currentState := s.state
	currentSecrets := s.secrets
	s.mu.RUnlock()
	token := strings.TrimSpace(request.APIToken)
	source := "saved"
	if token == "" {
		if currentToken != "" {
			token = currentToken
		} else if env := strings.TrimSpace(s.options.CraftyToken); env != "" {
			token = env
			source = "environment"
		}
	}
	if token == "" {
		return CraftyStatus{}, fmt.Errorf("Crafty API token is required")
	}

	validateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	status := s.craftyStatusWith(validateCtx, config, token, source)
	if !status.Connected {
		return CraftyStatus{}, fmt.Errorf("validate Crafty connection: %s", status.Detail)
	}

	nextState := currentState
	nextState.Crafty = config
	nextState.UpdatedAt = time.Now().UTC()
	nextSecrets := currentSecrets
	nextSecrets.SchemaVersion = ProviderSecretsSchemaVersion
	if strings.TrimSpace(request.APIToken) != "" {
		nextSecrets.CraftyAPIToken = strings.TrimSpace(request.APIToken)
		source = "saved"
		status.CredentialSource = source
	}

	if strings.TrimSpace(request.APIToken) != "" {
		if err := writeSecretJSONAtomic(filepath.Join(s.options.StateDir, "secrets.json"), nextSecrets); err != nil {
			return CraftyStatus{}, fmt.Errorf("persist Crafty credential: %w", err)
		}
	}
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), nextState); err != nil {
		if strings.TrimSpace(request.APIToken) != "" {
			_ = writeSecretJSONAtomic(filepath.Join(s.options.StateDir, "secrets.json"), currentSecrets)
		}
		return CraftyStatus{}, fmt.Errorf("persist Crafty settings: %w", err)
	}

	s.mu.Lock()
	s.state = nextState
	if strings.TrimSpace(request.APIToken) != "" {
		s.secrets = nextSecrets
	}
	s.mu.Unlock()
	return status, nil
}

func (s *Service) ClearCraftyCredential() (CraftyStatus, error) {
	s.mu.RLock()
	next := s.secrets
	s.mu.RUnlock()
	next.SchemaVersion = ProviderSecretsSchemaVersion
	next.CraftyAPIToken = ""
	if err := writeSecretJSONAtomic(filepath.Join(s.options.StateDir, "secrets.json"), next); err != nil {
		return CraftyStatus{}, fmt.Errorf("persist Crafty credential: %w", err)
	}
	s.mu.Lock()
	s.secrets = next
	s.mu.Unlock()
	return s.CraftyStatus(context.Background()), nil
}

func (s *Service) StartServer(ctx context.Context) (CraftyStatus, error) {
	return s.craftyAction(ctx, "start_server")
}

func (s *Service) StopServer(ctx context.Context) (CraftyStatus, error) {
	return s.craftyAction(ctx, "stop_server")
}

func (s *Service) craftyAction(ctx context.Context, action string) (CraftyStatus, error) {
	config, token, source := s.effectiveCraftyConfig()
	if strings.TrimSpace(config.URL) == "" || strings.TrimSpace(config.ServerID) == "" || strings.TrimSpace(token) == "" {
		return CraftyStatus{}, fmt.Errorf("Crafty is not configured")
	}
	desired := ""
	switch action {
	case "start_server":
		desired = "running"
	case "stop_server":
		desired = "stopped"
	default:
		return CraftyStatus{}, fmt.Errorf("unsupported Crafty server action %q", action)
	}
	if err := s.craftyRequest(
		ctx,
		config,
		token,
		http.MethodPost,
		"/servers/"+url.PathEscape(config.ServerID)+"/action/"+action,
		nil,
	); err != nil {
		return CraftyStatus{}, err
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	last := CraftyStatus{
		Configured: true,
		State: "unknown",
		URL: config.URL,
		ServerID: config.ServerID,
		CredentialSource: source,
		AllowInsecure: config.AllowInsecure,
		Detail: "Crafty accepted the server action; waiting for the requested state.",
	}
	for {
		status := s.craftyStatusWith(waitCtx, config, token, source)
		last = status
		if status.Connected && status.State == desired {
			return status, nil
		}
		select {
		case <-waitCtx.Done():
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			return last, fmt.Errorf(
				"Crafty accepted %s but the server did not reach %s state: %s",
				action,
				desired,
				last.Detail,
			)
		case <-ticker.C:
		}
	}
}

func (s *Service) requireServerStopped(ctx context.Context) error {
	status := s.CraftyStatus(ctx)
	if !status.Configured {
		return fmt.Errorf("Crafty is not configured; FPBPack cannot prove the server is stopped")
	}
	if !status.Connected {
		return fmt.Errorf("Crafty server state is unavailable: %s", status.Detail)
	}
	if status.State != "stopped" {
		return fmt.Errorf("Minecraft server must be stopped before changing live mod files (current state: %s)", status.State)
	}
	return nil
}

func (s *Service) craftyRequest(
	ctx context.Context,
	config CraftySettings,
	token string,
	method string,
	path string,
	target any,
) error {
	base := strings.TrimRight(strings.TrimSpace(config.URL), "/")
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid Crafty URL")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if config.AllowInsecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // user opt-in for Crafty's common self-signed certificate
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: transport}
	request, err := http.NewRequestWithContext(ctx, method, base+"/api/v2"+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Crafty request failed: %w", err)
	}
	defer response.Body.Close()

	var envelope craftyEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode Crafty response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || strings.EqualFold(envelope.Status, "error") {
		message := strings.TrimSpace(envelope.Error)
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("Crafty API: %s", message)
	}
	if target != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("decode Crafty data: %w", err)
		}
	}
	return nil
}
