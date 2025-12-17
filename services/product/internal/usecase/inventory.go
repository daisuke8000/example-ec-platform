package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type InventoryUseCase interface {
	GetInventory(ctx context.Context, skuID uuid.UUID) (*domain.Inventory, error)
	UpdateInventory(ctx context.Context, skuID uuid.UUID, quantity int64) error
	BatchReserveInventory(ctx context.Context, input BatchReserveInput) (*domain.Reservation, error)
	ConfirmReservation(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error
	ReleaseReservation(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error
	GetReservationStatus(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error)
}

type BatchReserveInput struct {
	Items          []ReserveItem
	IdempotencyKey string
	TTL            time.Duration
}

type ReserveItem struct {
	SKUID    uuid.UUID
	Quantity int64
}

type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error)
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

type TxManager interface {
	DoWithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error
}

type TxInventoryRepository interface {
	domain.InventoryRepository
	ReserveWithTx(ctx context.Context, tx pgx.Tx, skuID uuid.UUID, amount int64) error
}

type TxReservationRepository interface {
	domain.ReservationRepository
	CreateWithTx(ctx context.Context, tx pgx.Tx, reservation *domain.Reservation) error
}

type inventoryUseCase struct {
	inventoryRepo   TxInventoryRepository
	reservationRepo TxReservationRepository
	idempotency     IdempotencyStore
	txManager       TxManager
	maxBatchSize    int
	defaultTTL      time.Duration
	idempotencyTTL  time.Duration
}

func NewInventoryUseCase(
	inventoryRepo TxInventoryRepository,
	reservationRepo TxReservationRepository,
	idempotency IdempotencyStore,
	txManager TxManager,
	maxBatchSize int,
	defaultTTL time.Duration,
	idempotencyTTL time.Duration,
) InventoryUseCase {
	return &inventoryUseCase{
		inventoryRepo:   inventoryRepo,
		reservationRepo: reservationRepo,
		idempotency:     idempotency,
		txManager:       txManager,
		maxBatchSize:    maxBatchSize,
		defaultTTL:      defaultTTL,
		idempotencyTTL:  idempotencyTTL,
	}
}

func (uc *inventoryUseCase) GetInventory(ctx context.Context, skuID uuid.UUID) (*domain.Inventory, error) {
	return uc.inventoryRepo.FindBySKUID(ctx, skuID)
}

func (uc *inventoryUseCase) UpdateInventory(ctx context.Context, skuID uuid.UUID, quantity int64) error {
	return uc.inventoryRepo.UpdateQuantity(ctx, skuID, quantity)
}

func (uc *inventoryUseCase) BatchReserveInventory(ctx context.Context, input BatchReserveInput) (*domain.Reservation, error) {
	if len(input.Items) == 0 {
		return nil, domain.ErrInvalidQuantity
	}
	if len(input.Items) > uc.maxBatchSize {
		return nil, domain.ErrBatchSizeExceeded
	}

	var lockAcquired bool
	if input.IdempotencyKey != "" {
		locked, err := uc.idempotency.SetNX(ctx, input.IdempotencyKey, "processing", uc.idempotencyTTL)
		if err != nil {
			return nil, err
		}
		if !locked {
			existingID, err := uc.idempotency.Get(ctx, input.IdempotencyKey)
			if err != nil {
				return nil, err
			}
			if existingID == "processing" {
				return nil, domain.ErrIdempotencyKeyExists
			}
			id, parseErr := uuid.Parse(existingID)
			if parseErr == nil {
				return uc.reservationRepo.FindByID(ctx, id)
			}
			return nil, domain.ErrIdempotencyKeyExists
		}
		lockAcquired = true
	}

	var committed bool
	defer func() {
		if lockAcquired && !committed {
			_ = uc.idempotency.Del(context.Background(), input.IdempotencyKey)
		}
	}()

	sortedItems := make([]ReserveItem, len(input.Items))
	copy(sortedItems, input.Items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].SKUID.String() < sortedItems[j].SKUID.String()
	})

	ttl := input.TTL
	if ttl == 0 {
		ttl = uc.defaultTTL
	}

	reservationItems := make([]domain.ReservationItem, len(sortedItems))
	for i, item := range sortedItems {
		reservationItems[i] = domain.ReservationItem{
			SKUID:    item.SKUID,
			Quantity: item.Quantity,
		}
	}

	reservation, err := domain.NewReservation(reservationItems, ttl)
	if err != nil {
		return nil, err
	}

	err = uc.txManager.DoWithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		for _, item := range sortedItems {
			if err := uc.inventoryRepo.ReserveWithTx(ctx, tx, item.SKUID, item.Quantity); err != nil {
				return err
			}
		}
		return uc.reservationRepo.CreateWithTx(ctx, tx, reservation)
	})

	if err != nil {
		return nil, err
	}

	committed = true
	if input.IdempotencyKey != "" {
		_ = uc.idempotency.Set(ctx, input.IdempotencyKey, reservation.ID.String(), uc.idempotencyTTL)
	}

	return reservation, nil
}

func (uc *inventoryUseCase) ConfirmReservation(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error {
	if idempotencyKey != "" {
		if _, err := uc.idempotency.Get(ctx, "confirm:"+idempotencyKey); err == nil {
			return nil
		}
	}

	reservation, err := uc.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return err
	}

	if err := reservation.Confirm(); err != nil {
		return err
	}

	for _, item := range reservation.Items {
		if err := uc.inventoryRepo.ConfirmReservation(ctx, item.SKUID, item.Quantity); err != nil {
			return err
		}
	}

	if err := uc.reservationRepo.UpdateStatus(ctx, reservationID, domain.ReservationStatusConfirmed); err != nil {
		return err
	}

	if idempotencyKey != "" {
		_ = uc.idempotency.Set(ctx, "confirm:"+idempotencyKey, "done", uc.idempotencyTTL)
	}

	return nil
}

func (uc *inventoryUseCase) ReleaseReservation(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error {
	if idempotencyKey != "" {
		if _, err := uc.idempotency.Get(ctx, "release:"+idempotencyKey); err == nil {
			return nil
		}
	}

	reservation, err := uc.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return err
	}

	if err := reservation.Release(); err != nil {
		return err
	}

	for _, item := range reservation.Items {
		if err := uc.inventoryRepo.ReleaseReservation(ctx, item.SKUID, item.Quantity); err != nil {
			return err
		}
	}

	if err := uc.reservationRepo.UpdateStatus(ctx, reservationID, domain.ReservationStatusReleased); err != nil {
		return err
	}

	if idempotencyKey != "" {
		_ = uc.idempotency.Set(ctx, "release:"+idempotencyKey, "done", uc.idempotencyTTL)
	}

	return nil
}

func (uc *inventoryUseCase) GetReservationStatus(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error) {
	return uc.reservationRepo.FindByID(ctx, reservationID)
}
