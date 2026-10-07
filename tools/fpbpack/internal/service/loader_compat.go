package service

import (
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func compatibleLoadersForInventory(configured string, inv inventory.Inventory) []string {
	primary := strings.ToLower(strings.TrimSpace(configured))
	if primary == "" {
		primary = "neoforge"
	}
	result := []string{primary}
	if primary != "neoforge" || !inventoryHasConnector(inv) {
		return result
	}
	return append(result, "fabric")
}

func inventoryHasConnector(inv inventory.Inventory) bool {
	for _, mod := range inv.Mods {
		for _, metadata := range mod.Metadata {
			if strings.EqualFold(strings.TrimSpace(metadata.ModID), "connector") {
				return true
			}
		}
	}
	return false
}

func (s *Service) catalogCompatibleLoaders() []string {
	s.mu.RLock()
	inv := s.snapshot.Inventory
	s.mu.RUnlock()
	return compatibleLoadersForInventory(s.options.Loader, inv)
}

func additionalLoaders(loaders []string) []string {
	if len(loaders) <= 1 {
		return nil
	}
	return append([]string(nil), loaders[1:]...)
}
