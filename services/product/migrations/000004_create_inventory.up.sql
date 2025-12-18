-- ==============================================================================
-- Migration: Create inventory table
-- Product Service - Stock Level Tracking
-- ==============================================================================

-- Inventory table (tracks stock per SKU)
CREATE TABLE IF NOT EXISTS product_service.inventory (
    sku_id UUID PRIMARY KEY REFERENCES product_service.skus(id) ON DELETE CASCADE,
    quantity BIGINT NOT NULL DEFAULT 0,    -- Total stock quantity
    reserved BIGINT NOT NULL DEFAULT 0,    -- Quantity reserved by pending orders
    version BIGINT NOT NULL DEFAULT 1,     -- For optimistic locking
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Quantity must be non-negative
    CONSTRAINT chk_inventory_quantity_positive CHECK (quantity >= 0),

    -- Reserved must be non-negative
    CONSTRAINT chk_inventory_reserved_positive CHECK (reserved >= 0),

    -- Available quantity (quantity - reserved) must be non-negative
    CONSTRAINT chk_inventory_available CHECK (quantity >= reserved)
);

-- Index for low stock alerts (quantity minus reserved)
CREATE INDEX IF NOT EXISTS idx_inventory_low_available
    ON product_service.inventory((quantity - reserved))
    WHERE (quantity - reserved) < 10;

COMMENT ON TABLE product_service.inventory IS 'Stock levels with reservation tracking';
COMMENT ON COLUMN product_service.inventory.quantity IS 'Total stock quantity';
COMMENT ON COLUMN product_service.inventory.reserved IS 'Quantity reserved by pending orders';
COMMENT ON COLUMN product_service.inventory.version IS 'Optimistic locking version';
