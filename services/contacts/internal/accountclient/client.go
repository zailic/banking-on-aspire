package accountclient

import (
	"context"

	accountsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	accounts accountsv1.AccountsServiceClient
}

func New(accounts accountsv1.AccountsServiceClient) *Client {
	if accounts == nil {
		panic("accounts client is required")
	}
	return &Client{accounts: accounts}
}

func (c *Client) ValidateInternalAccount(ctx context.Context, name string) error {
	outgoing := metadata.MD{}
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		if authorization := incoming.Get("authorization"); len(authorization) > 0 {
			outgoing.Set("authorization", authorization...)
		}
	}
	_, err := c.accounts.GetAccount(
		metadata.NewOutgoingContext(ctx, outgoing),
		&accountsv1.GetAccountRequest{Name: name},
	)
	return err
}
