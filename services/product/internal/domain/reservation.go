package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ReservationStatus int32

const (
	ReservationStatusPending   ReservationStatus = 0
	ReservationStatusConfirmed ReservationStatus = 1
	ReservationStatusReleased  ReservationStatus = 2
	ReservationStatusExpired   ReservationStatus = 3
)

func (s ReservationStatus) String() string {
	switch s {
	case ReservationStatusPending:
		return "PENDING"
	case ReservationStatusConfirmed:
		return "CONFIRMED"
	case ReservationStatusReleased:
		return "RELEASED"
	case ReservationStatusExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}

func (s ReservationStatus) IsValid() bool {
	return s >= ReservationStatusPending && s <= ReservationStatusExpired
}

func (s ReservationStatus) IsFinal() bool {
	return s == ReservationStatusConfirmed || s == ReservationStatusReleased || s == ReservationStatusExpired
}

type ReservationItem struct {
	SKUID    uuid.UUID
	Quantity int64
}

type Reservation struct {
	ID        uuid.UUID
	Status    ReservationStatus
	Items     []ReservationItem
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ReservationRepository interface {
	Create(ctx context.Context, reservation *Reservation) error
	FindByID(ctx context.Context, id uuid.UUID) (*Reservation, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status ReservationStatus) error
	FindExpiredPending(ctx context.Context, limit int) ([]*Reservation, error)
	BatchUpdateExpired(ctx context.Context, ids []uuid.UUID) error
}

func NewReservation(items []ReservationItem, ttl time.Duration) (*Reservation, error) {
	if len(items) == 0 {
		return nil, ErrInvalidQuantity
	}

	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, ErrInvalidQuantity
		}
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}

	return &Reservation{
		ID:        id,
		Status:    ReservationStatusPending,
		Items:     items,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *Reservation) IsExpired() bool {
	return time.Now().UTC().After(r.ExpiresAt)
}

func (r *Reservation) IsPending() bool {
	return r.Status == ReservationStatusPending
}

func (r *Reservation) CanConfirm() bool {
	return r.Status == ReservationStatusPending && !r.IsExpired()
}

func (r *Reservation) CanRelease() bool {
	return r.Status == ReservationStatusPending
}

func (r *Reservation) Confirm() error {
	if !r.CanConfirm() {
		if r.IsExpired() {
			return ErrReservationExpired
		}
		return ErrReservationNotPending
	}
	r.Status = ReservationStatusConfirmed
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Reservation) Release() error {
	if !r.CanRelease() {
		return ErrReservationNotPending
	}
	r.Status = ReservationStatusReleased
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Reservation) Expire() error {
	if r.Status != ReservationStatusPending {
		return ErrReservationNotPending
	}
	r.Status = ReservationStatusExpired
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Reservation) TotalQuantity() int64 {
	var total int64
	for _, item := range r.Items {
		total += item.Quantity
	}
	return total
}

func (r *Reservation) GetItemBySKUID(skuID uuid.UUID) *ReservationItem {
	for i := range r.Items {
		if r.Items[i].SKUID == skuID {
			return &r.Items[i]
		}
	}
	return nil
}
