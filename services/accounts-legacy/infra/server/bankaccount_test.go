// Package server contains the preserved legacy Dapr actor implementation.
package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
)

type mockTokenVerifier struct {
	claims       *keycloak.KeycloakClaims
	err          error
	receivedRaw  string
	receivedRole string
}

func (m *mockTokenVerifier) VerifyWithClientRole(
	ctx context.Context,
	rawToken string,
	requiredRole string,
) (*keycloak.KeycloakClaims, error) {
	m.receivedRaw = rawToken
	m.receivedRole = requiredRole
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantErr   error
	}{
		{
			name:      "valid bearer token",
			header:    "Bearer abc.def.ghi",
			wantToken: "abc.def.ghi",
		},
		{
			name:    "missing bearer prefix",
			header:  "Token abc.def.ghi",
			wantErr: ErrInvalidAuthHeader,
		},
		{
			name:    "empty token",
			header:  "Bearer   ",
			wantErr: ErrEmptyToken,
		},
		{
			name:    "no header",
			header:  "",
			wantErr: ErrInvalidAuthHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			gotToken, err := bearerToken(req)
			if err != tt.wantErr {
				t.Fatalf("bearerToken() error = %v, want %v", err, tt.wantErr)
			}
			if gotToken != tt.wantToken {
				t.Fatalf("bearerToken() token = %q, want %q", gotToken, tt.wantToken)
			}
		})
	}
}

func TestAuthorizeUnauthorizedWhenHeaderMissing(t *testing.T) {
	server := &BankAccountServer{
		tokenVerifier: &mockTokenVerifier{},
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	nextCalled := false
	h := server.authorize(
		roleAccountsBalanceRead,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/accounts/demo-account-usd/balance", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if nextCalled {
		t.Fatal("expected next handler to not be called")
	}
}

func TestAuthorizeForbiddenWhenVerifierFails(t *testing.T) {
	verifier := &mockTokenVerifier{err: errors.New("forbidden")}
	server := &BankAccountServer{
		tokenVerifier: verifier,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	nextCalled := false
	h := server.authorize(
		roleAccountsBalanceRead,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/accounts/demo-account-usd/balance", nil)
	req.Header.Set("Authorization", "Bearer good.token")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if nextCalled {
		t.Fatal("expected next handler to not be called")
	}
	if verifier.receivedRaw != "good.token" {
		t.Fatalf("raw token = %q, want %q", verifier.receivedRaw, "good.token")
	}
	if verifier.receivedRole != roleAccountsBalanceRead {
		t.Fatalf("required role = %q, want %q", verifier.receivedRole, roleAccountsBalanceRead)
	}
}

func TestAuthorizeForbiddenResponseBody(t *testing.T) {
	server := &BankAccountServer{
		tokenVerifier: &mockTokenVerifier{err: errors.New("forbidden")},
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	h := server.authorize(
		roleAccountsBalanceRead,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	req := httptest.NewRequest(http.MethodGet, "/accounts/demo-account-usd/balance", nil)
	req.Header.Set("Authorization", "Bearer good.token")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "Forbidden") {
		t.Fatalf("expected forbidden response body to mention forbidden, got %q", w.Body.String())
	}
}

func TestAuthorizeSuccessAttachesClaimsAndCallsNext(t *testing.T) {
	verifier := &mockTokenVerifier{claims: &keycloak.KeycloakClaims{PreferredUsername: "ionut"}}
	server := &BankAccountServer{
		tokenVerifier: verifier,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	nextCalled := false
	h := server.authorize(
		roleAccountsBalanceRead,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			claims, ok := r.Context().Value(claimsContextKey{}).(*keycloak.KeycloakClaims)
			if !ok || claims == nil {
				t.Fatal("expected claims in context")
			}
			if claims.PreferredUsername != "ionut" {
				t.Fatalf("preferred username = %q, want %q", claims.PreferredUsername, "ionut")
			}
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/accounts/demo-account-usd/balance", nil)
	req.Header.Set("Authorization", "Bearer good.token")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}
