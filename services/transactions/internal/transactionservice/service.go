package transactionservice

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"dev.local/banking-on-aspire/platform/auth/keycloak"
	transactionsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/transactions/v1"
	"dev.local/banking-on-aspire/services/transactions/internal/transactionrepo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultPageSize = 50

type Service struct {
	transactionsv1.UnimplementedTransactionsServiceServer
	repository transactionrepo.Repository
}

func New(repository transactionrepo.Repository) *Service {
	if repository == nil {
		panic("transactions repository is required")
	}
	return &Service{repository: repository}
}

func (s *Service) ListTransactions(
	ctx context.Context,
	req *transactionsv1.ListTransactionsRequest,
) (*transactionsv1.ListTransactionsResponse, error) {
	if req == nil || !validUserName(req.GetParent()) {
		return nil, status.Error(codes.InvalidArgument, "parent must have the form users/{user}")
	}
	if req.GetPageSize() < 0 {
		return nil, status.Error(codes.InvalidArgument, "page_size must not be negative")
	}
	claims, ok := keycloak.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return nil, status.Error(codes.Unauthenticated, "authenticated identity is required")
	}
	owner, err := s.repository.ResolveUserName(ctx, claims.Subject)
	if errors.Is(err, transactionrepo.ErrUserNotFound) {
		return nil, status.Error(
			codes.PermissionDenied,
			"the authenticated identity has no active user profile",
		)
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "transaction owner resolution failed")
	}
	if owner != req.GetParent() {
		return nil, status.Error(
			codes.PermissionDenied,
			"the requested transactions are owned by another identity",
		)
	}
	offset, err := decodePageToken(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "page_token is invalid")
	}
	pageSize := int(req.GetPageSize())
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	transactions, err := s.repository.List(ctx, owner, pageSize+1, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "transaction persistence failed")
	}
	response := &transactionsv1.ListTransactionsResponse{Transactions: transactions}
	if len(transactions) > pageSize {
		response.Transactions = transactions[:pageSize]
		response.NextPageToken = encodePageToken(offset + pageSize)
	}
	return response, nil
}

func validUserName(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && parts[0] == "users" && strings.TrimSpace(parts[1]) != ""
}

func encodePageToken(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, errors.New("invalid offset")
	}
	return offset, nil
}
