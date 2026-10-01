package contactrepo

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	contactsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/contacts/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed migrations/001_create_contacts.sql
var createContactsSchema string

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, connectionString string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("configure contacts database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to contacts database: %w", err)
	}
	repository := &PostgresRepository{pool: pool}
	return repository, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) Migrate(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, createContactsSchema); err != nil {
		return fmt.Errorf("migrate contacts database: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ResolveUserName(ctx context.Context, subject string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `
		SELECT user_id
		FROM users
		WHERE keycloak_subject = $1
		  AND status = 'active'`, subject).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve contact owner: %w", err)
	}
	return "users/" + userID, nil
}

func (r *PostgresRepository) List(
	ctx context.Context,
	parent string,
) ([]*contactsv1.Contact, error) {
	userID := strings.TrimPrefix(parent, "users/")
	rows, err := r.pool.Query(ctx, `
		SELECT contact_id, display_name, destination_type, internal_account,
		       external_routing_number, external_account_number,
		       created_at, updated_at, etag
		FROM contacts
		WHERE user_id = $1
		ORDER BY contact_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	contacts := make([]*contactsv1.Contact, 0)
	for rows.Next() {
		contact, err := scanContact(userID, rows)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list contacts rows: %w", err)
	}
	return contacts, nil
}

func (r *PostgresRepository) Get(ctx context.Context, name string) (*contactsv1.Contact, error) {
	userID, contactID := resourceIDs(name)
	contact, err := scanContact(userID, r.pool.QueryRow(ctx, `
		SELECT contact_id, display_name, destination_type, internal_account,
		       external_routing_number, external_account_number,
		       created_at, updated_at, etag
		FROM contacts
		WHERE user_id = $1 AND contact_id = $2`, userID, contactID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return contact, err
}

func (r *PostgresRepository) Create(ctx context.Context, contact *contactsv1.Contact) error {
	row, err := contactRow(contact)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO contacts (
			user_id, contact_id, display_name, destination_type,
			internal_account, external_routing_number, external_account_number,
			created_at, updated_at, etag)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		row.userID, row.contactID, row.displayName, row.destinationType,
		row.internalAccount, row.externalRoutingNumber, row.externalAccountNumber,
		row.createdAt, row.updatedAt, row.etag)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create contact: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Update(
	ctx context.Context,
	contact *contactsv1.Contact,
	expectedEtag string,
) error {
	row, err := contactRow(contact)
	if err != nil {
		return err
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE contacts
		SET display_name = $3,
		    destination_type = $4,
		    internal_account = $5,
		    external_routing_number = $6,
		    external_account_number = $7,
		    updated_at = $8,
		    etag = $9
		WHERE user_id = $1
		  AND contact_id = $2
		  AND ($10 = '' OR etag = $10)`,
		row.userID, row.contactID, row.displayName, row.destinationType,
		row.internalAccount, row.externalRoutingNumber, row.externalAccountNumber,
		row.updatedAt, row.etag, expectedEtag)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	return r.missingOrConflict(ctx, row.userID, row.contactID)
}

func (r *PostgresRepository) Delete(ctx context.Context, name, expectedEtag string) error {
	userID, contactID := resourceIDs(name)
	command, err := r.pool.Exec(ctx, `
		DELETE FROM contacts
		WHERE user_id = $1
		  AND contact_id = $2
		  AND ($3 = '' OR etag = $3)`, userID, contactID, expectedEtag)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	return r.missingOrConflict(ctx, userID, contactID)
}

func (r *PostgresRepository) missingOrConflict(
	ctx context.Context,
	userID, contactID string,
) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM contacts WHERE user_id = $1 AND contact_id = $2
		)`, userID, contactID).Scan(&exists); err != nil {
		return fmt.Errorf("check contact existence: %w", err)
	}
	if exists {
		return ErrConflict
	}
	return ErrNotFound
}

type rowScanner interface {
	Scan(...any) error
}

func scanContact(userID string, scanner rowScanner) (*contactsv1.Contact, error) {
	var (
		contactID, displayName, destinationType       string
		internalAccount, routingNumber, accountNumber *string
		createdAt, updatedAt                          time.Time
		etag                                          string
	)
	if err := scanner.Scan(
		&contactID, &displayName, &destinationType, &internalAccount,
		&routingNumber, &accountNumber, &createdAt, &updatedAt, &etag); err != nil {
		return nil, err
	}
	contact := &contactsv1.Contact{
		Name:        "users/" + userID + "/contacts/" + contactID,
		DisplayName: displayName,
		CreateTime:  timestamppb.New(createdAt),
		UpdateTime:  timestamppb.New(updatedAt),
		Etag:        etag,
	}
	if destinationType == "internal" {
		contact.Destination = &contactsv1.Contact_InternalAccount{
			InternalAccount: valueOrEmpty(internalAccount),
		}
	} else {
		contact.Destination = &contactsv1.Contact_ExternalAccount{
			ExternalAccount: &contactsv1.ExternalBankAccount{
				RoutingNumber: valueOrEmpty(routingNumber),
				AccountNumber: valueOrEmpty(accountNumber),
			},
		}
	}
	return contact, nil
}

type databaseRow struct {
	userID, contactID, displayName, destinationType               string
	internalAccount, externalRoutingNumber, externalAccountNumber *string
	createdAt, updatedAt                                          time.Time
	etag                                                          string
}

func contactRow(contact *contactsv1.Contact) (databaseRow, error) {
	userID, contactID := resourceIDs(contact.GetName())
	row := databaseRow{
		userID:      userID,
		contactID:   contactID,
		displayName: contact.GetDisplayName(),
		createdAt: contact.GetCreateTime().
			AsTime(),
		updatedAt: contact.GetUpdateTime().AsTime(),
		etag:      contact.GetEtag(),
	}
	switch destination := contact.GetDestination().(type) {
	case *contactsv1.Contact_InternalAccount:
		row.destinationType = "internal"
		row.internalAccount = &destination.InternalAccount
	case *contactsv1.Contact_ExternalAccount:
		row.destinationType = "external"
		routingNumber := destination.ExternalAccount.GetRoutingNumber()
		accountNumber := destination.ExternalAccount.GetAccountNumber()
		row.externalRoutingNumber = &routingNumber
		row.externalAccountNumber = &accountNumber
	default:
		return databaseRow{}, fmt.Errorf("contact destination is missing")
	}
	return row, nil
}

func resourceIDs(name string) (string, string) {
	parts := strings.Split(name, "/")
	return parts[1], parts[3]
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

var _ Repository = (*PostgresRepository)(nil)
var _ UserResolver = (*PostgresRepository)(nil)
