package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"dev.local/banking-on-aspire/services/contacts/internal/contactrepo"
)

const usersDatabaseURIEnvironment = "USERSDB_URI"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	connectionString := strings.TrimSpace(os.Getenv(usersDatabaseURIEnvironment))
	if connectionString == "" {
		logger.Error(
			"users database connection string is not configured",
			"environment",
			usersDatabaseURIEnvironment,
		)
		os.Exit(1)
	}

	ctx := context.Background()
	repository, err := contactrepo.OpenPostgres(ctx, connectionString)
	if err != nil {
		logger.Error("failed to connect to users database", "error", err)
		os.Exit(1)
	}
	defer repository.Close()

	if err := repository.Migrate(ctx); err != nil {
		logger.Error("failed to migrate contacts schema", "error", err)
		os.Exit(1)
	}
	logger.Info("contacts schema migration completed")
}
