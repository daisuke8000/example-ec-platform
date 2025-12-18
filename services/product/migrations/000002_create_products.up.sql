-- ==============================================================================
-- Migration: Create products table
-- Product Service - Product Catalog
-- ==============================================================================

-- Drop old products table if exists (from init-db.sql)
-- Note: In production, migrate data first before dropping
DROP TABLE IF EXISTS product_service.inventory CASCADE;
DROP TABLE IF EXISTS product_service.products CASCADE;

-- Products table with status management
CREATE TABLE IF NOT EXISTS product_service.products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category_id UUID REFERENCES product_service.categories(id),
    status SMALLINT NOT NULL DEFAULT 1,  -- 0=UNSPECIFIED, 1=DRAFT, 2=PUBLISHED, 3=HIDDEN
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Unique product name within same category (active records only)
    CONSTRAINT uk_products_name_category UNIQUE NULLS NOT DISTINCT (name, category_id, (deleted_at IS NULL)),

    -- Status must be valid
    CONSTRAINT chk_products_status CHECK (status >= 0 AND status <= 3)
);

-- Index for category filtering
CREATE INDEX IF NOT EXISTS idx_products_category
    ON product_service.products(category_id)
    WHERE deleted_at IS NULL;

-- Index for status filtering (PUBLISHED products for public queries)
CREATE INDEX IF NOT EXISTS idx_products_status
    ON product_service.products(status)
    WHERE deleted_at IS NULL;

-- Index for active products
CREATE INDEX IF NOT EXISTS idx_products_active
    ON product_service.products(deleted_at)
    WHERE deleted_at IS NULL;

-- Full-text search: Generated column for consistent search vector
-- Using STORED generated column ensures index and query use identical expressions
ALTER TABLE product_service.products
    ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', name || ' ' || COALESCE(description, ''))) STORED;

-- GIN index on generated search_vector column
CREATE INDEX IF NOT EXISTS idx_products_search
    ON product_service.products USING GIN (search_vector);

COMMENT ON TABLE product_service.products IS 'Product catalog with lifecycle status';
COMMENT ON COLUMN product_service.products.status IS '1=DRAFT, 2=PUBLISHED, 3=HIDDEN';
COMMENT ON COLUMN product_service.products.deleted_at IS 'Soft delete - app must cascade to SKUs when deleting products';
