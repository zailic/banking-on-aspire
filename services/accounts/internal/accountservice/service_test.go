package accountservice_test

import (
	"context"
	"testing"

	"github.com/zailic/banking-on-aspire/platform/auth/keycloak"
	accountsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"github.com/zailic/banking-on-aspire/services/accounts/internal/accountrepo"
	"github.com/zailic/banking-on-aspire/services/accounts/internal/accountservice"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userResolver struct {
	name string
	err  error
}

func (r userResolver) ResolveUserName(
	context.Context,
	string,
) (string, error) {
	return r.name, r.err
}
func (r userResolver) ValidateUserName(context.Context, string) error { return r.err }
func authenticated() context.Context {
	return keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "alice-sub"})
}

func TestCreateAccountInitializesServerManagedFields(t *testing.T) {
	repo := accountrepo.NewMemory()
	service := accountservice.New(repo, userResolver{})
	created, err := service.CreateAccount(context.Background(), &accountsv1.CreateAccountRequest{
		Parent:    "users/alice",
		AccountId: "savings-02",
		Account: &accountsv1.Account{
			DisplayName:  "  Rainy day  ",
			Type:         accountsv1.AccountType_ACCOUNT_TYPE_SAVINGS,
			CurrencyCode: "eur",
			Status:       accountsv1.AccountStatus_ACCOUNT_STATUS_CLOSED,
		},
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.GetName() != "accounts/savings-02" || created.GetOwner() != "users/alice" ||
		created.GetDisplayName() != "Rainy day" {
		t.Fatalf("CreateAccount() identity = %v", created)
	}
	if created.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN ||
		created.GetCurrencyCode() != "EUR" ||
		created.GetAvailableBalance().GetCurrencyCode() != "EUR" ||
		created.GetAvailableBalance().GetUnits() != 0 {
		t.Fatalf("CreateAccount() defaults = %v", created)
	}
	if len(created.GetAccountNumber()) != 12 || created.GetEtag() == "" ||
		created.GetCreateTime() == nil ||
		created.GetUpdateTime() == nil {
		t.Fatalf("CreateAccount() managed fields = %v", created)
	}
	stored, err := repo.Get(context.Background(), created.GetName())
	if err != nil || stored.GetAccountNumber() != created.GetAccountNumber() {
		t.Fatalf("stored account = %v, %v", stored, err)
	}
}

func TestCreateAccountRejectsUnknownOwnerAndInvalidInput(t *testing.T) {
	service := accountservice.New(
		accountrepo.NewMemory(),
		userResolver{err: accountrepo.ErrUserNotFound},
	)
	valid := &accountsv1.Account{
		DisplayName:  "Everyday",
		Type:         accountsv1.AccountType_ACCOUNT_TYPE_CHECKING,
		CurrencyCode: "RON",
	}
	_, err := service.CreateAccount(
		context.Background(),
		&accountsv1.CreateAccountRequest{Parent: "users/missing", Account: valid},
	)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown owner code = %v", status.Code(err))
	}
	_, err = service.CreateAccount(
		context.Background(),
		&accountsv1.CreateAccountRequest{
			Parent:  "users/alice",
			Account: &accountsv1.Account{DisplayName: "Everyday", CurrencyCode: "RON"},
		},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid type code = %v", status.Code(err))
	}
}
func account() *accountsv1.Account {
	return &accountsv1.Account{
		Name:             "accounts/checking-01",
		Owner:            "users/alice",
		DisplayName:      "Everyday account",
		Type:             accountsv1.AccountType_ACCOUNT_TYPE_CHECKING,
		Status:           accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN,
		AvailableBalance: &money.Money{CurrencyCode: "USD", Units: 1250},
		Etag:             "v1",
		AccountNumber:    "1000000001",
		CurrencyCode:     "USD",
	}
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
	if err != nil || got.GetAvailableBalance().GetUnits() != 1250 ||
		got.GetAccountNumber() != "1000000001" {
		t.Fatalf("GetAccount()=%v,%v", got, err)
	}
	closed, err := service.CloseAccount(
		ctx,
		&accountsv1.CloseAccountRequest{Name: got.GetName(), Etag: got.GetEtag()},
	)
	if err != nil || closed.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_CLOSED ||
		closed.GetEtag() == "v1" {
		t.Fatalf("CloseAccount()=%v,%v", closed, err)
	}
}

func TestAccountsRejectCrossOwnerAndMissingIdentity(t *testing.T) {
	service := accountservice.New(accountrepo.NewMemory(account()), userResolver{name: "users/bob"})
	_, err := service.GetAccount(
		authenticated(),
		&accountsv1.GetAccountRequest{Name: "accounts/checking-01"},
	)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-owner code=%v", status.Code(err))
	}
	_, err = service.ListAccounts(
		context.Background(),
		&accountsv1.ListAccountsRequest{Parent: "users/bob"},
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing identity code=%v", status.Code(err))
	}
}

func TestCloseRejectsStaleEtag(t *testing.T) {
	service := accountservice.New(
		accountrepo.NewMemory(account()),
		userResolver{name: "users/alice"},
	)
	_, err := service.CloseAccount(
		authenticated(),
		&accountsv1.CloseAccountRequest{Name: "accounts/checking-01", Etag: "stale"},
	)
	if status.Code(err) != codes.Aborted {
		t.Fatalf("code=%v", status.Code(err))
	}
}

func TestSendPaymentDebitsOnceAndIsIdempotent(t *testing.T) {
	repo := accountrepo.NewMemory(account())
	repo.AddBeneficiary("users/alice/contacts/landlord", "users/alice")
	service := accountservice.New(repo, userResolver{name: "users/alice"})
	request := &accountsv1.SendPaymentRequest{
		Parent: "accounts/checking-01", Beneficiary: "users/alice/contacts/landlord",
		Amount:    &money.Money{CurrencyCode: "usd", Units: 250, Nanos: 500_000_000},
		Reference: "  August rent  ", RequestId: "payment-request-01",
	}
	first, err := service.SendPayment(authenticated(), request)
	if err != nil {
		t.Fatalf("SendPayment() error = %v", err)
	}
	second, err := service.SendPayment(authenticated(), request)
	if err != nil {
		t.Fatalf("idempotent SendPayment() error = %v", err)
	}
	if first.GetName() != second.GetName() || first.GetReference() != "August rent" ||
		first.GetStatus() != accountsv1.PaymentStatus_PAYMENT_STATUS_COMPLETED {
		t.Fatalf("payments = %v, %v", first, second)
	}
	updated, err := repo.Get(context.Background(), "accounts/checking-01")
	if err != nil || updated.GetAvailableBalance().GetUnits() != 999 ||
		updated.GetAvailableBalance().GetNanos() != 500_000_000 {
		t.Fatalf("updated account = %v, %v", updated, err)
	}
}

func TestSendPaymentRejectsInvalidDestinationFundsAndCurrency(t *testing.T) {
	repo := accountrepo.NewMemory(account())
	repo.AddBeneficiary("users/alice/contacts/landlord", "users/alice")
	service := accountservice.New(repo, userResolver{name: "users/alice"})

	tests := []struct {
		name        string
		beneficiary string
		amount      *money.Money
		code        codes.Code
	}{
		{
			name:        "unknown beneficiary",
			beneficiary: "users/alice/contacts/missing",
			amount:      &money.Money{CurrencyCode: "USD", Units: 1},
			code:        codes.NotFound,
		},
		{
			name:        "currency mismatch",
			beneficiary: "users/alice/contacts/landlord",
			amount:      &money.Money{CurrencyCode: "EUR", Units: 1},
			code:        codes.InvalidArgument,
		},
		{
			name:        "insufficient funds",
			beneficiary: "users/alice/contacts/landlord",
			amount:      &money.Money{CurrencyCode: "USD", Units: 1251},
			code:        codes.FailedPrecondition,
		},
		{
			name:        "zero amount",
			beneficiary: "users/alice/contacts/landlord",
			amount:      &money.Money{CurrencyCode: "USD"},
			code:        codes.InvalidArgument,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SendPayment(authenticated(), &accountsv1.SendPaymentRequest{
				Parent: "accounts/checking-01", Beneficiary: test.beneficiary,
				Amount: test.amount, RequestId: "request-" + test.name,
			})
			if status.Code(err) != test.code {
				t.Fatalf("code = %v, want %v; error = %v", status.Code(err), test.code, err)
			}
		})
	}
}

func TestSendPaymentRejectsIdempotencyKeyReuseWithDifferentData(t *testing.T) {
	repo := accountrepo.NewMemory(account())
	repo.AddBeneficiary("users/alice/contacts/landlord", "users/alice")
	service := accountservice.New(repo, userResolver{name: "users/alice"})
	request := &accountsv1.SendPaymentRequest{
		Parent:      "accounts/checking-01",
		Beneficiary: "users/alice/contacts/landlord",
		Amount:      &money.Money{CurrencyCode: "USD", Units: 10},
		RequestId:   "same-request",
	}
	if _, err := service.SendPayment(authenticated(), request); err != nil {
		t.Fatalf("first SendPayment() error = %v", err)
	}
	request.Amount.Units = 11
	if _, err := service.SendPayment(
		authenticated(),
		request,
	); status.Code(
		err,
	) != codes.AlreadyExists {
		t.Fatalf("reused request code = %v, error = %v", status.Code(err), err)
	}
}

func TestSendPaymentCreditsInternalBeneficiaryAtomically(t *testing.T) {
	destination := &accountsv1.Account{
		Name:             "accounts/savings-02",
		Owner:            "users/bob",
		DisplayName:      "Bob savings",
		Type:             accountsv1.AccountType_ACCOUNT_TYPE_SAVINGS,
		Status:           accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN,
		AvailableBalance: &money.Money{CurrencyCode: "USD", Units: 20},
		CurrencyCode:     "USD",
		Etag:             "destination-v1",
		AccountNumber:    "2000000002",
	}
	repo := accountrepo.NewMemory(account(), destination)
	repo.AddBeneficiary("users/alice/contacts/bob", "users/alice", "accounts/savings-02")
	service := accountservice.New(repo, userResolver{name: "users/alice"})
	_, err := service.SendPayment(authenticated(), &accountsv1.SendPaymentRequest{
		Parent: "accounts/checking-01", Beneficiary: "users/alice/contacts/bob",
		Amount: &money.Money{CurrencyCode: "USD", Units: 30}, RequestId: "internal-payment-01",
	})
	if err != nil {
		t.Fatalf("SendPayment() error = %v", err)
	}
	credited, err := repo.Get(context.Background(), "accounts/savings-02")
	if err != nil || credited.GetAvailableBalance().GetUnits() != 50 {
		t.Fatalf("credited account = %v, %v", credited, err)
	}
}

func TestDepositFundsCreditsOnceAndIsIdempotent(t *testing.T) {
	repo := accountrepo.NewMemory(account())
	service := accountservice.New(repo, userResolver{name: "users/alice"})
	request := &accountsv1.DepositFundsRequest{
		Parent:    "accounts/checking-01",
		Amount:    &money.Money{CurrencyCode: "usd", Units: 50, Nanos: 250_000_000},
		Reference: "  Demo cash-in  ",
		RequestId: "deposit-request-01",
	}
	first, err := service.DepositFunds(authenticated(), request)
	if err != nil {
		t.Fatalf("DepositFunds() error = %v", err)
	}
	second, err := service.DepositFunds(authenticated(), request)
	if err != nil {
		t.Fatalf("idempotent DepositFunds() error = %v", err)
	}
	if first.GetName() != second.GetName() || first.GetReference() != "Demo cash-in" {
		t.Fatalf("deposits = %v, %v", first, second)
	}
	updated, err := repo.Get(context.Background(), "accounts/checking-01")
	if err != nil || updated.GetAvailableBalance().GetUnits() != 1300 ||
		updated.GetAvailableBalance().GetNanos() != 250_000_000 {
		t.Fatalf("updated account = %v, %v", updated, err)
	}
}

func TestDepositFundsRejectsCrossOwnerAndCurrencyMismatch(t *testing.T) {
	repo := accountrepo.NewMemory(account())
	request := &accountsv1.DepositFundsRequest{
		Parent:    "accounts/checking-01",
		Amount:    &money.Money{CurrencyCode: "USD", Units: 10},
		RequestId: "deposit-1",
	}
	_, err := accountservice.New(repo, userResolver{name: "users/bob"}).
		DepositFunds(authenticated(), request)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-owner code = %v", status.Code(err))
	}
	request.Amount.CurrencyCode = "EUR"
	_, err = accountservice.New(repo, userResolver{name: "users/alice"}).
		DepositFunds(authenticated(), request)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("currency mismatch code = %v", status.Code(err))
	}
}
