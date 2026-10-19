package service

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/dependencies"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

// Validate cached target JARs together with unchanged (including unmanaged) files.
// This catches reverse constraints that provider discovery cannot see.
func (s *Service) validateProposedDependencies(plan *planning.Plan) {
	if plan.Status == planning.StatusBlocked {
		return
	}
	s.mu.RLock()
	live := append([]inventory.ModFile(nil), s.snapshot.Inventory.Mods...)
	s.mu.RUnlock()
	fresh, scanErr := s.scanDependencyInventory()
	if scanErr != nil {
		plan.Status = planning.StatusBlocked
		plan.Verified = false
		plan.VerifiedAt = nil
		plan.Blockers = append(plan.Blockers, planning.Finding{Code: "dependency_inventory_unreadable", Message: scanErr.Error()})
		return
	}
	live = fresh.Mods
	plan.DependencyInventory = map[string]string{}
	for _, file := range live {
		plan.DependencyInventory[file.Path] = file.SHA512
	}
	byPath := map[string]inventory.ModFile{}
	for _, file := range live {
		byPath[file.Path] = file
	}
	caches := s.prefetchedByHash(*plan)
	for _, change := range plan.Changes {
		originals := map[string]inventory.ModFile{}
		for _, operation := range change.Operations {
			originals[operation.CurrentPath] = byPath[operation.CurrentPath]
			delete(byPath, operation.CurrentPath)
		}
		for _, operation := range change.Operations {
			if operation.Action == "remove" {
				continue
			}
			metadata, err := inventory.ReadMetadata(caches[strings.ToLower(operation.TargetSHA512)])
			if inventory.RuntimeMetadataUnverified(err) {
				plan.Warnings = append(plan.Warnings, planning.Finding{Code: "dependency_metadata_unverified", CandidateKey: change.CandidateKey, Name: change.Name, Path: operation.TargetPath, Message: err.Error()})
			} else if err != nil {
				plan.Blockers = append(plan.Blockers, planning.Finding{Code: "dependency_metadata_unreadable", CandidateKey: change.CandidateKey, Name: change.Name, Path: operation.TargetPath, Message: fmt.Sprintf("Cannot inspect target JAR dependencies: %v", err)})
				continue
			}
			file := originals[operation.CurrentPath]
			file.Path = operation.TargetPath
			file.Filename = change.Artifact.Filename
			file.SHA512 = operation.TargetSHA512
			file.Error = ""
			if err != nil {
				file.Error = err.Error()
			}
			file.Metadata = metadata
			if file.Location == "" || filepath.Dir(operation.CurrentPath) != filepath.Dir(operation.TargetPath) {
				file.Location = inventory.Location(change.Artifact.Deployment)
				file.Group = change.Artifact.AutoModpackGroup
			}
			byPath[file.Path] = file
		}
	}
	files := make([]inventory.ModFile, 0, len(byPath))
	for _, file := range byPath {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	// Existing unrelated findings remain in Issues; a batch may repair a broken pack
	// incrementally, but it may not introduce or retain problems on touched artifacts.
	before := map[string]bool{}
	for _, finding := range dependencies.Analyze(live) {
		before[finding.Code+"\x00"+finding.Path+"\x00"+finding.Message] = true
	}
	touched := map[string]bool{}
	for _, change := range plan.Changes {
		for _, op := range change.Operations {
			touched[op.CurrentPath] = true
			touched[op.TargetPath] = true
		}
	}
	for _, finding := range dependencies.Analyze(files) {
		if before[finding.Code+"\x00"+finding.Path+"\x00"+finding.Message] && !touched[finding.Path] {
			continue
		}
		f := planning.Finding{Code: finding.Code, Name: finding.Mod, Path: finding.Path, Message: finding.Message}
		if finding.Level == "blocking" {
			plan.Blockers = append(plan.Blockers, f)
		} else {
			plan.Warnings = append(plan.Warnings, f)
		}
	}
	if len(plan.Blockers) > 0 {
		plan.Status = planning.StatusBlocked
		plan.Verified = false
		plan.VerifiedAt = nil
	}
}

func (s *Service) scanDependencyInventory() (inventory.Inventory, error) {
	return inventory.Scan(inventory.ScanOptions{AllowEmpty: true, ServerRoot: s.options.ServerRoot, ServerModsPath: s.options.ServerModsPath, ClientModsPath: s.options.ClientModsPath})
}

func (s *Service) validateDependencySnapshot(plan planning.Plan) error {
	if plan.DependencyInventory == nil {
		return nil
	}
	current, err := s.scanDependencyInventory()
	if err != nil {
		return err
	}
	if len(current.Mods) != len(plan.DependencyInventory) {
		return fmt.Errorf("installed JAR set changed; review the pending changes again")
	}
	for _, file := range current.Mods {
		if !strings.EqualFold(file.SHA512, plan.DependencyInventory[file.Path]) {
			return fmt.Errorf("%s changed; review the pending changes again", file.Path)
		}
	}
	return nil
}
