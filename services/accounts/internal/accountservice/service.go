package accountservice

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zailic/banking-on-aspire/platform/auth/keycloak"
	accountsv1 "github.com/zailic/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"github.com/zailic/banking-on-aspire/services/accounts/internal/accountrepo"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultPageSize = 50

type Service struct {
	accountsv1.UnimplementedAccountsServiceServer
	repository accountrepo.Repository
	users      accountrepo.UserResolver
}

func (s *Service) CreateAccount(
	ctx context.Context,
	req *accountsv1.CreateAccountRequest,
) (*accountsv1.Account, error) {
	if req == nil || !validUserName(req.GetParent()) {
		return nil, status.Error(codes.InvalidArgument, "parent must have the form users/{user}")
	}
	input := req.GetAccount()
	if input == nil {
		return nil, status.Error(codes.InvalidArgument, "account is required")
	}
	displayName := strings.TrimSpace(input.GetDisplayName())
	if displayName == "" {
		return nil, status.Error(codes.InvalidArgument, "account.display_name is required")
	}
	if input.GetType() != accountsv1.AccountType_ACCOUNT_TYPE_CHECKING &&
		input.GetType() != accountsv1.AccountType_ACCOUNT_TYPE_SAVINGS {
		return nil, status.Error(codes.InvalidArgument, "account.type must be checking or savings")
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(input.GetCurrencyCode()))
	if !validCurrencyCode(currencyCode) {
		return nil, status.Error(
			codes.InvalidArgument,
			"account.currency_code must be a three-letter ISO 4217 code",
		)
	}
	accountID := strings.TrimSpace(req.GetAccountId())
	if accountID == "" {
		accountID = "acc-" + randomHex(10)
	} else if !validID(accountID) {
		return nil, status.Error(
			codes.InvalidArgument,
			"account_id must start with a lowercase letter and contain only lowercase letters, digits or hyphens",
		)
	}
	if err := s.users.ValidateUserName(ctx, req.GetParent()); err != nil {
		if errors.Is(err, accountrepo.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "account owner is not an active user")
		}
		return nil, status.Error(codes.Internal, "account owner validation failed")
	}
	now := time.Now().UTC()
	account := &accountsv1.Account{
		Name:             "accounts/" + accountID,
		Owner:            req.GetParent(),
		DisplayName:      displayName,
		Type:             input.GetType(),
		Status:           accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN,
		AvailableBalance: &money.Money{CurrencyCode: currencyCode},
		CreateTime:       timestamppb.New(now),
		UpdateTime:       timestamppb.New(now),
		Etag:             newEtag(),
		AccountNumber:    newAccountNumber(),
		CurrencyCode:     currencyCode,
	}
	if err := s.repository.Create(ctx, account); err != nil {
		return nil, repositoryError(err)
	}
	return account, nil
}

func New(repository accountrepo.Repository, users accountrepo.UserResolver) *Service {
	if repository == nil || users == nil {
		panic("accounts repository and user resolver are required")
	}
	return &Service{repository: repository, users: users}
}

func (s *Service) ListAccounts(
	ctx context.Context,
	req *accountsv1.ListAccountsRequest,
) (*accountsv1.ListAccountsResponse, error) {
	if req == nil || !validUserName(req.GetParent()) {
		return nil, status.Error(codes.InvalidArgument, "parent must have the form users/{user}")
	}
	if req.GetPageSize() < 0 {
		return nil, status.Error(codes.InvalidArgument, "page_size must not be negative")
	}
	if err := s.authorizeOwner(ctx, req.GetParent()); err != nil {
		return nil, err
	}
	accounts, err := s.repository.List(ctx, req.GetParent())
	if err != nil {
		return nil, repositoryError(err)
	}
	offset, err := decodePageToken(req.GetPageToken())
	if err != nil || offset > len(accounts) {
		return nil, status.Error(codes.InvalidArgument, "page_token is invalid")
	}
	pageSize := int(req.GetPageSize())
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	end := min(offset+pageSize, len(accounts))
	response := &accountsv1.ListAccountsResponse{Accounts: accounts[offset:end]}
	if end < len(accounts) {
		response.NextPageToken = encodePageToken(end)
	}
	return response, nil
}

func (s *Service) GetAccount(
	ctx context.Context,
	req *accountsv1.GetAccountRequest,
) (*accountsv1.Account, error) {
	if req == nil || !validAccountName(req.GetName()) {
		return nil, status.Error(
			codes.InvalidArgument,
			"name must have the form accounts/{account}",
		)
	}
	account, err := s.repository.Get(ctx, req.GetName())
	if err != nil {
		return nil, repositoryError(err)
	}
	if err := s.authorizeOwner(ctx, account.GetOwner()); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Service) CloseAccount(
	ctx context.Context,
	req *accountsv1.CloseAccountRequest,
) (*accountsv1.Account, error) {
	if req == nil || !validAccountName(req.GetName()) {
		return nil, status.Error(
			codes.InvalidArgument,
			"name must have the form accounts/{account}",
		)
	}
	current, err := s.repository.Get(ctx, req.GetName())
	if err != nil {
		return nil, repositoryError(err)
	}
	if err := s.authorizeOwner(ctx, current.GetOwner()); err != nil {
		return nil, err
	}
	closed, err := s.repository.CloseAccount(ctx, req.GetName(), req.GetEtag(), newEtag())
	if err != nil {
		return nil, repositoryError(err)
	}
	return closed, nil
}

func (s *Service) SendPayment(
	ctx context.Context,
	req *accountsv1.SendPaymentRequest,
) (*accountsv1.Payment, error) {
	if req == nil || !validAccountName(req.GetParent()) {
		return nil, status.Error(
			codes.InvalidArgument,
			"parent must have the form accounts/{account}",
		)
	}
	if !validBeneficiaryName(req.GetBeneficiary()) {
		return nil, status.Error(
			codes.InvalidArgument,
			"beneficiary must have the form users/{user}/contacts/{contact}",
		)
	}
	requestID := strings.TrimSpace(req.GetRequestId())
	if requestID == "" || len(requestID) > 128 {
		return nil, status.Error(
			codes.InvalidArgument,
			"request_id is required and must not exceed 128 characters",
		)
	}
	reference := strings.TrimSpace(req.GetReference())
	if utf8.RuneCountInString(reference) > 140 {
		return nil, status.Error(codes.InvalidArgument, "reference must not exceed 140 characters")
	}
	amount := req.GetAmount()
	if !validPositiveMoney(amount) {
		return nil, status.Error(
			codes.InvalidArgument,
			"amount must be positive and use a three-letter ISO 4217 currency code",
		)
	}
	current, err := s.repository.Get(ctx, req.GetParent())
	if err != nil {
		return nil, repositoryError(err)
	}
	if err := s.authorizeOwner(ctx, current.GetOwner()); err != nil {
		return nil, err
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(amount.GetCurrencyCode()))
	if currencyCode != current.GetCurrencyCode() {
		return nil, status.Error(
			codes.InvalidArgument,
			"amount currency must match the source account",
		)
	}
	now := time.Now().UTC()
	payment := &accountsv1.Payment{
		Name:          "payments/pay-" + randomHex(10),
		SourceAccount: req.GetParent(),
		Beneficiary:   req.GetBeneficiary(),
		Amount: &money.Money{
			CurrencyCode: currencyCode,
			Units:        amount.GetUnits(),
			Nanos:        amount.GetNanos(),
		},
		Reference:  reference,
		Status:     accountsv1.PaymentStatus_PAYMENT_STATUS_COMPLETED,
		CreateTime: timestamppb.New(now),
		RequestId:  requestID,
	}
	completed, err := s.repository.SendPayment(ctx, payment)
	if err != nil {
		return nil, repositoryError(err)
	}
	return completed, nil
}

func (s *Service) DepositFunds(
	ctx context.Context,
	req *accountsv1.DepositFundsRequest,
) (*accountsv1.Deposit, error) {
	if req == nil || !validAccountName(req.GetParent()) {
		return nil, status.Error(
			codes.InvalidArgument,
			"parent must have the form accounts/{account}",
		)
	}
	requestID := strings.TrimSpace(req.GetRequestId())
	if requestID == "" || len(requestID) > 128 {
		return nil, status.Error(
			codes.InvalidArgument,
			"request_id is required and must not exceed 128 characters",
		)
	}
	reference := strings.TrimSpace(req.GetReference())
	if utf8.RuneCountInString(reference) > 140 {
		return nil, status.Error(codes.InvalidArgument, "reference must not exceed 140 characters")
	}
	if !validPositiveMoney(req.GetAmount()) {
		return nil, status.Error(
			codes.InvalidArgument,
			"amount must be positive and use a three-letter ISO 4217 currency code",
		)
	}
	current, err := s.repository.Get(ctx, req.GetParent())
	if err != nil {
		return nil, repositoryError(err)
	}
	if err := s.authorizeOwner(ctx, current.GetOwner()); err != nil {
		return nil, err
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(req.GetAmount().GetCurrencyCode()))
	if currencyCode != current.GetCurrencyCode() {
		return nil, status.Error(codes.InvalidArgument, "amount currency must match the account")
	}
	now := time.Now().UTC()
	deposit := &accountsv1.Deposit{
		Name:    "deposits/dep-" + randomHex(10),
		Account: req.GetParent(),
		Amount: &money.Money{
			CurrencyCode: currencyCode,
			Units:        req.GetAmount().GetUnits(),
			Nanos:        req.GetAmount().GetNanos(),
		},
		Reference:  reference,
		CreateTime: timestamppb.New(now),
		RequestId:  requestID,
	}
	completed, err := s.repository.DepositFunds(ctx, deposit)
	if err != nil {
		return nil, repositoryError(err)
	}
	return completed, nil
}

func (s *Service) authorizeOwner(ctx context.Context, owner string) error {
	claims, ok := keycloak.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return status.Error(codes.Unauthenticated, "authenticated identity is required")
	}
	userName, err := s.users.ResolveUserName(ctx, claims.Subject)
	if errors.Is(err, accountrepo.ErrUserNotFound) {
		return status.Error(
			codes.PermissionDenied,
			"the authenticated identity has no active user profile",
		)
	}
	if err != nil {
		return status.Error(codes.Internal, "account owner resolution failed")
	}
	if userName != owner {
		return status.Error(
			codes.PermissionDenied,
			"the requested account is owned by another identity",
		)
	}
	return nil
}

func repositoryError(err error) error {
	switch {
	case errors.Is(err, accountrepo.ErrNotFound):
		return status.Error(codes.NotFound, "account not found")
	case errors.Is(err, accountrepo.ErrConflict):
		return status.Error(codes.Aborted, "etag does not match the current account")
	case errors.Is(err, accountrepo.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "account already exists")
	case errors.Is(err, accountrepo.ErrBeneficiaryNotFound):
		return status.Error(codes.NotFound, "beneficiary not found for the account owner")
	case errors.Is(err, accountrepo.ErrInsufficientFunds):
		return status.Error(codes.FailedPrecondition, "account has insufficient available funds")
	case errors.Is(err, accountrepo.ErrAccountNotOpen):
		return status.Error(codes.FailedPrecondition, "source account is not open")
	case errors.Is(err, accountrepo.ErrIdempotencyConflict):
		return status.Error(
			codes.AlreadyExists,
			"request_id was already used for different operation data",
		)
	case errors.Is(err, accountrepo.ErrDestinationInvalid):
		return status.Error(
			codes.FailedPrecondition,
			"internal beneficiary account cannot receive this payment",
		)
	case errors.Is(err, accountrepo.ErrCurrencyMismatch):
		return status.Error(codes.InvalidArgument, "amount currency must match the account")
	default:
		return status.Error(codes.Internal, "account persistence failed")
	}
}

func validID(value string) bool {
	if len(value) == 0 || len(value) > 63 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func validCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func validUserName(value string) bool {
	p := strings.Split(value, "/")
	return len(p) == 2 && p[0] == "users" && p[1] != ""
}

func validAccountName(value string) bool {
	p := strings.Split(value, "/")
	return len(p) == 2 && p[0] == "accounts" && p[1] != ""
}

func validBeneficiaryName(value string) bool {
	p := strings.Split(value, "/")
	return len(p) == 4 && p[0] == "users" && p[1] != "" && p[2] == "contacts" && p[3] != ""
}

func validPositiveMoney(value *money.Money) bool {
	if value == nil ||
		!validCurrencyCode(strings.ToUpper(strings.TrimSpace(value.GetCurrencyCode()))) {
		return false
	}
	if value.GetUnits() < 0 || value.GetNanos() < 0 || value.GetNanos() >= 1_000_000_000 {
		return false
	}
	return value.GetUnits() > 0 || value.GetNanos() > 0
}

func newEtag() string {
	return randomHex(16)
}

func randomHex(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Errorf("generate account identifier: %w", err))
	}
	return hex.EncodeToString(value)
}

func newAccountNumber() string {
	value := make([]byte, 12)
	for index := range value {
		var candidate [1]byte
		for {
			if _, err := rand.Read(candidate[:]); err != nil {
				panic(fmt.Errorf("generate account number: %w", err))
			}
			if candidate[0] < 250 {
				value[index] = '0' + candidate[0]%10
				break
			}
		}
	}
	return string(value)
}

func encodePageToken(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	value, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(value))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid offset")
	}
	return offset, nil
}
