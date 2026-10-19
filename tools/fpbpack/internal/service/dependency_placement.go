package service

import (
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func providerDependencyAvailability(owner catalog.Entry, installed []catalog.Entry, version string) (allAvailable, anyVisible, uncertain bool) {
	allAvailable = true
	for _, scope := range dependencySources(owner) {
		visible, possible := false, false
		for _, entry := range installed {
			if version != "" && entry.VersionID != version {
				continue
			}
			for _, source := range dependencySources(entry) {
				if source.Location == scope.Location && (scope.Location == inventory.LocationServer || normalizeAutoModpackGroup(source.Location, source.Group) == "main" || normalizeAutoModpackGroup(source.Location, source.Group) == normalizeAutoModpackGroup(scope.Location, scope.Group)) {
					visible = true
				}
				if scope.Location == inventory.LocationClient && source.Location == inventory.LocationServer {
					possible = true
				}
			}
		}
		allAvailable = allAvailable && (visible || possible)
		anyVisible = anyVisible || visible
		uncertain = uncertain || (!visible && possible)
	}
	return
}

func dependencySources(entry catalog.Entry) []catalog.Source {
	sources := append([]catalog.Source(nil), entry.SourcePaths...)
	if len(sources) == 0 {
		sources = []catalog.Source{{Location: entry.Deployment, Group: entry.AutoModpackGroup}}
	}
	for index := range sources {
		if sources[index].Location == "" {
			sources[index].Location = inventory.LocationServer
		}
	}
	return sources
}
