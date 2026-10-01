package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	transactionsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/transactions/v1"
	"dev.local/banking-on-aspire/platform/observability"
	"dev.local/banking-on-aspire/services/transactions/internal/subscriber"
	"dev.local/banking-on-aspire/services/transactions/internal/transactionrepo"
	"dev.local/banking-on-aspire/services/transactions/internal/transactionservice"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

const keycloakClientID = "banking-on-aspire-app"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTelemetry, err := observability.Configure(ctx, "transactions")
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
	repository, err := transactionrepo.OpenPostgres(
		ctx,
		required("TRANSACTIONSDB_URI"),
		required("USERSDB_URI"),
	)
	if err != nil {
		logger.Error("failed to initialize transactions repository", "error", err)
		os.Exit(1)
	}
	defer repository.Close()
	discoveryURL := strings.TrimRight(required("KEYCLOAK_HTTP"), "/") + "/realms/banking-on-aspire"
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
	grpcPort := environmentOrDefault("TRANSACTIONS_GRPC_PORT", "8087")
	listener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Error("failed to listen for transactions gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(keycloak.UnaryServerInterceptor(verifier, keycloakClientID)),
	)
	transactionsv1.RegisterTransactionsServiceServer(grpcServer, transactionservice.New(repository))
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	httpServer := &http.Server{
		Addr:              ":" + environmentOrDefault("TRANSACTIONS_HTTP_PORT", "8086"),
		Handler:           otelhttp.NewHandler(subscriber.New(repository), "transactions.events"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("starting transactions Dapr subscriber", "address", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("transactions subscriber stopped unexpectedly", "error", err)
			stop()
		}
	}()
	go func() {
		<-ctx.Done()
		healthServer.Shutdown()
		grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	logger.Info("starting transactions gRPC service", "port", grpcPort)
	if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		logger.Error("transactions service stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func required(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		slog.Error("required environment variable is not configured", "environment", name)
		os.Exit(1)
	}
	return value
}

func environmentOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
