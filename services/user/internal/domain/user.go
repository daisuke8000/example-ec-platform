package domain

import (
	"context"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const (
	MinPasswordLength = 8
	MaxNameLength     = 100
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         *string
	IsDeleted    bool
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmptyEmail
	}
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return ErrEmptyPassword
	}
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

func ValidateName(name *string) error {
	if name != nil && utf8.RuneCountInString(*name) > MaxNameLength {
		return ErrNameTooLong
	}
	return nil
}

func NewUser(email, passwordHash string, name *string) *User {
	now := time.Now().UTC()
	return &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		IsDeleted:    false,
		DeletedAt:    nil,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
