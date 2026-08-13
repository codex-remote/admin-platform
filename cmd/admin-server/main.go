package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/server"
	"github.com/ai-coding-remote/admin-platform/internal/store"
)

func main() {
	defaultRoot, _ := filepath.Abs("..")
	listen := flag.String("listen", envOr("ADMIN_LISTEN", "127.0.0.1:18880"), "HTTP listen address")
	workspace := flag.String("workspace-root", envOr("ADMIN_WORKSPACE_ROOT", defaultRoot), "CodexRemote workspace root")
	data := flag.String("data-dir", envOr("ADMIN_DATA_DIR", defaultDataDir()), "persistent artifact directory")
	databaseURL := flag.String("database-url", envOr("ADMIN_DATABASE_URL", "postgres://codexremote_admin_app@127.0.0.1:5432/codexremote_admin?sslmode=disable&timezone=UTC"), "PostgreSQL connection URL")
	token := flag.String("ingest-token", os.Getenv("ADMIN_INGEST_TOKEN"), "bearer token for event ingestion")
	flag.Parse()
	if err := validateListen(*listen); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(*data, 0o700); err != nil {
		fatal(err)
	}
	db, err := store.Open(*databaseURL)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	app := server.New(db, server.Config{WorkspaceRoot: *workspace, DataDir: *data, IngestToken: *token})
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	httpServer := &http.Server{Addr: *listen, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Minute, WriteTimeout: 30 * time.Minute, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	slog.Info("Admin Platform listening", "address", "http://"+*listen, "data_dir", *data)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".data"
	}
	return filepath.Join(home, "Library", "Application Support", "CodexRemote Admin")
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func validateListen(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("non-loopback listen address %q is disabled until Admin authentication is implemented", address)
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
