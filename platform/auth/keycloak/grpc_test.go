package keycloak

import (
	"context"
	"errors"
	"reflect"
	"testing"

	_ "github.com/zailic/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	_ "github.com/zailic/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	_ "github.com/zailic/banking-on-aspire/platform/gen/go/banking/transactions/v1"
	_ "github.com/zailic/banking-on-aspire/platform/gen/go/banking/users/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type verifierStub struct {
	claims *Claims
	err    error
}

func (v verifierStub) Verify(context.Context, string) (*Claims, error) { return v.claims, v.err }

func TestUnaryServerInterceptorAddsClaims(t *testing.T) {
	interceptor := UnaryServerInterceptor(verifierStub{claims: &Claims{
		Subject: "subject",
		ResourceAccess: map[string]ResourceAccess{
			"banking-on-aspire-app": {Roles: []string{"users.profile.read"}},
		},
	}}, "banking-on-aspire-app")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/banking.users.v1.UsersService/GetUser"},
		func(ctx context.Context, _ any) (any, error) {
			claims, ok := ClaimsFromContext(ctx)
			if !ok || claims.Subject != "subject" {
				t.Fatal("claims missing from context")
			}
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

func TestUnaryServerInterceptorRejectsInvalidToken(t *testing.T) {
	interceptor := UnaryServerInterceptor(verifierStub{err: errors.New("invalid")}, "banking-on-aspire-app")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/banking.users.v1.UsersService/GetUser"},
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestUnaryServerInterceptorRejectsMissingRole(t *testing.T) {
	interceptor := UnaryServerInterceptor(verifierStub{claims: &Claims{Subject: "subject"}}, "banking-on-aspire-app")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/banking.users.v1.UsersService/GetUser"},
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestUnaryServerInterceptorRejectsUnknownMethod(t *testing.T) {
	interceptor := UnaryServerInterceptor(verifierStub{}, "banking-on-aspire-app")
	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/banking.auth.v1.AuthorizationPolicy/Reset"},
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want %v", status.Code(err), codes.Internal)
	}
}

func TestUnaryServerInterceptorAllowsHealthCheck(t *testing.T) {
	interceptor := UnaryServerInterceptor(verifierStub{err: errors.New("must not be called")}, "banking-on-aspire-app")
	called := false
	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(context.Context, any) (any, error) { called = true; return nil, nil })
	if err != nil || !called {
		t.Fatalf("health check err = %v, called = %v", err, called)
	}
}

func TestAuthorizationPoliciesMatchRPCContracts(t *testing.T) {
	tests := map[string][]string{
		"/banking.accounts.v1.AccountsService/CreateAccount":            {"accounts.create"},
		"/banking.accounts.v1.AccountsService/ListAccounts":             {"accounts.balance.read"},
		"/banking.accounts.v1.AccountsService/GetAccount":               {"accounts.balance.read"},
		"/banking.accounts.v1.AccountsService/CloseAccount":             {"accounts.close"},
		"/banking.accounts.v1.AccountsService/SendPayment":              {"payments.send"},
		"/banking.accounts.v1.AccountsService/DepositFunds":             {"accounts.deposit"},
		"/banking.users.v1.UsersService/GetUser":                        {"users.profile.read"},
		"/banking.users.v1.UsersService/GetOrCreateCurrentUser":         {"users.profile.write", "users.profile.read"},
		"/banking.contacts.v1.ContactsService/ListContacts":             {"contacts.read"},
		"/banking.contacts.v1.ContactsService/GetContact":               {"contacts.read"},
		"/banking.contacts.v1.ContactsService/CreateContact":            {"contacts.write"},
		"/banking.contacts.v1.ContactsService/UpdateContact":            {"contacts.write"},
		"/banking.contacts.v1.ContactsService/DeleteContact":            {"contacts.write"},
		"/banking.transactions.v1.TransactionsService/ListTransactions": {"transactions.read"},
	}
	for method, wantRoles := range tests {
		policy, err := authorizationPolicy(method)
		if err != nil {
			t.Fatalf("authorizationPolicy(%q) error = %v", method, err)
		}
		if policy == nil || !reflect.DeepEqual(policy.GetRequiredPermissions(), wantRoles) {
			t.Errorf("authorizationPolicy(%q) permissions = %v, want %v", method, policy.GetRequiredPermissions(), wantRoles)
		}
	}
}

func TestHasRequiredPermissionsRequiresEveryPermission(t *testing.T) {
	claims := &Claims{ResourceAccess: map[string]ResourceAccess{
		"banking-on-aspire-app": {Roles: []string{"users.profile.read", "contacts.read"}},
	}}
	if !hasRequiredPermissions(claims, "banking-on-aspire-app", []string{"users.profile.read", "contacts.read"}) {
		t.Fatal("expected access when every required permission is present")
	}
	if hasRequiredPermissions(claims, "banking-on-aspire-app", []string{"users.profile.read", "contacts.write"}) {
		t.Fatal("expected denial when one required permission is missing")
	}
}

func TestHealthRPCDoesNotDeclareApplicationPolicy(t *testing.T) {
	policy, err := authorizationPolicy("/grpc.health.v1.Health/Check")
	if err != nil {
		t.Fatalf("authorizationPolicy() error = %v", err)
	}
	if policy != nil {
		t.Fatalf("health policy = %v, want nil", policy)
	}
}
