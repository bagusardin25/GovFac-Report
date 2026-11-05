package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const passwordResetTTL = 30 * time.Minute

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type Options struct {
	SessionTTL time.Duration
	// Frontend page that reads ?token= and calls POST /password/reset.
	PasswordResetURL string
}

type Service struct {
	repo       Repository
	mailer     Mailer
	opts       Options
	bcryptCost int
	now        func() time.Time
}

func NewService(repo Repository, mailer Mailer, opts Options) *Service {
	dummyHash() // warm it up so the first unknown-email login isn't slower than the rest
	return &Service{
		repo:       repo,
		mailer:     mailer,
		opts:       opts,
		bcryptCost: bcrypt.DefaultCost,
		now:        time.Now,
	}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*User, error) {
	if errs := in.validate(); errs != nil {
		return nil, errs
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &User{
		Name:         strings.TrimSpace(in.Name),
		Email:        normalizeEmail(in.Email),
		PasswordHash: string(hash),
		Role:         RoleUser,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	email := normalizeEmail(in.Email)
	if email == "" || in.Password == "" {
		return nil, ErrInvalidCredentials
	}

	u, err := s.repo.FindUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		// We still pay the bcrypt cost so response time doesn't reveal
		// whether the email is registered.
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(in.Password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	expiresAt := s.now().Add(s.opts.SessionTTL)
	err = s.repo.CreateSession(ctx, &Session{
		UserID:    u.ID,
		TokenHash: hashToken(token),
		IPAddress: truncate(in.IPAddress, 64),
		UserAgent: truncate(in.UserAgent, 512),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, ExpiresAt: expiresAt, User: u}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}
	return s.repo.FindUserBySession(ctx, hashToken(token), s.now())
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashToken(token))
}

// Unknown emails return nil on purpose: callers must not be able to tell
// whether an account exists.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := s.repo.FindUserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	token, err := newToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	err = s.repo.ReplacePasswordReset(ctx, &PasswordReset{
		UserID:    u.ID,
		TokenHash: hashToken(token),
		ExpiresAt: s.now().Add(passwordResetTTL),
	})
	if err != nil {
		return err
	}

	link, err := resetLink(s.opts.PasswordResetURL, token)
	if err != nil {
		return err
	}
	return s.mailer.Send(ctx, u.Email, resetEmailSubject, resetEmailBody(u.Name, link))
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if msg := validatePassword(password); msg != "" {
		return ValidationError{"password": msg}
	}
	if token == "" {
		return ErrResetTokenInvalid
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.ResetPassword(ctx, hashToken(token), string(hash), s.now())
}

var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("govfac-timing-equalizer"), bcrypt.DefaultCost)
	return h
})

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
