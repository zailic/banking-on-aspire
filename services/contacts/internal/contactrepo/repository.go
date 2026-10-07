package contactrepo

import (
	"context"
	"errors"

	contactsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/contacts/v1"
)

var (
	ErrNotFound      = errors.New("contact not found")
	ErrAlreadyExists = errors.New("contact already exists")
	ErrConflict      = errors.New("contact version conflict")
	ErrUserNotFound  = errors.New("user identity not found")
)

// Repository is the persistence boundary for the Contacts service.
type Repository interface {
	List(context.Context, string) ([]*contactsv1.Contact, error)
	Get(context.Context, string) (*contactsv1.Contact, error)
	Create(context.Context, *contactsv1.Contact) error
	Update(context.Context, *contactsv1.Contact, string) error
	Delete(context.Context, string, string) error
}

// UserResolver maps an immutable identity-provider subject to the canonical
// application user resource that may own contacts.
type UserResolver interface {
	ResolveUserName(context.Context, string) (string, error)
}
