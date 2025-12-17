package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type PostgresReservationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReservationRepository(pool *pgxpool.Pool) *PostgresReservationRepository {
	return &PostgresReservationRepository{pool: pool}
}

func (r *PostgresReservationRepository) Create(ctx context.Context, reservation *domain.Reservation) error {
	itemsJSON, err := json.Marshal(reservation.Items)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO product_service.reservations (id, status, items, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.pool.Exec(ctx, query,
		reservation.ID,
		reservation.Status,
		itemsJSON,
		reservation.ExpiresAt,
		reservation.CreatedAt,
		reservation.UpdatedAt,
	)
	return err
}

func (r *PostgresReservationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	query := `
		SELECT id, status, items, expires_at, created_at, updated_at
		FROM product_service.reservations
		WHERE id = $1
	`
	var res domain.Reservation
	var itemsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&res.ID,
		&res.Status,
		&itemsJSON,
		&res.ExpiresAt,
		&res.CreatedAt,
		&res.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrReservationNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(itemsJSON, &res.Items); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PostgresReservationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReservationStatus) error {
	query := `
		UPDATE product_service.reservations
		SET status = $2, updated_at = $3
		WHERE id = $1
	`
	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id, status, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrReservationNotFound
	}
	return nil
}

func (r *PostgresReservationRepository) FindExpiredPending(ctx context.Context, limit int) ([]*domain.Reservation, error) {
	query := `
		SELECT id, status, items, expires_at, created_at, updated_at
		FROM product_service.reservations
		WHERE status = $1 AND expires_at < $2
		ORDER BY expires_at
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.pool.Query(ctx, query, domain.ReservationStatusPending, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reservations []*domain.Reservation
	for rows.Next() {
		var res domain.Reservation
		var itemsJSON []byte

		if err := rows.Scan(
			&res.ID,
			&res.Status,
			&itemsJSON,
			&res.ExpiresAt,
			&res.CreatedAt,
			&res.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(itemsJSON, &res.Items); err != nil {
			return nil, err
		}
		reservations = append(reservations, &res)
	}
	return reservations, rows.Err()
}

func (r *PostgresReservationRepository) BatchUpdateExpired(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	query := `
		UPDATE product_service.reservations
		SET status = $1, updated_at = $2
		WHERE id = ANY($3) AND status = $4
	`
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, query, domain.ReservationStatusExpired, now, ids, domain.ReservationStatusPending)
	return err
}

func (r *PostgresReservationRepository) CreateWithTx(ctx context.Context, tx pgx.Tx, reservation *domain.Reservation) error {
	itemsJSON, err := json.Marshal(reservation.Items)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO product_service.reservations (id, status, items, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.Exec(ctx, query,
		reservation.ID,
		reservation.Status,
		itemsJSON,
		reservation.ExpiresAt,
		reservation.CreatedAt,
		reservation.UpdatedAt,
	)
	return err
}
