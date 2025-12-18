package connect

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	productv1 "github.com/daisuke8000/example-ec-platform/gen/product/v1"
	"github.com/daisuke8000/example-ec-platform/gen/product/v1/productv1connect"
	"github.com/daisuke8000/example-ec-platform/services/product/internal/usecase"
)

type InventoryHandler struct {
	productv1connect.UnimplementedInventoryServiceHandler
	inventoryUC usecase.InventoryUseCase
}

func NewInventoryHandler(inventoryUC usecase.InventoryUseCase) *InventoryHandler {
	return &InventoryHandler{inventoryUC: inventoryUC}
}

func (h *InventoryHandler) GetInventory(
	ctx context.Context,
	req *connect.Request[productv1.GetInventoryRequest],
) (*connect.Response[productv1.GetInventoryResponse], error) {
	skuID, err := uuid.Parse(req.Msg.SkuId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	inv, err := h.inventoryUC.GetInventory(ctx, skuID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.GetInventoryResponse{
		Inventory: toProtoInventory(inv),
	}), nil
}

func (h *InventoryHandler) UpdateInventory(
	ctx context.Context,
	req *connect.Request[productv1.UpdateInventoryRequest],
) (*connect.Response[productv1.UpdateInventoryResponse], error) {
	skuID, err := uuid.Parse(req.Msg.SkuId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.inventoryUC.UpdateInventory(ctx, skuID, req.Msg.Quantity)
	if err != nil {
		return nil, toConnectError(err)
	}

	inv, err := h.inventoryUC.GetInventory(ctx, skuID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.UpdateInventoryResponse{
		Inventory: toProtoInventory(inv),
	}), nil
}

func (h *InventoryHandler) BatchReserveInventory(
	ctx context.Context,
	req *connect.Request[productv1.BatchReserveInventoryRequest],
) (*connect.Response[productv1.BatchReserveInventoryResponse], error) {
	items := make([]usecase.ReserveItem, len(req.Msg.Items))
	for i, item := range req.Msg.Items {
		skuID, err := uuid.Parse(item.SkuId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		items[i] = usecase.ReserveItem{
			SKUID:    skuID,
			Quantity: item.Quantity,
		}
	}

	input := usecase.BatchReserveInput{
		Items:          items,
		IdempotencyKey: req.Msg.IdempotencyKey,
	}

	reservation, err := h.inventoryUC.BatchReserveInventory(ctx, input)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.BatchReserveInventoryResponse{
		Reservation: toProtoReservation(reservation),
	}), nil
}

func (h *InventoryHandler) ConfirmReservation(
	ctx context.Context,
	req *connect.Request[productv1.ConfirmReservationRequest],
) (*connect.Response[productv1.ConfirmReservationResponse], error) {
	reservationID, err := uuid.Parse(req.Msg.ReservationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.inventoryUC.ConfirmReservation(ctx, reservationID, req.Msg.IdempotencyKey)
	if err != nil {
		return nil, toConnectError(err)
	}

	reservation, err := h.inventoryUC.GetReservationStatus(ctx, reservationID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.ConfirmReservationResponse{
		Reservation: toProtoReservation(reservation),
	}), nil
}

func (h *InventoryHandler) ReleaseInventory(
	ctx context.Context,
	req *connect.Request[productv1.ReleaseInventoryRequest],
) (*connect.Response[productv1.ReleaseInventoryResponse], error) {
	reservationID, err := uuid.Parse(req.Msg.ReservationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = h.inventoryUC.ReleaseReservation(ctx, reservationID, req.Msg.IdempotencyKey)
	if err != nil {
		return nil, toConnectError(err)
	}

	reservation, err := h.inventoryUC.GetReservationStatus(ctx, reservationID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.ReleaseInventoryResponse{
		Reservation: toProtoReservation(reservation),
	}), nil
}

func (h *InventoryHandler) GetReservationStatus(
	ctx context.Context,
	req *connect.Request[productv1.GetReservationStatusRequest],
) (*connect.Response[productv1.GetReservationStatusResponse], error) {
	reservationID, err := uuid.Parse(req.Msg.ReservationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	reservation, err := h.inventoryUC.GetReservationStatus(ctx, reservationID)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&productv1.GetReservationStatusResponse{
		Reservation: toProtoReservation(reservation),
	}), nil
}
