-- ==============================================================================
-- Migration: Create SKUs table
-- Product Service - Stock Keeping Units (Product Variants)
-- ==============================================================================

-- SKUs table (product variants)
-- NOTE: ON DELETE CASCADE is for physical delete safety only.
-- For soft deletes, app MUST cascade deleted_at to SKUs when parent product is soft-deleted.
CREATE TABLE IF NOT EXISTS product_service.skus (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES product_service.products(id) ON DELETE CASCADE,
    sku_code VARCHAR(100) NOT NULL,
    price_amount BIGINT NOT NULL,          -- Smallest currency unit (cents, yen)
    price_currency VARCHAR(3) NOT NULL DEFAULT 'JPY',  -- ISO 4217
    attributes JSONB DEFAULT '{}',          -- e.g., {"color": "blue", "size": "M"}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Unique SKU code (active records only)
    CONSTRAINT uk_skus_code UNIQUE (sku_code) WHERE (deleted_at IS NULL),

    -- Price must be non-negative
    CONSTRAINT chk_skus_price_positive CHECK (price_amount >= 0)
);

-- Index for product variants lookup
CREATE INDEX IF NOT EXISTS idx_skus_product
    ON product_service.skus(product_id)
    WHERE deleted_at IS NULL;

-- Index for price range queries
CREATE INDEX IF NOT EXISTS idx_skus_price
    ON product_service.skus(price_amount)
    WHERE deleted_at IS NULL;

-- Index for active SKUs
CREATE INDEX IF NOT EXISTS idx_skus_active
    ON product_service.skus(deleted_at)
    WHERE deleted_at IS NULL;

-- GIN index for JSONB attributes queries
CREATE INDEX IF NOT EXISTS idx_skus_attributes
    ON product_service.skus
    USING gin(attributes);

COMMENT ON TABLE product_service.skus IS 'Product variants with individual pricing';
COMMENT ON COLUMN product_service.skus.sku_code IS 'Unique identifier like SHIRT-BLU-M';
COMMENT ON COLUMN product_service.skus.price_amount IS 'Price in smallest currency unit (cents, yen)';
COMMENT ON COLUMN product_service.skus.attributes IS 'Variant attributes as JSON (color, size, etc.)';
