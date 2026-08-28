package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"dev.local/banking-on-aspire/services/users/internal/userrepo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	connectionString := strings.TrimSpace(os.Getenv("USERSDB_URI"))
	if connectionString == "" {
		logger.Error("users database connection string is not configured", "environment", "USERSDB_URI")
		os.Exit(1)
	}
	repository, err := userrepo.OpenPostgres(context.Background(), connectionString)
	if err != nil {
		logger.Error("failed to connect to users database", "error", err)
		os.Exit(1)
	}
	defer repository.Close()
	if err := repository.Migrate(context.Background()); err != nil {
		logger.Error("failed to migrate users schema", "error", err)
		os.Exit(1)
	}
	logger.Info("users schema migration completed")
}
