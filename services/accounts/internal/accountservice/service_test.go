package accountservice_test

import (
	"context"
	"testing"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"dev.local/banking-on-aspire/services/accounts/internal/accountrepo"
	"dev.local/banking-on-aspire/services/accounts/internal/accountservice"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userResolver struct {
	name string
	err  error
}

func (r userResolver) ResolveUserName(context.Context, string) (string, error) { return r.name, r.err }
func (r userResolver) ValidateUserName(context.Context, string) error          { return r.err }
func authenticated() context.Context {
	return keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "alice-sub"})
}

func TestCreateAccountInitializesServerManagedFields(t *testing.T) {
	repo := accountrepo.NewMemory()
	service := accountservice.New(repo, userResolver{})
	created, err := service.CreateAccount(context.Background(), &accountsv1.CreateAccountRequest{
		Parent:    "users/alice",
		AccountId: "savings-02",
		Account:   &accountsv1.Account{DisplayName: "  Rainy day  ", Type: accountsv1.AccountType_ACCOUNT_TYPE_SAVINGS, CurrencyCode: "eur", Status: accountsv1.AccountStatus_ACCOUNT_STATUS_CLOSED},
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.GetName() != "accounts/savings-02" || created.GetOwner() != "users/alice" || created.GetDisplayName() != "Rainy day" {
		t.Fatalf("CreateAccount() identity = %v", created)
	}
	if created.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN || created.GetCurrencyCode() != "EUR" || created.GetAvailableBalance().GetCurrencyCode() != "EUR" || created.GetAvailableBalance().GetUnits() != 0 {
		t.Fatalf("CreateAccount() defaults = %v", created)
	}
	if len(created.GetAccountNumber()) != 12 || created.GetEtag() == "" || created.GetCreateTime() == nil || created.GetUpdateTime() == nil {
		t.Fatalf("CreateAccount() managed fields = %v", created)
	}
	stored, err := repo.Get(context.Background(), created.GetName())
	if err != nil || stored.GetAccountNumber() != created.GetAccountNumber() {
		t.Fatalf("stored account = %v, %v", stored, err)
	}
}

func TestCreateAccountRejectsUnknownOwnerAndInvalidInput(t *testing.T) {
	service := accountservice.New(accountrepo.NewMemory(), userResolver{err: accountrepo.ErrUserNotFound})
	valid := &accountsv1.Account{DisplayName: "Everyday", Type: accountsv1.AccountType_ACCOUNT_TYPE_CHECKING, CurrencyCode: "RON"}
	_, err := service.CreateAccount(context.Background(), &accountsv1.CreateAccountRequest{Parent: "users/missing", Account: valid})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown owner code = %v", status.Code(err))
	}
	_, err = service.CreateAccount(context.Background(), &accountsv1.CreateAccountRequest{Parent: "users/alice", Account: &accountsv1.Account{DisplayName: "Everyday", CurrencyCode: "RON"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid type code = %v", status.Code(err))
	}
}
func account() *accountsv1.Account {
	return &accountsv1.Account{Name: "accounts/checking-01", Owner: "users/alice", DisplayName: "Everyday account", Type: accountsv1.AccountType_ACCOUNT_TYPE_CHECKING, Status: accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN, AvailableBalance: &money.Money{CurrencyCode: "USD", Units: 1250}, Etag: "v1", AccountNumber: "1000000001"}
}

func TestAccountReadAndCloseLifecycle(t *testing.T) {
	repo := accountrepo.NewMemory(account())
	service := accountservice.New(repo, userResolver{name: "users/alice"})
	ctx := authenticated()
	listed, err := service.ListAccounts(ctx, &accountsv1.ListAccountsRequest{Parent: "users/alice"})
	if err != nil || len(listed.GetAccounts()) != 1 {
		t.Fatalf("ListAccounts()=%v,%v", listed, err)
	}
	got, err := service.GetAccount(ctx, &accountsv1.GetAccountRequest{Name: "accounts/checking-01"})
	if err != nil || got.GetAvailableBalance().GetUnits() != 1250 || got.GetAccountNumber() != "1000000001" {
		t.Fatalf("GetAccount()=%v,%v", got, err)
	}
	closed, err := service.CloseAccount(ctx, &accountsv1.CloseAccountRequest{Name: got.GetName(), Etag: got.GetEtag()})
	if err != nil || closed.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_CLOSED || closed.GetEtag() == "v1" {
		t.Fatalf("CloseAccount()=%v,%v", closed, err)
	}
}

func TestAccountsRejectCrossOwnerAndMissingIdentity(t *testing.T) {
	service := accountservice.New(accountrepo.NewMemory(account()), userResolver{name: "users/bob"})
	_, err := service.GetAccount(authenticated(), &accountsv1.GetAccountRequest{Name: "accounts/checking-01"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-owner code=%v", status.Code(err))
	}
	_, err = service.ListAccounts(context.Background(), &accountsv1.ListAccountsRequest{Parent: "users/bob"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing identity code=%v", status.Code(err))
	}
}

func TestCloseRejectsStaleEtag(t *testing.T) {
	service := accountservice.New(accountrepo.NewMemory(account()), userResolver{name: "users/alice"})
	_, err := service.CloseAccount(authenticated(), &accountsv1.CloseAccountRequest{Name: "accounts/checking-01", Etag: "stale"})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("code=%v", status.Code(err))
	}
}
