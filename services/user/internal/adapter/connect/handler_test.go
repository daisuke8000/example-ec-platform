package connect

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	v1 "github.com/daisuke8000/example-ec-platform/gen/user/v1"
	"github.com/daisuke8000/example-ec-platform/gen/user/v1/userv1connect"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/usecase"
)

// mockUserUseCase is a test double for usecase.UserUseCase.
type mockUserUseCase struct {
	createUserFn     func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error)
	getUserFn        func(ctx context.Context, id uuid.UUID) (*domain.User, error)
	updateUserFn     func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error)
	deleteUserFn     func(ctx context.Context, id uuid.UUID) error
	verifyPasswordFn func(ctx context.Context, email, password string) (*domain.User, error)
}

func (m *mockUserUseCase) CreateUser(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, input)
	}
	return nil, nil
}

func (m *mockUserUseCase) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, id)
	}
	return nil, nil
}

func (m *mockUserUseCase) UpdateUser(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, id, input)
	}
	return nil, nil
}

func (m *mockUserUseCase) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, id)
	}
	return nil
}

func (m *mockUserUseCase) VerifyPassword(ctx context.Context, email, password string) (*domain.User, error) {
	if m.verifyPasswordFn != nil {
		return m.verifyPasswordFn(ctx, email, password)
	}
	return nil, nil
}

func newTestServer(uc *mockUserUseCase) (*httptest.Server, userv1connect.UserServiceClient) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewUserServiceHandler(uc, logger)

	mux := http.NewServeMux()
	path, h := userv1connect.NewUserServiceHandler(handler)
	mux.Handle(path, h)

	server := httptest.NewServer(mux)
	client := userv1connect.NewUserServiceClient(http.DefaultClient, server.URL)

	return server, client
}

func TestCreateUser(t *testing.T) {
	testUser := createTestUser()
	name := "Test User"

	tests := []struct {
		name     string
		req      *v1.CreateUserRequest
		mockFn   func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error)
		wantCode connect.Code
	}{
		{
			name: "creates user successfully",
			req: &v1.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     &name,
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return testUser, nil
			},
			wantCode: 0, // No error
		},
		{
			name: "returns already exists for duplicate email",
			req: &v1.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return nil, domain.ErrEmailAlreadyExists
			},
			wantCode: connect.CodeAlreadyExists,
		},
		{
			name: "returns invalid argument for invalid email",
			req: &v1.CreateUserRequest{
				Email:    "invalid",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return nil, domain.ErrInvalidEmail
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "returns invalid argument for short password",
			req: &v1.CreateUserRequest{
				Email:    "test@example.com",
				Password: "short",
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return nil, domain.ErrPasswordTooShort
			},
			wantCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{createUserFn: tt.mockFn}
			server, client := newTestServer(mock)
			defer server.Close()

			resp, err := client.CreateUser(context.Background(), connect.NewRequest(tt.req))

			if tt.wantCode == 0 {
				if err != nil {
					t.Errorf("CreateUser() error = %v, want nil", err)
					return
				}
				if resp.Msg.GetUser() == nil {
					t.Error("CreateUser() returned nil user")
				}
			} else {
				if err == nil {
					t.Error("CreateUser() expected error, got nil")
					return
				}
				var connectErr *connect.Error
				if !errors.As(err, &connectErr) {
					t.Errorf("CreateUser() error is not a Connect error: %v", err)
					return
				}
				if connectErr.Code() != tt.wantCode {
					t.Errorf("CreateUser() error code = %v, want %v", connectErr.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	testUser := createTestUser()

	tests := []struct {
		name     string
		req      *v1.GetUserRequest
		mockFn   func(ctx context.Context, id uuid.UUID) (*domain.User, error)
		wantCode connect.Code
	}{
		{
			name: "gets user successfully",
			req: &v1.GetUserRequest{
				Id: testUser.ID.String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
				return testUser, nil
			},
			wantCode: 0,
		},
		{
			name: "returns not found for non-existent user",
			req: &v1.GetUserRequest{
				Id: uuid.New().String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			wantCode: connect.CodeNotFound,
		},
		{
			name: "returns invalid argument for invalid UUID",
			req: &v1.GetUserRequest{
				Id: "invalid-uuid",
			},
			wantCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{getUserFn: tt.mockFn}
			server, client := newTestServer(mock)
			defer server.Close()

			resp, err := client.GetUser(context.Background(), connect.NewRequest(tt.req))

			if tt.wantCode == 0 {
				if err != nil {
					t.Errorf("GetUser() error = %v, want nil", err)
					return
				}
				if resp.Msg.GetUser() == nil {
					t.Error("GetUser() returned nil user")
				}
			} else {
				if err == nil {
					t.Error("GetUser() expected error, got nil")
					return
				}
				var connectErr *connect.Error
				if !errors.As(err, &connectErr) {
					t.Errorf("GetUser() error is not a Connect error: %v", err)
					return
				}
				if connectErr.Code() != tt.wantCode {
					t.Errorf("GetUser() error code = %v, want %v", connectErr.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	testUser := createTestUser()
	newEmail := "new@example.com"
	newName := "New Name"

	tests := []struct {
		name     string
		req      *v1.UpdateUserRequest
		mockFn   func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error)
		wantCode connect.Code
	}{
		{
			name: "updates user successfully",
			req: &v1.UpdateUserRequest{
				Id:    testUser.ID.String(),
				Email: &newEmail,
				Name:  &newName,
			},
			mockFn: func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
				testUser.Email = *input.Email
				testUser.Name = input.Name
				return testUser, nil
			},
			wantCode: 0,
		},
		{
			name: "returns not found for non-existent user",
			req: &v1.UpdateUserRequest{
				Id:   uuid.New().String(),
				Name: &newName,
			},
			mockFn: func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			wantCode: connect.CodeNotFound,
		},
		{
			name: "returns already exists for duplicate email",
			req: &v1.UpdateUserRequest{
				Id:    testUser.ID.String(),
				Email: &newEmail,
			},
			mockFn: func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
				return nil, domain.ErrEmailAlreadyExists
			},
			wantCode: connect.CodeAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{updateUserFn: tt.mockFn}
			server, client := newTestServer(mock)
			defer server.Close()

			resp, err := client.UpdateUser(context.Background(), connect.NewRequest(tt.req))

			if tt.wantCode == 0 {
				if err != nil {
					t.Errorf("UpdateUser() error = %v, want nil", err)
					return
				}
				if resp.Msg.GetUser() == nil {
					t.Error("UpdateUser() returned nil user")
				}
			} else {
				if err == nil {
					t.Error("UpdateUser() expected error, got nil")
					return
				}
				var connectErr *connect.Error
				if !errors.As(err, &connectErr) {
					t.Errorf("UpdateUser() error is not a Connect error: %v", err)
					return
				}
				if connectErr.Code() != tt.wantCode {
					t.Errorf("UpdateUser() error code = %v, want %v", connectErr.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	testUser := createTestUser()

	tests := []struct {
		name     string
		req      *v1.DeleteUserRequest
		mockFn   func(ctx context.Context, id uuid.UUID) error
		wantCode connect.Code
	}{
		{
			name: "deletes user successfully",
			req: &v1.DeleteUserRequest{
				Id: testUser.ID.String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
			wantCode: 0,
		},
		{
			name: "returns not found for non-existent user",
			req: &v1.DeleteUserRequest{
				Id: uuid.New().String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) error {
				return domain.ErrUserNotFound
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{deleteUserFn: tt.mockFn}
			server, client := newTestServer(mock)
			defer server.Close()

			_, err := client.DeleteUser(context.Background(), connect.NewRequest(tt.req))

			if tt.wantCode == 0 {
				if err != nil {
					t.Errorf("DeleteUser() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("DeleteUser() expected error, got nil")
					return
				}
				var connectErr *connect.Error
				if !errors.As(err, &connectErr) {
					t.Errorf("DeleteUser() error is not a Connect error: %v", err)
					return
				}
				if connectErr.Code() != tt.wantCode {
					t.Errorf("DeleteUser() error code = %v, want %v", connectErr.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	testUser := createTestUser()

	tests := []struct {
		name     string
		req      *v1.VerifyPasswordRequest
		mockFn   func(ctx context.Context, email, password string) (*domain.User, error)
		wantCode connect.Code
	}{
		{
			name: "verifies password successfully",
			req: &v1.VerifyPasswordRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, email, password string) (*domain.User, error) {
				return testUser, nil
			},
			wantCode: 0,
		},
		{
			name: "returns unauthenticated for invalid credentials",
			req: &v1.VerifyPasswordRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			mockFn: func(ctx context.Context, email, password string) (*domain.User, error) {
				return nil, domain.ErrInvalidCredentials
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "returns unauthenticated for non-existent user (prevents enumeration)",
			req: &v1.VerifyPasswordRequest{
				Email:    "nonexistent@example.com",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, email, password string) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			wantCode: connect.CodeUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{verifyPasswordFn: tt.mockFn}
			server, client := newTestServer(mock)
			defer server.Close()

			resp, err := client.VerifyPassword(context.Background(), connect.NewRequest(tt.req))

			if tt.wantCode == 0 {
				if err != nil {
					t.Errorf("VerifyPassword() error = %v, want nil", err)
					return
				}
				if resp.Msg.GetUserId() == "" {
					t.Error("VerifyPassword() returned empty user_id")
				}
			} else {
				if err == nil {
					t.Error("VerifyPassword() expected error, got nil")
					return
				}
				var connectErr *connect.Error
				if !errors.As(err, &connectErr) {
					t.Errorf("VerifyPassword() error is not a Connect error: %v", err)
					return
				}
				if connectErr.Code() != tt.wantCode {
					t.Errorf("VerifyPassword() error code = %v, want %v", connectErr.Code(), tt.wantCode)
				}
			}
		})
	}
}

func createTestUser() *domain.User {
	name := "Test User"
	now := time.Now().UTC()
	return &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: "hashed",
		Name:         &name,
		IsDeleted:    false,
		DeletedAt:    nil,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
