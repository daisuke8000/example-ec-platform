// Package repository provides data access implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
)

// PostgreSQL error code for unique constraint violation.
const pgUniqueViolation = "23505"

// PostgresUserRepository implements UserRepository using PostgreSQL.
type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresUserRepository creates a new PostgreSQL-backed user repository.
func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

// Create persists a new user record.
// Returns ErrEmailAlreadyExists if the email is already taken.
func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO user_service.users (id, email, password_hash, name, is_deleted, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.IsDeleted,
		user.DeletedAt,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}

	return nil
}

// FindByID retrieves a user by their unique identifier.
// Returns ErrUserNotFound if the user doesn't exist or is soft-deleted.
func (r *PostgresUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, is_deleted, deleted_at, created_at, updated_at
		FROM user_service.users
		WHERE id = $1 AND is_deleted = FALSE
	`

	return r.scanUser(ctx, query, id)
}

// FindByEmail retrieves a user by their email address.
// Returns ErrUserNotFound if the user doesn't exist or is soft-deleted.
func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, is_deleted, deleted_at, created_at, updated_at
		FROM user_service.users
		WHERE email = $1 AND is_deleted = FALSE
	`

	return r.scanUser(ctx, query, email)
}

// scanUser executes a query and scans the result into a User struct.
func (r *PostgresUserRepository) scanUser(ctx context.Context, query string, args ...any) (*domain.User, error) {
	var user domain.User

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.IsDeleted,
		&user.DeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// Update modifies an existing user's profile.
// Returns ErrUserNotFound if the user doesn't exist or is soft-deleted.
// Returns ErrEmailAlreadyExists if updating to an email that's already taken.
func (r *PostgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE user_service.users
		SET email = $2, name = $3, updated_at = $4
		WHERE id = $1 AND is_deleted = FALSE
	`

	user.UpdatedAt = time.Now().UTC()

	result, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.Name,
		user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// SoftDelete marks a user as deleted without removing the record.
// Returns ErrUserNotFound if the user doesn't exist or is already soft-deleted.
func (r *PostgresUserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE user_service.users
		SET is_deleted = TRUE, deleted_at = $2, updated_at = $2
		WHERE id = $1 AND is_deleted = FALSE
	`

	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}
