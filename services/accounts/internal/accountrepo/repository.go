package accountrepo

import (
	"context"
	"errors"
	"sync"

	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"google.golang.org/protobuf/proto"
)

var (
	ErrNotFound      = errors.New("account not found")
	ErrConflict      = errors.New("account version conflict")
	ErrUserNotFound  = errors.New("user identity not found")
	ErrAlreadyExists = errors.New("account already exists")
)

type Repository interface {
	Create(context.Context, *accountsv1.Account) error
	List(context.Context, string) ([]*accountsv1.Account, error)
	Get(context.Context, string) (*accountsv1.Account, error)
	CloseAccount(context.Context, string, string, string) (*accountsv1.Account, error)
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
	mu       sync.RWMutex
	accounts map[string]*accountsv1.Account
}

func NewMemory(accounts ...*accountsv1.Account) *Memory {
	result := &Memory{accounts: make(map[string]*accountsv1.Account)}
	for _, account := range accounts {
		result.accounts[account.GetName()] = proto.Clone(account).(*accountsv1.Account)
	}
	return result
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

func (m *Memory) CloseAccount(_ context.Context, name, expectedEtag, nextEtag string) (*accountsv1.Account, error) {
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

var _ Repository = (*Memory)(nil)
