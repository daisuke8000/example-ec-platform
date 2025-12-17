package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type PostgresCategoryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCategoryRepository(pool *pgxpool.Pool) *PostgresCategoryRepository {
	return &PostgresCategoryRepository{pool: pool}
}

func (r *PostgresCategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	query := `
		INSERT INTO product_service.categories (id, name, description, parent_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		category.ID,
		category.Name,
		category.Description,
		category.ParentID,
		category.CreatedAt,
		category.UpdatedAt,
	)
	return err
}

func (r *PostgresCategoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	query := `
		SELECT id, name, description, parent_id, created_at, updated_at, deleted_at
		FROM product_service.categories
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanCategory(ctx, query, id)
}

func (r *PostgresCategoryRepository) FindByParentID(ctx context.Context, parentID *uuid.UUID) ([]*domain.Category, error) {
	var query string
	var args []any

	if parentID == nil {
		query = `
			SELECT id, name, description, parent_id, created_at, updated_at, deleted_at
			FROM product_service.categories
			WHERE parent_id IS NULL AND deleted_at IS NULL
			ORDER BY name
		`
	} else {
		query = `
			SELECT id, name, description, parent_id, created_at, updated_at, deleted_at
			FROM product_service.categories
			WHERE parent_id = $1 AND deleted_at IS NULL
			ORDER BY name
		`
		args = append(args, *parentID)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCategories(rows)
}

func (r *PostgresCategoryRepository) FindAll(ctx context.Context) ([]*domain.Category, error) {
	query := `
		SELECT id, name, description, parent_id, created_at, updated_at, deleted_at
		FROM product_service.categories
		WHERE deleted_at IS NULL
		ORDER BY name
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCategories(rows)
}

func (r *PostgresCategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	query := `
		UPDATE product_service.categories
		SET name = $2, description = $3, parent_id = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL
	`
	category.UpdatedAt = time.Now().UTC()

	result, err := r.pool.Exec(ctx, query,
		category.ID,
		category.Name,
		category.Description,
		category.ParentID,
		category.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

func (r *PostgresCategoryRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE product_service.categories
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	now := time.Now().UTC()
	result, err := r.pool.Exec(ctx, query, id, now)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrCategoryNotFound
	}
	return nil
}

func (r *PostgresCategoryRepository) ExistsByNameAndParent(ctx context.Context, name string, parentID *uuid.UUID, excludeID *uuid.UUID) (bool, error) {
	var query string
	var args []any

	if parentID == nil {
		if excludeID == nil {
			query = `SELECT EXISTS(SELECT 1 FROM product_service.categories WHERE name = $1 AND parent_id IS NULL AND deleted_at IS NULL)`
			args = []any{name}
		} else {
			query = `SELECT EXISTS(SELECT 1 FROM product_service.categories WHERE name = $1 AND parent_id IS NULL AND id != $2 AND deleted_at IS NULL)`
			args = []any{name, *excludeID}
		}
	} else {
		if excludeID == nil {
			query = `SELECT EXISTS(SELECT 1 FROM product_service.categories WHERE name = $1 AND parent_id = $2 AND deleted_at IS NULL)`
			args = []any{name, *parentID}
		} else {
			query = `SELECT EXISTS(SELECT 1 FROM product_service.categories WHERE name = $1 AND parent_id = $2 AND id != $3 AND deleted_at IS NULL)`
			args = []any{name, *parentID, *excludeID}
		}
	}

	var exists bool
	err := r.pool.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func (r *PostgresCategoryRepository) scanCategory(ctx context.Context, query string, args ...any) (*domain.Category, error) {
	var c domain.Category
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&c.ID,
		&c.Name,
		&c.Description,
		&c.ParentID,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCategoryNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *PostgresCategoryRepository) scanCategories(rows pgx.Rows) ([]*domain.Category, error) {
	var categories []*domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.Description,
			&c.ParentID,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.DeletedAt,
		); err != nil {
			return nil, err
		}
		categories = append(categories, &c)
	}
	return categories, rows.Err()
}
