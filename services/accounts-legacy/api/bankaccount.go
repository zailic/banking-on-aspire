// Package api contains the preserved legacy Dapr actor implementation.
package api

import (
	"context"

	"dev.local/banking-on-aspire/services/accounts-legacy/domain"
)

const BankAccountActorType = "BankAccount"

type BankAccountClientStub struct {
	id           string
	GetBalance   func(ctx context.Context) (*domain.Money, error)
	Deposit      func(ctx context.Context, amount domain.Money) (*domain.Money, error)
	Withdraw     func(ctx context.Context, amount domain.Money) (*domain.Money, error)
	CloseAccount func(ctx context.Context) error
}

func (b *BankAccountClientStub) Type() string {
	return BankAccountActorType
}

func (b *BankAccountClientStub) ID() string {
	return b.id
}

func NewBankAccountClientStub(id string) *BankAccountClientStub {
	return &BankAccountClientStub{
		id: id,
	}
}
