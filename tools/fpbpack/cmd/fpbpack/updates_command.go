package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func runUpdates(args []string) int {
	flags := flag.NewFlagSet("updates", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	reportPath := flags.String("report", "", "accepted migration report JSON")
	outputPath := flags.String("output", "fpbpack-updates.json", "path for the generated update report")
	minecraft := flags.String("minecraft", "1.21.1", "Minecraft version that candidate releases must support")
	loader := flags.String("loader", "neoforge", "mod loader that candidate releases must support")
	modrinthAPI := flags.String("modrinth-api", updatecheck.DefaultModrinthAPI, "Modrinth API base URL")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reportPath == "" {
		fmt.Fprintln(os.Stderr, "--report is required")
		return 2
	}

	file, err := os.Open(*reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open migration report: %v\n", err)
		return 1
	}
	defer file.Close()
	var cat catalog.Report
	if err := json.NewDecoder(file).Decode(&cat); err != nil {
		fmt.Fprintf(os.Stderr, "decode migration report: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	result := updatecheck.Discover(ctx, cat, updatecheck.Options{
		Minecraft: *minecraft,
		Loader: *loader,
		ModrinthBaseURL: *modrinthAPI,
		CurseForgeAPIKey: os.Getenv("FPBPACK_CURSEFORGE_API_KEY"),
	})
	if ctx.Err() != nil {
		fmt.Fprintf(os.Stderr, "update discovery timed out: %v\n", ctx.Err())
		return 1
	}
	if err := writeUpdateReport(*outputPath, result); err != nil {
		fmt.Fprintf(os.Stderr, "write update report: %v\n", err)
		return 1
	}

	fmt.Println("FPBPack updates")
	fmt.Printf("Safe:        %d\n", result.Summary.Safe)
	fmt.Printf("Review:      %d\n", result.Summary.Review)
	fmt.Printf("Blocked:     %d\n", result.Summary.Blocked)
	fmt.Printf("Ignored:     %d\n", result.Summary.Ignored)
	fmt.Printf("Up to date:  %d\n", result.Summary.UpToDate)
	fmt.Printf("Output:      %s\n", *outputPath)
	return 0
}

func writeUpdateReport(path string, report updatecheck.Report) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".fpbpack-updates-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
