// Package keycloak provides shared OpenID Connect token verification and claims
// used by banking services. It does not call the Keycloak Admin API.
package keycloak

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

var ErrForbidden = errors.New("forbidden")

type Claims struct {
	Subject           string                    `json:"sub"`
	PreferredUsername string                    `json:"preferred_username"`
	Name              string                    `json:"name"`
	Email             string                    `json:"email"`
	RealmAccess       RealmAccess               `json:"realm_access"`
	ResourceAccess    map[string]ResourceAccess `json:"resource_access"`
}

// KeycloakClaims preserves the name used by the legacy Accounts service.
type KeycloakClaims = Claims

type RealmAccess struct {
	Roles []string `json:"roles"`
}

type ResourceAccess struct {
	Roles []string `json:"roles"`
}

func (c Claims) HasClientRole(clientID, requiredRole string) bool {
	access, ok := c.ResourceAccess[clientID]
	return ok && slices.Contains(access.Roles, requiredRole)
}

type TokenVerifier struct {
	verifier *oidc.IDTokenVerifier
	clientID string
}

func NewTokenVerifier(ctx context.Context, discoveryURL, clientID string, expectedIssuerURL ...string) (*TokenVerifier, error) {
	if len(expectedIssuerURL) > 1 {
		return nil, errors.New("only one expected issuer URL may be configured")
	}
	if len(expectedIssuerURL) == 1 && strings.TrimSpace(expectedIssuerURL[0]) != "" {
		// Discovery intentionally uses the private cluster URL, while tokens carry the
		// explicitly configured public issuer. Never derive this value from a request.
		ctx = oidc.InsecureIssuerURLContext(ctx, strings.TrimRight(expectedIssuerURL[0], "/"))
	}

	provider, err := oidc.NewProvider(ctx, strings.TrimRight(discoveryURL, "/"))
	if err != nil {
		return nil, err
	}
	return &TokenVerifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		clientID: clientID,
	}, nil
}

func (v *TokenVerifier) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	var claims Claims
	if err := token.Claims(&claims); err != nil {
		return nil, err
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("token subject is missing")
	}
	return &claims, nil
}

func (v *TokenVerifier) VerifyWithClientRole(ctx context.Context, rawToken, requiredRole string) (*Claims, error) {
	claims, err := v.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if !claims.HasClientRole(v.clientID, requiredRole) {
		return nil, ErrForbidden
	}
	return claims, nil
}

func (v *TokenVerifier) ClientID() string { return v.clientID }
