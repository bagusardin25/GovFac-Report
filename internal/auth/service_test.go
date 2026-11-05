package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func newTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	svc := NewService(repo, time.Hour)
	svc.bcryptCost = bcrypt.MinCost
	return svc, repo
}

func validRegisterInput() RegisterInput {
	return RegisterInput{
		Name:     "Budi Santoso",
		Email:    "Budi@Example.com ",
		Password: "rahasia123",
	}
}

func mustRegister(t *testing.T, svc *Service) *User {
	t.Helper()
	u, err := svc.Register(context.Background(), validRegisterInput())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return u
}

func TestRegister(t *testing.T) {
	svc, _ := newTestService()
	u := mustRegister(t, svc)

	if u.ID == 0 {
		t.Error("expected an ID")
	}
	if u.Email != "budi@example.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if u.Role != RoleUser {
		t.Errorf("role = %q, want %q", u.Role, RoleUser)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("rahasia123")); err != nil {
		t.Errorf("password hash doesn't match: %v", err)
	}
}

func TestRegister_Validation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*RegisterInput)
		field  string
	}{
		{"empty name", func(in *RegisterInput) { in.Name = "  " }, "name"},
		{"long name", func(in *RegisterInput) { in.Name = strings.Repeat("a", 101) }, "name"},
		{"empty email", func(in *RegisterInput) { in.Email = "" }, "email"},
		{"bad email", func(in *RegisterInput) { in.Email = "budi-at-example" }, "email"},
		{"display name email", func(in *RegisterInput) { in.Email = "Budi <budi@example.com>" }, "email"},
		{"short password", func(in *RegisterInput) { in.Password = "1234567" }, "password"},
		{"long password", func(in *RegisterInput) { in.Password = strings.Repeat("x", 73) }, "password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService()
			in := validRegisterInput()
			tt.modify(&in)

			_, err := svc.Register(context.Background(), in)
			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if _, ok := invalid[tt.field]; !ok {
				t.Errorf("expected an error on %q, got %v", tt.field, invalid)
			}
		})
	}
}

func TestRegister_DuplicateEmailIgnoresCase(t *testing.T) {
	svc, _ := newTestService()
	mustRegister(t, svc)

	again := validRegisterInput()
	again.Email = "BUDI@example.COM"
	if _, err := svc.Register(context.Background(), again); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestLogin(t *testing.T) {
	svc, repo := newTestService()
	u := mustRegister(t, svc)

	res, err := svc.Login(context.Background(), LoginInput{Email: "budi@example.com", Password: "rahasia123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.Token == "" || res.User.ID != u.ID {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, stored := repo.sessions[res.Token]; stored {
		t.Error("raw token was stored; only its hash should be")
	}

	got, err := svc.Authenticate(context.Background(), res.Token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("Authenticate returned user %d, want %d", got.ID, u.ID)
	}
}

func TestLogin_BadCredentials(t *testing.T) {
	svc, _ := newTestService()
	mustRegister(t, svc)

	cases := map[string]LoginInput{
		"wrong password": {Email: "budi@example.com", Password: "salah12345"},
		"unknown email":  {Email: "siapa@example.com", Password: "rahasia123"},
		"empty":          {},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Login(context.Background(), in); !errors.Is(err, ErrInvalidCredentials) {
				t.Errorf("err = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestAuthenticate_Expired(t *testing.T) {
	svc, _ := newTestService()
	mustRegister(t, svc)
	res, err := svc.Login(context.Background(), LoginInput{Email: "budi@example.com", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}

	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := svc.Authenticate(context.Background(), res.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestLogout(t *testing.T) {
	svc, _ := newTestService()
	mustRegister(t, svc)
	res, err := svc.Login(context.Background(), LoginInput{Email: "budi@example.com", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Logout(context.Background(), res.Token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), res.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
}
