package handler

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	userv1 "github.com/daisuke8000/example-ec-platform/gen/user/v1"
	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"
	pkgmw "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"

	"github.com/daisuke8000/example-ec-platform/bff/internal/authz"
)

var _ userv1connect.UserServiceHandler = (*UserServiceProxy)(nil)

type UserServiceProxy struct {
	userv1connect.UnimplementedUserServiceHandler
	client     userv1connect.UserServiceClient
	authorizer *authz.Authorizer
	logger     *slog.Logger
}

func NewUserServiceProxy(
	client userv1connect.UserServiceClient,
	authorizer *authz.Authorizer,
	logger *slog.Logger,
) *UserServiceProxy {
	return &UserServiceProxy{
		client:     client,
		authorizer: authorizer,
		logger:     logger,
	}
}

// CreateUser is a public endpoint for user registration.
func (p *UserServiceProxy) CreateUser(
	ctx context.Context,
	req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.CreateUserResponse], error) {
	resp, err := p.client.CreateUser(ctx, req)
	if err != nil {
		return nil, p.handleError(ctx, "CreateUser", err)
	}
	return resp, nil
}

func (p *UserServiceProxy) GetUser(
	ctx context.Context,
	req *connect.Request[userv1.GetUserRequest],
) (*connect.Response[userv1.GetUserResponse], error) {
	if err := p.authorizer.CanAccessUser(ctx, req.Msg.GetId()); err != nil {
		p.logAuthzError(ctx, "GetUser", req.Msg.GetId(), err)
		return nil, err
	}

	resp, err := p.client.GetUser(ctx, req)
	if err != nil {
		return nil, p.handleError(ctx, "GetUser", err)
	}
	return resp, nil
}

func (p *UserServiceProxy) UpdateUser(
	ctx context.Context,
	req *connect.Request[userv1.UpdateUserRequest],
) (*connect.Response[userv1.UpdateUserResponse], error) {
	if err := p.authorizer.CanAccessUser(ctx, req.Msg.GetId()); err != nil {
		p.logAuthzError(ctx, "UpdateUser", req.Msg.GetId(), err)
		return nil, err
	}

	resp, err := p.client.UpdateUser(ctx, req)
	if err != nil {
		return nil, p.handleError(ctx, "UpdateUser", err)
	}
	return resp, nil
}

func (p *UserServiceProxy) DeleteUser(
	ctx context.Context,
	req *connect.Request[userv1.DeleteUserRequest],
) (*connect.Response[userv1.DeleteUserResponse], error) {
	if err := p.authorizer.CanAccessUser(ctx, req.Msg.GetId()); err != nil {
		p.logAuthzError(ctx, "DeleteUser", req.Msg.GetId(), err)
		return nil, err
	}

	resp, err := p.client.DeleteUser(ctx, req)
	if err != nil {
		return nil, p.handleError(ctx, "DeleteUser", err)
	}
	return resp, nil
}

// VerifyPassword is blocked at BFF level (internal use only via Hydra Login Provider).
func (p *UserServiceProxy) VerifyPassword(
	ctx context.Context,
	_ *connect.Request[userv1.VerifyPasswordRequest],
) (*connect.Response[userv1.VerifyPasswordResponse], error) {
	return nil, connect.NewError(connect.CodePermissionDenied,
		errors.New("this endpoint is not available via BFF"))
}

func (p *UserServiceProxy) handleError(ctx context.Context, method string, err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		if connectErr.Code() == connect.CodeInternal {
			p.logger.ErrorContext(ctx, "internal error from user service",
				slog.String("method", method),
				slog.String("error", err.Error()),
			)
			return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
		return connectErr
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, errors.New("request timeout"))
	}

	p.logger.ErrorContext(ctx, "unexpected error from user service",
		slog.String("method", method),
		slog.String("error", err.Error()),
	)
	return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
}

func (p *UserServiceProxy) logAuthzError(ctx context.Context, method, targetUserID string, err error) {
	p.logger.WarnContext(ctx, "authorization denied",
		slog.String("method", method),
		slog.String("current_user_id", pkgmw.GetUserID(ctx)),
		slog.String("target_user_id", targetUserID),
		slog.String("reason", err.Error()),
	)
}
