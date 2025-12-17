package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type PostgresInventoryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresInventoryRepository(pool *pgxpool.Pool) *PostgresInventoryRepository {
	return &PostgresInventoryRepository{pool: pool}
}

func (r *PostgresInventoryRepository) Create(ctx context.Context, inventory *domain.Inventory) error {
	query := `
		INSERT INTO product_service.inventory (sku_id, quantity, reserved, version)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, query,
		inventory.SKUID,
		inventory.Quantity,
		inventory.Reserved,
		inventory.Version,
	)
	return err
}

func (r *PostgresInventoryRepository) FindBySKUID(ctx context.Context, skuID uuid.UUID) (*domain.Inventory, error) {
	query := `
		SELECT sku_id, quantity, reserved, version
		FROM product_service.inventory
		WHERE sku_id = $1
	`
	var inv domain.Inventory
	err := r.pool.QueryRow(ctx, query, skuID).Scan(
		&inv.SKUID,
		&inv.Quantity,
		&inv.Reserved,
		&inv.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInventoryNotFound
		}
		return nil, err
	}
	return &inv, nil
}

func (r *PostgresInventoryRepository) FindBySKUIDs(ctx context.Context, skuIDs []uuid.UUID) ([]*domain.Inventory, error) {
	if len(skuIDs) == 0 {
		return []*domain.Inventory{}, nil
	}

	query := `
		SELECT sku_id, quantity, reserved, version
		FROM product_service.inventory
		WHERE sku_id = ANY($1)
	`
	rows, err := r.pool.Query(ctx, query, skuIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inventories []*domain.Inventory
	for rows.Next() {
		var inv domain.Inventory
		if err := rows.Scan(&inv.SKUID, &inv.Quantity, &inv.Reserved, &inv.Version); err != nil {
			return nil, err
		}
		inventories = append(inventories, &inv)
	}
	return inventories, rows.Err()
}

func (r *PostgresInventoryRepository) Update(ctx context.Context, inventory *domain.Inventory) error {
	query := `
		UPDATE product_service.inventory
		SET quantity = $2, reserved = $3, version = version + 1, updated_at = NOW()
		WHERE sku_id = $1 AND version = $4
	`
	result, err := r.pool.Exec(ctx, query,
		inventory.SKUID,
		inventory.Quantity,
		inventory.Reserved,
		inventory.Version,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrOptimisticLockConflict
	}
	inventory.Version++
	return nil
}

func (r *PostgresInventoryRepository) UpdateQuantity(ctx context.Context, skuID uuid.UUID, quantity int64) error {
	query := `
		UPDATE product_service.inventory
		SET quantity = $2, version = version + 1, updated_at = NOW()
		WHERE sku_id = $1 AND quantity - reserved <= $2 - reserved
	`
	result, err := r.pool.Exec(ctx, query, skuID, quantity)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInsufficientStock
	}
	return nil
}

func (r *PostgresInventoryRepository) Reserve(ctx context.Context, skuID uuid.UUID, amount int64, expectedVersion int64) error {
	query := `
		UPDATE product_service.inventory
		SET reserved = reserved + $2, version = version + 1, updated_at = NOW()
		WHERE sku_id = $1 AND version = $3 AND quantity - reserved >= $2
	`
	result, err := r.pool.Exec(ctx, query, skuID, amount, expectedVersion)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		inv, findErr := r.FindBySKUID(ctx, skuID)
		if findErr != nil {
			return findErr
		}
		if inv.Version != expectedVersion {
			return domain.ErrOptimisticLockConflict
		}
		return domain.ErrInsufficientStock
	}
	return nil
}

func (r *PostgresInventoryRepository) ConfirmReservation(ctx context.Context, skuID uuid.UUID, amount int64) error {
	query := `
		UPDATE product_service.inventory
		SET quantity = quantity - $2, reserved = reserved - $2, version = version + 1, updated_at = NOW()
		WHERE sku_id = $1 AND reserved >= $2
	`
	result, err := r.pool.Exec(ctx, query, skuID, amount)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInvalidReserved
	}
	return nil
}

func (r *PostgresInventoryRepository) ReleaseReservation(ctx context.Context, skuID uuid.UUID, amount int64) error {
	query := `
		UPDATE product_service.inventory
		SET reserved = reserved - $2, version = version + 1, updated_at = NOW()
		WHERE sku_id = $1 AND reserved >= $2
	`
	result, err := r.pool.Exec(ctx, query, skuID, amount)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInvalidReserved
	}
	return nil
}

func (r *PostgresInventoryRepository) ReserveWithTx(ctx context.Context, tx pgx.Tx, skuID uuid.UUID, amount int64) error {
	query := `
		UPDATE product_service.inventory
		SET reserved = reserved + $2, version = version + 1, updated_at = NOW()
		WHERE sku_id = $1 AND quantity - reserved >= $2
	`
	result, err := tx.Exec(ctx, query, skuID, amount)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInsufficientStock
	}
	return nil
}
