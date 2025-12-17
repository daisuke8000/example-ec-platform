package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

const pgUniqueViolation = "23505"

type PostgresSKURepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSKURepository(pool *pgxpool.Pool) *PostgresSKURepository {
	return &PostgresSKURepository{pool: pool}
}

func (r *PostgresSKURepository) Create(ctx context.Context, sku *domain.SKU) error {
	query := `
		INSERT INTO product_service.skus (id, product_id, sku_code, price_amount, price_currency, attributes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		sku.ID,
		sku.ProductID,
		sku.SKUCode,
		sku.Price.Amount,
		sku.Price.Currency,
		sku.Attributes,
		sku.CreatedAt,
		sku.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.ErrSKUCodeAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresSKURepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.SKU, error) {
	query := `
		SELECT id, product_id, sku_code, price_amount, price_currency, attributes, created_at, updated_at, deleted_at
		FROM product_service.skus
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanSKU(ctx, query, id)
}

func (r *PostgresSKURepository) FindByIDWithInventory(ctx context.Context, id uuid.UUID) (*domain.SKUWithInventory, error) {
	query := `
		SELECT s.id, s.product_id, s.sku_code, s.price_amount, s.price_currency, s.attributes, s.created_at, s.updated_at, s.deleted_at,
		       i.sku_id, i.quantity, i.reserved, i.version
		FROM product_service.skus s
		LEFT JOIN product_service.inventory i ON s.id = i.sku_id
		WHERE s.id = $1 AND s.deleted_at IS NULL
	`
	var s domain.SKU
	var inv struct {
		SKUID    *uuid.UUID
		Quantity *int64
		Reserved *int64
		Version  *int64
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.ProductID,
		&s.SKUCode,
		&s.Price.Amount,
		&s.Price.Currency,
		&s.Attributes,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.DeletedAt,
		&inv.SKUID,
		&inv.Quantity,
		&inv.Reserved,
		&inv.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSKUNotFound
		}
		return nil, err
	}

	result := &domain.SKUWithInventory{SKU: &s}
	if inv.SKUID != nil {
		result.Inventory = &domain.Inventory{
			SKUID:    *inv.SKUID,
			Quantity: *inv.Quantity,
			Reserved: *inv.Reserved,
			Version:  *inv.Version,
		}
	}
	return result, nil
}

func (r *PostgresSKURepository) FindByProductID(ctx context.Context, productID uuid.UUID) ([]*domain.SKU, error) {
	query := `
		SELECT id, product_id, sku_code, price_amount, price_currency, attributes, created_at, updated_at, deleted_at
		FROM product_service.skus
		WHERE product_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`
	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSKUs(rows)
}

func (r *PostgresSKURepository) FindBySKUCode(ctx context.Context, skuCode string) (*domain.SKU, error) {
	query := `
		SELECT id, product_id, sku_code, price_amount, price_currency, attributes, created_at, updated_at, deleted_at
		FROM product_service.skus
		WHERE sku_code = $1 AND deleted_at IS NULL
	`
	return r.scanSKU(ctx, query, skuCode)
}

func (r *PostgresSKURepository) Update(ctx context.Context, sku *domain.SKU) error {
	query := `
		UPDATE product_service.skus
		SET sku_code = $2, price_amount = $3, price_currency = $4, attributes = $5, updated_at = $6
		WHERE id = $1 AND deleted_at IS NULL
	`
	sku.UpdatedAt = time.Now().UTC()

	result, err := r.pool.Exec(ctx, query,
		sku.ID,
		sku.SKUCode,
		sku.Price.Amount,
		sku.Price.Currency,
		sku.Attributes,
		sku.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.ErrSKUCodeAlreadyExists
		}
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrSKUNotFound
	}
	return nil
}

func (r *PostgresSKURepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE product_service.skus
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	now := time.Now().UTC()

	result, err := r.pool.Exec(ctx, query, id, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrSKUNotFound
	}
	return nil
}

func (r *PostgresSKURepository) ExistsBySKUCode(ctx context.Context, skuCode string, excludeID *uuid.UUID) (bool, error) {
	var query string
	var args []any

	if excludeID == nil {
		query = `SELECT EXISTS(SELECT 1 FROM product_service.skus WHERE sku_code = $1 AND deleted_at IS NULL)`
		args = []any{skuCode}
	} else {
		query = `SELECT EXISTS(SELECT 1 FROM product_service.skus WHERE sku_code = $1 AND id != $2 AND deleted_at IS NULL)`
		args = []any{skuCode, *excludeID}
	}

	var exists bool
	err := r.pool.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func (r *PostgresSKURepository) scanSKU(ctx context.Context, query string, args ...any) (*domain.SKU, error) {
	var s domain.SKU
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&s.ID,
		&s.ProductID,
		&s.SKUCode,
		&s.Price.Amount,
		&s.Price.Currency,
		&s.Attributes,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSKUNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *PostgresSKURepository) scanSKUs(rows pgx.Rows) ([]*domain.SKU, error) {
	var skus []*domain.SKU
	for rows.Next() {
		var s domain.SKU
		if err := rows.Scan(
			&s.ID,
			&s.ProductID,
			&s.SKUCode,
			&s.Price.Amount,
			&s.Price.Currency,
			&s.Attributes,
			&s.CreatedAt,
			&s.UpdatedAt,
			&s.DeletedAt,
		); err != nil {
			return nil, err
		}
		skus = append(skus, &s)
	}
	return skus, rows.Err()
}
