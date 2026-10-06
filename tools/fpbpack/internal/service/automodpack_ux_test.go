package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newAutoModpackTestService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "automodpack"), 0o755); err != nil {
		t.Fatal(err)
	}
	return newAutoModpackTestServiceAt(t, root, stateDir)
}

func newAutoModpackTestServiceAt(t *testing.T, root, stateDir string) *Service {
	t.Helper()
	svc, err := New(context.Background(), Options{ServerRoot: root, StateDir: stateDir})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestUpdateAutoModpackRawConfigDoesNotRequireStoppedServer(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "automodpack"), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := []byte(`modpack {
  name: "Before"
  General {
    main {
      required: true
      default-selected: true
    }
  }
}
`)
	if err := os.WriteFile(filepath.Join(root, "automodpack", "server.conf"), initial, 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newAutoModpackTestServiceAt(t, root, stateDir)

	sum := sha256.Sum256(initial)
	status, err := svc.UpdateAutoModpackRawConfig(context.Background(), AutoModpackRawConfigRequest{
		ExpectedSHA256: hex.EncodeToString(sum[:]),
		RawConfig: strings.Replace(string(initial), "Before", "After", 1),
	})
	if err != nil {
		t.Fatalf("raw config save unexpectedly required a stopped/configured server: %v", err)
	}
	if status.Config.Name != "After" {
		t.Fatalf("raw config did not save: %#v", status.Config)
	}
}

func TestAutoModpackGroupFilesPagesAndFiltersProjection(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "automodpack", "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	projection := `{
  "contentToken": "abc",
  "journalHead": 3,
  "policy": {
    "categories": {
      "General": {
        "main": {
          "files": {
            "config/a.json": {"size":"10","type":"config","editable":true,"sha1":"a"},
            "mods/b.jar": {"size":"20","type":"mod","editable":false,"sha1":"b"},
            "mods/c.jar": {"size":"30","type":"mod","editable":false,"sha1":"c"}
          }
        }
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(root, "automodpack", "server", "current-projection.json"), []byte(projection), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newAutoModpackTestServiceAt(t, root, stateDir)

	page, err := svc.AutoModpackGroupFiles("main", 0, 1, "mods/")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Files) != 1 || !page.HasMore {
		t.Fatalf("unexpected first page: %#v", page)
	}
	if !strings.HasPrefix(page.Files[0].Path, "mods/") {
		t.Fatalf("filter was not applied: %#v", page.Files)
	}

	second, err := svc.AutoModpackGroupFiles("main", 1, 1, "mods/")
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 2 || len(second.Files) != 1 || second.HasMore {
		t.Fatalf("unexpected second page: %#v", second)
	}
}

func TestAutoModpackGenerationDiffShowsHeadToTargetChanges(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	serverDir := filepath.Join(root, "automodpack", "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	journal := strings.Join([]string{
		`{"seq":1,"changes":[{"path":"a","fromSha1":"","fromSize":0,"toSha1":"a1","toSize":10},{"path":"b","fromSha1":"","fromSize":0,"toSha1":"b1","toSize":20}]}`,
		`{"seq":2,"changes":[{"path":"a","fromSha1":"a1","fromSize":10,"toSha1":"a2","toSize":11},{"path":"b","fromSha1":"b1","fromSize":20,"toSha1":"","toSize":0},{"path":"c","fromSha1":"","fromSize":0,"toSha1":"c1","toSize":30}]}`,
		`{"seq":3,"changes":[{"path":"a","fromSha1":"a2","fromSize":11,"toSha1":"a3","toSize":12}]}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(serverDir, "journal.jsonl"), []byte(journal), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newAutoModpackTestServiceAt(t, root, stateDir)

	diff, err := svc.AutoModpackGenerationDiff(1)
	if err != nil {
		t.Fatal(err)
	}
	if diff.HeadSequence != 3 || diff.TargetSequence != 1 {
		t.Fatalf("unexpected sequences: %#v", diff)
	}
	if diff.Added != 1 || diff.Changed != 1 || diff.Removed != 1 {
		t.Fatalf("unexpected rollback summary: %#v", diff)
	}
	actions := map[string]string{}
	for _, entry := range diff.Entries {
		actions[entry.Path] = entry.Action
	}
	if actions["a"] != "change" || actions["b"] != "add" || actions["c"] != "remove" {
		t.Fatalf("unexpected rollback actions: %#v", actions)
	}
}

func TestTerminalLinesAfterUsesBufferOverlap(t *testing.T) {
	before := []string{"one", "two", "three"}
	after := []string{"two", "three", "four", "five"}
	got := terminalLinesAfter(before, after)
	if strings.Join(got, ",") != "four,five" {
		t.Fatalf("unexpected appended terminal lines: %#v", got)
	}
}
