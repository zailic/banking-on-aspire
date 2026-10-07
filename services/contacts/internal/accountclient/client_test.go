package accountclient

import (
	"context"
	"testing"

	accountsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type accountsClientStub struct {
	accountsv1.AccountsServiceClient
	authorization []string
	name          string
}

func (s *accountsClientStub) GetAccount(
	ctx context.Context,
	request *accountsv1.GetAccountRequest,
	_ ...grpc.CallOption,
) (*accountsv1.Account, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	s.authorization = md.Get("authorization")
	s.name = request.GetName()
	return &accountsv1.Account{Name: request.GetName()}, nil
}

func TestValidateInternalAccountForwardsAuthorization(t *testing.T) {
	stub := &accountsClientStub{}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer caller-token",
		"unrelated", "must-not-be-forwarded",
	))
	if err := New(stub).ValidateInternalAccount(ctx, "accounts/checking-01"); err != nil {
		t.Fatalf("ValidateInternalAccount() error = %v", err)
	}
	if stub.name != "accounts/checking-01" || len(stub.authorization) != 1 ||
		stub.authorization[0] != "Bearer caller-token" {
		t.Fatalf("name = %q, authorization = %v", stub.name, stub.authorization)
	}
}
