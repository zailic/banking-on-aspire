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

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	usersv1 "dev.local/banking-on-aspire/platform/gen/go/banking/users/v1"
	"dev.local/banking-on-aspire/services/users/internal/userrepo"
	"dev.local/banking-on-aspire/services/users/internal/userservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

const keycloakClientID = "banking-on-aspire-app"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectionString := requiredEnvironment(logger, "USERSDB_URI")
	keycloakURL := strings.TrimRight(requiredEnvironment(logger, "KEYCLOAK_HTTP"), "/")
	port := strings.TrimSpace(os.Getenv("USERS_PORT"))
	if port == "" {
		port = "8084"
	}

	repository, err := userrepo.OpenPostgres(ctx, connectionString)
	if err != nil {
		logger.Error("failed to initialize users repository", "error", err)
		os.Exit(1)
	}
	defer repository.Close()
	discoveryURL := keycloakURL + "/realms/banking-on-aspire"
	verifier, err := keycloak.NewTokenVerifier(
		ctx,
		discoveryURL,
		keycloakClientID,
		os.Getenv("KEYCLOAK_ISSUER"),
	)
	if err != nil {
		logger.Error("failed to initialize Keycloak verifier", "error", err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(keycloak.UnaryServerInterceptor(verifier, keycloakClientID)),
	)
	usersv1.RegisterUsersServiceServer(server, userservice.New(repository))
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	go func() { <-ctx.Done(); healthServer.Shutdown(); server.GracefulStop() }()
	logger.Info("starting users gRPC service", "port", port)
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		logger.Error("users service stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func requiredEnvironment(logger *slog.Logger, name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		logger.Error("required environment variable is not configured", "environment", name)
		os.Exit(1)
	}
	return value
}
