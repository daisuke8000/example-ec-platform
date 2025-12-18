package domain

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxSKUCodeLength = 100

type Money struct {
	Amount   int64
	Currency string
}

func NewMoney(amount int64, currency string) (*Money, error) {
	if amount < 0 {
		return nil, ErrInvalidPrice
	}
	if currency == "" {
		currency = "JPY"
	}
	return &Money{
		Amount:   amount,
		Currency: currency,
	}, nil
}

type SKU struct {
	ID         uuid.UUID
	ProductID  uuid.UUID
	SKUCode    string
	Price      Money
	Attributes map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

type SKUWithInventory struct {
	SKU       *SKU
	Inventory *Inventory
}

type SKURepository interface {
	Create(ctx context.Context, sku *SKU) error
	FindByID(ctx context.Context, id uuid.UUID) (*SKU, error)
	FindByIDWithInventory(ctx context.Context, id uuid.UUID) (*SKUWithInventory, error)
	FindByProductID(ctx context.Context, productID uuid.UUID) ([]*SKU, error)
	FindBySKUCode(ctx context.Context, skuCode string) (*SKU, error)
	Update(ctx context.Context, sku *SKU) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ExistsBySKUCode(ctx context.Context, skuCode string, excludeID *uuid.UUID) (bool, error)
}

func NewSKU(productID uuid.UUID, skuCode string, price Money, attributes map[string]string) (*SKU, error) {
	if err := ValidateSKUCode(skuCode); err != nil {
		return nil, err
	}
	if price.Amount < 0 {
		return nil, ErrInvalidPrice
	}

	if attributes == nil {
		attributes = make(map[string]string)
	}

	now := time.Now().UTC()
	return &SKU{
		ID:         uuid.New(),
		ProductID:  productID,
		SKUCode:    skuCode,
		Price:      price,
		Attributes: attributes,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func ValidateSKUCode(code string) error {
	if code == "" {
		return ErrEmptySKUCode
	}
	if utf8.RuneCountInString(code) > MaxSKUCodeLength {
		return ErrSKUCodeTooLong
	}
	return nil
}

func (s *SKU) IsDeleted() bool {
	return s.DeletedAt != nil
}

func (s *SKU) Update(skuCode string, price Money, attributes map[string]string) error {
	if err := ValidateSKUCode(skuCode); err != nil {
		return err
	}
	if price.Amount < 0 {
		return ErrInvalidPrice
	}

	s.SKUCode = skuCode
	s.Price = price
	if attributes != nil {
		s.Attributes = attributes
	}
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *SKU) UpdatePrice(price Money) error {
	if price.Amount < 0 {
		return ErrInvalidPrice
	}
	s.Price = price
	s.UpdatedAt = time.Now().UTC()
	return nil
}
