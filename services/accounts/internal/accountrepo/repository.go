package accountrepo

import (
	"context"
	"errors"
	"sync"

	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/proto"
)

var (
	ErrNotFound            = errors.New("account not found")
	ErrConflict            = errors.New("account version conflict")
	ErrUserNotFound        = errors.New("user identity not found")
	ErrAlreadyExists       = errors.New("account already exists")
	ErrBeneficiaryNotFound = errors.New("beneficiary not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrAccountNotOpen      = errors.New("account is not open")
	ErrIdempotencyConflict = errors.New("idempotency key was reused with different payment data")
	ErrDestinationInvalid  = errors.New("internal beneficiary account cannot receive the payment")
	ErrCurrencyMismatch    = errors.New("amount currency does not match the account")
)

type Repository interface {
	Create(context.Context, *accountsv1.Account) error
	List(context.Context, string) ([]*accountsv1.Account, error)
	Get(context.Context, string) (*accountsv1.Account, error)
	CloseAccount(context.Context, string, string, string) (*accountsv1.Account, error)
	SendPayment(context.Context, *accountsv1.Payment) (*accountsv1.Payment, error)
	DepositFunds(context.Context, *accountsv1.Deposit) (*accountsv1.Deposit, error)
}

type UserResolver interface {
	ResolveUserName(context.Context, string) (string, error)
	ValidateUserName(context.Context, string) error
}

func (m *Memory) Create(_ context.Context, account *accountsv1.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.accounts[account.GetName()]; exists {
		return ErrAlreadyExists
	}
	for _, existing := range m.accounts {
		if existing.GetAccountNumber() == account.GetAccountNumber() {
			return ErrAlreadyExists
		}
	}
	m.accounts[account.GetName()] = proto.Clone(account).(*accountsv1.Account)
	return nil
}

type Memory struct {
	mu            sync.RWMutex
	accounts      map[string]*accountsv1.Account
	beneficiaries map[string]memoryBeneficiary
	payments      map[string]*accountsv1.Payment
	deposits      map[string]*accountsv1.Deposit
}

func NewMemory(accounts ...*accountsv1.Account) *Memory {
	result := &Memory{
		accounts:      make(map[string]*accountsv1.Account),
		beneficiaries: make(map[string]memoryBeneficiary),
		payments:      make(map[string]*accountsv1.Payment),
		deposits:      make(map[string]*accountsv1.Deposit),
	}
	for _, account := range accounts {
		result.accounts[account.GetName()] = proto.Clone(account).(*accountsv1.Account)
	}
	return result
}

type memoryBeneficiary struct {
	owner           string
	internalAccount string
}

func (m *Memory) AddBeneficiary(name, owner string, internalAccount ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	beneficiary := memoryBeneficiary{owner: owner}
	if len(internalAccount) > 0 {
		beneficiary.internalAccount = internalAccount[0]
	}
	m.beneficiaries[name] = beneficiary
}

func (m *Memory) List(_ context.Context, owner string) ([]*accountsv1.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*accountsv1.Account, 0)
	for _, account := range m.accounts {
		if account.GetOwner() == owner {
			result = append(result, proto.Clone(account).(*accountsv1.Account))
		}
	}
	return result, nil
}

func (m *Memory) Get(_ context.Context, name string) (*accountsv1.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	account, ok := m.accounts[name]
	if !ok {
		return nil, ErrNotFound
	}
	return proto.Clone(account).(*accountsv1.Account), nil
}

func (m *Memory) CloseAccount(
	_ context.Context,
	name, expectedEtag, nextEtag string,
) (*accountsv1.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	account, ok := m.accounts[name]
	if !ok {
		return nil, ErrNotFound
	}
	if expectedEtag != "" && account.GetEtag() != expectedEtag {
		return nil, ErrConflict
	}
	account.Status = accountsv1.AccountStatus_ACCOUNT_STATUS_CLOSED
	account.Etag = nextEtag
	return proto.Clone(account).(*accountsv1.Account), nil
}

func (m *Memory) SendPayment(
	_ context.Context,
	payment *accountsv1.Payment,
) (*accountsv1.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	account, ok := m.accounts[payment.GetSourceAccount()]
	if !ok {
		return nil, ErrNotFound
	}
	idempotencyKey := payment.GetSourceAccount() + "\x00" + payment.GetRequestId()
	if existing, exists := m.payments[idempotencyKey]; exists {
		if !samePayment(existing, payment) {
			return nil, ErrIdempotencyConflict
		}
		return proto.Clone(existing).(*accountsv1.Payment), nil
	}
	if account.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN {
		return nil, ErrAccountNotOpen
	}
	beneficiary, exists := m.beneficiaries[payment.GetBeneficiary()]
	if !exists || beneficiary.owner != account.GetOwner() {
		return nil, ErrBeneficiaryNotFound
	}
	remaining, ok := subtractMoney(account.GetAvailableBalance(), payment.GetAmount())
	if !ok {
		return nil, ErrInsufficientFunds
	}
	var destination *accountsv1.Account
	if beneficiary.internalAccount != "" {
		if beneficiary.internalAccount == payment.GetSourceAccount() {
			return nil, ErrDestinationInvalid
		}
		destination, exists = m.accounts[beneficiary.internalAccount]
		if !exists || destination.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN ||
			destination.GetCurrencyCode() != payment.GetAmount().GetCurrencyCode() {
			return nil, ErrDestinationInvalid
		}
	}
	account.AvailableBalance = remaining
	account.UpdateTime = payment.GetCreateTime()
	account.Etag = payment.GetName()
	if destination != nil {
		destination.AvailableBalance = addMoney(
			destination.GetAvailableBalance(),
			payment.GetAmount(),
		)
		destination.UpdateTime = payment.GetCreateTime()
		destination.Etag = payment.GetName()
	}
	m.payments[idempotencyKey] = proto.Clone(payment).(*accountsv1.Payment)
	return proto.Clone(payment).(*accountsv1.Payment), nil
}

func (m *Memory) DepositFunds(
	_ context.Context,
	deposit *accountsv1.Deposit,
) (*accountsv1.Deposit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	account, ok := m.accounts[deposit.GetAccount()]
	if !ok {
		return nil, ErrNotFound
	}
	idempotencyKey := deposit.GetAccount() + "\x00" + deposit.GetRequestId()
	if existing, exists := m.deposits[idempotencyKey]; exists {
		if !sameDeposit(existing, deposit) {
			return nil, ErrIdempotencyConflict
		}
		return proto.Clone(existing).(*accountsv1.Deposit), nil
	}
	if account.GetStatus() != accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN {
		return nil, ErrAccountNotOpen
	}
	if account.GetCurrencyCode() != deposit.GetAmount().GetCurrencyCode() {
		return nil, ErrCurrencyMismatch
	}
	account.AvailableBalance = addMoney(account.GetAvailableBalance(), deposit.GetAmount())
	account.UpdateTime = deposit.GetCreateTime()
	account.Etag = deposit.GetName()
	m.deposits[idempotencyKey] = proto.Clone(deposit).(*accountsv1.Deposit)
	return proto.Clone(deposit).(*accountsv1.Deposit), nil
}

func samePayment(left, right *accountsv1.Payment) bool {
	return left.GetBeneficiary() == right.GetBeneficiary() &&
		left.GetReference() == right.GetReference() &&
		proto.Equal(left.GetAmount(), right.GetAmount())
}

func sameDeposit(left, right *accountsv1.Deposit) bool {
	return left.GetReference() == right.GetReference() &&
		proto.Equal(left.GetAmount(), right.GetAmount())
}

func subtractMoney(balance, amount interface {
	GetUnits() int64
	GetNanos() int32
	GetCurrencyCode() string
}) (*money.Money, bool) {
	units := balance.GetUnits() - amount.GetUnits()
	nanos := balance.GetNanos() - amount.GetNanos()
	if nanos < 0 {
		units--
		nanos += 1_000_000_000
	}
	if units < 0 {
		return nil, false
	}
	return &money.Money{CurrencyCode: balance.GetCurrencyCode(), Units: units, Nanos: nanos}, true
}

func addMoney(balance, amount interface {
	GetUnits() int64
	GetNanos() int32
	GetCurrencyCode() string
}) *money.Money {
	units := balance.GetUnits() + amount.GetUnits()
	nanos := balance.GetNanos() + amount.GetNanos()
	if nanos >= 1_000_000_000 {
		units++
		nanos -= 1_000_000_000
	}
	return &money.Money{CurrencyCode: balance.GetCurrencyCode(), Units: units, Nanos: nanos}
}

var _ Repository = (*Memory)(nil)
