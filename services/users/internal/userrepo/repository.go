package userrepo

import (
	"context"
	"errors"

	usersv1 "dev.local/banking-on-aspire/platform/gen/go/banking/users/v1"
)

var (
	ErrNotFound = errors.New("user not found")
	ErrConflict = errors.New("user identity conflict")
)

type Identity struct {
	Subject, Username, DisplayName, Email string
}

type Repository interface {
	Get(context.Context, string) (*usersv1.User, error)
	Resolve(context.Context, Identity) (*usersv1.User, error)
}
