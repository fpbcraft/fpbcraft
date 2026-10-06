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
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/httpapi"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/service"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/webui"
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
	serverRoot := flags.String("server-root", envOrDefault("FPBPACK_SERVER_ROOT", "/server"), "Crafty/Minecraft server root")
	stateDir := flags.String("state-dir", envOrDefault("FPBPACK_STATE_DIR", "/data"), "FPBPack durable state/cache directory")
	serverMods := flags.String("server-mods", inventory.DefaultServerModsPath, "server/common mods path relative to server root")
	clientMods := flags.String("client-mods", inventory.DefaultClientModsPath, "AutoModpack client-only mods path relative to server root")
	minecraft := flags.String("minecraft", "1.21.1", "Minecraft version for update compatibility")
	loader := flags.String("loader", "neoforge", "mod loader for update compatibility")
	modrinthAPI := flags.String("modrinth-api", inventory.DefaultModrinthAPI, "Modrinth API base URL")
	bootstrapReport := flags.String("bootstrap-report", "", "legacy migration report to import only when state.json does not exist")
	refreshInterval := flags.Duration("refresh-interval", 6*time.Hour, "automatic inventory/update refresh interval; 0 disables periodic refresh")
	listen := flags.String("listen", "0.0.0.0:8787", "HTTP listen address")
	webDir := flags.String("web-dir", "", "serve GUI files from this directory instead of embedded assets")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := service.New(ctx, service.Options{
		ServerRoot:      *serverRoot,
		StateDir:        *stateDir,
		ServerModsPath:  *serverMods,
		ClientModsPath:  *clientMods,
		Minecraft:       *minecraft,
		Loader:          *loader,
		ModrinthBaseURL: *modrinthAPI,
		CurseForgeAPIKey: strings.TrimSpace(os.Getenv("FPBPACK_CURSEFORGE_API_KEY")),
		GitHubToken: strings.TrimSpace(os.Getenv("FPBPACK_GITHUB_TOKEN")),
		CraftyURL: strings.TrimSpace(os.Getenv("FPBPACK_CRAFTY_URL")),
		CraftyServerID: strings.TrimSpace(os.Getenv("FPBPACK_CRAFTY_SERVER_ID")),
		CraftyToken: strings.TrimSpace(os.Getenv("FPBPACK_CRAFTY_TOKEN")),
		CraftyAllowInsecure: strings.EqualFold(strings.TrimSpace(os.Getenv("FPBPACK_CRAFTY_INSECURE")), "true"),
		BootstrapReport: *bootstrapReport,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "serve startup failed: %v\n", err)
		return 2
	}

	webHandler := webui.Handler()
	if *webDir != "" {
		webHandler = http.FileServer(http.Dir(*webDir))
	}

	server := &http.Server{
		Addr: *listen,
		Handler: httpapi.NewHandlerWithOptions(app.Snapshot, version, httpapi.ServerOptions{
			Updates:      app.Updates,
			Catalog:      app.Catalog,
			CatalogPreview: app.CatalogPreview,
			SearchCatalog: app.SearchCatalog,
			CatalogVersions: app.CatalogVersions,
			CreateCatalogPlan: app.CreateCatalogPlan,
			Refresh:      app.Refresh,
			RefreshInventory: app.RefreshInventory,
			CheckUpdates: app.CheckUpdates,
			CreatePlan:   app.CreatePlan,
			CreatePlacementPlan: app.CreatePlacementPlan,
			Plans:        app.Plans,
			Plan:         app.Plan,
			History:      app.History,
			Retention:    app.Settings,
			UpdateRetention: app.UpdateSettings,
			Rules:        app.Rules,
			SetRule:      app.SetRule,
			ClearRule:    app.ClearRule,
			RefreshStatus: app.RefreshStatus,
			Logs:          app.Logs,
			Providers:     app.ProviderStatuses,
			SetProviderCredential: app.SetProviderCredential,
			ClearProviderCredential: app.ClearProviderCredential,
			ManageMod:      app.ManageMod,
			RefreshMod:     app.RefreshModMetadata,
			CraftyStatus:   app.CraftyStatus,
			SetCraftyConfig: app.SetCraftyConfig,
			ClearCraftyCredential: app.ClearCraftyCredential,
			StartServer:    app.StartServer,
			StopServer:     app.StopServer,
			NeoForgeStatus: app.NeoForgeStatus,
			ChangeNeoForge: app.ChangeNeoForge,
			ApplyPlan:      app.ApplyPlan,
			RestoreBackup:  app.RestoreBackup,
			AcceptManualArtifact: app.AcceptManualArtifact,
			AutoModpackStatus: app.AutoModpackStatus,
			UpdateAutoModpackConfig: app.UpdateAutoModpackConfig,
			MigrateAutoModpackGroup: app.MigrateAutoModpackGroup,
			RunAutoModpackAction: app.RunAutoModpackAction,
			BackgroundContext: ctx,
			Web:          webHandler,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	if *refreshInterval > 0 {
		go runPeriodicRefresh(
			ctx,
			*refreshInterval,
			app.Refresh,
			func(err error) {
				if ctx.Err() == nil {
					fmt.Fprintf(os.Stderr, "automatic refresh failed: %v\n", err)
				}
			},
		)
	}

	fmt.Printf("FPBPack listening on http://%s (GUI + management API)\n", *listen)
	if *refreshInterval > 0 {
		fmt.Printf("Automatic refresh interval: %s (first run after the interval; no startup refresh)\n", refreshInterval.String())
	} else {
		fmt.Println("Automatic refresh disabled; use the GUI/API to refresh manually.")
	}
	fmt.Printf("Server root: %s\n", *serverRoot)
	fmt.Printf("State dir:   %s\n", *stateDir)

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

func runPeriodicRefresh(
	ctx context.Context,
	interval time.Duration,
	refresh func(context.Context) error,
	onError func(error),
) {
	if interval <= 0 || refresh == nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	periodicRefreshLoop(ctx, ticker.C, refresh, onError)
}

func periodicRefreshLoop(
	ctx context.Context,
	ticks <-chan time.Time,
	refresh func(context.Context) error,
	onError func(error),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if err := refresh(ctx); err != nil && onError != nil && ctx.Err() == nil {
				onError(err)
			}
		}
	}
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
