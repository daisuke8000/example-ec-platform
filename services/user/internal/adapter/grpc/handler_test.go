package grpc

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/daisuke8000/example-ec-platform/gen/user/v1"
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

func newTestHandler(uc *mockUserUseCase) *UserServiceHandler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewUserServiceHandler(uc, logger)
}

func TestCreateUser(t *testing.T) {
	testUser := createTestUser()
	name := "Test User"

	tests := []struct {
		name     string
		req      *pb.CreateUserRequest
		mockFn   func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error)
		wantCode codes.Code
	}{
		{
			name: "creates user successfully",
			req: &pb.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     &name,
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return testUser, nil
			},
			wantCode: codes.OK,
		},
		{
			name: "returns already exists for duplicate email",
			req: &pb.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return nil, domain.ErrEmailAlreadyExists
			},
			wantCode: codes.AlreadyExists,
		},
		{
			name: "returns invalid argument for invalid email",
			req: &pb.CreateUserRequest{
				Email:    "invalid",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return nil, domain.ErrInvalidEmail
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "returns invalid argument for short password",
			req: &pb.CreateUserRequest{
				Email:    "test@example.com",
				Password: "short",
			},
			mockFn: func(ctx context.Context, input usecase.CreateUserInput) (*domain.User, error) {
				return nil, domain.ErrPasswordTooShort
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{createUserFn: tt.mockFn}
			handler := newTestHandler(mock)

			resp, err := handler.CreateUser(context.Background(), tt.req)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Errorf("CreateUser() error = %v, want nil", err)
					return
				}
				if resp.GetUser() == nil {
					t.Error("CreateUser() returned nil user")
				}
			} else {
				if err == nil {
					t.Error("CreateUser() expected error, got nil")
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("CreateUser() error is not a gRPC status error: %v", err)
					return
				}
				if st.Code() != tt.wantCode {
					t.Errorf("CreateUser() error code = %v, want %v", st.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	testUser := createTestUser()

	tests := []struct {
		name     string
		req      *pb.GetUserRequest
		mockFn   func(ctx context.Context, id uuid.UUID) (*domain.User, error)
		wantCode codes.Code
	}{
		{
			name: "gets user successfully",
			req: &pb.GetUserRequest{
				Id: testUser.ID.String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
				return testUser, nil
			},
			wantCode: codes.OK,
		},
		{
			name: "returns not found for non-existent user",
			req: &pb.GetUserRequest{
				Id: uuid.New().String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			wantCode: codes.NotFound,
		},
		{
			name: "returns invalid argument for invalid UUID",
			req: &pb.GetUserRequest{
				Id: "invalid-uuid",
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{getUserFn: tt.mockFn}
			handler := newTestHandler(mock)

			resp, err := handler.GetUser(context.Background(), tt.req)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Errorf("GetUser() error = %v, want nil", err)
					return
				}
				if resp.GetUser() == nil {
					t.Error("GetUser() returned nil user")
				}
			} else {
				if err == nil {
					t.Error("GetUser() expected error, got nil")
					return
				}
				st, _ := status.FromError(err)
				if st.Code() != tt.wantCode {
					t.Errorf("GetUser() error code = %v, want %v", st.Code(), tt.wantCode)
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
		req      *pb.UpdateUserRequest
		mockFn   func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error)
		wantCode codes.Code
	}{
		{
			name: "updates user successfully",
			req: &pb.UpdateUserRequest{
				Id:    testUser.ID.String(),
				Email: &newEmail,
				Name:  &newName,
			},
			mockFn: func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
				testUser.Email = *input.Email
				testUser.Name = input.Name
				return testUser, nil
			},
			wantCode: codes.OK,
		},
		{
			name: "returns not found for non-existent user",
			req: &pb.UpdateUserRequest{
				Id:   uuid.New().String(),
				Name: &newName,
			},
			mockFn: func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			wantCode: codes.NotFound,
		},
		{
			name: "returns already exists for duplicate email",
			req: &pb.UpdateUserRequest{
				Id:    testUser.ID.String(),
				Email: &newEmail,
			},
			mockFn: func(ctx context.Context, id uuid.UUID, input usecase.UpdateUserInput) (*domain.User, error) {
				return nil, domain.ErrEmailAlreadyExists
			},
			wantCode: codes.AlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{updateUserFn: tt.mockFn}
			handler := newTestHandler(mock)

			resp, err := handler.UpdateUser(context.Background(), tt.req)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Errorf("UpdateUser() error = %v, want nil", err)
					return
				}
				if resp.GetUser() == nil {
					t.Error("UpdateUser() returned nil user")
				}
			} else {
				if err == nil {
					t.Error("UpdateUser() expected error, got nil")
					return
				}
				st, _ := status.FromError(err)
				if st.Code() != tt.wantCode {
					t.Errorf("UpdateUser() error code = %v, want %v", st.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	testUser := createTestUser()

	tests := []struct {
		name     string
		req      *pb.DeleteUserRequest
		mockFn   func(ctx context.Context, id uuid.UUID) error
		wantCode codes.Code
	}{
		{
			name: "deletes user successfully",
			req: &pb.DeleteUserRequest{
				Id: testUser.ID.String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
			wantCode: codes.OK,
		},
		{
			name: "returns not found for non-existent user",
			req: &pb.DeleteUserRequest{
				Id: uuid.New().String(),
			},
			mockFn: func(ctx context.Context, id uuid.UUID) error {
				return domain.ErrUserNotFound
			},
			wantCode: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{deleteUserFn: tt.mockFn}
			handler := newTestHandler(mock)

			_, err := handler.DeleteUser(context.Background(), tt.req)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Errorf("DeleteUser() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("DeleteUser() expected error, got nil")
					return
				}
				st, _ := status.FromError(err)
				if st.Code() != tt.wantCode {
					t.Errorf("DeleteUser() error code = %v, want %v", st.Code(), tt.wantCode)
				}
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	testUser := createTestUser()

	tests := []struct {
		name     string
		req      *pb.VerifyPasswordRequest
		mockFn   func(ctx context.Context, email, password string) (*domain.User, error)
		wantCode codes.Code
	}{
		{
			name: "verifies password successfully",
			req: &pb.VerifyPasswordRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, email, password string) (*domain.User, error) {
				return testUser, nil
			},
			wantCode: codes.OK,
		},
		{
			name: "returns unauthenticated for invalid credentials",
			req: &pb.VerifyPasswordRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			mockFn: func(ctx context.Context, email, password string) (*domain.User, error) {
				return nil, domain.ErrInvalidCredentials
			},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserUseCase{verifyPasswordFn: tt.mockFn}
			handler := newTestHandler(mock)

			resp, err := handler.VerifyPassword(context.Background(), tt.req)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Errorf("VerifyPassword() error = %v, want nil", err)
					return
				}
				if resp.GetUserId() == "" {
					t.Error("VerifyPassword() returned empty user_id")
				}
			} else {
				if err == nil {
					t.Error("VerifyPassword() expected error, got nil")
					return
				}
				st, _ := status.FromError(err)
				if st.Code() != tt.wantCode {
					t.Errorf("VerifyPassword() error code = %v, want %v", st.Code(), tt.wantCode)
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
