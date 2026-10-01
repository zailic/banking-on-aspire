package userrepo

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	usersv1 "dev.local/banking-on-aspire/platform/gen/go/banking/users/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed migrations/001_create_users.sql
var usersSchema string

type PostgresRepository struct{ pool *pgxpool.Pool }

func OpenPostgres(ctx context.Context, connectionString string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("configure users database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to users database: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() { r.pool.Close() }

func (r *PostgresRepository) Migrate(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, usersSchema); err != nil {
		return fmt.Errorf("migrate users database: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, name string) (*usersv1.User, error) {
	userID := strings.TrimPrefix(name, "users/")
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT user_id, keycloak_subject, username, display_name, email,
		       status, created_at, updated_at, etag
		FROM users WHERE user_id = $1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

func (r *PostgresRepository) Resolve(
	ctx context.Context,
	identity Identity,
) (*usersv1.User, error) {
	now := time.Now().UTC()
	user, err := scanUser(r.pool.QueryRow(ctx, `
		WITH resolved AS (
			INSERT INTO users (
				user_id, keycloak_subject, username, display_name, email,
				status, created_at, updated_at, etag)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), 'active', $6, $6, $7)
			ON CONFLICT (keycloak_subject) DO UPDATE
			SET username = EXCLUDED.username,
			    display_name = EXCLUDED.display_name,
			    email = EXCLUDED.email,
			    updated_at = EXCLUDED.updated_at,
			    etag = EXCLUDED.etag
			WHERE (users.username, users.display_name, users.email)
			      IS DISTINCT FROM
			      (EXCLUDED.username, EXCLUDED.display_name, EXCLUDED.email)
			RETURNING user_id, keycloak_subject, username, display_name, email,
			          status, created_at, updated_at, etag
		)
		SELECT user_id, keycloak_subject, username, display_name, email,
		       status, created_at, updated_at, etag
		FROM resolved
		UNION ALL
		SELECT user_id, keycloak_subject, username, display_name, email,
		       status, created_at, updated_at, etag
		FROM users
		WHERE keycloak_subject = $2
		LIMIT 1`,
		newIdentifier("usr"), identity.Subject, identity.Username,
		identity.DisplayName, identity.Email, now, newIdentifier("etag")))
	if isUniqueViolation(err) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("resolve user identity: %w", err)
	}
	return user, nil
}

type rowScanner interface{ Scan(...any) error }

func scanUser(scanner rowScanner) (*usersv1.User, error) {
	var userID, subject, username, displayName, status, etag string
	var email *string
	var createdAt, updatedAt time.Time
	if err := scanner.Scan(&userID, &subject, &username, &displayName, &email,
		&status, &createdAt, &updatedAt, &etag); err != nil {
		return nil, err
	}
	return &usersv1.User{
		Name:            "users/" + userID,
		KeycloakSubject: subject,
		Username:        username,
		DisplayName:     displayName,
		Email:           valueOrEmpty(email),
		Status:          protoStatus(status),
		CreateTime:      timestamppb.New(createdAt),
		UpdateTime:      timestamppb.New(updatedAt),
		Etag:            etag,
	}, nil
}

func protoStatus(value string) usersv1.UserStatus {
	switch value {
	case "active":
		return usersv1.UserStatus_USER_STATUS_ACTIVE
	case "suspended":
		return usersv1.UserStatus_USER_STATUS_SUSPENDED
	case "closed":
		return usersv1.UserStatus_USER_STATUS_CLOSED
	default:
		return usersv1.UserStatus_USER_STATUS_UNSPECIFIED
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func newIdentifier(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Errorf("generate %s identifier: %w", prefix, err))
	}
	return prefix + "-" + hex.EncodeToString(value)
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

var _ Repository = (*PostgresRepository)(nil)
