package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DefaultNeoForgeMavenBaseURL = "https://maven.neoforged.net/releases/net/neoforged/neoforge"

var (
	neoForgeArgsPattern  = regexp.MustCompile(`(?i)(libraries[\\/]+net[\\/]+neoforged[\\/]+neoforge[\\/]+)([^\\/[:space:]]+)([\\/]+(unix|win)_args\.txt)`)
	neoForgeJarPattern   = regexp.MustCompile(`(?i)(libraries[\\/]+net[\\/]+neoforged[\\/]+neoforge[\\/]+)([^\\/[:space:]]+)([\\/]+neoforge-)([^\\/[:space:]]+)(-server\.jar)`)
	validNeoForgeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-+][A-Za-z0-9._-]+)?$`)

)

type NeoForgeVersion struct {
	Version string `json:"version"`
	Channel string `json:"channel"`
	Current bool   `json:"current,omitempty"`
}

type NeoForgeStatus struct {
	Minecraft      string            `json:"minecraft"`
	CurrentVersion string            `json:"current_version,omitempty"`
	LatestVersion  string            `json:"latest_version,omitempty"`
	Versions       []NeoForgeVersion `json:"versions"`
	ServerState    string            `json:"server_state"`
	Detail         string            `json:"detail,omitempty"`
}

type NeoForgeChangeResult struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Direction   string `json:"direction"`
}

type neoForgeMavenMetadata struct {
	Versioning struct {
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
	} `xml:"versioning"`
}

type craftyServerConfig struct {
	ExecutionCommand string
	Executable       string
}

func (s *Service) NeoForgeStatus(ctx context.Context) (NeoForgeStatus, error) {
	versions, err := s.listNeoForgeVersions(ctx)
	if err != nil {
		return NeoForgeStatus{}, err
	}

	current, currentDetail := s.detectNeoForgeVersion(ctx)
	for i := range versions {
		versions[i].Current = versions[i].Version == current
	}
	latest := ""
	for _, version := range versions {
		if version.Channel == "release" {
			latest = version.Version
			break
		}
	}
	if latest == "" && len(versions) > 0 {
		latest = versions[0].Version
	}

	crafty := s.CraftyStatus(ctx)
	detail := currentDetail
	if detail == "" {
		detail = crafty.Detail
	}
	return NeoForgeStatus{
		Minecraft:      s.options.Minecraft,
		CurrentVersion: current,
		LatestVersion:  latest,
		Versions:       versions,
		ServerState:    crafty.State,
		Detail:         detail,
	}, nil
}

func (s *Service) ChangeNeoForge(ctx context.Context, target string) (NeoForgeChangeResult, error) {
	target = strings.TrimSpace(target)
	if !validNeoForgeVersion.MatchString(target) {
		return NeoForgeChangeResult{}, fmt.Errorf("invalid NeoForge version %q", target)
	}
	prefix, err := neoForgeSeriesPrefix(s.options.Minecraft)
	if err != nil {
		return NeoForgeChangeResult{}, err
	}
	if !strings.HasPrefix(target, prefix) {
		return NeoForgeChangeResult{}, fmt.Errorf(
			"NeoForge %s is not compatible with Minecraft %s; expected the %s series",
			target,
			s.options.Minecraft,
			strings.TrimSuffix(prefix, "."),
		)
	}

	if !s.refreshMu.TryLock() {
		return NeoForgeChangeResult{}, fmt.Errorf("provider refresh is in progress; retry the NeoForge change after it finishes")
	}
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	versions, err := s.listNeoForgeVersions(ctx)
	if err != nil {
		return NeoForgeChangeResult{}, fmt.Errorf("load NeoForge versions: %w", err)
	}
	found := false
	for _, version := range versions {
		if version.Version == target {
			found = true
			break
		}
	}
	if !found {
		return NeoForgeChangeResult{}, fmt.Errorf(
			"NeoForge %s was not found in the official Maven metadata for Minecraft %s",
			target,
			s.options.Minecraft,
		)
	}

	if err := s.requireServerStopped(ctx); err != nil {
		return NeoForgeChangeResult{}, err
	}

	currentConfig, err := s.loadCraftyServerConfig(ctx)
	if err != nil {
		return NeoForgeChangeResult{}, fmt.Errorf("load Crafty server configuration: %w", err)
	}
	current := neoForgeVersionFromText(currentConfig.ExecutionCommand)
	if current == "" {
		current = neoForgeVersionFromText(currentConfig.Executable)
	}
	if current == "" {
		current, _ = s.detectNeoForgeVersion(ctx)
	}
	if current == "" {
		return NeoForgeChangeResult{}, fmt.Errorf(
			"could not determine the active NeoForge version from Crafty or the server files",
		)
	}

	nextCommand, err := rewriteNeoForgeExecutionCommand(currentConfig.ExecutionCommand, target)
	if err != nil {
		return NeoForgeChangeResult{}, err
	}
	nextExecutable := rewriteNeoForgeExecutable(currentConfig.Executable, target)
	if strings.TrimSpace(nextExecutable) == "" || nextExecutable == currentConfig.Executable {
		nextExecutable = filepath.ToSlash(filepath.Join(
			"libraries",
			"net",
			"neoforged",
			"neoforge",
			target,
			"neoforge-"+target+"-server.jar",
		))
	}

	installer, err := s.downloadNeoForgeInstaller(ctx, target)
	if err != nil {
		return NeoForgeChangeResult{}, err
	}

	restoreJVMArgs, err := s.preserveUserJVMArgs()
	if err != nil {
		return NeoForgeChangeResult{}, err
	}
	if err := s.runNeoForgeInstaller(ctx, installer); err != nil {
		_ = restoreJVMArgs()
		return NeoForgeChangeResult{}, err
	}
	if err := restoreJVMArgs(); err != nil {
		return NeoForgeChangeResult{}, fmt.Errorf("restore user_jvm_args.txt: %w", err)
	}
	if err := s.verifyNeoForgeRuntime(target); err != nil {
		return NeoForgeChangeResult{}, err
	}

	if err := s.updateCraftyNeoForgeConfig(ctx, nextCommand, nextExecutable); err != nil {
		return NeoForgeChangeResult{}, err
	}
	verifiedConfig, err := s.loadCraftyServerConfig(ctx)
	verifiedCommandVersion := neoForgeVersionFromText(verifiedConfig.ExecutionCommand)
	commandVerified := verifiedCommandVersion == target ||
		(verifiedCommandVersion == "" && verifiedConfig.ExecutionCommand == nextCommand)
	executableVerified := neoForgeVersionFromText(verifiedConfig.Executable) == target
	if err != nil || !commandVerified || !executableVerified {
		rollbackErr := s.updateCraftyNeoForgeConfig(
			context.Background(),
			currentConfig.ExecutionCommand,
			currentConfig.Executable,
		)
		if rollbackErr != nil {
			return NeoForgeChangeResult{}, fmt.Errorf(
				"verify Crafty NeoForge configuration failed and rollback also failed: verify=%v rollback=%v",
				err,
				rollbackErr,
			)
		}
		if err != nil {
			return NeoForgeChangeResult{}, fmt.Errorf("verify Crafty NeoForge configuration: %w", err)
		}
		return NeoForgeChangeResult{}, fmt.Errorf(
			"Crafty did not retain the requested NeoForge %s launch configuration; previous configuration restored",
			target,
		)
	}

	direction := "reinstall"
	switch compareNeoForgeVersions(target, current) {
	case 1:
		direction = "upgrade"
	case -1:
		direction = "downgrade"
	}
	s.logEvent(
		"info",
		"neoforge",
		fmt.Sprintf("NeoForge %s completed: %s -> %s; server remains stopped", direction, current, target),
	)
	return NeoForgeChangeResult{
		FromVersion: current,
		ToVersion:   target,
		Direction:   direction,
	}, nil
}

func (s *Service) listNeoForgeVersions(ctx context.Context) ([]NeoForgeVersion, error) {
	prefix, err := neoForgeSeriesPrefix(s.options.Minecraft)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(s.options.NeoForgeBaseURL, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/maven-metadata.xml", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "fpbpack/neoforge-version-manager")
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch NeoForge Maven metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch NeoForge Maven metadata: %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read NeoForge Maven metadata: %w", err)
	}
	var metadata neoForgeMavenMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parse NeoForge Maven metadata: %w", err)
	}

	versions := make([]NeoForgeVersion, 0, len(metadata.Versioning.Versions.Version))
	seen := make(map[string]struct{})
	for _, raw := range metadata.Versioning.Versions.Version {
		version := strings.TrimSpace(raw)
		if !validNeoForgeVersion.MatchString(version) || !strings.HasPrefix(version, prefix) {
			continue
		}
		if _, duplicate := seen[version]; duplicate {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, NeoForgeVersion{
			Version: version,
			Channel: neoForgeChannel(version),
		})
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareNeoForgeVersions(versions[i].Version, versions[j].Version) > 0
	})
	if len(versions) == 0 {
		return nil, fmt.Errorf(
			"official NeoForge Maven metadata contains no %s versions for Minecraft %s",
			strings.TrimSuffix(prefix, "."),
			s.options.Minecraft,
		)
	}
	return versions, nil
}

func neoForgeSeriesPrefix(minecraft string) (string, error) {
	parts := strings.Split(strings.TrimSpace(minecraft), ".")
	if len(parts) < 3 || parts[0] != "1" {
		return "", fmt.Errorf("cannot map Minecraft %q to a NeoForge release series", minecraft)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid Minecraft version %q", minecraft)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid Minecraft version %q", minecraft)
	}
	return fmt.Sprintf("%d.%d.", minor, patch), nil
}

func neoForgeChannel(version string) string {
	lower := strings.ToLower(version)
	switch {
	case strings.Contains(lower, "alpha"):
		return "alpha"
	case strings.Contains(lower, "beta"):
		return "beta"
	case strings.Contains(lower, "rc"):
		return "rc"
	default:
		return "release"
	}
}

func compareNeoForgeVersions(left, right string) int {
	leftBase, leftSuffix := splitNeoForgeVersion(left)
	rightBase, rightSuffix := splitNeoForgeVersion(right)
	for i := 0; i < 3; i++ {
		if leftBase[i] < rightBase[i] {
			return -1
		}
		if leftBase[i] > rightBase[i] {
			return 1
		}
	}
	if leftSuffix == rightSuffix {
		return 0
	}
	if leftSuffix == "" {
		return 1
	}
	if rightSuffix == "" {
		return -1
	}
	if leftSuffix < rightSuffix {
		return -1
	}
	return 1
}

func splitNeoForgeVersion(version string) ([3]int, string) {
	var numbers [3]int
	base := version
	suffix := ""
	if index := strings.IndexAny(version, "-+"); index >= 0 {
		base = version[:index]
		suffix = version[index+1:]
	}
	parts := strings.Split(base, ".")
	for i := 0; i < len(parts) && i < len(numbers); i++ {
		numbers[i], _ = strconv.Atoi(parts[i])
	}
	return numbers, suffix
}

func (s *Service) detectNeoForgeVersion(ctx context.Context) (string, string) {
	if config, err := s.loadCraftyServerConfig(ctx); err == nil {
		if version := neoForgeVersionFromText(config.ExecutionCommand); version != "" {
			return version, ""
		}
		if version := neoForgeVersionFromText(config.Executable); version != "" {
			return version, ""
		}
	}

	for _, name := range []string{"run.sh", "run.bat"} {
		content, err := os.ReadFile(filepath.Join(s.options.ServerRoot, name))
		if err == nil {
			if version := neoForgeVersionFromText(string(content)); version != "" {
				return version, "Detected from " + name + "; Crafty launch configuration did not expose a NeoForge version."
			}
		}
	}

	root := filepath.Join(s.options.ServerRoot, "libraries", "net", "neoforged", "neoforge")
	entries, err := os.ReadDir(root)
	if err == nil {
		found := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() && validNeoForgeVersion.MatchString(entry.Name()) {
				found = append(found, entry.Name())
			}
		}
		if len(found) == 1 {
			return found[0], "Detected from the only installed NeoForge runtime directory."
		}
		if len(found) > 1 {
			return "", "Multiple NeoForge runtimes are installed; Crafty must expose the active version before FPBPack can change it."
		}
	}
	return "", "FPBPack could not determine the active NeoForge version."
}

func neoForgeVersionFromText(value string) string {
	if match := neoForgeArgsPattern.FindStringSubmatch(value); len(match) >= 3 {
		return match[2]
	}
	if match := neoForgeJarPattern.FindStringSubmatch(value); len(match) >= 3 {
		return match[2]
	}
	return ""
}

func rewriteNeoForgeExecutionCommand(command, target string) (string, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", fmt.Errorf("Crafty server execution command is empty")
	}
	if strings.HasSuffix(strings.ToLower(trimmed), "run.sh") ||
		strings.HasSuffix(strings.ToLower(trimmed), "run.bat") {
		return command, nil
	}
	if !neoForgeArgsPattern.MatchString(command) {
		return "", fmt.Errorf(
			"Crafty execution command does not reference a NeoForge unix_args.txt/win_args.txt path; refusing to overwrite a custom launch command",
		)
	}
	return neoForgeArgsPattern.ReplaceAllString(command, `${1}`+target+`${3}`), nil
}

func rewriteNeoForgeExecutable(executable, target string) string {
	if !neoForgeJarPattern.MatchString(executable) {
		return executable
	}
	return neoForgeJarPattern.ReplaceAllString(executable, `${1}`+target+`${3}`+target+`${5}`)
}

func (s *Service) loadCraftyServerConfig(ctx context.Context) (craftyServerConfig, error) {
	config, token, _ := s.effectiveCraftyConfig()
	if strings.TrimSpace(config.URL) == "" ||
		strings.TrimSpace(config.ServerID) == "" ||
		strings.TrimSpace(token) == "" {
		return craftyServerConfig{}, fmt.Errorf("Crafty is not configured")
	}
	var data map[string]any
	if err := s.craftyRequest(
		ctx,
		config,
		token,
		http.MethodGet,
		"/servers/"+url.PathEscape(config.ServerID),
		&data,
	); err != nil {
		return craftyServerConfig{}, err
	}
	executionCommand, _ := data["execution_command"].(string)
	executable, _ := data["executable"].(string)
	if strings.TrimSpace(executionCommand) == "" {
		return craftyServerConfig{}, fmt.Errorf("Crafty server configuration did not include execution_command")
	}
	return craftyServerConfig{
		ExecutionCommand: executionCommand,
		Executable:       executable,
	}, nil
}

func (s *Service) updateCraftyNeoForgeConfig(ctx context.Context, command, executable string) error {
	config, token, _ := s.effectiveCraftyConfig()
	if strings.TrimSpace(config.URL) == "" ||
		strings.TrimSpace(config.ServerID) == "" ||
		strings.TrimSpace(token) == "" {
		return fmt.Errorf("Crafty is not configured")
	}
	payload := map[string]string{
		"execution_command": command,
		"executable":       executable,
	}
	if err := s.craftyRequestWithBody(
		ctx,
		config,
		token,
		http.MethodPatch,
		"/servers/"+url.PathEscape(config.ServerID),
		payload,
		nil,
	); err != nil {
		return fmt.Errorf("update Crafty NeoForge launch configuration: %w", err)
	}
	return nil
}

func (s *Service) downloadNeoForgeInstaller(ctx context.Context, version string) (string, error) {
	base := strings.TrimRight(s.options.NeoForgeBaseURL, "/") + "/" + version
	filename := "neoforge-" + version + "-installer.jar"
	expected, err := s.fetchNeoForgeSHA512(ctx, base+"/"+filename+".sha512")
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(s.options.StateDir, "cache", "neoforge")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create NeoForge cache: %w", err)
	}
	target := filepath.Join(cacheDir, filename)
	if actual, err := fileSHA512(target); err == nil && strings.EqualFold(actual, expected) {
		return target, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+filename, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "fpbpack/neoforge-version-manager")
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download NeoForge %s installer: %w", version, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download NeoForge %s installer: %s", version, response.Status)
	}

	tmp, err := os.CreateTemp(cacheDir, ".neoforge-installer-*")
	if err != nil {
		return "", fmt.Errorf("create NeoForge installer cache file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	hash := sha512.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(response.Body, 512<<20))
	closeErr := tmp.Close()
	if copyErr != nil {
		return "", fmt.Errorf("download NeoForge %s installer: %w", version, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close NeoForge installer cache file: %w", closeErr)
	}
	if written >= 512<<20 {
		return "", fmt.Errorf("NeoForge %s installer exceeded the 512 MiB safety limit", version)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return "", fmt.Errorf(
			"NeoForge %s installer SHA-512 mismatch: expected %s, got %s",
			version,
			expected,
			actual,
		)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return "", fmt.Errorf("cache NeoForge installer: %w", err)
	}
	return target, nil
}

func (s *Service) fetchNeoForgeSHA512(ctx context.Context, source string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "fpbpack/neoforge-version-manager")
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch NeoForge installer SHA-512: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("fetch NeoForge installer SHA-512: %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<10))
	if err != nil {
		return "", fmt.Errorf("read NeoForge installer SHA-512: %w", err)
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 || len(fields[0]) != sha512.Size*2 {
		return "", fmt.Errorf("NeoForge installer SHA-512 response is invalid")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", fmt.Errorf("NeoForge installer SHA-512 response is invalid: %w", err)
	}
	return strings.ToLower(fields[0]), nil
}

func fileSHA512(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Service) preserveUserJVMArgs() (func() error, error) {
	path := filepath.Join(s.options.ServerRoot, "user_jvm_args.txt")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return func() error { return nil }, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read user_jvm_args.txt before NeoForge install: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat user_jvm_args.txt before NeoForge install: %w", err)
	}
	mode := info.Mode().Perm()
	return func() error {
		return os.WriteFile(path, content, mode)
	}, nil
}

func (s *Service) runNeoForgeInstaller(ctx context.Context, installer string) error {
	installCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	java := strings.TrimSpace(s.options.JavaExecutable)
	if java == "" {
		java = "java"
	}
	command := exec.CommandContext(installCtx, java, "-jar", installer, "--installServer")
	command.Dir = s.options.ServerRoot
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	if installCtx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("NeoForge installer exceeded the 10 minute timeout")
	}
	message := strings.TrimSpace(string(output))
	if len(message) > 4000 {
		message = message[len(message)-4000:]
	}
	if message == "" {
		return fmt.Errorf("NeoForge installer failed: %w", err)
	}
	return fmt.Errorf("NeoForge installer failed: %w: %s", err, message)
}

func (s *Service) verifyNeoForgeRuntime(version string) error {
	root := filepath.Join(
		s.options.ServerRoot,
		"libraries",
		"net",
		"neoforged",
		"neoforge",
		version,
	)
	for _, name := range []string{
		"unix_args.txt",
		"neoforge-" + version + "-server.jar",
	} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.IsDir() {
			return fmt.Errorf("NeoForge %s installer did not produce %s", version, name)
		}
	}
	return nil
}
