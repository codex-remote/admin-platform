package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/codex-remote/admin-platform/internal/store"
)

func main() {
	databaseURL := flag.String("database-url", envOr("ADMIN_MIGRATOR_DATABASE_URL", "postgres://codexremote_admin_migrator@127.0.0.1:5432/codexremote_admin?sslmode=disable&timezone=UTC"), "PostgreSQL migrator URL")
	flag.Parse()
	db, err := store.Open(*databaseURL)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.Migrate(ctx); err != nil {
		fatal(err)
	}
	fmt.Println("database migrations applied")
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
