package userservice

import (
	"context"
	"testing"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	usersv1 "dev.local/banking-on-aspire/platform/gen/go/banking/users/v1"
	"dev.local/banking-on-aspire/services/users/internal/userrepo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type repositoryStub struct{ user *usersv1.User }

func (r *repositoryStub) Get(context.Context, string) (*usersv1.User, error) { return r.user, nil }

func (r *repositoryStub) Resolve(
	_ context.Context,
	identity userrepo.Identity,
) (*usersv1.User, error) {
	r.user = &usersv1.User{Name: "users/usr-test", KeycloakSubject: identity.Subject,
		Username: identity.Username, DisplayName: identity.DisplayName, Email: identity.Email,
		Status: usersv1.UserStatus_USER_STATUS_ACTIVE}
	return r.user, nil
}

func TestGetOrCreateCurrentUserUsesAuthenticatedClaims(t *testing.T) {
	repository := &repositoryStub{}
	service := New(repository)
	ctx := keycloak.WithClaims(context.Background(), &keycloak.Claims{
		Subject:           "kc-subject",
		PreferredUsername: "alice",
		Name:              "Alice",
		Email:             "alice@example.com",
	})
	user, err := service.GetOrCreateCurrentUser(ctx, &usersv1.GetOrCreateCurrentUserRequest{})
	if err != nil {
		t.Fatalf("GetOrCreateCurrentUser() error = %v", err)
	}
	if user.GetKeycloakSubject() != "kc-subject" || user.GetUsername() != "alice" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestGetUserRejectsAnotherIdentity(t *testing.T) {
	service := New(
		&repositoryStub{user: &usersv1.User{Name: "users/usr-test", KeycloakSubject: "owner"}},
	)
	ctx := keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "other"})
	_, err := service.GetUser(ctx, &usersv1.GetUserRequest{Name: "users/usr-test"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("GetUser() code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestGetUserRequiresAuthentication(t *testing.T) {
	service := New(&repositoryStub{})
	_, err := service.GetUser(context.Background(), &usersv1.GetUserRequest{Name: "users/usr-test"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetUser() code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}
