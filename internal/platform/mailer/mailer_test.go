package mailer

import (
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestBuildMessage(t *testing.T) {
	from := &mail.Address{Name: "GovFac Report", Address: "no-reply@govfac.local"}
	msg, err := buildMessage(from, "budi@example.com", "Atur ulang password", "Halo,\nbaris dua", time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)

	for _, want := range []string{
		"From: \"GovFac Report\" <no-reply@govfac.local>\r\n",
		"To: budi@example.com\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"\r\n\r\nHalo,\r\nbaris dua",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("message missing %q:\n%s", want, s)
		}
	}
}

func TestBuildMessage_RejectsHeaderInjection(t *testing.T) {
	from := &mail.Address{Address: "no-reply@govfac.local"}
	if _, err := buildMessage(from, "a@example.com\r\nBcc: evil@example.com", "x", "y", time.Now()); err == nil {
		t.Error("expected an error for a recipient with a line break")
	}
	if _, err := buildMessage(from, "a@example.com", "x\r\nBcc: evil@example.com", "y", time.Now()); err == nil {
		t.Error("expected an error for a subject with a line break")
	}
}
