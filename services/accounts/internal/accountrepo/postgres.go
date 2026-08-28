package accountrepo

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	accountsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/accounts/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed migrations/001_create_accounts.sql
var accountsSchema string

//go:embed migrations/002_add_account_number.sql
var accountNumberSchema string

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
	_, err := r.pool.Exec(ctx, accountsSchema+"\n"+accountNumberSchema)
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
