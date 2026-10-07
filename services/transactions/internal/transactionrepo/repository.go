package transactionrepo

import (
	"context"
	"errors"

	eventsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/events/v1"
	transactionsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/transactions/v1"
)

var ErrUserNotFound = errors.New("user identity not found")

type Repository interface {
	ApplyPaymentSent(context.Context, *eventsv1.PaymentSentEvent) error
	ApplyFundsDeposited(context.Context, *eventsv1.FundsDepositedEvent) error
	List(context.Context, string, int, int) ([]*transactionsv1.Transaction, error)
	ResolveUserName(context.Context, string) (string, error)
}
