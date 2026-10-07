// Package server contains the preserved legacy Dapr actor implementation.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zailic/banking-on-aspire/services/accounts-legacy/domain"

	"github.com/zailic/banking-on-aspire/platform/auth/keycloak"
	api "github.com/zailic/banking-on-aspire/services/accounts-legacy/api"
	"github.com/dapr/go-sdk/client"
)

type claimsContextKey struct{}

type tokenVerifier interface {
	VerifyWithClientRole(
		ctx context.Context,
		rawToken string,
		requiredRole string,
	) (*keycloak.KeycloakClaims, error)
}

var usersMap = map[string]string{
	"local-dev": "demo-account-usd",
}

const (
	roleAccountsBalanceRead       = "accounts.balance.read"
	roleTransactionsDepositWrite  = "transactions.deposit.create"
	roleTransactionsWithdrawWrite = "transactions.withdraw.create"
	roleAccountsClose             = "accounts.close"
)

type BankAccountServer struct {
	daprClient    client.Client
	tokenVerifier tokenVerifier
	logger        *slog.Logger
}

func (s *BankAccountServer) deposit(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accounts")
	claims := r.Context().Value(claimsContextKey{}).(*keycloak.KeycloakClaims)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	clientID := claims.PreferredUsername
	if usersMap[clientID] != accountID {
		http.Error(w, "Forbiddens", http.StatusForbidden)
		return
	}
	actor := api.NewBankAccountClientStub(accountID)
	s.daprClient.ImplActorClientStub(actor)
	ctx := r.Context()
	amount, err := actor.Deposit(ctx, domain.Money{Amount: 100, Currency: "USD"})
	if err != nil {
		s.logger.Error("deposit failed", "account_id", accountID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(amount.Format()))
}

func (s *BankAccountServer) getBalance(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accounts")
	claims := r.Context().Value(claimsContextKey{}).(*keycloak.KeycloakClaims)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	clientID := claims.PreferredUsername
	if usersMap[clientID] != accountID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	actor := api.NewBankAccountClientStub(accountID)
	s.daprClient.ImplActorClientStub(actor)
	ctx := r.Context()
	balance, err := actor.GetBalance(ctx)
	if err != nil {
		s.logger.Error("get balance failed", "account_id", accountID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(balance.Format()))
}

func (s *BankAccountServer) withdraw(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accounts")
	claims := r.Context().Value(claimsContextKey{}).(*keycloak.KeycloakClaims)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	clientID := claims.PreferredUsername
	if usersMap[clientID] != accountID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	actor := api.NewBankAccountClientStub(accountID)
	s.daprClient.ImplActorClientStub(actor)
	ctx := r.Context()
	amount, err := actor.Withdraw(ctx, domain.Money{Amount: 50, Currency: "USD"})
	if err != nil {
		s.logger.Error("withdraw failed", "account_id", accountID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(amount.Format()))
}

func (s *BankAccountServer) closeAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("accounts")
	claims := r.Context().Value(claimsContextKey{}).(*keycloak.KeycloakClaims)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	clientID := claims.PreferredUsername
	if usersMap[clientID] != accountID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	actor := api.NewBankAccountClientStub(accountID)
	s.daprClient.ImplActorClientStub(actor)
	ctx := r.Context()
	err := actor.CloseAccount(ctx)
	if err != nil {
		s.logger.Error("close account failed", "account_id", accountID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Account closed"))
}

func (s *BankAccountServer) authorize(
	requiredRole string,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken, err := bearerToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := s.tokenVerifier.VerifyWithClientRole(r.Context(), rawToken, requiredRole)
		if err != nil {
			s.logger.Warn(
				"token verification failed",
				"error", err,
				"required_role", requiredRole,
				"path", r.URL.Path,
			)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		s.logger.Debug(
			"authorized request",
			"user", claims.PreferredUsername,
			"required_role", requiredRole,
			"path", r.URL.Path,
		)

		ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (s *BankAccountServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(
		"GET /accounts/{accounts}/balance",
		s.authorize(roleAccountsBalanceRead, http.HandlerFunc(s.getBalance)),
	)
	mux.Handle(
		"POST /accounts/{accounts}/withdraw",
		s.authorize(roleTransactionsWithdrawWrite, http.HandlerFunc(s.withdraw)),
	)
	mux.Handle(
		"POST /accounts/{accounts}/deposit",
		s.authorize(roleTransactionsDepositWrite, http.HandlerFunc(s.deposit)),
	)
	mux.Handle(
		"POST /accounts/{accounts}/close",
		s.authorize(roleAccountsClose, http.HandlerFunc(s.closeAccount)),
	)

	return mux
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", ErrInvalidAuthHeader
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", ErrEmptyToken
	}

	return token, nil
}

func NewBankAccountServer(
	ctx context.Context,
	keycloakIssuerURL string,
	keycloakClientID string,
) *BankAccountServer {
	daprClient, err := client.NewClient()
	if err != nil {
		panic(err)
	}

	tokenVerifier, err := keycloak.NewTokenVerifier(ctx, keycloakIssuerURL, keycloakClientID)
	if err != nil {
		panic(err)
	}
	return &BankAccountServer{
		daprClient:    daprClient,
		tokenVerifier: tokenVerifier,
		logger:        slog.Default().With("component", "bank-account-server"),
	}
}
