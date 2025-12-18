-- ==============================================================================
-- Migration: Create categories table
-- Product Service - Category Management
-- ==============================================================================

-- Categories table with hierarchical structure (Adjacency List)
CREATE TABLE IF NOT EXISTS product_service.categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    parent_id UUID REFERENCES product_service.categories(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Unique category name within same parent (active records only)
    CONSTRAINT uk_categories_name_parent UNIQUE NULLS NOT DISTINCT (name, parent_id, (deleted_at IS NULL))
);

-- Index for parent category queries
CREATE INDEX IF NOT EXISTS idx_categories_parent
    ON product_service.categories(parent_id)
    WHERE deleted_at IS NULL;

-- Index for active categories
CREATE INDEX IF NOT EXISTS idx_categories_active
    ON product_service.categories(deleted_at)
    WHERE deleted_at IS NULL;

COMMENT ON TABLE product_service.categories IS 'Product categories with hierarchical structure';
COMMENT ON COLUMN product_service.categories.parent_id IS 'References parent category for tree structure';
COMMENT ON COLUMN product_service.categories.deleted_at IS 'Soft delete timestamp';
