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

type Service struct {
	repo       Repository
	sessionTTL time.Duration
	bcryptCost int
	now        func() time.Time
}

func NewService(repo Repository, sessionTTL time.Duration) *Service {
	dummyHash() // warm it up so the first unknown-email login isn't slower than the rest
	return &Service{
		repo:       repo,
		sessionTTL: sessionTTL,
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
	expiresAt := s.now().Add(s.sessionTTL)
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
