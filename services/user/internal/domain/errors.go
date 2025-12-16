package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrEmptyEmail         = errors.New("email cannot be empty")
	ErrEmptyPassword      = errors.New("password cannot be empty")
	ErrNameTooLong        = errors.New("name must be 100 characters or less")
)
