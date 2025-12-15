-- ==============================================================================
-- EC-Platform Database Initialization
-- Creates schemas for each service (schema separation pattern)
-- ==============================================================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ------------------------------------------------------------------------------
-- User Service Schema
-- ------------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS user_service;

-- Users table
CREATE TABLE IF NOT EXISTS user_service.users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    is_deleted BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for active users lookup
CREATE INDEX IF NOT EXISTS idx_users_email_active
    ON user_service.users(email)
    WHERE is_deleted = FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_deleted
    ON user_service.users(is_deleted)
    WHERE is_deleted = FALSE;

-- ------------------------------------------------------------------------------
-- Product Service Schema
-- ------------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS product_service;

-- Products table
CREATE TABLE IF NOT EXISTS product_service.products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    image_url VARCHAR(500),
    is_deleted BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Inventory table (logical separation within Product Service)
CREATE TABLE IF NOT EXISTS product_service.inventory (
    product_id UUID PRIMARY KEY REFERENCES product_service.products(id),
    quantity INT NOT NULL DEFAULT 0,
    reserved INT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for low stock alerts
CREATE INDEX IF NOT EXISTS idx_inventory_low_stock
    ON product_service.inventory(quantity)
    WHERE quantity < 10;

-- Index for active products
CREATE INDEX IF NOT EXISTS idx_products_active
    ON product_service.products(is_deleted)
    WHERE is_deleted = FALSE;

-- ------------------------------------------------------------------------------
-- Order Service Schema (includes Cart functionality)
-- ------------------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS order_service;

-- Carts table (persistent carts for logged-in users)
CREATE TABLE IF NOT EXISTS order_service.carts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,  -- References user_service.users (no FK across schemas)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Cart items table
CREATE TABLE IF NOT EXISTS order_service.cart_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cart_id UUID NOT NULL REFERENCES order_service.carts(id) ON DELETE CASCADE,
    product_id UUID NOT NULL,  -- References product_service.products (no FK across schemas)
    quantity INT NOT NULL DEFAULT 1,
    added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for user cart lookup
CREATE INDEX IF NOT EXISTS idx_carts_user_id
    ON order_service.carts(user_id);

-- Index for cart items
CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id
    ON order_service.cart_items(cart_id);

-- Orders table
CREATE TABLE IF NOT EXISTS order_service.orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,  -- References user_service.users (no FK across schemas)
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    total_amount DECIMAL(10, 2) NOT NULL,
    shipping_address JSONB,
    idempotency_key VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Order items table
CREATE TABLE IF NOT EXISTS order_service.order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES order_service.orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL,  -- References product_service.products (no FK across schemas)
    quantity INT NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for user orders lookup
CREATE INDEX IF NOT EXISTS idx_orders_user_id
    ON order_service.orders(user_id);

-- Index for idempotency key (UNIQUE per user)
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_idempotency
    ON order_service.orders(user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Index for order items
CREATE INDEX IF NOT EXISTS idx_order_items_order_id
    ON order_service.order_items(order_id);

-- ------------------------------------------------------------------------------
-- Updated timestamp trigger function
-- ------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to all tables with updated_at
DO $$
DECLARE
    t record;
BEGIN
    FOR t IN
        SELECT table_schema, table_name
        FROM information_schema.columns
        WHERE column_name = 'updated_at'
        AND table_schema IN ('user_service', 'product_service', 'order_service')
    LOOP
        EXECUTE format('
            DROP TRIGGER IF EXISTS update_%I_%I_updated_at ON %I.%I;
            CREATE TRIGGER update_%I_%I_updated_at
                BEFORE UPDATE ON %I.%I
                FOR EACH ROW
                EXECUTE FUNCTION update_updated_at_column();
        ', t.table_schema, t.table_name, t.table_schema, t.table_name,
           t.table_schema, t.table_name, t.table_schema, t.table_name);
    END LOOP;
END;
$$;

-- ------------------------------------------------------------------------------
-- Grant permissions (adjust based on your security requirements)
-- ------------------------------------------------------------------------------
-- In production, create separate users for each service with limited permissions
GRANT ALL PRIVILEGES ON SCHEMA user_service TO ecplatform;
GRANT ALL PRIVILEGES ON SCHEMA product_service TO ecplatform;
GRANT ALL PRIVILEGES ON SCHEMA order_service TO ecplatform;

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA user_service TO ecplatform;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA product_service TO ecplatform;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA order_service TO ecplatform;
