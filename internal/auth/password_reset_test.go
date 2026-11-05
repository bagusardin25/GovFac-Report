package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func requestReset(t *testing.T, svc *Service, mail *fakeMailer) string {
	t.Helper()
	if err := svc.RequestPasswordReset(context.Background(), "BUDI@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	return tokenFromMail(t, waitForMail(t, mail))
}

func TestRequestPasswordReset_SendsLink(t *testing.T) {
	svc, repo, mail := newTestServiceWithMailer()
	mustRegister(t, svc)

	if err := svc.RequestPasswordReset(context.Background(), "budi@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	m := waitForMail(t, mail)
	if m.to != "budi@example.com" || m.subject != resetEmailSubject {
		t.Errorf("unexpected email: to=%q subject=%q", m.to, m.subject)
	}
	token := tokenFromMail(t, m)
	if _, stored := repo.resets[token]; stored {
		t.Error("raw reset token was stored; only its hash should be")
	}
	if _, ok := repo.resets[hashToken(token)]; !ok {
		t.Error("reset token hash not stored")
	}
}

func TestRequestPasswordReset_UnknownEmailIsSilent(t *testing.T) {
	svc, _, mail := newTestServiceWithMailer()

	if err := svc.RequestPasswordReset(context.Background(), "siapa@example.com"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(mail.sent) != 0 {
		t.Error("no email should be sent for an unknown address")
	}
}

func TestResetPassword(t *testing.T) {
	svc, _, mail := newTestServiceWithMailer()
	mustRegister(t, svc)
	old, err := svc.Login(context.Background(), LoginInput{Email: "budi@example.com", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}
	token := requestReset(t, svc, mail)

	if err := svc.ResetPassword(context.Background(), token, "passwordbaru1"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if _, err := svc.Login(context.Background(), LoginInput{Email: "budi@example.com", Password: "rahasia123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password still works: err = %v", err)
	}
	if _, err := svc.Login(context.Background(), LoginInput{Email: "budi@example.com", Password: "passwordbaru1"}); err != nil {
		t.Errorf("new password rejected: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), old.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Error("existing sessions should be revoked after a reset")
	}
	if err := svc.ResetPassword(context.Background(), token, "passwordlain1"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("token reused: err = %v, want ErrResetTokenInvalid", err)
	}
}

func TestResetPassword_OnlyNewestLinkWorks(t *testing.T) {
	svc, _, mail := newTestServiceWithMailer()
	mustRegister(t, svc)
	first := requestReset(t, svc, mail)
	second := requestReset(t, svc, mail)

	if err := svc.ResetPassword(context.Background(), first, "passwordbaru1"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("older link: err = %v, want ErrResetTokenInvalid", err)
	}
	if err := svc.ResetPassword(context.Background(), second, "passwordbaru1"); err != nil {
		t.Errorf("newest link: %v", err)
	}
}

func TestResetPassword_Rejects(t *testing.T) {
	svc, _, mail := newTestServiceWithMailer()
	mustRegister(t, svc)
	token := requestReset(t, svc, mail)

	var invalid ValidationError
	if err := svc.ResetPassword(context.Background(), token, "pendek"); !errors.As(err, &invalid) || invalid["password"] == "" {
		t.Errorf("weak password: err = %v, want password ValidationError", err)
	}
	if err := svc.ResetPassword(context.Background(), "", "passwordbaru1"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("empty token: err = %v", err)
	}
	if err := svc.ResetPassword(context.Background(), "made-up", "passwordbaru1"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("unknown token: err = %v", err)
	}

	svc.now = func() time.Time { return time.Now().Add(passwordResetTTL + time.Minute) }
	if err := svc.ResetPassword(context.Background(), token, "passwordbaru1"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("expired token: err = %v, want ErrResetTokenInvalid", err)
	}
}
