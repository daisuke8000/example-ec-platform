package handler_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"

	userv1 "github.com/daisuke8000/example-ec-platform/gen/user/v1"
	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"

	"github.com/daisuke8000/example-ec-platform/bff/internal/authz"
	"github.com/daisuke8000/example-ec-platform/bff/internal/handler"
	pkgmw "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
)

type mockUserServiceClient struct {
	userv1connect.UserServiceClient
	createUserFn     func(context.Context, *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error)
	getUserFn        func(context.Context, *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error)
	updateUserFn     func(context.Context, *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error)
	deleteUserFn     func(context.Context, *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error)
	verifyPasswordFn func(context.Context, *connect.Request[userv1.VerifyPasswordRequest]) (*connect.Response[userv1.VerifyPasswordResponse], error)
}

func (m *mockUserServiceClient) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (m *mockUserServiceClient) GetUser(ctx context.Context, req *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (m *mockUserServiceClient) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (m *mockUserServiceClient) DeleteUser(ctx context.Context, req *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error) {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (m *mockUserServiceClient) VerifyPassword(ctx context.Context, req *connect.Request[userv1.VerifyPasswordRequest]) (*connect.Response[userv1.VerifyPasswordResponse], error) {
	if m.verifyPasswordFn != nil {
		return m.verifyPasswordFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestUserServiceProxy_CreateUser(t *testing.T) {
	mockClient := &mockUserServiceClient{
		createUserFn: func(_ context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
			return connect.NewResponse(&userv1.CreateUserResponse{
				User: &userv1.User{
					Id:    "user-123",
					Email: req.Msg.GetEmail(),
				},
			}), nil
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	req := connect.NewRequest(&userv1.CreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
	})

	resp, err := proxy.CreateUser(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.GetUser().GetEmail() != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", resp.Msg.GetUser().GetEmail())
	}
}

func TestUserServiceProxy_GetUser_Authorized(t *testing.T) {
	userID := "user-123"

	mockClient := &mockUserServiceClient{
		getUserFn: func(_ context.Context, _ *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
			return connect.NewResponse(&userv1.GetUserResponse{
				User: &userv1.User{
					Id:    userID,
					Email: "test@example.com",
				},
			}), nil
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	// User accessing their own data
	ctx := pkgmw.WithUserID(context.Background(), userID)
	req := connect.NewRequest(&userv1.GetUserRequest{Id: userID})

	resp, err := proxy.GetUser(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.GetUser().GetId() != userID {
		t.Errorf("expected user ID %s, got %s", userID, resp.Msg.GetUser().GetId())
	}
}

func TestUserServiceProxy_GetUser_Unauthorized(t *testing.T) {
	mockClient := &mockUserServiceClient{}
	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	// User trying to access another user's data
	ctx := pkgmw.WithUserID(context.Background(), "user-123")
	req := connect.NewRequest(&userv1.GetUserRequest{Id: "other-user"})

	_, err := proxy.GetUser(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodePermissionDenied {
		t.Errorf("expected CodePermissionDenied, got %v", connectErr.Code())
	}
}

func TestUserServiceProxy_GetUser_Unauthenticated(t *testing.T) {
	mockClient := &mockUserServiceClient{}
	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	// No user in context
	req := connect.NewRequest(&userv1.GetUserRequest{Id: "user-123"})

	_, err := proxy.GetUser(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
	}
}

func TestUserServiceProxy_GetUser_AdminBypass(t *testing.T) {
	mockClient := &mockUserServiceClient{
		getUserFn: func(_ context.Context, _ *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
			return connect.NewResponse(&userv1.GetUserResponse{
				User: &userv1.User{
					Id:    "other-user",
					Email: "other@example.com",
				},
			}), nil
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	// Admin accessing another user's data
	ctx := pkgmw.WithUserID(context.Background(), "admin-user")
	ctx = pkgmw.WithScopes(ctx, "admin")
	req := connect.NewRequest(&userv1.GetUserRequest{Id: "other-user"})

	resp, err := proxy.GetUser(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.GetUser().GetId() != "other-user" {
		t.Errorf("expected other-user, got %s", resp.Msg.GetUser().GetId())
	}
}

func TestUserServiceProxy_VerifyPassword_Blocked(t *testing.T) {
	mockClient := &mockUserServiceClient{}
	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	req := connect.NewRequest(&userv1.VerifyPasswordRequest{
		Email:    "test@example.com",
		Password: "password",
	})

	_, err := proxy.VerifyPassword(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodePermissionDenied {
		t.Errorf("expected CodePermissionDenied, got %v", connectErr.Code())
	}
}

func TestUserServiceProxy_UpdateUser_Authorized(t *testing.T) {
	userID := "user-123"

	mockClient := &mockUserServiceClient{
		updateUserFn: func(_ context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
			return connect.NewResponse(&userv1.UpdateUserResponse{
				User: &userv1.User{
					Id:    req.Msg.GetId(),
					Email: "updated@example.com",
				},
			}), nil
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	ctx := pkgmw.WithUserID(context.Background(), userID)
	email := "updated@example.com"
	req := connect.NewRequest(&userv1.UpdateUserRequest{
		Id:    userID,
		Email: &email,
	})

	resp, err := proxy.UpdateUser(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.GetUser().GetEmail() != "updated@example.com" {
		t.Errorf("expected updated@example.com, got %s", resp.Msg.GetUser().GetEmail())
	}
}

func TestUserServiceProxy_DeleteUser_Authorized(t *testing.T) {
	userID := "user-123"

	mockClient := &mockUserServiceClient{
		deleteUserFn: func(_ context.Context, _ *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error) {
			return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	ctx := pkgmw.WithUserID(context.Background(), userID)
	req := connect.NewRequest(&userv1.DeleteUserRequest{Id: userID})

	_, err := proxy.DeleteUser(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserServiceProxy_HandleError_Internal(t *testing.T) {
	mockClient := &mockUserServiceClient{
		getUserFn: func(_ context.Context, _ *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("database error"))
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	userID := "user-123"
	ctx := pkgmw.WithUserID(context.Background(), userID)
	req := connect.NewRequest(&userv1.GetUserRequest{Id: userID})

	_, err := proxy.GetUser(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	// Internal errors should be sanitized
	if connectErr.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", connectErr.Code())
	}

	if connectErr.Message() != "internal server error" {
		t.Errorf("expected sanitized message, got %s", connectErr.Message())
	}
}

func TestUserServiceProxy_HandleError_NotFound(t *testing.T) {
	mockClient := &mockUserServiceClient{
		getUserFn: func(_ context.Context, _ *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
		},
	}

	proxy := handler.NewUserServiceProxy(mockClient, authz.NewAuthorizer(), newTestLogger())

	userID := "user-123"
	ctx := pkgmw.WithUserID(context.Background(), userID)
	req := connect.NewRequest(&userv1.GetUserRequest{Id: userID})

	_, err := proxy.GetUser(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	// NotFound should pass through
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}
