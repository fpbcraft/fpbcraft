package service

import (
 "path/filepath"
 "testing"
 "time"

 "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
 "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
 updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestUpdatesAfterCatalogMutationPreservesUnchangedAndDropsChanged(t *testing.T) {
 makeEntry := func(name, sha string) catalog.Entry {
  return catalog.Entry{Provider:"modrinth",ProjectID:name,VersionID:"v1",SHA512:sha,Deployment:inventory.LocationServer}
 }
 old := catalog.Report{Managed: []catalog.Entry{makeEntry("first","first-old"),makeEntry("second","second-old")}}
 next := catalog.Report{Managed: []catalog.Entry{makeEntry("first","first-new"),makeEntry("second","second-old")}}
 report := updatecheck.Report{GeneratedAt:time.Now().UTC(),Candidates:[]updatecheck.Candidate{
  {Key:"modrinth:first",Classification:updatecheck.ClassificationSafe,Target:&updatecheck.Release{ID:"v2"}},
  {Key:"modrinth:second",Classification:updatecheck.ClassificationSafe,Target:&updatecheck.Release{ID:"v2"}},
 }}
 result := updatesAfterCatalogMutation(old,next,report,true)
 if len(result.Candidates)!=1 || result.Candidates[0].Key!="modrinth:second" || result.Summary.Safe!=1 {
  t.Fatalf("expected only unaffected update to remain: %+v",result)
 }
 dir:=t.TempDir()
 if err:=writeJSONAtomic(filepath.Join(dir,"updates.json"),result);err!=nil {t.Fatal(err)}
 svc:=&Service{options:Options{StateDir:dir},state:State{Catalog:next}}
 svc.loadRuntimeCaches()
 if !svc.hasUpdate || len(svc.updates.Candidates)!=1 || svc.updates.Candidates[0].Key!="modrinth:second" {
  t.Fatalf("persisted update cache lost on restart: %+v",svc.updates)
 }
}

func TestUpdatesAfterPlacementChangeKeepsDiscoveredTarget(t *testing.T) {
 old:=catalog.Report{Managed:[]catalog.Entry{{Provider:"modrinth",ProjectID:"same",VersionID:"v1",SHA512:"same",Deployment:inventory.LocationServer}}}
 next:=catalog.Report{Managed:[]catalog.Entry{{Provider:"modrinth",ProjectID:"same",VersionID:"v1",SHA512:"same",Deployment:inventory.LocationClient,AutoModpackGroup:"visual-client-mods"}}}
 report:=updatecheck.Report{Candidates:[]updatecheck.Candidate{{Key:"modrinth:same",Classification:updatecheck.ClassificationSafe,Target:&updatecheck.Release{ID:"v2"}}}}
 result:=updatesAfterCatalogMutation(old,next,report,true)
 if len(result.Candidates)!=1 || result.Candidates[0].Deployment!=inventory.LocationClient || result.Candidates[0].AutoModpackGroup!="visual-client-mods" {
  t.Fatalf("placement-only change unnecessarily invalidated the update: %+v",result)
 }
}
