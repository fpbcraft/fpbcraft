package service

import (
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

func TestCreateRestorePointCopiesOnlyPlannedCurrentArtifacts(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(serverRoot, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("current mod bytes")
	sourcePath := filepath.Join(serverRoot, "mods", "example.jar")
	if err := os.WriteFile(sourcePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha512.Sum512(content)
	sha := hex.EncodeToString(sum[:])

	service := &Service{options: Options{ServerRoot: serverRoot, StateDir: stateDir}}
	plan := planning.Plan{
		ID: "plan-0123456789abcdef",
		Status: planning.StatusReady,
		Verified: true,
		RequiresBackup: true,
		Changes: []planning.Change{{
			Name: "Example",
			Operations: []planning.FileOperation{{
				Action: "replace",
				CurrentPath: "mods/example.jar",
				CurrentSHA512: sha,
				TargetPath: "mods/example-new.jar",
				TargetSHA512: "new",
			}},
		}},
	}
	if err := service.createRestorePoint(&plan); err != nil {
		t.Fatal(err)
	}
	if plan.BackupID == "" {
		t.Fatal("backup ID was not assigned")
	}
	var manifest planning.BackupManifest
	if err := readJSON(filepath.Join(stateDir, "backups", plan.BackupID, "manifest.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].SHA512 != sha {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}

func TestSafeRelativePathRejectsTraversal(t *testing.T) {
	for _, value := range []string{"../mods/a.jar", "/mods/a.jar", "."} {
		if _, err := safeRelativePath(value); err == nil {
			t.Fatalf("expected unsafe path %q to fail", value)
		}
	}
}
