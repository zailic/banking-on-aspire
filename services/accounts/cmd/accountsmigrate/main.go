package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"dev.local/banking-on-aspire/services/accounts/internal/accountrepo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	uri := strings.TrimSpace(os.Getenv("USERSDB_URI"))
	if uri == "" {
		logger.Error("users database connection string is not configured")
		os.Exit(1)
	}
	repo, err := accountrepo.OpenPostgres(context.Background(), uri)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	if err := repo.Migrate(context.Background()); err != nil {
		logger.Error("failed to migrate accounts schema", "error", err)
		os.Exit(1)
	}
	logger.Info("accounts schema migration completed")
}
