package auth

import (
	"fmt"
	"net/url"
)

const resetEmailSubject = "Atur ulang password GovFac Report"

func resetEmailBody(name, link string) string {
	return fmt.Sprintf(`Halo %s,

Kami menerima permintaan untuk mengatur ulang password akun GovFac Report kamu.
Buka tautan berikut untuk membuat password baru. Tautan ini berlaku 30 menit dan hanya bisa dipakai sekali.

%s

Kalau kamu tidak merasa meminta ini, abaikan email ini. Password kamu tidak akan berubah.

Tim GovFac Report
`, name, link)
}

func resetLink(base, token string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse password reset url: %w", err)
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
