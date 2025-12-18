package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type PostgresProductRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresProductRepository(pool *pgxpool.Pool) *PostgresProductRepository {
	return &PostgresProductRepository{pool: pool}
}

func (r *PostgresProductRepository) Create(ctx context.Context, product *domain.Product) error {
	query := `
		INSERT INTO product_service.products (id, name, description, category_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query,
		product.ID,
		product.Name,
		product.Description,
		product.CategoryID,
		product.Status,
		product.CreatedAt,
		product.UpdatedAt,
	)
	return err
}

func (r *PostgresProductRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	query := `
		SELECT id, name, description, category_id, status, created_at, updated_at, deleted_at
		FROM product_service.products
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanProduct(ctx, query, id)
}

func (r *PostgresProductRepository) FindByIDWithSKUs(ctx context.Context, id uuid.UUID) (*domain.ProductWithSKUs, error) {
	product, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, sku_code, price_amount, price_currency, attributes, created_at, updated_at, deleted_at
		FROM product_service.skus
		WHERE product_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &domain.ProductWithSKUs{
		Product: product,
		SKUs:    skus,
	}, nil
}

func (r *PostgresProductRepository) List(ctx context.Context, filter domain.ProductFilter, pagination domain.Pagination) ([]*domain.Product, int64, error) {
	baseQuery := `FROM product_service.products WHERE deleted_at IS NULL`
	args := make([]any, 0)
	argIdx := 1

	if filter.CategoryID != nil {
		baseQuery += fmt.Sprintf(" AND category_id = $%d", argIdx)
		args = append(args, *filter.CategoryID)
		argIdx++
	}

	if filter.Status != nil {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}

	if filter.Search != nil && *filter.Search != "" {
		baseQuery += fmt.Sprintf(" AND search_vector @@ plainto_tsquery('english', $%d)", argIdx)
		args = append(args, *filter.Search)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) " + baseQuery
	var totalCount int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	selectQuery := `SELECT id, name, description, category_id, status, created_at, updated_at, deleted_at ` + baseQuery
	selectQuery += " ORDER BY created_at DESC"

	if pagination.PageSize > 0 {
		selectQuery += fmt.Sprintf(" LIMIT %d", pagination.PageSize)
	}

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	products, err := r.scanProducts(rows)
	if err != nil {
		return nil, 0, err
	}

	return products, totalCount, nil
}

func (r *PostgresProductRepository) Update(ctx context.Context, product *domain.Product) error {
	query := `
		UPDATE product_service.products
		SET name = $2, description = $3, category_id = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL
	`
	product.UpdatedAt = time.Now().UTC()

	result, err := r.pool.Exec(ctx, query,
		product.ID,
		product.Name,
		product.Description,
		product.CategoryID,
		product.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func (r *PostgresProductRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ProductStatus) error {
	query := `
		UPDATE product_service.products
		SET status = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
	`
	now := time.Now().UTC()

	result, err := r.pool.Exec(ctx, query, id, status, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func (r *PostgresProductRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE product_service.products
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	now := time.Now().UTC()

	result, err := r.pool.Exec(ctx, query, id, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func (r *PostgresProductRepository) SoftDeleteWithSKUs(ctx context.Context, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()

	skuQuery := `
		UPDATE product_service.skus
		SET deleted_at = $2, updated_at = $2
		WHERE product_id = $1 AND deleted_at IS NULL
	`
	if _, err := tx.Exec(ctx, skuQuery, id, now); err != nil {
		return err
	}

	productQuery := `
		UPDATE product_service.products
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := tx.Exec(ctx, productQuery, id, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}

	return tx.Commit(ctx)
}

func (r *PostgresProductRepository) scanProduct(ctx context.Context, query string, args ...any) (*domain.Product, error) {
	var p domain.Product
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&p.ID,
		&p.Name,
		&p.Description,
		&p.CategoryID,
		&p.Status,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *PostgresProductRepository) scanProducts(rows pgx.Rows) ([]*domain.Product, error) {
	var products []*domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.CategoryID,
			&p.Status,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.DeletedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, &p)
	}
	return products, rows.Err()
}
