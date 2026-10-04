package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}

	switch args[0] {
	case "inventory":
		return runInventory(args[1:])
	case "version", "--version", "-version":
		fmt.Printf("fpbpack %s\n", version)
		return 0
	case "help", "--help", "-h":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `fpbpack - FPBCraft modpack management utility

Usage:
  fpbpack inventory --server-root PATH [options]
  fpbpack version

Inventory is read-only. It scans:
  mods/*.jar
  automodpack/host-modpack/main/mods/*.jar

Options are available with:
  fpbpack inventory --help`)
}

func runInventory(args []string) int {
	flags := flag.NewFlagSet("inventory", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	serverRoot := flags.String("server-root", "", "Crafty/Minecraft server root")
	serverMods := flags.String("server-mods", inventory.DefaultServerModsPath, "server/common mods path relative to server root")
	clientMods := flags.String("client-mods", inventory.DefaultClientModsPath, "AutoModpack client-only mods path relative to server root")
	jsonPath := flags.String("json", "", "also write complete inventory JSON to this path")
	offline := flags.Bool("offline", false, "skip Modrinth exact-hash lookup")
	modrinthAPI := flags.String("modrinth-api", inventory.DefaultModrinthAPI, "Modrinth API base URL")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *serverRoot == "" {
		fmt.Fprintln(os.Stderr, "--server-root is required")
		return 2
	}

	result, err := inventory.Scan(inventory.ScanOptions{
		ServerRoot:     *serverRoot,
		ServerModsPath: *serverMods,
		ClientModsPath: *clientMods,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "inventory failed: %v\n", err)
		return 1
	}

	lookupFailed := false
	if !*offline && len(result.Mods) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		matches, err := (inventory.ModrinthClient{BaseURL: *modrinthAPI}).Match(ctx, result.Mods)
		if err != nil {
			result.ModrinthError = err.Error()
			lookupFailed = true
		} else {
			inventory.ApplyModrinthMatches(&result, matches)
		}
	}

	renderInventory(result)

	if *jsonPath != "" {
		if err := writeJSON(*jsonPath, result); err != nil {
			fmt.Fprintf(os.Stderr, "write inventory JSON: %v\n", err)
			return 1
		}
		fmt.Printf("\nJSON: %s\n", *jsonPath)
	}

	if lookupFailed {
		fmt.Fprintln(os.Stderr, "\nModrinth matching failed; local inventory is complete but remote matches are not. Exit status 2.")
		return 2
	}
	return 0
}

func renderInventory(result inventory.Inventory) {
	fmt.Println("FPBPack inventory")
	fmt.Printf("Server root: %s\n", result.ServerRoot)
	fmt.Printf("Server/common JARs: %d\n", result.Summary.Server)
	fmt.Printf("Client-only JARs:  %d\n", result.Summary.Client)
	fmt.Printf("Total JARs:        %d\n", result.Summary.Total)
	if result.ModrinthChecked {
		fmt.Printf("Modrinth exact:    %d\n", result.Summary.ModrinthExact)
		fmt.Printf("Unmatched:         %d\n", result.Summary.Unmatched)
	} else if result.ModrinthError != "" {
		fmt.Printf("Modrinth:          LOOKUP FAILED (%s)\n", result.ModrinthError)
	} else {
		fmt.Println("Modrinth:          skipped")
	}
	if result.Summary.MetadataUnreadable > 0 {
		fmt.Printf("Metadata errors:   %d\n", result.Summary.MetadataUnreadable)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("WARNING: %s\n", warning)
	}

	fmt.Println()
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "LOCATION\tMATCH\tMOD ID\tVERSION\tFILE")
	for _, mod := range result.Mods {
		match := "-"
		if mod.Modrinth != nil {
			match = "modrinth"
		} else if !result.ModrinthChecked {
			match = "unchecked"
		}
		modID, modVersion := metadataDisplay(mod.Metadata)
		if mod.Error != "" {
			modID = "metadata-error"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", mod.Location, match, modID, modVersion, mod.Filename)
	}
	_ = writer.Flush()
}

func metadataDisplay(metadata []inventory.ModMetadata) (string, string) {
	if len(metadata) == 0 {
		return "-", "-"
	}
	ids := make([]string, 0, len(metadata))
	versions := make([]string, 0, len(metadata))
	for _, mod := range metadata {
		if mod.ModID != "" {
			ids = append(ids, mod.ModID)
		}
		if mod.Version != "" {
			versions = append(versions, mod.Version)
		}
	}
	id := strings.Join(ids, ",")
	if id == "" {
		id = "-"
	}
	modVersion := strings.Join(versions, ",")
	if modVersion == "" {
		modVersion = "-"
	}
	return id, modVersion
}

func writeJSON(path string, result inventory.Inventory) error {
	if parent := filepath.Dir(path); parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
