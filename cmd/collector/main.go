package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/collector"
)

func main() {
	defaultRoot, _ := filepath.Abs("..")
	adminURL := flag.String("admin-url", envOr("ADMIN_URL", "http://127.0.0.1:18880"), "Admin Server URL")
	workspace := flag.String("workspace-root", envOr("ADMIN_WORKSPACE_ROOT", defaultRoot), "CodexRemote workspace root")
	stateDir := flag.String("state-dir", envOr("COLLECTOR_STATE_DIR", defaultStateDir()), "checkpoint and spool directory")
	token := flag.String("ingest-token", os.Getenv("ADMIN_INGEST_TOKEN"), "Admin ingest bearer token")
	simulator := flag.String("simulator", envOr("COLLECTOR_SIMULATOR", "booted"), "Simulator UDID or booted")
	bundleID := flag.String("iphone-bundle-id", envOr("IPHONE_BUNDLE_ID", "com.leehooo.codexremote.dev925r8v9794"), "iPhone app bundle identifier")
	collectorID := flag.String("collector-id", envOr("COLLECTOR_ID", "local-mac"), "stable collector identifier")
	interval := flag.Duration("interval", 2*time.Second, "collection interval")
	once := flag.Bool("once", false, "run one collection cycle")
	flag.Parse()
	value, err := collector.New(collector.Config{AdminURL: *adminURL, IngestToken: *token, StateDir: *stateDir,
		WorkspaceRoot: *workspace, SimulatorUDID: *simulator, IPhoneBundleID: *bundleID, Interval: *interval, CollectorID: *collectorID})
	if err != nil {
		fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *once {
		if err := value.RunOnce(ctx); err != nil {
			fatal(err)
		}
		return
	}
	slog.Info("Diagnostics Collector started", "admin_url", *adminURL, "state_dir", *stateDir, "simulator", *simulator)
	if err := value.Run(ctx); err != nil && err != context.Canceled {
		fatal(err)
	}
}

func defaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".collector"
	}
	return filepath.Join(home, "Library", "Application Support", "CodexRemote Admin", "collector")
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
