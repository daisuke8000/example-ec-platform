package connect

import (
	"errors"

	"connectrpc.com/connect"

	"github.com/daisuke8000/example-ec-platform/services/product/internal/domain"
)

func toConnectError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrProductNotFound),
		errors.Is(err, domain.ErrSKUNotFound),
		errors.Is(err, domain.ErrCategoryNotFound),
		errors.Is(err, domain.ErrInventoryNotFound),
		errors.Is(err, domain.ErrReservationNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	case errors.Is(err, domain.ErrSKUCodeAlreadyExists),
		errors.Is(err, domain.ErrCategoryNameExists):
		return connect.NewError(connect.CodeAlreadyExists, err)

	case errors.Is(err, domain.ErrInsufficientStock):
		return connect.NewError(connect.CodeResourceExhausted, err)

	case errors.Is(err, domain.ErrOptimisticLockConflict),
		errors.Is(err, domain.ErrReservationExpired):
		return connect.NewError(connect.CodeAborted, err)

	case errors.Is(err, domain.ErrReservationNotPending),
		errors.Is(err, domain.ErrInvalidProductStatus),
		errors.Is(err, domain.ErrInvalidReservationStatus):
		return connect.NewError(connect.CodeFailedPrecondition, err)

	case errors.Is(err, domain.ErrInvalidQuantity),
		errors.Is(err, domain.ErrBatchSizeExceeded),
		errors.Is(err, domain.ErrEmptyProductName),
		errors.Is(err, domain.ErrProductNameTooLong),
		errors.Is(err, domain.ErrEmptySKUCode),
		errors.Is(err, domain.ErrSKUCodeTooLong),
		errors.Is(err, domain.ErrEmptyCategoryName),
		errors.Is(err, domain.ErrCategoryNameTooLong),
		errors.Is(err, domain.ErrInvalidPrice):
		return connect.NewError(connect.CodeInvalidArgument, err)

	case errors.Is(err, domain.ErrIdempotencyKeyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)

	default:
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
}
