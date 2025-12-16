// Package connect provides the Connect-go transport layer for user service operations.
package connect

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/daisuke8000/example-ec-platform/gen/user/v1"
	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/usecase"
)

// UserServiceHandler implements the Connect-go UserServiceHandler interface.
type UserServiceHandler struct {
	userv1connect.UnimplementedUserServiceHandler
	uc     usecase.UserUseCase
	logger *slog.Logger
}

// NewUserServiceHandler creates a new Connect-go handler for user operations.
func NewUserServiceHandler(uc usecase.UserUseCase, logger *slog.Logger) *UserServiceHandler {
	return &UserServiceHandler{
		uc:     uc,
		logger: logger,
	}
}

// CreateUser handles user registration requests.
func (h *UserServiceHandler) CreateUser(
	ctx context.Context,
	req *connect.Request[v1.CreateUserRequest],
) (*connect.Response[v1.CreateUserResponse], error) {
	h.logger.InfoContext(ctx, "CreateUser request received",
		slog.String("email", req.Msg.GetEmail()),
	)

	input := usecase.CreateUserInput{
		Email:    req.Msg.GetEmail(),
		Password: req.Msg.GetPassword(),
		Name:     req.Msg.Name,
	}

	user, err := h.uc.CreateUser(ctx, input)
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateUser failed",
			slog.String("email", req.Msg.GetEmail()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "CreateUser succeeded",
		slog.String("user_id", user.ID.String()),
	)

	return connect.NewResponse(&v1.CreateUserResponse{
		User: domainUserToProto(user),
	}), nil
}

// GetUser handles user retrieval requests.
func (h *UserServiceHandler) GetUser(
	ctx context.Context,
	req *connect.Request[v1.GetUserRequest],
) (*connect.Response[v1.GetUserResponse], error) {
	h.logger.InfoContext(ctx, "GetUser request received",
		slog.String("user_id", req.Msg.GetId()),
	)

	id, err := uuid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("invalid user ID format"))
	}

	user, err := h.uc.GetUser(ctx, id)
	if err != nil {
		h.logger.ErrorContext(ctx, "GetUser failed",
			slog.String("user_id", req.Msg.GetId()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	return connect.NewResponse(&v1.GetUserResponse{
		User: domainUserToProto(user),
	}), nil
}

// UpdateUser handles user profile update requests.
func (h *UserServiceHandler) UpdateUser(
	ctx context.Context,
	req *connect.Request[v1.UpdateUserRequest],
) (*connect.Response[v1.UpdateUserResponse], error) {
	h.logger.InfoContext(ctx, "UpdateUser request received",
		slog.String("user_id", req.Msg.GetId()),
	)

	id, err := uuid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("invalid user ID format"))
	}

	input := usecase.UpdateUserInput{
		Email: req.Msg.Email,
		Name:  req.Msg.Name,
	}

	user, err := h.uc.UpdateUser(ctx, id, input)
	if err != nil {
		h.logger.ErrorContext(ctx, "UpdateUser failed",
			slog.String("user_id", req.Msg.GetId()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "UpdateUser succeeded",
		slog.String("user_id", user.ID.String()),
	)

	return connect.NewResponse(&v1.UpdateUserResponse{
		User: domainUserToProto(user),
	}), nil
}

// DeleteUser handles user deletion requests.
func (h *UserServiceHandler) DeleteUser(
	ctx context.Context,
	req *connect.Request[v1.DeleteUserRequest],
) (*connect.Response[v1.DeleteUserResponse], error) {
	h.logger.InfoContext(ctx, "DeleteUser request received",
		slog.String("user_id", req.Msg.GetId()),
	)

	id, err := uuid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("invalid user ID format"))
	}

	if err := h.uc.DeleteUser(ctx, id); err != nil {
		h.logger.ErrorContext(ctx, "DeleteUser failed",
			slog.String("user_id", req.Msg.GetId()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "DeleteUser succeeded",
		slog.String("user_id", req.Msg.GetId()),
	)

	return connect.NewResponse(&v1.DeleteUserResponse{}), nil
}

// VerifyPassword handles credential verification requests.
// Security: Returns the same error for both non-existent users and invalid passwords
// to prevent account enumeration attacks (OWASP A07:2021).
func (h *UserServiceHandler) VerifyPassword(
	ctx context.Context,
	req *connect.Request[v1.VerifyPasswordRequest],
) (*connect.Response[v1.VerifyPasswordResponse], error) {
	// Note: We intentionally don't log the email here to avoid information leakage
	h.logger.InfoContext(ctx, "VerifyPassword request received")

	user, err := h.uc.VerifyPassword(ctx, req.Msg.GetEmail(), req.Msg.GetPassword())
	if err != nil {
		// Log at debug level to avoid flooding logs with failed login attempts
		h.logger.DebugContext(ctx, "VerifyPassword failed",
			slog.String("error", err.Error()),
		)
		// Security: Return unified error for both UserNotFound and InvalidCredentials
		// to prevent account enumeration attacks
		if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrInvalidCredentials) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid email or password"))
		}
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "VerifyPassword succeeded",
		slog.String("user_id", user.ID.String()),
	)

	return connect.NewResponse(&v1.VerifyPasswordResponse{
		UserId: user.ID.String(),
	}), nil
}

// mapDomainError converts domain errors to Connect errors.
func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("email already exists"))
	case errors.Is(err, domain.ErrInvalidCredentials):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("invalid email or password"))
	case errors.Is(err, domain.ErrInvalidEmail):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid email format"))
	case errors.Is(err, domain.ErrPasswordTooShort):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("password must be at least 8 characters"))
	case errors.Is(err, domain.ErrEmptyEmail):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("email cannot be empty"))
	case errors.Is(err, domain.ErrEmptyPassword):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("password cannot be empty"))
	case errors.Is(err, domain.ErrNameTooLong):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("name is too long"))
	default:
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
}

func domainUserToProto(user *domain.User) *v1.User {
	return &v1.User{
		Id:        user.ID.String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: timestamppb.New(user.CreatedAt),
		UpdatedAt: timestamppb.New(user.UpdatedAt),
	}
}
