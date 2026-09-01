package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"dev.local/banking-on-aspire/services/transactions/internal/transactionrepo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	repository, err := transactionrepo.OpenPostgres(context.Background(), required("TRANSACTIONSDB_URI"), required("USERSDB_URI"))
	if err != nil {
		logger.Error("failed to connect to transaction databases", "error", err)
		os.Exit(1)
	}
	defer repository.Close()
	if err := repository.Migrate(context.Background()); err != nil {
		logger.Error("failed to migrate transactions schema", "error", err)
		os.Exit(1)
	}
	logger.Info("transactions schema migration completed")
}

func required(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		slog.Error("required environment variable is not configured", "environment", name)
		os.Exit(1)
	}
	return value
}
