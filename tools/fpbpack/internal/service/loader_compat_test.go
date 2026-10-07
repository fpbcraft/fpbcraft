package service

import (
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestCompatibleLoadersForInventoryEnablesFabricOnlyWithConnector(t *testing.T) {
	without := compatibleLoadersForInventory("neoforge", inventory.Inventory{})
	if len(without) != 1 || without[0] != "neoforge" {
		t.Fatalf("without connector = %+v", without)
	}
	with := compatibleLoadersForInventory("neoforge", inventory.Inventory{
		Mods: []inventory.ModFile{{
			Metadata: []inventory.ModMetadata{{Loader: "neoforge", ModID: "connector"}},
		}},
	})
	if len(with) != 2 || with[0] != "neoforge" || with[1] != "fabric" {
		t.Fatalf("with connector = %+v", with)
	}
}

func TestCompatibleLoadersDoesNotAddFabricToOtherPrimaryLoaders(t *testing.T) {
	loaders := compatibleLoadersForInventory("forge", inventory.Inventory{
		Mods: []inventory.ModFile{{
			Metadata: []inventory.ModMetadata{{Loader: "forge", ModID: "connector"}},
		}},
	})
	if len(loaders) != 1 || loaders[0] != "forge" {
		t.Fatalf("loaders = %+v", loaders)
	}
}
