package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
)

// mockUserRepository is a test double for domain.UserRepository.
type mockUserRepository struct {
	users         map[uuid.UUID]*domain.User
	emailIndex    map[string]uuid.UUID
	createErr     error
	findByIDErr   error
	findByEmailErr error
	updateErr     error
	softDeleteErr error
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users:      make(map[uuid.UUID]*domain.User),
		emailIndex: make(map[string]uuid.UUID),
	}
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.emailIndex[user.Email]; exists {
		return domain.ErrEmailAlreadyExists
	}
	m.users[user.ID] = user
	m.emailIndex[user.Email] = user.ID
	return nil
}

func (m *mockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	user, exists := m.users[id]
	if !exists || user.IsDeleted {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

func (m *mockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	id, exists := m.emailIndex[email]
	if !exists {
		return nil, domain.ErrUserNotFound
	}
	user := m.users[id]
	if user.IsDeleted {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

func (m *mockUserRepository) Update(ctx context.Context, user *domain.User) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	existing, exists := m.users[user.ID]
	if !exists || existing.IsDeleted {
		return domain.ErrUserNotFound
	}
	// Check email uniqueness if email changed
	if existing.Email != user.Email {
		if existingID, emailExists := m.emailIndex[user.Email]; emailExists && existingID != user.ID {
			return domain.ErrEmailAlreadyExists
		}
		delete(m.emailIndex, existing.Email)
		m.emailIndex[user.Email] = user.ID
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteErr != nil {
		return m.softDeleteErr
	}
	user, exists := m.users[id]
	if !exists || user.IsDeleted {
		return domain.ErrUserNotFound
	}
	user.IsDeleted = true
	return nil
}

// seedUser adds a user to the mock repository for testing.
func (m *mockUserRepository) seedUser(user *domain.User) {
	m.users[user.ID] = user
	m.emailIndex[user.Email] = user.ID
}

func TestUserUseCase_CreateUser(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateUserInput
		setup   func(*mockUserRepository)
		wantErr error
	}{
		{
			name: "creates user successfully",
			input: CreateUserInput{
				Email:    "test@example.com",
				Password: "password123",
				Name:     stringPtr("Test User"),
			},
			wantErr: nil,
		},
		{
			name: "creates user without name",
			input: CreateUserInput{
				Email:    "test@example.com",
				Password: "password123",
				Name:     nil,
			},
			wantErr: nil,
		},
		{
			name: "fails with invalid email",
			input: CreateUserInput{
				Email:    "invalid-email",
				Password: "password123",
			},
			wantErr: domain.ErrInvalidEmail,
		},
		{
			name: "fails with empty email",
			input: CreateUserInput{
				Email:    "",
				Password: "password123",
			},
			wantErr: domain.ErrEmptyEmail,
		},
		{
			name: "fails with short password",
			input: CreateUserInput{
				Email:    "test@example.com",
				Password: "short",
			},
			wantErr: domain.ErrPasswordTooShort,
		},
		{
			name: "fails with empty password",
			input: CreateUserInput{
				Email:    "test@example.com",
				Password: "",
			},
			wantErr: domain.ErrEmptyPassword,
		},
		{
			name: "fails with duplicate email",
			input: CreateUserInput{
				Email:    "existing@example.com",
				Password: "password123",
			},
			setup: func(m *mockUserRepository) {
				existingUser := domain.NewUser("existing@example.com", "hash", nil)
				m.seedUser(existingUser)
			},
			wantErr: domain.ErrEmailAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewUserUseCase(repo, 4) // Use low cost for fast tests

			user, err := uc.CreateUser(context.Background(), tt.input)

			if err != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr == nil {
				if user == nil {
					t.Error("CreateUser() returned nil user on success")
					return
				}
				if user.Email != tt.input.Email {
					t.Errorf("Email = %q, want %q", user.Email, tt.input.Email)
				}
				// Verify password was hashed
				if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.input.Password)); err != nil {
					t.Error("Password was not hashed correctly")
				}
			}
		})
	}
}

func TestUserUseCase_GetUser(t *testing.T) {
	existingUser := domain.NewUser("test@example.com", "hash", nil)

	tests := []struct {
		name    string
		id      uuid.UUID
		setup   func(*mockUserRepository)
		wantErr error
	}{
		{
			name: "finds existing user",
			id:   existingUser.ID,
			setup: func(m *mockUserRepository) {
				m.seedUser(existingUser)
			},
			wantErr: nil,
		},
		{
			name:    "returns not found for non-existent user",
			id:      uuid.New(),
			wantErr: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewUserUseCase(repo, 4)

			user, err := uc.GetUser(context.Background(), tt.id)

			if err != tt.wantErr {
				t.Errorf("GetUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr == nil && user == nil {
				t.Error("GetUser() returned nil user on success")
			}
		})
	}
}

func TestUserUseCase_UpdateUser(t *testing.T) {
	existingUser := domain.NewUser("test@example.com", "hash", stringPtr("Original Name"))

	tests := []struct {
		name      string
		id        uuid.UUID
		input     UpdateUserInput
		setup     func(*mockUserRepository)
		wantErr   error
		checkUser func(*testing.T, *domain.User)
	}{
		{
			name: "updates email",
			id:   existingUser.ID,
			input: UpdateUserInput{
				Email: stringPtr("new@example.com"),
			},
			setup: func(m *mockUserRepository) {
				// Clone to avoid mutation
				user := *existingUser
				m.seedUser(&user)
			},
			wantErr: nil,
			checkUser: func(t *testing.T, user *domain.User) {
				if user.Email != "new@example.com" {
					t.Errorf("Email = %q, want %q", user.Email, "new@example.com")
				}
			},
		},
		{
			name: "updates name",
			id:   existingUser.ID,
			input: UpdateUserInput{
				Name: stringPtr("New Name"),
			},
			setup: func(m *mockUserRepository) {
				user := *existingUser
				m.seedUser(&user)
			},
			wantErr: nil,
			checkUser: func(t *testing.T, user *domain.User) {
				if user.Name == nil || *user.Name != "New Name" {
					t.Errorf("Name = %v, want %q", user.Name, "New Name")
				}
			},
		},
		{
			name:    "fails for non-existent user",
			id:      uuid.New(),
			input:   UpdateUserInput{Name: stringPtr("New Name")},
			wantErr: domain.ErrUserNotFound,
		},
		{
			name: "fails with invalid email format",
			id:   existingUser.ID,
			input: UpdateUserInput{
				Email: stringPtr("invalid-email"),
			},
			setup: func(m *mockUserRepository) {
				user := *existingUser
				m.seedUser(&user)
			},
			wantErr: domain.ErrInvalidEmail,
		},
		{
			name: "fails with duplicate email",
			id:   existingUser.ID,
			input: UpdateUserInput{
				Email: stringPtr("other@example.com"),
			},
			setup: func(m *mockUserRepository) {
				user := *existingUser
				m.seedUser(&user)
				otherUser := domain.NewUser("other@example.com", "hash", nil)
				m.seedUser(otherUser)
			},
			wantErr: domain.ErrEmailAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewUserUseCase(repo, 4)

			user, err := uc.UpdateUser(context.Background(), tt.id, tt.input)

			if err != tt.wantErr {
				t.Errorf("UpdateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr == nil && tt.checkUser != nil {
				tt.checkUser(t, user)
			}
		})
	}
}

func TestUserUseCase_DeleteUser(t *testing.T) {
	existingUser := domain.NewUser("test@example.com", "hash", nil)

	tests := []struct {
		name    string
		id      uuid.UUID
		setup   func(*mockUserRepository)
		wantErr error
	}{
		{
			name: "soft deletes user",
			id:   existingUser.ID,
			setup: func(m *mockUserRepository) {
				user := *existingUser
				m.seedUser(&user)
			},
			wantErr: nil,
		},
		{
			name:    "fails for non-existent user",
			id:      uuid.New(),
			wantErr: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewUserUseCase(repo, 4)

			err := uc.DeleteUser(context.Background(), tt.id)

			if err != tt.wantErr {
				t.Errorf("DeleteUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserUseCase_VerifyPassword(t *testing.T) {
	// Create a user with a known password hash
	password := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), 4)
	existingUser := domain.NewUser("test@example.com", string(hashedPassword), nil)

	tests := []struct {
		name     string
		email    string
		password string
		setup    func(*mockUserRepository)
		wantErr  error
	}{
		{
			name:     "verifies correct password",
			email:    "test@example.com",
			password: password,
			setup: func(m *mockUserRepository) {
				m.seedUser(existingUser)
			},
			wantErr: nil,
		},
		{
			name:     "fails with wrong password",
			email:    "test@example.com",
			password: "wrongpassword",
			setup: func(m *mockUserRepository) {
				m.seedUser(existingUser)
			},
			wantErr: domain.ErrInvalidCredentials,
		},
		{
			name:     "fails with non-existent email",
			email:    "nonexistent@example.com",
			password: password,
			wantErr:  domain.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewUserUseCase(repo, 4)

			user, err := uc.VerifyPassword(context.Background(), tt.email, tt.password)

			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr == nil && user == nil {
				t.Error("VerifyPassword() returned nil user on success")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
