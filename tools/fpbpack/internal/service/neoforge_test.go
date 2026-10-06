package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNeoForgeSeriesPrefix(t *testing.T) {
	tests := map[string]string{
		"1.21.1": "21.1.",
		"1.21.4": "21.4.",
		"1.20.6": "20.6.",
	}
	for minecraft, want := range tests {
		got, err := neoForgeSeriesPrefix(minecraft)
		if err != nil {
			t.Fatalf("neoForgeSeriesPrefix(%q): %v", minecraft, err)
		}
		if got != want {
			t.Fatalf("neoForgeSeriesPrefix(%q) = %q, want %q", minecraft, got, want)
		}
	}
	if _, err := neoForgeSeriesPrefix("snapshot"); err == nil {
		t.Fatal("expected invalid Minecraft version to fail")
	}
}

func TestCompareNeoForgeVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"21.1.201", "21.1.200", 1},
		{"21.1.200", "21.1.201", -1},
		{"21.1.201", "21.1.201", 0},
		{"21.1.201", "21.1.201-beta", 1},
		{"21.1.201-beta", "21.1.201", -1},
	}
	for _, test := range tests {
		if got := compareNeoForgeVersions(test.left, test.right); got != test.want {
			t.Fatalf("compareNeoForgeVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestRewriteNeoForgeExecutionCommand(t *testing.T) {
	command := "java -Xms4G -Xmx8G @libraries/net/neoforged/neoforge/21.1.180/unix_args.txt nogui"
	got, err := rewriteNeoForgeExecutionCommand(command, "21.1.201")
	if err != nil {
		t.Fatal(err)
	}
	want := "java -Xms4G -Xmx8G @libraries/net/neoforged/neoforge/21.1.201/unix_args.txt nogui"
	if got != want {
		t.Fatalf("rewritten command = %q, want %q", got, want)
	}

	script := "./run.sh"
	got, err = rewriteNeoForgeExecutionCommand(script, "21.1.201")
	if err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("run script command changed: %q", got)
	}

	if _, err := rewriteNeoForgeExecutionCommand("java -jar custom-launcher.jar", "21.1.201"); err == nil {
		t.Fatal("expected custom launch command to be rejected")
	}
}

func TestRewriteNeoForgeExecutable(t *testing.T) {
	executable := "libraries/net/neoforged/neoforge/21.1.180/neoforge-21.1.180-server.jar"
	got := rewriteNeoForgeExecutable(executable, "21.1.201")
	want := "libraries/net/neoforged/neoforge/21.1.201/neoforge-21.1.201-server.jar"
	if got != want {
		t.Fatalf("rewritten executable = %q, want %q", got, want)
	}
	if version := neoForgeVersionFromText(got); version != "21.1.201" {
		t.Fatalf("detected version = %q, want 21.1.201", version)
	}
}

func TestListNeoForgeVersionsFiltersMinecraftSeriesAndSorts(t *testing.T) {
	const metadata = `<?xml version="1.0" encoding="UTF-8"?>
<metadata>
  <versioning>
    <versions>
      <version>21.1.180</version>
      <version>21.4.10</version>
      <version>21.1.201-beta</version>
      <version>21.1.200</version>
      <version>21.1.201</version>
    </versions>
  </versioning>
</metadata>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/maven-metadata.xml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(metadata))
	}))
	defer server.Close()

	service := &Service{options: Options{
		Minecraft:         "1.21.1",
		NeoForgeBaseURL: server.URL,
	}}
	versions, err := service.listNeoForgeVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(versions))
	for _, version := range versions {
		got = append(got, version.Version+":"+version.Channel)
	}
	want := []string{
		"21.1.201:release",
		"21.1.201-beta:beta",
		"21.1.200:release",
		"21.1.180:release",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("versions = %v, want %v", got, want)
	}
}

func TestNeoForgeVersionFromWindowsCommand(t *testing.T) {
	command := `java -Xmx8G @libraries\net\neoforged\neoforge\21.1.190\win_args.txt nogui`
	if got := neoForgeVersionFromText(command); got != "21.1.190" {
		t.Fatalf("detected version = %q", got)
	}
}
