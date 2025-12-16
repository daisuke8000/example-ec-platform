package domain

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{
			name:    "valid email",
			email:   "user@example.com",
			wantErr: nil,
		},
		{
			name:    "valid email with plus sign",
			email:   "user+tag@example.com",
			wantErr: nil,
		},
		{
			name:    "valid email with subdomain",
			email:   "user@mail.example.co.jp",
			wantErr: nil,
		},
		{
			name:    "empty email",
			email:   "",
			wantErr: ErrEmptyEmail,
		},
		{
			name:    "missing @",
			email:   "userexample.com",
			wantErr: ErrInvalidEmail,
		},
		{
			name:    "missing domain",
			email:   "user@",
			wantErr: ErrInvalidEmail,
		},
		{
			name:    "missing local part",
			email:   "@example.com",
			wantErr: ErrInvalidEmail,
		},
		{
			name:    "missing TLD",
			email:   "user@example",
			wantErr: ErrInvalidEmail,
		},
		{
			name:    "spaces in email",
			email:   "user @example.com",
			wantErr: ErrInvalidEmail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if err != tt.wantErr {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "valid password - exactly 8 chars",
			password: "12345678",
			wantErr:  nil,
		},
		{
			name:     "valid password - long",
			password: "thisisaverylongpassword123!",
			wantErr:  nil,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  ErrEmptyPassword,
		},
		{
			name:     "too short - 7 chars",
			password: "1234567",
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "too short - 1 char",
			password: "a",
			wantErr:  ErrPasswordTooShort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if err != tt.wantErr {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	name := "Test User"
	user := NewUser("test@example.com", "hashedpassword", &name)

	if user.ID.String() == "" {
		t.Error("NewUser() should generate a UUID")
	}
	if user.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "test@example.com")
	}
	if user.PasswordHash != "hashedpassword" {
		t.Errorf("PasswordHash = %q, want %q", user.PasswordHash, "hashedpassword")
	}
	if user.Name == nil || *user.Name != "Test User" {
		t.Errorf("Name = %v, want %q", user.Name, "Test User")
	}
	if user.IsDeleted {
		t.Error("IsDeleted should be false for new user")
	}
	if user.DeletedAt != nil {
		t.Error("DeletedAt should be nil for new user")
	}
	if user.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestNewUser_NilName(t *testing.T) {
	user := NewUser("test@example.com", "hashedpassword", nil)

	if user.Name != nil {
		t.Errorf("Name = %v, want nil", user.Name)
	}
}
