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
	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	contactsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	"dev.local/banking-on-aspire/services/contacts/internal/accountclient"
	"dev.local/banking-on-aspire/services/contacts/internal/contactrepo"
	"dev.local/banking-on-aspire/services/contacts/internal/contactservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

const usersDatabaseURIEnvironment = "USERSDB_URI"
const keycloakClientID = "banking-on-aspire-app"

func listenAddress() string {
	port := strings.TrimSpace(os.Getenv("CONTACTS_PORT"))
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	address := listenAddress()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectionString := strings.TrimSpace(os.Getenv(usersDatabaseURIEnvironment))
	if connectionString == "" {
		logger.Error(
			"users database connection string is not configured",
			"environment",
			usersDatabaseURIEnvironment,
		)
		os.Exit(1)
	}
	repository, err := contactrepo.OpenPostgres(ctx, connectionString)
	if err != nil {
		logger.Error("failed to initialize contacts repository", "error", err)
		os.Exit(1)
	}
	defer repository.Close()
	keycloakURL := strings.TrimRight(requiredEnvironment("KEYCLOAK_HTTP"), "/")
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
	accountsConnection, err := grpc.NewClient(
		grpcAddress(requiredEnvironment("ACCOUNTS_GRPC")),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to initialize Accounts client", "error", err)
		os.Exit(1)
	}
	defer accountsConnection.Close()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("failed to listen", "address", address, "error", err)
		os.Exit(1)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(keycloak.UnaryServerInterceptor(verifier, keycloakClientID)),
	)
	contactsv1.RegisterContactsServiceServer(server, contactservice.New(
		repository,
		repository,
		accountclient.New(accountsv1.NewAccountsServiceClient(accountsConnection)),
	))
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	go func() {
		<-ctx.Done()
		healthServer.Shutdown()
		server.GracefulStop()
	}()

	logger.Info("starting contacts gRPC service", "address", address)
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		logger.Error("contacts service stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func grpcAddress(value string) string {
	if strings.HasPrefix(value, "grpc://") {
		return "dns:///" + strings.TrimPrefix(value, "grpc://")
	}
	return value
}

func requiredEnvironment(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		slog.Error("required environment variable is not configured", "environment", name)
		os.Exit(1)
	}
	return value
}
