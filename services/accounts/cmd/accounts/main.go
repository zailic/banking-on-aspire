package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"dev.local/banking-on-aspire/platform/observability"
	"dev.local/banking-on-aspire/services/accounts/internal/accountrepo"
	"dev.local/banking-on-aspire/services/accounts/internal/accountservice"
	"dev.local/banking-on-aspire/services/accounts/internal/outbox"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

const keycloakClientID = "banking-on-aspire-app"

func required(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		slog.Error("required environment variable is not configured", "environment", name)
		os.Exit(1)
	}
	return value
}
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTelemetry, err := observability.Configure(ctx, "accounts")
	if err != nil {
		logger.Error("failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Warn("failed to shut down OpenTelemetry", "error", err)
		}
	}()
	repo, err := accountrepo.OpenPostgres(ctx, required("USERSDB_URI"))
	if err != nil {
		logger.Error("failed to initialize accounts repository", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	daprHTTPPort := required("DAPR_HTTP_PORT")
	verifier, err := keycloak.NewTokenVerifier(ctx, strings.TrimRight(required("KEYCLOAK_HTTP"), "/")+"/realms/banking-on-aspire", keycloakClientID)
	if err != nil {
		logger.Error("failed to initialize Keycloak verifier", "error", err)
		os.Exit(1)
	}
	port := strings.TrimSpace(os.Getenv("ACCOUNTS_PORT"))
	if port == "" {
		port = "8085"
	}
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Error("failed to listen", "error", err)
		os.Exit(1)
	}
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(keycloak.UnaryServerInterceptor(verifier, keycloakClientID)),
	)
	accountsv1.RegisterAccountsServiceServer(server, accountservice.New(repo, repo))
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	go outbox.New(repo, daprHTTPPort, logger).Run(ctx)
	go func() { <-ctx.Done(); healthServer.Shutdown(); server.GracefulStop() }()
	logger.Info("starting accounts gRPC service", "port", port)
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		logger.Error("accounts service stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
