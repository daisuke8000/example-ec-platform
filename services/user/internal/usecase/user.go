package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
)

type UserUseCase interface {
	CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*domain.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	VerifyPassword(ctx context.Context, email, password string) (*domain.User, error)
}

type CreateUserInput struct {
	Email    string
	Password string
	Name     *string
}

type UpdateUserInput struct {
	Email *string
	Name  *string
}

type userUseCase struct {
	repo       domain.UserRepository
	bcryptCost int
	dummyHash  []byte
}

func NewUserUseCase(repo domain.UserRepository, bcryptCost int) UserUseCase {
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing-safe"), bcryptCost)
	if err != nil {
		panic(fmt.Sprintf("failed to generate dummy hash: %v", err))
	}
	return &userUseCase{
		repo:       repo,
		bcryptCost: bcryptCost,
		dummyHash:  dummyHash,
	}
}

func (uc *userUseCase) CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	if err := domain.ValidateEmail(input.Email); err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(input.Password); err != nil {
		return nil, err
	}
	if err := domain.ValidateName(input.Name); err != nil {
		return nil, err
	}

	_, err := uc.repo.FindByEmail(ctx, input.Email)
	if err == nil {
		return nil, domain.ErrEmailAlreadyExists
	}
	if err != domain.ErrUserNotFound {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), uc.bcryptCost)
	if err != nil {
		return nil, err
	}

	user := domain.NewUser(input.Email, string(hashedPassword), input.Name)
	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (uc *userUseCase) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *userUseCase) UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*domain.User, error) {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Email != nil {
		if err := domain.ValidateEmail(*input.Email); err != nil {
			return nil, err
		}
		if *input.Email != user.Email {
			existingUser, err := uc.repo.FindByEmail(ctx, *input.Email)
			if err == nil && existingUser.ID != id {
				return nil, domain.ErrEmailAlreadyExists
			}
			if err != nil && err != domain.ErrUserNotFound {
				return nil, err
			}
		}
		user.Email = *input.Email
	}

	if input.Name != nil {
		if err := domain.ValidateName(input.Name); err != nil {
			return nil, err
		}
		user.Name = input.Name
	}

	if err := uc.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (uc *userUseCase) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return uc.repo.SoftDelete(ctx, id)
}

// VerifyPassword is timing-safe: performs bcrypt comparison even for non-existent users.
func (uc *userUseCase) VerifyPassword(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		if err == domain.ErrUserNotFound {
			_ = bcrypt.CompareHashAndPassword(uc.dummyHash, []byte(password))
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return user, nil
}
