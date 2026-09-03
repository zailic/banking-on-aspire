package accountrepo

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	eventsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/events/v1"
	"dev.local/banking-on-aspire/platform/observability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed migrations/001_create_accounts.sql
var accountsSchema string

//go:embed migrations/002_add_account_number.sql
var accountNumberSchema string

//go:embed migrations/003_create_payments.sql
var paymentsSchema string

//go:embed migrations/004_create_deposits.sql
var depositsSchema string

//go:embed migrations/005_add_outbox_trace_context.sql
var outboxTraceContextSchema string

type PostgresRepository struct{ pool *pgxpool.Pool }

func OpenPostgres(ctx context.Context, connectionString string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("configure accounts database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to accounts database: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}
func (r *PostgresRepository) Close() { r.pool.Close() }
func (r *PostgresRepository) Migrate(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, accountsSchema+"\n"+accountNumberSchema+"\n"+paymentsSchema+"\n"+depositsSchema+"\n"+outboxTraceContextSchema)
	if err != nil {
		return fmt.Errorf("migrate accounts database: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ResolveUserName(ctx context.Context, subject string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT user_id FROM users WHERE keycloak_subject=$1 AND status='active'`, subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve account owner: %w", err)
	}
	return "users/" + id, nil
}

func (r *PostgresRepository) ValidateUserName(ctx context.Context, name string) error {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE user_id=$1 AND status='active')`, strings.TrimPrefix(name, "users/")).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate account owner: %w", err)
	}
	if !exists {
		return ErrUserNotFound
	}
	return nil
}

const accountColumns = `account_id, user_id, display_name, account_type, status, currency_code, balance_units, balance_nanos, created_at, updated_at, etag, account_number`

func (r *PostgresRepository) Create(ctx context.Context, account *accountsv1.Account) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO accounts (account_id, user_id, display_name, account_type, status, currency_code, balance_units, balance_nanos, created_at, updated_at, etag, account_number) VALUES ($1,$2,$3,$4,'open',$5,0,0,$6,$7,$8,$9)`,
		strings.TrimPrefix(account.GetName(), "accounts/"), strings.TrimPrefix(account.GetOwner(), "users/"), account.GetDisplayName(), databaseType(account.GetType()), account.GetCurrencyCode(), account.GetCreateTime().AsTime(), account.GetUpdateTime().AsTime(), account.GetEtag(), account.GetAccountNumber())
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyExists
	}
	return fmt.Errorf("create account: %w", err)
}

func (r *PostgresRepository) List(ctx context.Context, owner string) ([]*accountsv1.Account, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+accountColumns+` FROM accounts WHERE user_id=$1 ORDER BY account_id`, strings.TrimPrefix(owner, "users/"))
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	result := make([]*accountsv1.Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list account rows: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) Get(ctx context.Context, name string) (*accountsv1.Account, error) {
	account, err := scanAccount(r.pool.QueryRow(ctx, `SELECT `+accountColumns+` FROM accounts WHERE account_id=$1`, strings.TrimPrefix(name, "accounts/")))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return account, err
}

func (r *PostgresRepository) CloseAccount(ctx context.Context, name, expectedEtag, nextEtag string) (*accountsv1.Account, error) {
	accountID := strings.TrimPrefix(name, "accounts/")
	account, err := scanAccount(r.pool.QueryRow(ctx, `UPDATE accounts SET status='closed', updated_at=$2, etag=$3 WHERE account_id=$1 AND status<>'closed' AND ($4='' OR etag=$4) RETURNING `+accountColumns, accountID, time.Now().UTC(), nextEtag, expectedEtag))
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("close account: %w", err)
	}
	current, getErr := r.Get(ctx, name)
	if getErr != nil {
		return nil, getErr
	}
	if expectedEtag != "" && current.GetEtag() != expectedEtag {
		return nil, ErrConflict
	}
	return current, nil
}

func (r *PostgresRepository) SendPayment(ctx context.Context, payment *accountsv1.Payment) (*accountsv1.Payment, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin payment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	accountID := strings.TrimPrefix(payment.GetSourceAccount(), "accounts/")
	var ownerID, status, currency string
	var balanceUnits int64
	var balanceNanos int32
	err = tx.QueryRow(ctx, `SELECT user_id, status, currency_code, balance_units, balance_nanos FROM accounts WHERE account_id=$1 FOR UPDATE`, accountID).
		Scan(&ownerID, &status, &currency, &balanceUnits, &balanceNanos)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock payment source: %w", err)
	}

	existing, err := scanPayment(tx.QueryRow(ctx, `SELECT payment_id, source_account_id, beneficiary_user_id, beneficiary_contact_id, currency_code, amount_units, amount_nanos, reference, status, request_id, created_at FROM payments WHERE source_account_id=$1 AND request_id=$2`, accountID, payment.GetRequestId()))
	if err == nil {
		if !samePayment(existing, payment) {
			return nil, ErrIdempotencyConflict
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("read idempotent payment: %w", err)
	}
	if status != "open" {
		return nil, ErrAccountNotOpen
	}

	beneficiaryUserID, beneficiaryContactID, ok := parseBeneficiaryName(payment.GetBeneficiary())
	if !ok || beneficiaryUserID != ownerID {
		return nil, ErrBeneficiaryNotFound
	}
	var destinationType string
	var internalAccount *string
	if err := tx.QueryRow(ctx, `SELECT destination_type, internal_account FROM contacts WHERE user_id=$1 AND contact_id=$2`, beneficiaryUserID, beneficiaryContactID).Scan(&destinationType, &internalAccount); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBeneficiaryNotFound
	} else if err != nil {
		return nil, fmt.Errorf("validate beneficiary: %w", err)
	}

	balance := &money.Money{CurrencyCode: currency, Units: balanceUnits, Nanos: balanceNanos}
	remaining, ok := subtractMoney(balance, payment.GetAmount())
	if !ok {
		return nil, ErrInsufficientFunds
	}
	var destinationID, destinationOwnerID string
	var destinationBalance *money.Money
	if destinationType == "internal" {
		if internalAccount == nil || !validStoredAccountName(*internalAccount) || *internalAccount == payment.GetSourceAccount() {
			return nil, ErrDestinationInvalid
		}
		destinationID = strings.TrimPrefix(*internalAccount, "accounts/")
		var destinationStatus, destinationCurrency string
		var destinationUnits int64
		var destinationNanos int32
		if err := tx.QueryRow(ctx, `SELECT user_id, status, currency_code, balance_units, balance_nanos FROM accounts WHERE account_id=$1 FOR UPDATE`, destinationID).Scan(&destinationOwnerID, &destinationStatus, &destinationCurrency, &destinationUnits, &destinationNanos); errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDestinationInvalid
		} else if err != nil {
			return nil, fmt.Errorf("lock internal payment destination: %w", err)
		}
		if destinationStatus != "open" || destinationCurrency != payment.GetAmount().GetCurrencyCode() {
			return nil, ErrDestinationInvalid
		}
		destinationBalance = addMoney(&money.Money{CurrencyCode: destinationCurrency, Units: destinationUnits, Nanos: destinationNanos}, payment.GetAmount())
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_units=$2, balance_nanos=$3, updated_at=$4, etag=$5 WHERE account_id=$1`, accountID, remaining.GetUnits(), remaining.GetNanos(), payment.GetCreateTime().AsTime(), payment.GetName()); err != nil {
		return nil, fmt.Errorf("debit payment source: %w", err)
	}
	if destinationBalance != nil {
		if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_units=$2, balance_nanos=$3, updated_at=$4, etag=$5 WHERE account_id=$1`, destinationID, destinationBalance.GetUnits(), destinationBalance.GetNanos(), payment.GetCreateTime().AsTime(), payment.GetName()); err != nil {
			return nil, fmt.Errorf("credit internal payment destination: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO payments (payment_id, source_account_id, beneficiary_user_id, beneficiary_contact_id, currency_code, amount_units, amount_nanos, reference, status, request_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'completed',$9,$10)`,
		strings.TrimPrefix(payment.GetName(), "payments/"), accountID, beneficiaryUserID, beneficiaryContactID, payment.GetAmount().GetCurrencyCode(), payment.GetAmount().GetUnits(), payment.GetAmount().GetNanos(), payment.GetReference(), payment.GetRequestId(), payment.GetCreateTime().AsTime()); err != nil {
		return nil, fmt.Errorf("store payment: %w", err)
	}
	event := &eventsv1.PaymentSentEvent{
		EventId: strings.TrimPrefix(payment.GetName(), "payments/"), SchemaVersion: 1,
		Payment: payment.GetName(), SourceAccount: payment.GetSourceAccount(), SourceOwner: "users/" + ownerID,
		Beneficiary: payment.GetBeneficiary(), Amount: payment.GetAmount(), Reference: payment.GetReference(),
		OccurredTime: payment.GetCreateTime(),
	}
	if destinationID != "" {
		event.DestinationAccount = "accounts/" + destinationID
		event.DestinationOwner = "users/" + destinationOwnerID
	}
	payload, err := protojson.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal payment event: %w", err)
	}
	traceParent, traceState := observability.InjectTraceContext(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO account_outbox (event_id, event_type, aggregate_name, payload, occurred_at, trace_parent, trace_state) VALUES ($1,'banking.events.v1.PaymentSentEvent',$2,$3::jsonb,$4,$5,$6)`,
		event.GetEventId(), payment.GetSourceAccount(), payload, payment.GetCreateTime().AsTime(), traceParent, traceState); err != nil {
		return nil, fmt.Errorf("store payment outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit payment: %w", err)
	}
	return payment, nil
}

func (r *PostgresRepository) DepositFunds(ctx context.Context, deposit *accountsv1.Deposit) (*accountsv1.Deposit, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin deposit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	accountID := strings.TrimPrefix(deposit.GetAccount(), "accounts/")
	var ownerID, status, currency string
	var balanceUnits int64
	var balanceNanos int32
	err = tx.QueryRow(ctx, `SELECT user_id, status, currency_code, balance_units, balance_nanos FROM accounts WHERE account_id=$1 FOR UPDATE`, accountID).
		Scan(&ownerID, &status, &currency, &balanceUnits, &balanceNanos)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock deposit account: %w", err)
	}
	existing, err := scanDeposit(tx.QueryRow(ctx, `SELECT deposit_id, account_id, currency_code, amount_units, amount_nanos, reference, request_id, created_at FROM deposits WHERE account_id=$1 AND request_id=$2`, accountID, deposit.GetRequestId()))
	if err == nil {
		if !sameDeposit(existing, deposit) {
			return nil, ErrIdempotencyConflict
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("read idempotent deposit: %w", err)
	}
	if status != "open" {
		return nil, ErrAccountNotOpen
	}
	if currency != deposit.GetAmount().GetCurrencyCode() {
		return nil, ErrCurrencyMismatch
	}
	updated := addMoney(&money.Money{CurrencyCode: currency, Units: balanceUnits, Nanos: balanceNanos}, deposit.GetAmount())
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_units=$2, balance_nanos=$3, updated_at=$4, etag=$5 WHERE account_id=$1`, accountID, updated.GetUnits(), updated.GetNanos(), deposit.GetCreateTime().AsTime(), deposit.GetName()); err != nil {
		return nil, fmt.Errorf("credit deposit account: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO deposits (deposit_id, account_id, currency_code, amount_units, amount_nanos, reference, request_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		strings.TrimPrefix(deposit.GetName(), "deposits/"), accountID, currency, deposit.GetAmount().GetUnits(), deposit.GetAmount().GetNanos(), deposit.GetReference(), deposit.GetRequestId(), deposit.GetCreateTime().AsTime()); err != nil {
		return nil, fmt.Errorf("store deposit: %w", err)
	}
	event := &eventsv1.FundsDepositedEvent{
		EventId: strings.TrimPrefix(deposit.GetName(), "deposits/"), SchemaVersion: 1,
		Deposit: deposit.GetName(), Account: deposit.GetAccount(), Owner: "users/" + ownerID,
		Amount: deposit.GetAmount(), Reference: deposit.GetReference(), OccurredTime: deposit.GetCreateTime(),
	}
	payload, err := protojson.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal deposit event: %w", err)
	}
	traceParent, traceState := observability.InjectTraceContext(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO account_outbox (event_id, event_type, aggregate_name, payload, occurred_at, trace_parent, trace_state) VALUES ($1,'banking.events.v1.FundsDepositedEvent',$2,$3::jsonb,$4,$5,$6)`,
		event.GetEventId(), deposit.GetAccount(), payload, deposit.GetCreateTime().AsTime(), traceParent, traceState); err != nil {
		return nil, fmt.Errorf("store deposit outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit deposit: %w", err)
	}
	return deposit, nil
}

type OutboxEvent struct {
	ID          string
	Type        string
	Payload     []byte
	TraceParent string
	TraceState  string
}

func (r *PostgresRepository) PendingOutboxEvents(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT event_id, event_type, payload, trace_parent, trace_state FROM account_outbox WHERE published_at IS NULL ORDER BY occurred_at, event_id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending outbox events: %w", err)
	}
	defer rows.Close()
	events := make([]OutboxEvent, 0)
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.ID, &event.Type, &event.Payload, &event.TraceParent, &event.TraceState); err != nil {
			return nil, fmt.Errorf("scan pending outbox event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *PostgresRepository) MarkOutboxEventPublished(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE account_outbox SET published_at=NOW() WHERE event_id=$1 AND published_at IS NULL`, eventID)
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

func scanPayment(scanner rowScanner) (*accountsv1.Payment, error) {
	var paymentID, accountID, userID, contactID, currency, reference, status, requestID string
	var units int64
	var nanos int32
	var created time.Time
	if err := scanner.Scan(&paymentID, &accountID, &userID, &contactID, &currency, &units, &nanos, &reference, &status, &requestID, &created); err != nil {
		return nil, err
	}
	return &accountsv1.Payment{
		Name: "payments/" + paymentID, SourceAccount: "accounts/" + accountID,
		Beneficiary: "users/" + userID + "/contacts/" + contactID,
		Amount:      &money.Money{CurrencyCode: currency, Units: units, Nanos: nanos},
		Reference:   reference, Status: accountsv1.PaymentStatus_PAYMENT_STATUS_COMPLETED,
		RequestId: requestID, CreateTime: timestamppb.New(created),
	}, nil
}

func scanDeposit(scanner rowScanner) (*accountsv1.Deposit, error) {
	var depositID, accountID, currency, reference, requestID string
	var units int64
	var nanos int32
	var created time.Time
	if err := scanner.Scan(&depositID, &accountID, &currency, &units, &nanos, &reference, &requestID, &created); err != nil {
		return nil, err
	}
	return &accountsv1.Deposit{
		Name: "deposits/" + depositID, Account: "accounts/" + accountID,
		Amount:    &money.Money{CurrencyCode: currency, Units: units, Nanos: nanos},
		Reference: reference, RequestId: requestID, CreateTime: timestamppb.New(created),
	}, nil
}

func parseBeneficiaryName(name string) (string, string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) != 4 || parts[0] != "users" || parts[1] == "" || parts[2] != "contacts" || parts[3] == "" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func validStoredAccountName(name string) bool {
	parts := strings.Split(name, "/")
	return len(parts) == 2 && parts[0] == "accounts" && parts[1] != ""
}

type rowScanner interface{ Scan(...any) error }

func scanAccount(scanner rowScanner) (*accountsv1.Account, error) {
	var id, userID, displayName, accountType, status, currency, etag, accountNumber string
	var units int64
	var nanos int32
	var created, updated time.Time
	if err := scanner.Scan(&id, &userID, &displayName, &accountType, &status, &currency, &units, &nanos, &created, &updated, &etag, &accountNumber); err != nil {
		return nil, err
	}
	return &accountsv1.Account{Name: "accounts/" + id, Owner: "users/" + userID, DisplayName: displayName, Type: protoType(accountType), Status: protoStatus(status), AvailableBalance: &money.Money{CurrencyCode: currency, Units: units, Nanos: nanos}, CreateTime: timestamppb.New(created), UpdateTime: timestamppb.New(updated), Etag: etag, AccountNumber: accountNumber, CurrencyCode: currency}, nil
}
func databaseType(value accountsv1.AccountType) string {
	if value == accountsv1.AccountType_ACCOUNT_TYPE_CHECKING {
		return "checking"
	}
	return "savings"
}
func protoType(value string) accountsv1.AccountType {
	if value == "checking" {
		return accountsv1.AccountType_ACCOUNT_TYPE_CHECKING
	}
	if value == "savings" {
		return accountsv1.AccountType_ACCOUNT_TYPE_SAVINGS
	}
	return accountsv1.AccountType_ACCOUNT_TYPE_UNSPECIFIED
}
func protoStatus(value string) accountsv1.AccountStatus {
	switch value {
	case "open":
		return accountsv1.AccountStatus_ACCOUNT_STATUS_OPEN
	case "frozen":
		return accountsv1.AccountStatus_ACCOUNT_STATUS_FROZEN
	case "closed":
		return accountsv1.AccountStatus_ACCOUNT_STATUS_CLOSED
	}
	return accountsv1.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED
}

var _ Repository = (*PostgresRepository)(nil)
var _ UserResolver = (*PostgresRepository)(nil)
