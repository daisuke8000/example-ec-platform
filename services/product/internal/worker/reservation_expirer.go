package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type ReservationExpirer struct {
	txManager       TxManager
	reservationRepo domain.ReservationRepository
	inventoryRepo   domain.InventoryRepository
	logger          *slog.Logger
	interval        time.Duration
	batchSize       int
}

func NewReservationExpirer(
	txManager TxManager,
	reservationRepo domain.ReservationRepository,
	inventoryRepo domain.InventoryRepository,
	logger *slog.Logger,
	interval time.Duration,
	batchSize int,
) *ReservationExpirer {
	return &ReservationExpirer{
		txManager:       txManager,
		reservationRepo: reservationRepo,
		inventoryRepo:   inventoryRepo,
		logger:          logger,
		interval:        interval,
		batchSize:       batchSize,
	}
}

func (w *ReservationExpirer) Start(ctx context.Context) {
	w.logger.Info("reservation expirer starting", "interval", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("reservation expirer shutting down")
			return
		case <-ticker.C:
			w.processExpired(ctx)
		}
	}
}

func (w *ReservationExpirer) processExpired(ctx context.Context) {
	reservations, err := w.reservationRepo.FindExpiredPending(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("failed to find expired reservations", "error", err)
		return
	}

	if len(reservations) == 0 {
		return
	}

	for _, res := range reservations {
		if ctx.Err() != nil {
			w.logger.Info("context cancelled, stopping process loop")
			return
		}

		logger := w.logger.With("reservation_id", res.ID)

		err := w.txManager.Do(ctx, func(txCtx context.Context) error {
			return w.expireReservation(txCtx, res)
		})

		if err != nil {
			logger.Error("failed to expire reservation", "error", err)
			continue
		}

		logger.Info("expired reservation successfully")
	}
}

func (w *ReservationExpirer) expireReservation(ctx context.Context, res *domain.Reservation) error {
	for _, item := range res.Items {
		if err := w.inventoryRepo.ReleaseReservation(ctx, item.SKUID, item.Quantity); err != nil {
			return err
		}
	}

	return w.reservationRepo.UpdateStatus(ctx, res.ID, domain.ReservationStatusExpired)
}
