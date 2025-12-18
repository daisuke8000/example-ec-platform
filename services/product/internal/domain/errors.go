package domain

import "errors"

var (
	ErrProductNotFound     = errors.New("product not found")
	ErrSKUNotFound         = errors.New("sku not found")
	ErrCategoryNotFound    = errors.New("category not found")
	ErrInventoryNotFound   = errors.New("inventory not found")
	ErrReservationNotFound = errors.New("reservation not found")
)

var (
	ErrEmptyProductName    = errors.New("product name cannot be empty")
	ErrProductNameTooLong  = errors.New("product name must be 255 characters or less")
	ErrEmptySKUCode        = errors.New("sku code cannot be empty")
	ErrSKUCodeTooLong      = errors.New("sku code must be 100 characters or less")
	ErrInvalidPrice        = errors.New("price must be non-negative")
	ErrEmptyCategoryName   = errors.New("category name cannot be empty")
	ErrCategoryNameTooLong = errors.New("category name must be 255 characters or less")
	ErrSelfParentCategory  = errors.New("category cannot be its own parent")
	ErrInvalidQuantity     = errors.New("quantity must be non-negative")
	ErrInvalidReserved     = errors.New("reserved must be non-negative")
)

var (
	ErrSKUCodeAlreadyExists   = errors.New("sku code already exists")
	ErrCategoryNameExists     = errors.New("category name already exists in same parent")
	ErrOptimisticLockConflict = errors.New("concurrent modification detected")
	ErrIdempotencyKeyExists   = errors.New("idempotency key already processed")
)

var (
	ErrInsufficientStock     = errors.New("insufficient stock available")
	ErrReservationExpired    = errors.New("reservation has expired")
	ErrReservationNotPending = errors.New("reservation is not in pending status")
	ErrBatchSizeExceeded     = errors.New("batch size exceeds maximum limit")
)

var (
	ErrInvalidProductStatus     = errors.New("invalid product status")
	ErrInvalidReservationStatus = errors.New("invalid reservation status")
)
