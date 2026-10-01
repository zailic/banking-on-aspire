package userservice

import (
	"context"
	"errors"
	"strings"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	usersv1 "dev.local/banking-on-aspire/platform/gen/go/banking/users/v1"
	"dev.local/banking-on-aspire/services/users/internal/userrepo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	usersv1.UnimplementedUsersServiceServer
	repository userrepo.Repository
}

func New(repository userrepo.Repository) *Service {
	if repository == nil {
		panic("users repository is required")
	}
	return &Service{repository: repository}
}

func (s *Service) GetUser(ctx context.Context, req *usersv1.GetUserRequest) (*usersv1.User, error) {
	if req == nil || !validUserName(req.GetName()) {
		return nil, status.Error(codes.InvalidArgument, "name must have the form users/{user}")
	}
	claims, ok := keycloak.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authenticated identity is required")
	}
	user, err := s.repository.Get(ctx, req.GetName())
	if err != nil {
		return nil, repositoryError(err)
	}
	if user.GetKeycloakSubject() != claims.Subject {
		return nil, status.Error(
			codes.PermissionDenied,
			"the requested profile is owned by another identity",
		)
	}
	return user, nil
}

func (s *Service) GetOrCreateCurrentUser(
	ctx context.Context,
	_ *usersv1.GetOrCreateCurrentUserRequest,
) (*usersv1.User, error) {
	claims, ok := keycloak.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authenticated identity is required")
	}
	username := strings.TrimSpace(claims.PreferredUsername)
	if username == "" {
		username = claims.Subject
	}
	displayName := strings.TrimSpace(claims.Name)
	if displayName == "" {
		displayName = username
	}
	user, err := s.repository.Resolve(ctx, userrepo.Identity{
		Subject: claims.Subject, Username: username,
		DisplayName: displayName, Email: strings.TrimSpace(claims.Email),
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	return user, nil
}

func validUserName(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && parts[0] == "users" && parts[1] != ""
}

func repositoryError(err error) error {
	switch {
	case errors.Is(err, userrepo.ErrNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, userrepo.ErrConflict):
		return status.Error(codes.AlreadyExists, "identity conflicts with an existing user")
	default:
		return status.Error(codes.Internal, "user persistence failed")
	}
}
