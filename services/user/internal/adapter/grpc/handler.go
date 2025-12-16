// Package grpc provides the gRPC transport layer for user service operations.
package grpc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/daisuke8000/example-ec-platform/gen/user/v1"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/usecase"
)

// UserServiceHandler implements the gRPC UserService interface.
type UserServiceHandler struct {
	pb.UnimplementedUserServiceServer
	uc     usecase.UserUseCase
	logger *slog.Logger
}

// NewUserServiceHandler creates a new gRPC handler for user operations.
func NewUserServiceHandler(uc usecase.UserUseCase, logger *slog.Logger) *UserServiceHandler {
	return &UserServiceHandler{
		uc:     uc,
		logger: logger,
	}
}

// CreateUser handles user registration requests.
func (h *UserServiceHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	h.logger.InfoContext(ctx, "CreateUser request received",
		slog.String("email", req.GetEmail()),
	)

	input := usecase.CreateUserInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
		Name:     req.Name,
	}

	user, err := h.uc.CreateUser(ctx, input)
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateUser failed",
			slog.String("email", req.GetEmail()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "CreateUser succeeded",
		slog.String("user_id", user.ID.String()),
	)

	return &pb.CreateUserResponse{
		User: domainUserToProto(user),
	}, nil
}

// GetUser handles user retrieval requests.
func (h *UserServiceHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	h.logger.InfoContext(ctx, "GetUser request received",
		slog.String("user_id", req.GetId()),
	)

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format: %v", err)
	}

	user, err := h.uc.GetUser(ctx, id)
	if err != nil {
		h.logger.ErrorContext(ctx, "GetUser failed",
			slog.String("user_id", req.GetId()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	return &pb.GetUserResponse{
		User: domainUserToProto(user),
	}, nil
}

// UpdateUser handles user profile update requests.
func (h *UserServiceHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	h.logger.InfoContext(ctx, "UpdateUser request received",
		slog.String("user_id", req.GetId()),
	)

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format: %v", err)
	}

	input := usecase.UpdateUserInput{
		Email: req.Email,
		Name:  req.Name,
	}

	user, err := h.uc.UpdateUser(ctx, id, input)
	if err != nil {
		h.logger.ErrorContext(ctx, "UpdateUser failed",
			slog.String("user_id", req.GetId()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "UpdateUser succeeded",
		slog.String("user_id", user.ID.String()),
	)

	return &pb.UpdateUserResponse{
		User: domainUserToProto(user),
	}, nil
}

// DeleteUser handles user deletion requests.
func (h *UserServiceHandler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	h.logger.InfoContext(ctx, "DeleteUser request received",
		slog.String("user_id", req.GetId()),
	)

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID format: %v", err)
	}

	if err := h.uc.DeleteUser(ctx, id); err != nil {
		h.logger.ErrorContext(ctx, "DeleteUser failed",
			slog.String("user_id", req.GetId()),
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "DeleteUser succeeded",
		slog.String("user_id", req.GetId()),
	)

	return &pb.DeleteUserResponse{}, nil
}

// VerifyPassword handles credential verification requests.
func (h *UserServiceHandler) VerifyPassword(ctx context.Context, req *pb.VerifyPasswordRequest) (*pb.VerifyPasswordResponse, error) {
	// Note: We intentionally don't log the email here to avoid information leakage
	h.logger.InfoContext(ctx, "VerifyPassword request received")

	user, err := h.uc.VerifyPassword(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		// Log at debug level to avoid flooding logs with failed login attempts
		h.logger.DebugContext(ctx, "VerifyPassword failed",
			slog.String("error", err.Error()),
		)
		return nil, mapDomainError(err)
	}

	h.logger.InfoContext(ctx, "VerifyPassword succeeded",
		slog.String("user_id", user.ID.String()),
	)

	return &pb.VerifyPasswordResponse{
		UserId: user.ID.String(),
	}, nil
}

// mapDomainError converts domain errors to gRPC status errors.
func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, "email already exists")
	case errors.Is(err, domain.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "invalid email or password")
	case errors.Is(err, domain.ErrInvalidEmail):
		return status.Error(codes.InvalidArgument, "invalid email format")
	case errors.Is(err, domain.ErrPasswordTooShort):
		return status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	case errors.Is(err, domain.ErrEmptyEmail):
		return status.Error(codes.InvalidArgument, "email cannot be empty")
	case errors.Is(err, domain.ErrEmptyPassword):
		return status.Error(codes.InvalidArgument, "password cannot be empty")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// domainUserToProto converts a domain User to a protobuf User message.
// Note: password_hash is intentionally excluded from the response.
func domainUserToProto(user *domain.User) *pb.User {
	return &pb.User{
		Id:        user.ID.String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
