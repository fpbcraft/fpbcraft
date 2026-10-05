package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/httpapi"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func runDoctor(args []string) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	inventoryPath := flags.String("inventory", "", "current FPBPack inventory JSON")
	reportPath := flags.String("report", "", "accepted migration report JSON")
	jsonOutput := flags.Bool("json", false, "write machine-readable diagnostics JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *inventoryPath == "" || *reportPath == "" {
		fmt.Fprintln(os.Stderr, "--inventory and --report are required")
		return 2
	}

	snapshot, err := (management.Source{InventoryPath: *inventoryPath, ReportPath: *reportPath}).Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor failed: %v\n", err)
		return 2
	}

	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(snapshot.Diagnostics); err != nil {
			fmt.Fprintf(os.Stderr, "write diagnostics: %v\n", err)
			return 2
		}
	} else {
		renderDiagnostics(snapshot)
	}

	if snapshot.Diagnostics.Summary.Blocking > 0 {
		return 1
	}
	return 0
}

func renderDiagnostics(snapshot management.Snapshot) {
	summary := snapshot.Diagnostics.Summary
	fmt.Println("FPBPack doctor")
	fmt.Printf("Blocking:   %d\n", summary.Blocking)
	fmt.Printf("Warnings:   %d\n", summary.Warnings)
	fmt.Printf("Info:       %d\n", summary.Info)
	fmt.Printf("Actionable: %d\n", summary.Actionable)
	if len(snapshot.Diagnostics.Findings) == 0 {
		fmt.Println("\nNo findings.")
		return
	}
	fmt.Println()
	for _, finding := range snapshot.Diagnostics.Findings {
		label := string(finding.Level)
		if finding.Mod != "" {
			fmt.Printf("[%s] %s: %s", label, finding.Mod, finding.Message)
		} else {
			fmt.Printf("[%s] %s", label, finding.Message)
		}
		if finding.Path != "" {
			fmt.Printf(" (%s)", finding.Path)
		}
		fmt.Println()
	}
}

func runServe(args []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	inventoryPath := flags.String("inventory", "", "current FPBPack inventory JSON")
	reportPath := flags.String("report", "", "accepted migration report JSON")
	updatesPath := flags.String("updates", "", "cached update report JSON (optional)")
	listen := flags.String("listen", "127.0.0.1:8787", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *inventoryPath == "" || *reportPath == "" {
		fmt.Fprintln(os.Stderr, "--inventory and --report are required")
		return 2
	}

	source := management.Source{InventoryPath: *inventoryPath, ReportPath: *reportPath}
	if _, err := source.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "serve preflight failed: %v\n", err)
		return 2
	}

	var updatesLoader httpapi.UpdatesLoader
	if *updatesPath != "" {
		updatesLoader = func() (updatecheck.Report, error) {
			file, err := os.Open(*updatesPath)
			if err != nil {
				return updatecheck.Report{}, fmt.Errorf("open update report: %w", err)
			}
			defer file.Close()
			var report updatecheck.Report
			if err := json.NewDecoder(file).Decode(&report); err != nil {
				return updatecheck.Report{}, fmt.Errorf("decode update report: %w", err)
			}
			return report, nil
		}
	}

	server := &http.Server{
		Addr: *listen,
		Handler: httpapi.NewHandlerWithOptions(source.Load, version, httpapi.ServerOptions{
			Updates: updatesLoader,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	fmt.Printf("FPBPack API listening on http://%s (read-only)\n", *listen)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			return 1
		}
		return 0
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "serve shutdown failed: %v\n", err)
			return 1
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			return 1
		}
		return 0
	}
}
