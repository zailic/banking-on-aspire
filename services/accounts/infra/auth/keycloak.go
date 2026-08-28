package auth

import (
	"context"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
)

type KeycloakClaims struct {
	Subject           string                    `json:"sub"`
	PreferredUsername string                    `json:"preferred_username"`
	RealmAccess       RealmAccess               `json:"realm_access"`
	ResourceAccess    map[string]ResourceAccess `json:"resource_access"`
}

type RealmAccess struct {
	Roles []string `json:"roles"`
}

type ResourceAccess struct {
	Roles []string `json:"roles"`
}

type TokenVerifier struct {
	verifier *oidc.IDTokenVerifier
	clientID string
}

// HasClientRole checks if the claims contain the required role for the given client ID.
func (c KeycloakClaims) HasClientRole(
	clientID string,
	requiredRole string,
) bool {
	access, ok := c.ResourceAccess[clientID]
	if !ok {
		return false
	}

	return slices.Contains(access.Roles, requiredRole)
}

// NewTokenVerifier creates a new TokenVerifier for the given issuer URL and client ID.
func NewTokenVerifier(
	ctx context.Context,
	issuerURL string,
	clientID string,
) (*TokenVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}

	config := &oidc.Config{
		ClientID: clientID,
	}

	return &TokenVerifier{
		verifier: provider.Verifier(config),
		clientID: clientID,
	}, nil
}

// Verify verifies the given raw token and returns the claims if valid.
func (v *TokenVerifier) Verify(
	ctx context.Context,
	rawToken string,
) (*KeycloakClaims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	var claims KeycloakClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}

	return &claims, nil
}

// VerifyWithClientRole verifies the given raw token and checks if it
// contains the required role for the client ID.
func (v *TokenVerifier) VerifyWithClientRole(
	ctx context.Context,
	rawToken string,
	requiredRole string,
) (*KeycloakClaims, error) {
	claims, err := v.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	if !claims.HasClientRole(v.clientID, requiredRole) {
		return nil, ErrForbidden
	}

	return claims, nil
}

// ClientID returns the client ID associated with the TokenVerifier.
func (v *TokenVerifier) ClientID() string {
	return v.clientID
}
