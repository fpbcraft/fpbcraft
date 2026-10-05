package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func runCatalog(args []string) int {
	flags := flag.NewFlagSet("catalog", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	inventoryPath := flags.String("inventory", "", "inventory JSON produced by fpbpack inventory")
	output := flags.String("output", "modpack", "output directory for the generated Packwiz catalog")
	minecraft := flags.String("minecraft", "1.21.1", "Minecraft version")
	neoforge := flags.String("neoforge", "", "NeoForge version to include in pack.toml (optional during migration)")
	name := flags.String("name", "FPBCraft", "pack name")
	author := flags.String("author", "FPBCraft", "pack author")
	packVersion := flags.String("pack-version", "migration", "pack version label")
	force := flags.Bool("force", false, "replace a non-empty output directory")
	strict := flags.Bool("strict", false, "exit non-zero when unresolved artifacts or version conflicts remain")
	resolveCurseForge := flags.Bool("resolve-curseforge", false, "resolve unresolved JARs using an isolated Packwiz CurseForge detector")
	packwizPath := flags.String("packwiz", "packwiz", "Packwiz executable used by catalog resolvers")
	sourcesPath := flags.String("sources", "", "source registry JSON for exact GitHub and pinned custom artifacts")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *inventoryPath == "" {
		fmt.Fprintln(os.Stderr, "--inventory is required")
		return 2
	}

	file, err := os.Open(*inventoryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open inventory: %v\n", err)
		return 1
	}
	defer file.Close()

	var inv inventory.Inventory
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&inv); err != nil {
		fmt.Fprintf(os.Stderr, "decode inventory: %v\n", err)
		return 1
	}

	result, err := catalog.Build(inv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build catalog: %v\n", err)
		return 1
	}
	if err := catalog.Write(result, catalog.Options{
		Name: *name, Author: *author, Version: *packVersion,
		Minecraft: *minecraft, NeoForge: *neoforge, OutputPath: *output, Force: *force,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write catalog: %v\n", err)
		return 1
	}

	var curseForge catalog.CurseForgeDetectSummary
	if *resolveCurseForge {
		curseForge, err = catalog.ResolveCurseForgeWithPackwiz(inv, &result, *output, *packwizPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve CurseForge: %v\n", err)
			return 1
		}
	}

	var sourceSummary catalog.SourceResolutionSummary
	if *sourcesPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		sourceSummary, err = catalog.ResolveSourceRegistry(ctx, inv, &result, catalog.SourceResolveOptions{
			RegistryPath: *sourcesPath,
			OutputPath: *output,
			PackwizPath: *packwizPath,
		})
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve source registry: %v\n", err)
			return 1
		}
	}

	s := result.Report.Summary
	fmt.Println("FPBPack catalog")
	fmt.Printf("Inventory JARs:      %d\n", s.InventoryJARs)
	fmt.Printf("Unique artifacts:    %d\n", s.UniqueArtifacts)
	fmt.Printf("Exact duplicates:    %d\n", s.DuplicateArtifacts)
	fmt.Printf("Packwiz projects:    %d\n", s.GeneratedProjects)
	fmt.Printf("Unresolved:          %d\n", s.Unresolved)
	fmt.Printf("Version conflicts:   %d\n", s.ConflictProjects)
	fmt.Printf("Placement warnings:  %d\n", s.PlacementWarnings)
	fmt.Printf("Pinned artifacts:     %d\n", s.PinnedArtifacts)
	if *resolveCurseForge {
		fmt.Printf("CurseForge detected: %d\n", curseForge.Detected)
		fmt.Printf("CF still unmatched:  %d\n", curseForge.Unmatched)
	}
	if *sourcesPath != "" {
		fmt.Printf("GitHub verified:      %d\n", sourceSummary.GitHubVerified)
		fmt.Printf("Registry pinned:      %d\n", sourceSummary.Pinned)
		fmt.Printf("Still unresolved:     %d\n", sourceSummary.Remaining)
	}
	fmt.Printf("Output:              %s\n", *output)
	fmt.Printf("Report:              %s/migration-report.json\n", *output)

	if *strict && (s.Unresolved > 0 || s.ConflictProjects > 0) {
		fmt.Fprintln(os.Stderr, "catalog is incomplete; resolve migration-report.json findings before strict mode can pass")
		return 3
	}
	return 0
}
