// Package bank contains the preserved legacy Dapr actor implementation.
package bank

import (
	"context"

	"github.com/zailic/banking-on-aspire/services/accounts-legacy/domain"
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
