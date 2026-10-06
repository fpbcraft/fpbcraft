package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestChangeNeoForgeEndToEnd(t *testing.T) {
	const (
		currentVersion = "21.1.180"
		targetVersion  = "21.1.201"
	)
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(serverRoot, "user_jvm_args.txt"), []byte("-Xmx8G\n-Dfpbcraft=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	javaPath := filepath.Join(t.TempDir(), "fake-java")
	javaScript := `#!/bin/sh
set -eu
installer="$2"
filename="$(basename "$installer")"
version="${filename#neoforge-}"
version="${version%-installer.jar}"
root="$PWD/libraries/net/neoforged/neoforge/$version"
mkdir -p "$root"
printf 'generated args\n' > "$root/unix_args.txt"
printf 'generated server jar\n' > "$root/neoforge-$version-server.jar"
printf 'installer overwrote this\n' > "$PWD/user_jvm_args.txt"
`
	if err := os.WriteFile(javaPath, []byte(javaScript), 0o755); err != nil {
		t.Fatal(err)
	}

	installerBytes := []byte("fake NeoForge installer")
	installerHash := sha512.Sum512(installerBytes)
	installerHashHex := hex.EncodeToString(installerHash[:])

	var mu sync.Mutex
	executionCommand := "java -Xms4G -Xmx8G @libraries/net/neoforged/neoforge/" + currentVersion + "/unix_args.txt nogui"
	executable := "libraries/net/neoforged/neoforge/" + currentVersion + "/neoforge-" + currentVersion + "-server.jar"

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/maven-metadata.xml":
			_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><metadata><versioning><versions><version>%s</version><version>%s</version></versions></versioning></metadata>`, currentVersion, targetVersion)
		case r.Method == http.MethodGet && r.URL.Path == "/"+targetVersion+"/neoforge-"+targetVersion+"-installer.jar.sha512":
			_, _ = fmt.Fprintf(w, "%s  neoforge-%s-installer.jar\n", installerHashHex, targetVersion)
		case r.Method == http.MethodGet && r.URL.Path == "/"+targetVersion+"/neoforge-"+targetVersion+"-installer.jar":
			_, _ = w.Write(installerBytes)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/servers/server-1/stats":
			writeCraftyTestEnvelope(t, w, map[string]any{"running": false})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/servers/server-1":
			mu.Lock()
			data := map[string]any{
				"execution_command": executionCommand,
				"executable":       executable,
			}
			mu.Unlock()
			writeCraftyTestEnvelope(t, w, data)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v2/servers/server-1":
			var body struct {
				ExecutionCommand string `json:"execution_command"`
				Executable       string `json:"executable"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode Crafty PATCH: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			executionCommand = body.ExecutionCommand
			executable = body.Executable
			mu.Unlock()
			writeCraftyTestEnvelope(t, w, nil)
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer api.Close()

	s := &Service{
		options: Options{
			ServerRoot:        serverRoot,
			StateDir:          stateDir,
			Minecraft:         "1.21.1",
			Loader:            "neoforge",
			NeoForgeBaseURL: api.URL,
			JavaExecutable:    javaPath,
		},
		state: State{
			Crafty: CraftySettings{
				URL:      api.URL,
				ServerID: "server-1",
			},
		},
		secrets: ProviderSecrets{
			CraftyAPIToken: "test-token",
		},
	}

	result, err := s.ChangeNeoForge(context.Background(), targetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if result.FromVersion != currentVersion || result.ToVersion != targetVersion || result.Direction != "upgrade" {
		t.Fatalf("unexpected result: %+v", result)
	}

	mu.Lock()
	finalCommand := executionCommand
	finalExecutable := executable
	mu.Unlock()
	if !strings.Contains(finalCommand, "@libraries/net/neoforged/neoforge/"+targetVersion+"/unix_args.txt") {
		t.Fatalf("Crafty execution command was not updated: %q", finalCommand)
	}
	if !strings.Contains(finalCommand, "-Xms4G -Xmx8G") {
		t.Fatalf("Crafty JVM flags were not preserved: %q", finalCommand)
	}
	wantExecutable := "libraries/net/neoforged/neoforge/" + targetVersion + "/neoforge-" + targetVersion + "-server.jar"
	if finalExecutable != wantExecutable {
		t.Fatalf("Crafty executable = %q, want %q", finalExecutable, wantExecutable)
	}

	userArgs, err := os.ReadFile(filepath.Join(serverRoot, "user_jvm_args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(userArgs) != "-Xmx8G\n-Dfpbcraft=true\n" {
		t.Fatalf("user_jvm_args.txt was not restored: %q", string(userArgs))
	}
}

func writeCraftyTestEnvelope(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"data":   data,
	}); err != nil {
		t.Errorf("encode Crafty response: %v", err)
	}
}
