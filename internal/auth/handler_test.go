package auth

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	srv, _ := newTestServerWithMailer(t)
	return srv
}

func newTestServerWithMailer(t *testing.T) (http.Handler, *fakeMailer) {
	t.Helper()
	svc, _, mail := newTestServiceWithMailer()
	mux := http.NewServeMux()
	NewHandler(svc, slog.New(slog.DiscardHandler)).RegisterRoutes(mux)
	return mux, mail
}

func call(t *testing.T, srv http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

type errorResp struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}

var siti = registerRequest{Name: "Siti Aminah", Email: "Siti@Example.com", Password: "rahasia123"}

func TestRegisterEndpoint(t *testing.T) {
	srv := newTestServer(t)

	rec := call(t, srv, http.MethodPost, "/api/v1/auth/register", siti, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Error("response must not expose anything password-related")
	}
	got := decode[struct{ User userResponse }](t, rec)
	if got.User.Email != "siti@example.com" || got.User.Role != RoleUser {
		t.Errorf("unexpected user: %+v", got.User)
	}
}

func TestRegisterEndpoint_Errors(t *testing.T) {
	srv := newTestServer(t)
	call(t, srv, http.MethodPost, "/api/v1/auth/register", siti, "")

	dup := call(t, srv, http.MethodPost, "/api/v1/auth/register", siti, "")
	if dup.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want 409", dup.Code)
	}

	weak := siti
	weak.Email = "lain@example.com"
	weak.Password = "123"
	rec := call(t, srv, http.MethodPost, "/api/v1/auth/register", weak, "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("weak password: status = %d, want 422", rec.Code)
	}
	if body := decode[errorResp](t, rec); body.Fields["password"] == "" {
		t.Errorf("expected a password field error, got %+v", body)
	}

	if rec := call(t, srv, http.MethodPost, "/api/v1/auth/register", "{not json", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: status = %d, want 400", rec.Code)
	}
}

func TestLoginMeLogout(t *testing.T) {
	srv := newTestServer(t)
	call(t, srv, http.MethodPost, "/api/v1/auth/register", siti, "")

	bad := call(t, srv, http.MethodPost, "/api/v1/auth/login", loginRequest{Email: siti.Email, Password: "salah12345"}, "")
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status = %d, want 401", bad.Code)
	}

	rec := call(t, srv, http.MethodPost, "/api/v1/auth/login", loginRequest{Email: "SITI@example.com", Password: siti.Password}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status = %d, body = %s", rec.Code, rec.Body)
	}
	login := decode[loginResponse](t, rec)
	if login.Token == "" || login.TokenType != "Bearer" || login.ExpiresAt.IsZero() {
		t.Fatalf("unexpected login response: %+v", login)
	}

	me := call(t, srv, http.MethodGet, "/api/v1/auth/me", nil, login.Token)
	if me.Code != http.StatusOK {
		t.Fatalf("me: status = %d", me.Code)
	}
	if got := decode[struct{ User userResponse }](t, me); got.User.ID != login.User.ID {
		t.Errorf("me returned user %d, want %d", got.User.ID, login.User.ID)
	}

	if out := call(t, srv, http.MethodPost, "/api/v1/auth/logout", nil, login.Token); out.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d, want 204", out.Code)
	}
	if again := call(t, srv, http.MethodGet, "/api/v1/auth/me", nil, login.Token); again.Code != http.StatusUnauthorized {
		t.Errorf("me after logout: status = %d, want 401", again.Code)
	}
}

func TestForgotAndResetPasswordEndpoints(t *testing.T) {
	srv, mail := newTestServerWithMailer(t)
	call(t, srv, http.MethodPost, "/api/v1/auth/register", siti, "")

	unknown := call(t, srv, http.MethodPost, "/api/v1/auth/password/forgot", forgotPasswordRequest{Email: "siapa@example.com"}, "")
	known := call(t, srv, http.MethodPost, "/api/v1/auth/password/forgot", forgotPasswordRequest{Email: siti.Email}, "")
	if unknown.Code != http.StatusAccepted || known.Code != http.StatusAccepted {
		t.Fatalf("forgot: status unknown=%d known=%d, want 202 for both", unknown.Code, known.Code)
	}
	if unknown.Body.String() != known.Body.String() {
		t.Error("forgot must answer identically for registered and unknown emails")
	}
	token := tokenFromMail(t, waitForMail(t, mail))

	if rec := call(t, srv, http.MethodPost, "/api/v1/auth/password/forgot", forgotPasswordRequest{Email: "bukan-email"}, ""); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("forgot with bad email: status = %d, want 422", rec.Code)
	}

	if rec := call(t, srv, http.MethodPost, "/api/v1/auth/password/reset", resetPasswordRequest{Token: "made-up", Password: "passwordbaru1"}, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("reset with bad token: status = %d, want 400", rec.Code)
	}
	if rec := call(t, srv, http.MethodPost, "/api/v1/auth/password/reset", resetPasswordRequest{Token: token, Password: "123"}, ""); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("reset with weak password: status = %d, want 422", rec.Code)
	}
	if rec := call(t, srv, http.MethodPost, "/api/v1/auth/password/reset", resetPasswordRequest{Token: token, Password: "passwordbaru1"}, ""); rec.Code != http.StatusOK {
		t.Fatalf("reset: status = %d, body = %s", rec.Code, rec.Body)
	}

	if rec := call(t, srv, http.MethodPost, "/api/v1/auth/login", loginRequest{Email: siti.Email, Password: "passwordbaru1"}, ""); rec.Code != http.StatusOK {
		t.Errorf("login with new password: status = %d, want 200", rec.Code)
	}
}

func TestProtectedRoutesNeedToken(t *testing.T) {
	srv := newTestServer(t)

	rec := call(t, srv, http.MethodGet, "/api/v1/auth/me", nil, "")
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Errorf("no token: status = %d, WWW-Authenticate = %q", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Basic abc")
	basic := httptest.NewRecorder()
	srv.ServeHTTP(basic, req)
	if basic.Code != http.StatusUnauthorized {
		t.Errorf("basic auth: status = %d, want 401", basic.Code)
	}

	if rec := call(t, srv, http.MethodGet, "/api/v1/auth/me", nil, "made-up-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown token: status = %d, want 401", rec.Code)
	}
}
