package contactservice_test

import (
	"context"
	"net"
	"testing"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	contactsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	"dev.local/banking-on-aspire/services/contacts/internal/contactrepo"
	"dev.local/banking-on-aspire/services/contacts/internal/contactservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type userResolverStub struct {
	userName string
	err      error
}

type accountValidatorStub struct {
	err   error
	calls []string
}

func (v *accountValidatorStub) ValidateInternalAccount(_ context.Context, name string) error {
	v.calls = append(v.calls, name)
	return v.err
}

func (r userResolverStub) ResolveUserName(context.Context, string) (string, error) {
	return r.userName, r.err
}

func TestContactsLifecycle(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.CreateContact(ctx, &contactsv1.CreateContactRequest{
		Parent:    "users/alice",
		ContactId: "electric-company",
		Contact: &contactsv1.Contact{
			DisplayName: "Electric Company",
			Destination: &contactsv1.Contact_InternalAccount{
				InternalAccount: "accounts/utility-001",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateContact() error = %v", err)
	}
	if created.GetName() != "users/alice/contacts/electric-company" {
		t.Fatalf("CreateContact() name = %q", created.GetName())
	}
	if created.GetEtag() == "" || created.GetCreateTime() == nil || created.GetUpdateTime() == nil {
		t.Fatal("CreateContact() did not populate server-managed fields")
	}

	got, err := client.GetContact(ctx, &contactsv1.GetContactRequest{Name: created.GetName()})
	if err != nil {
		t.Fatalf("GetContact() error = %v", err)
	}
	updated, err := client.UpdateContact(ctx, &contactsv1.UpdateContactRequest{
		Contact: &contactsv1.Contact{
			Name:        got.GetName(),
			DisplayName: "Power Company",
			Etag:        got.GetEtag(),
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
	})
	if err != nil {
		t.Fatalf("UpdateContact() error = %v", err)
	}
	if updated.GetDisplayName() != "Power Company" || updated.GetEtag() == created.GetEtag() {
		t.Fatal("UpdateContact() did not update the requested field and etag")
	}

	listed, err := client.ListContacts(ctx, &contactsv1.ListContactsRequest{Parent: "users/alice"})
	if err != nil {
		t.Fatalf("ListContacts() error = %v", err)
	}
	if len(listed.GetContacts()) != 1 || listed.GetContacts()[0].GetName() != created.GetName() {
		t.Fatalf("ListContacts() contacts = %v", listed.GetContacts())
	}

	if _, err := client.DeleteContact(
		ctx,
		&contactsv1.DeleteContactRequest{Name: created.GetName(), Etag: updated.GetEtag()},
	); err != nil {
		t.Fatalf("DeleteContact() error = %v", err)
	}
	_, err = client.GetContact(ctx, &contactsv1.GetContactRequest{Name: created.GetName()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetContact() after delete code = %v, want %v", status.Code(err), codes.NotFound)
	}
}

func TestCreateContactRejectsInvalidDestination(t *testing.T) {
	client := newTestClient(t)
	_, err := client.CreateContact(context.Background(), &contactsv1.CreateContactRequest{
		Parent:    "users/alice",
		ContactId: "invalid",
		Contact:   &contactsv1.Contact{DisplayName: "Invalid"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateContact() code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestUpdateContactRejectsStaleEtag(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	created, err := client.CreateContact(ctx, &contactsv1.CreateContactRequest{
		Parent:    "users/alice",
		ContactId: "bob",
		Contact: &contactsv1.Contact{
			DisplayName: "Bob",
			Destination: &contactsv1.Contact_InternalAccount{InternalAccount: "accounts/bob"},
		},
	})
	if err != nil {
		t.Fatalf("CreateContact() error = %v", err)
	}
	created.Etag = "stale"
	_, err = client.UpdateContact(ctx, &contactsv1.UpdateContactRequest{
		Contact:    created,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
	})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("UpdateContact() code = %v, want %v", status.Code(err), codes.Aborted)
	}
}

func TestContactsRejectAnotherUsersResources(t *testing.T) {
	client := newTestClient(t)
	validContact := &contactsv1.Contact{
		Name:        "users/bob/contacts/friend",
		DisplayName: "Friend",
		Destination: &contactsv1.Contact_InternalAccount{InternalAccount: "accounts/friend"},
	}
	tests := map[string]func() error{
		"list": func() error {
			_, err := client.ListContacts(
				context.Background(),
				&contactsv1.ListContactsRequest{Parent: "users/bob"},
			)
			return err
		},
		"get": func() error {
			_, err := client.GetContact(
				context.Background(),
				&contactsv1.GetContactRequest{Name: validContact.GetName()},
			)
			return err
		},
		"create": func() error {
			_, err := client.CreateContact(context.Background(), &contactsv1.CreateContactRequest{
				Parent: "users/bob", ContactId: "friend", Contact: validContact,
			})
			return err
		},
		"update": func() error {
			_, err := client.UpdateContact(context.Background(), &contactsv1.UpdateContactRequest{
				Contact:    validContact,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
			})
			return err
		},
		"delete": func() error {
			_, err := client.DeleteContact(
				context.Background(),
				&contactsv1.DeleteContactRequest{Name: validContact.GetName()},
			)
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			if code := status.Code(call()); code != codes.PermissionDenied {
				t.Fatalf("code = %v, want %v", code, codes.PermissionDenied)
			}
		})
	}
}

func TestContactsRequireAuthenticatedIdentity(t *testing.T) {
	service := contactservice.New(
		contactrepo.NewMemory(),
		userResolverStub{userName: "users/alice"},
		&accountValidatorStub{},
	)
	_, err := service.ListContacts(
		context.Background(),
		&contactsv1.ListContactsRequest{Parent: "users/alice"},
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestContactsRejectIdentityWithoutActiveProfile(t *testing.T) {
	service := contactservice.New(
		contactrepo.NewMemory(),
		userResolverStub{err: contactrepo.ErrUserNotFound},
		&accountValidatorStub{},
	)
	ctx := keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "unknown-subject"})
	_, err := service.ListContacts(ctx, &contactsv1.ListContactsRequest{Parent: "users/alice"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func newTestClient(t *testing.T) contactsv1.ContactsServiceClient {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		return handler(keycloak.WithClaims(ctx, &keycloak.Claims{Subject: "alice-subject"}), req)
	}))
	contactsv1.RegisterContactsServiceServer(server, contactservice.New(
		contactrepo.NewMemory(),
		userResolverStub{userName: "users/alice"},
		&accountValidatorStub{},
	))
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Logf("test gRPC server stopped: %v", err)
		}
	}()

	connection, err := grpc.NewClient(
		"passthrough:///contacts-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() {
		_ = connection.Close()
		server.Stop()
		_ = listener.Close()
	})
	return contactsv1.NewContactsServiceClient(connection)
}

func TestCreateContactValidatesInternalAccount(t *testing.T) {
	validator := &accountValidatorStub{err: status.Error(codes.NotFound, "missing")}
	service := contactservice.New(
		contactrepo.NewMemory(),
		userResolverStub{userName: "users/alice"},
		validator,
	)
	ctx := keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "alice-subject"})
	_, err := service.CreateContact(ctx, &contactsv1.CreateContactRequest{
		Parent:    "users/alice",
		ContactId: "missing-account",
		Contact: &contactsv1.Contact{
			DisplayName: "Missing",
			Destination: &contactsv1.Contact_InternalAccount{InternalAccount: "accounts/missing"},
		},
	})
	if status.Code(err) != codes.InvalidArgument || len(validator.calls) != 1 ||
		validator.calls[0] != "accounts/missing" {
		t.Fatalf("code = %v, calls = %v, error = %v", status.Code(err), validator.calls, err)
	}
}

func TestCreateExternalContactDoesNotCallAccounts(t *testing.T) {
	validator := &accountValidatorStub{err: status.Error(codes.Unavailable, "must not be called")}
	service := contactservice.New(
		contactrepo.NewMemory(),
		userResolverStub{userName: "users/alice"},
		validator,
	)
	ctx := keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "alice-subject"})
	_, err := service.CreateContact(ctx, &contactsv1.CreateContactRequest{
		Parent:    "users/alice",
		ContactId: "external",
		Contact: &contactsv1.Contact{
			DisplayName: "External",
			Destination: &contactsv1.Contact_ExternalAccount{
				ExternalAccount: &contactsv1.ExternalBankAccount{
					RoutingNumber: "123",
					AccountNumber: "456",
				},
			},
		},
	})
	if err != nil || len(validator.calls) != 0 {
		t.Fatalf("error = %v, calls = %v", err, validator.calls)
	}
}
