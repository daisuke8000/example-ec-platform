package domain

import (
	"context"

	"github.com/google/uuid"
)

type Inventory struct {
	SKUID    uuid.UUID
	Quantity int64
	Reserved int64
	Version  int64
}

type InventoryRepository interface {
	Create(ctx context.Context, inventory *Inventory) error
	FindBySKUID(ctx context.Context, skuID uuid.UUID) (*Inventory, error)
	FindBySKUIDs(ctx context.Context, skuIDs []uuid.UUID) ([]*Inventory, error)
	Update(ctx context.Context, inventory *Inventory) error
	UpdateQuantity(ctx context.Context, skuID uuid.UUID, quantity int64) error
	Reserve(ctx context.Context, skuID uuid.UUID, amount int64, expectedVersion int64) error
	ConfirmReservation(ctx context.Context, skuID uuid.UUID, amount int64) error
	ReleaseReservation(ctx context.Context, skuID uuid.UUID, amount int64) error
}

func NewInventory(skuID uuid.UUID, quantity int64) (*Inventory, error) {
	if quantity < 0 {
		return nil, ErrInvalidQuantity
	}
	return &Inventory{
		SKUID:    skuID,
		Quantity: quantity,
		Reserved: 0,
		Version:  1,
	}, nil
}

func (i *Inventory) Available() int64 {
	return i.Quantity - i.Reserved
}

func (i *Inventory) CanReserve(amount int64) bool {
	return i.Available() >= amount
}

func (i *Inventory) Reserve(amount int64) error {
	if amount <= 0 {
		return ErrInvalidQuantity
	}
	if !i.CanReserve(amount) {
		return ErrInsufficientStock
	}
	i.Reserved += amount
	i.Version++
	return nil
}

func (i *Inventory) ConfirmReservation(amount int64) error {
	if amount <= 0 {
		return ErrInvalidQuantity
	}
	if i.Reserved < amount {
		return ErrInvalidReserved
	}
	i.Quantity -= amount
	i.Reserved -= amount
	i.Version++
	return nil
}

func (i *Inventory) ReleaseReservation(amount int64) error {
	if amount <= 0 {
		return ErrInvalidQuantity
	}
	if i.Reserved < amount {
		return ErrInvalidReserved
	}
	i.Reserved -= amount
	i.Version++
	return nil
}

func (i *Inventory) SetQuantity(quantity int64) error {
	if quantity < 0 {
		return ErrInvalidQuantity
	}
	if quantity < i.Reserved {
		return ErrInsufficientStock
	}
	i.Quantity = quantity
	i.Version++
	return nil
}

func (i *Inventory) IsLowStock(threshold int64) bool {
	return i.Available() < threshold
}
