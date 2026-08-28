package bank

import (
	"context"

	"dev.local/banking-on-aspire/services/accounts/domain"
)

type AccountStatus string

const (
	AccountStatusOpen   AccountStatus = "open"
	AccountStatusClosed AccountStatus = "closed"
)

type Depositor interface {
	Deposit(ctx context.Context, amount domain.Money) (*domain.Money, error)
}

type Withdrawer interface {
	Withdraw(ctx context.Context, amount domain.Money) (*domain.Money, error)
}

type Closer interface {
	Close(ctx context.Context) error
}

type BalanceReader interface {
	Balance(ctx context.Context) (*domain.Money, error)
}

type Account struct {
	Balance domain.Money  `json:"balance"`
	Status  AccountStatus `json:"status"`
}
