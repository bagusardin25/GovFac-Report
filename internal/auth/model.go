package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleGovernment = "pemerintah"
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

type Session struct {
	UserID    int64
	TokenHash string
	IPAddress string
	UserAgent string
	ExpiresAt time.Time
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      *User
}

var (
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	ErrUserNotFound       = errors.New("auth: user not found")
	ErrSessionNotFound    = errors.New("auth: session not found or expired")
)

// ValidationError maps a field to a message the client can show as-is.
type ValidationError map[string]string

func (v ValidationError) Error() string { return "auth: invalid input" }

const (
	minPasswordChars = 8
	maxPasswordBytes = 72 // bcrypt ignores anything past 72 bytes
	maxNameChars     = 100
	maxEmailBytes    = 254
)

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func (in RegisterInput) validate() ValidationError {
	errs := ValidationError{}

	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		errs["name"] = "Nama wajib diisi."
	case utf8.RuneCountInString(name) > maxNameChars:
		errs["name"] = "Nama maksimal 100 karakter."
	}

	if msg := validateEmail(in.Email); msg != "" {
		errs["email"] = msg
	}

	switch {
	case utf8.RuneCountInString(in.Password) < minPasswordChars:
		errs["password"] = "Password minimal 8 karakter."
	case len(in.Password) > maxPasswordBytes:
		errs["password"] = "Password maksimal 72 karakter."
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateEmail(raw string) string {
	email := normalizeEmail(raw)
	if email == "" {
		return "Email wajib diisi."
	}
	if len(email) > maxEmailBytes {
		return "Email terlalu panjang."
	}
	// ParseAddress also accepts "Name <a@b.c>", so we require the bare address.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "Format email tidak valid."
	}
	return ""
}
