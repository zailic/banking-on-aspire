package contactservice

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	contactsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	"dev.local/banking-on-aspire/services/contacts/internal/contactrepo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultPageSize = 50

type Service struct {
	contactsv1.UnimplementedContactsServiceServer
	repository contactrepo.Repository
	users      contactrepo.UserResolver
}

func New(repository contactrepo.Repository, users contactrepo.UserResolver) *Service {
	if repository == nil {
		panic("contacts repository is required")
	}
	if users == nil {
		panic("contacts user resolver is required")
	}
	return &Service{repository: repository, users: users}
}

func (s *Service) ListContacts(ctx context.Context, req *contactsv1.ListContactsRequest) (*contactsv1.ListContactsResponse, error) {
	if req == nil || !validParent(req.GetParent()) {
		return nil, status.Error(codes.InvalidArgument, "parent must have the form users/{user}")
	}
	if req.GetPageSize() < 0 {
		return nil, status.Error(codes.InvalidArgument, "page_size must not be negative")
	}
	if err := s.authorizeParent(ctx, req.GetParent()); err != nil {
		return nil, err
	}

	pageSize := int(req.GetPageSize())
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset, err := decodePageToken(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "page_token is invalid")
	}

	contacts, err := s.repository.List(ctx, req.GetParent())
	if err != nil {
		return nil, repositoryError(err)
	}
	if offset > len(contacts) {
		return nil, status.Error(codes.InvalidArgument, "page_token is outside the result set")
	}

	end := min(offset+pageSize, len(contacts))
	result := &contactsv1.ListContactsResponse{Contacts: contacts[offset:end]}
	if end < len(contacts) {
		result.NextPageToken = encodePageToken(end)
	}
	return result, nil
}

func (s *Service) GetContact(ctx context.Context, req *contactsv1.GetContactRequest) (*contactsv1.Contact, error) {
	if req == nil || !validContactName(req.GetName()) {
		return nil, status.Error(codes.InvalidArgument, "name must have the form users/{user}/contacts/{contact}")
	}
	if err := s.authorizeParent(ctx, contactParent(req.GetName())); err != nil {
		return nil, err
	}
	contact, err := s.repository.Get(ctx, req.GetName())
	if err != nil {
		return nil, repositoryError(err)
	}
	return contact, nil
}

func (s *Service) CreateContact(ctx context.Context, req *contactsv1.CreateContactRequest) (*contactsv1.Contact, error) {
	if req == nil || !validParent(req.GetParent()) {
		return nil, status.Error(codes.InvalidArgument, "parent must have the form users/{user}")
	}
	if !validID(req.GetContactId()) {
		return nil, status.Error(codes.InvalidArgument, "contact_id must start with a letter and contain only lowercase letters, digits, or hyphens")
	}
	if err := validateContact(req.GetContact()); err != nil {
		return nil, err
	}
	if err := s.authorizeParent(ctx, req.GetParent()); err != nil {
		return nil, err
	}

	contact := cloneContact(req.GetContact())
	contact.Name = req.GetParent() + "/contacts/" + req.GetContactId()
	now := timestamppb.Now()
	contact.CreateTime = now
	contact.UpdateTime = now
	contact.Etag = newEtag()

	if err := s.repository.Create(ctx, contact); err != nil {
		return nil, repositoryError(err)
	}
	return contact, nil
}

func (s *Service) UpdateContact(ctx context.Context, req *contactsv1.UpdateContactRequest) (*contactsv1.Contact, error) {
	if req == nil || req.GetContact() == nil || !validContactName(req.GetContact().GetName()) {
		return nil, status.Error(codes.InvalidArgument, "contact.name must have the form users/{user}/contacts/{contact}")
	}
	if req.GetUpdateMask() == nil || len(req.GetUpdateMask().GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required")
	}
	if err := s.authorizeParent(ctx, contactParent(req.GetContact().GetName())); err != nil {
		return nil, err
	}

	current, err := s.repository.Get(ctx, req.GetContact().GetName())
	if err != nil {
		return nil, repositoryError(err)
	}

	updated := cloneContact(current)
	for _, path := range req.GetUpdateMask().GetPaths() {
		switch path {
		case "display_name":
			updated.DisplayName = req.GetContact().GetDisplayName()
		case "internal_account", "external_account", "destination":
			updated.Destination = req.GetContact().GetDestination()
		default:
			return nil, status.Errorf(codes.InvalidArgument, "field %q cannot be updated", path)
		}
	}
	if err := validateContact(updated); err != nil {
		return nil, err
	}
	updated.UpdateTime = timestamppb.Now()
	updated.Etag = newEtag()
	if err := s.repository.Update(ctx, updated, req.GetContact().GetEtag()); err != nil {
		return nil, repositoryError(err)
	}
	return updated, nil
}

func (s *Service) DeleteContact(ctx context.Context, req *contactsv1.DeleteContactRequest) (*emptypb.Empty, error) {
	if req == nil || !validContactName(req.GetName()) {
		return nil, status.Error(codes.InvalidArgument, "name must have the form users/{user}/contacts/{contact}")
	}
	if err := s.authorizeParent(ctx, contactParent(req.GetName())); err != nil {
		return nil, err
	}
	if err := s.repository.Delete(ctx, req.GetName(), req.GetEtag()); err != nil {
		return nil, repositoryError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) authorizeParent(ctx context.Context, parent string) error {
	claims, ok := keycloak.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return status.Error(codes.Unauthenticated, "authenticated identity is required")
	}
	userName, err := s.users.ResolveUserName(ctx, claims.Subject)
	if errors.Is(err, contactrepo.ErrUserNotFound) {
		return status.Error(codes.PermissionDenied, "the authenticated identity has no active user profile")
	}
	if err != nil {
		return status.Error(codes.Internal, "contact owner resolution failed")
	}
	if userName != parent {
		return status.Error(codes.PermissionDenied, "the requested contacts are owned by another identity")
	}
	return nil
}

func repositoryError(err error) error {
	switch {
	case errors.Is(err, contactrepo.ErrNotFound):
		return status.Error(codes.NotFound, "contact not found")
	case errors.Is(err, contactrepo.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "contact already exists")
	case errors.Is(err, contactrepo.ErrConflict):
		return status.Error(codes.Aborted, "etag does not match the current contact")
	default:
		return status.Error(codes.Internal, "contact persistence failed")
	}
}

func validateContact(contact *contactsv1.Contact) error {
	if contact == nil {
		return status.Error(codes.InvalidArgument, "contact is required")
	}
	if strings.TrimSpace(contact.GetDisplayName()) == "" {
		return status.Error(codes.InvalidArgument, "contact.display_name is required")
	}
	switch destination := contact.GetDestination().(type) {
	case *contactsv1.Contact_InternalAccount:
		if !validAccountName(destination.InternalAccount) {
			return status.Error(codes.InvalidArgument, "internal_account must have the form accounts/{account}")
		}
	case *contactsv1.Contact_ExternalAccount:
		if destination.ExternalAccount == nil || strings.TrimSpace(destination.ExternalAccount.GetRoutingNumber()) == "" || strings.TrimSpace(destination.ExternalAccount.GetAccountNumber()) == "" {
			return status.Error(codes.InvalidArgument, "external_account requires routing_number and account_number")
		}
	default:
		return status.Error(codes.InvalidArgument, "exactly one contact destination is required")
	}
	return nil
}

func validParent(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && parts[0] == "users" && parts[1] != ""
}

func validContactName(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 4 && parts[0] == "users" && parts[1] != "" && parts[2] == "contacts" && parts[3] != ""
}

func contactParent(name string) string {
	parts := strings.Split(name, "/")
	return strings.Join(parts[:2], "/")
}

func validAccountName(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && parts[0] == "accounts" && parts[1] != ""
}

func validID(value string) bool {
	if len(value) == 0 || len(value) > 63 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func cloneContact(contact *contactsv1.Contact) *contactsv1.Contact {
	return proto.Clone(contact).(*contactsv1.Contact)
}

func newEtag() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Errorf("generate contact etag: %w", err))
	}
	return hex.EncodeToString(value)
}

func encodePageToken(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid offset")
	}
	return offset, nil
}
