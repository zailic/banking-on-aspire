package transactionrepo

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	eventsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/events/v1"
	transactionsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/transactions/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed migrations/001_create_transactions.sql
var schema string

type PostgresRepository struct {
	transactions *pgxpool.Pool
	users        *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, transactionsURI, usersURI string) (*PostgresRepository, error) {
	transactions, err := pgxpool.New(ctx, transactionsURI)
	if err != nil {
		return nil, fmt.Errorf("configure transactions database: %w", err)
	}
	if err := transactions.Ping(ctx); err != nil {
		transactions.Close()
		return nil, fmt.Errorf("connect to transactions database: %w", err)
	}
	users, err := pgxpool.New(ctx, usersURI)
	if err != nil {
		transactions.Close()
		return nil, fmt.Errorf("configure users database: %w", err)
	}
	if err := users.Ping(ctx); err != nil {
		transactions.Close()
		users.Close()
		return nil, fmt.Errorf("connect to users database: %w", err)
	}
	return &PostgresRepository{transactions: transactions, users: users}, nil
}

func (r *PostgresRepository) Close() { r.transactions.Close(); r.users.Close() }
func (r *PostgresRepository) Migrate(ctx context.Context) error {
	_, err := r.transactions.Exec(ctx, schema)
	return err
}

func (r *PostgresRepository) ResolveUserName(ctx context.Context, subject string) (string, error) {
	var id string
	err := r.users.QueryRow(ctx, `SELECT user_id FROM users WHERE keycloak_subject=$1 AND status='active'`, subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve transaction owner: %w", err)
	}
	return "users/" + id, nil
}

func (r *PostgresRepository) ApplyPaymentSent(ctx context.Context, event *eventsv1.PaymentSentEvent) error {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var inserted string
	err = tx.QueryRow(ctx, `INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING RETURNING event_id`, event.GetEventId()).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("record processed event: %w", err)
	}
	if err := insertTransaction(ctx, tx, event.GetEventId()+"-debit", event.GetSourceOwner(), event.GetSourceAccount(), event.GetBeneficiary(), event.GetAmount(), event.GetReference(), event.GetEventId(), event.GetOccurredTime().AsTime(), "debit"); err != nil {
		return err
	}
	if event.GetDestinationOwner() != "" {
		if err := insertTransaction(ctx, tx, event.GetEventId()+"-credit", event.GetDestinationOwner(), event.GetDestinationAccount(), event.GetSourceAccount(), event.GetAmount(), event.GetReference(), event.GetEventId(), event.GetOccurredTime().AsTime(), "credit"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ApplyFundsDeposited(ctx context.Context, event *eventsv1.FundsDepositedEvent) error {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var inserted string
	err = tx.QueryRow(ctx, `INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING RETURNING event_id`, event.GetEventId()).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("record processed event: %w", err)
	}
	if err := insertTransaction(ctx, tx, event.GetEventId()+"-credit", event.GetOwner(), event.GetAccount(), "cash-in", event.GetAmount(), event.GetReference(), event.GetEventId(), event.GetOccurredTime().AsTime(), "credit"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertTransaction(ctx context.Context, tx pgx.Tx, id, owner, account, counterparty string, amount *money.Money, reference, eventID string, occurred time.Time, direction string) error {
	_, err := tx.Exec(ctx, `INSERT INTO transactions(transaction_id,user_id,account_name,counterparty,currency_code,amount_units,amount_nanos,direction,reference,source_event_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		id, strings.TrimPrefix(owner, "users/"), account, counterparty, amount.GetCurrencyCode(), amount.GetUnits(), amount.GetNanos(), direction, reference, eventID, occurred)
	if err != nil {
		return fmt.Errorf("insert %s transaction: %w", direction, err)
	}
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, owner string, limit, offset int) ([]*transactionsv1.Transaction, error) {
	rows, err := r.transactions.Query(ctx, `SELECT transaction_id,user_id,account_name,counterparty,currency_code,amount_units,amount_nanos,direction,reference,source_event_id,created_at FROM transactions WHERE user_id=$1 ORDER BY created_at DESC,transaction_id DESC LIMIT $2 OFFSET $3`, strings.TrimPrefix(owner, "users/"), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*transactionsv1.Transaction, 0)
	for rows.Next() {
		var id, userID, account, counterparty, currency, direction, reference, eventID string
		var units int64
		var nanos int32
		var created time.Time
		if err := rows.Scan(&id, &userID, &account, &counterparty, &currency, &units, &nanos, &direction, &reference, &eventID, &created); err != nil {
			return nil, err
		}
		dir := transactionsv1.TransactionDirection_TRANSACTION_DIRECTION_DEBIT
		if direction == "credit" {
			dir = transactionsv1.TransactionDirection_TRANSACTION_DIRECTION_CREDIT
		}
		result = append(result, &transactionsv1.Transaction{Name: "users/" + userID + "/transactions/" + id, Owner: "users/" + userID, Account: account, Counterparty: counterparty, Amount: &money.Money{CurrencyCode: currency, Units: units, Nanos: nanos}, Direction: dir, Reference: reference, SourceEventId: eventID, CreateTime: timestamppb.New(created)})
	}
	return result, rows.Err()
}

var _ Repository = (*PostgresRepository)(nil)
