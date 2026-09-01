package transactionservice

import (
	"context"
	"testing"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	eventsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/events/v1"
	transactionsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/transactions/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type repositoryStub struct {
	owner string
	items []*transactionsv1.Transaction
}

func (r *repositoryStub) ApplyPaymentSent(context.Context, *eventsv1.PaymentSentEvent) error {
	return nil
}
func (r *repositoryStub) ApplyFundsDeposited(context.Context, *eventsv1.FundsDepositedEvent) error {
	return nil
}
func (r *repositoryStub) ResolveUserName(context.Context, string) (string, error) {
	return r.owner, nil
}
func (r *repositoryStub) List(_ context.Context, _ string, limit, offset int) ([]*transactionsv1.Transaction, error) {
	end := min(offset+limit, len(r.items))
	if offset > len(r.items) {
		return nil, nil
	}
	return r.items[offset:end], nil
}

func authenticatedContext() context.Context {
	return keycloak.WithClaims(context.Background(), &keycloak.Claims{Subject: "subject-1"})
}

func TestListTransactionsPaginates(t *testing.T) {
	repo := &repositoryStub{owner: "users/user-1", items: []*transactionsv1.Transaction{{Name: "one"}, {Name: "two"}, {Name: "three"}}}
	first, err := New(repo).ListTransactions(authenticatedContext(), &transactionsv1.ListTransactionsRequest{Parent: repo.owner, PageSize: 2})
	if err != nil || len(first.GetTransactions()) != 2 || first.GetNextPageToken() == "" {
		t.Fatalf("first page = %#v, error = %v", first, err)
	}
	second, err := New(repo).ListTransactions(authenticatedContext(), &transactionsv1.ListTransactionsRequest{Parent: repo.owner, PageSize: 2, PageToken: first.GetNextPageToken()})
	if err != nil || len(second.GetTransactions()) != 1 || second.GetNextPageToken() != "" {
		t.Fatalf("second page = %#v, error = %v", second, err)
	}
}

func TestListTransactionsRejectsAnotherOwner(t *testing.T) {
	_, err := New(&repositoryStub{owner: "users/user-1"}).ListTransactions(authenticatedContext(), &transactionsv1.ListTransactionsRequest{Parent: "users/user-2"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}
