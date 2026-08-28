package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"dev.local/banking-on-aspire/services/accounts/actors"
	"dev.local/banking-on-aspire/services/accounts/infra/server"
	daprd "github.com/dapr/go-sdk/service/http"
	"github.com/go-chi/chi/v5"
)

const (
	keycloakClientID = "banking-on-aspire-app"
	keycloakRealmURL = "/realms/banking-on-aspire"
)

func resolveKeycloakIssuerURL() string {
	serviceURL := strings.TrimSpace(os.Getenv("KEYCLOAK_HTTP"))
	if serviceURL == "" {
		panic("KEYCLOAK_HTTP environment variable is not set")
	}
	return strings.TrimRight(serviceURL, "/") + keycloakRealmURL
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})))

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	bankAccountServer := server.NewBankAccountServer(ctx, resolveKeycloakIssuerURL(), keycloakClientID)
	mux := chi.NewMux()
	mux.Mount("/", bankAccountServer.Handler())

	service := daprd.NewServiceWithMux(":8080", mux)
	service.RegisterActorImplFactoryContext(actors.BankAccountServiceFactory)

	go func() {
		slog.Info("starting dapr service", "address", ":8080", "actor_type", actors.BankAccountActorType)
		if err := service.Start(); err != nil {
			slog.Error("failed to start dapr service", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	if err := service.Stop(); err != nil {
		slog.Warn("failed to stop dapr service cleanly", "error", err)
	}
}
