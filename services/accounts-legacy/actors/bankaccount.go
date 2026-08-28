// Package actors contains the preserved legacy Dapr actor implementation.
package actors

import (
	"context"

	"dev.local/banking-on-aspire/services/accounts-legacy/domain"
	"dev.local/banking-on-aspire/services/accounts-legacy/domain/bank"
	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
)

const BankAccountActorType = "BankAccount"
const defaultCurrency = "USD"

var _ bank.Withdrawer = (*BankAccountService)(nil)
var _ bank.BalanceReader = (*BankAccountService)(nil)
var _ bank.Depositor = (*BankAccountService)(nil)
var _ bank.Closer = (*BankAccountService)(nil)

type BankAccountService struct {
	actor.ServerImplBaseCtx
	daprClient dapr.Client
}

// Close implements [bank.Closer].
func (b *BankAccountService) Close(ctx context.Context) error {
	account, err := b.getAccount(ctx)
	if err != nil {
		return err
	}

	if account.Status == bank.AccountStatusClosed {
		return nil
	}

	account.Status = bank.AccountStatusClosed
	state := b.GetStateManager()
	err = state.Set(ctx, "account", account)
	if err != nil {
		return err
	}

	return nil
}

// Deposit implements [bank.Depositor].
func (b *BankAccountService) Deposit(ctx context.Context, amount domain.Money) (*domain.Money, error) {
	account, err := b.getAccount(ctx)
	if err != nil {
		return nil, err
	}

	if account.Status == bank.AccountStatusClosed {
		return nil, bank.ErrAccountClosed
	}

	account.Balance = account.Balance.Add(amount)
	state := b.GetStateManager()
	err = state.Set(ctx, "account", account)
	if err != nil {
		return nil, err
	}

	return &account.Balance, nil
}

// Balance implements [bank.BalanceReader].
func (b *BankAccountService) Balance(ctx context.Context) (*domain.Money, error) {
	account, err := b.getAccount(ctx)
	if err != nil {
		return nil, err
	}

	if account.Status == bank.AccountStatusClosed {
		return nil, bank.ErrAccountClosed
	}

	return &account.Balance, nil
}

// GetBalance is an alias kept for client stub compatibility.
func (b *BankAccountService) GetBalance(ctx context.Context) (*domain.Money, error) {
	return b.Balance(ctx)
}

// Withdraw implements [bank.Withdrawer].
func (b *BankAccountService) Withdraw(ctx context.Context, amount domain.Money) (*domain.Money, error) {
	account, err := b.getAccount(ctx)
	if err != nil {
		return nil, err
	}

	if account.Status == bank.AccountStatusClosed {
		return nil, bank.ErrAccountClosed
	}

	balance := account.Balance.Subtract(amount)
	account.Balance = balance
	state := b.GetStateManager()
	err = state.Set(ctx, "account", account)
	if err != nil {
		return nil, err
	}

	return &balance, nil
}

// CloseAccount is an alias kept for client stub compatibility.
func (b *BankAccountService) CloseAccount(ctx context.Context) error {
	return b.Close(ctx)
}

func (b *BankAccountService) getAccount(ctx context.Context) (*bank.Account, error) {
	state := b.GetStateManager()

	exists, err := state.Contains(ctx, "account")
	if err != nil {
		return nil, err
	}

	if !exists {
		account := &bank.Account{
			Balance: domain.Money{Amount: 0, Currency: defaultCurrency},
			Status:  bank.AccountStatusOpen,
		}
		if err := state.Set(ctx, "account", account); err != nil {
			return nil, err
		}
		return account, nil
	}

	var account bank.Account

	err = state.Get(ctx, "account", &account)
	if err != nil {
		return nil, err
	}

	updated := false
	if account.Status == "" {
		account.Status = bank.AccountStatusOpen
		updated = true
	}
	if account.Balance.Currency == "" {
		account.Balance.Currency = defaultCurrency
		updated = true
	}

	if updated {
		if err := state.Set(ctx, "account", &account); err != nil {
			return nil, err
		}
	}

	return &account, nil
}

func (b *BankAccountService) Type() string {
	return BankAccountActorType
}

func BankAccountServiceFactory() actor.ServerContext {
	daprClient, err := dapr.NewClient()
	if err != nil {
		panic(err)
	}

	return &BankAccountService{
		daprClient: daprClient,
	}
}
