package report

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bagusardin25/GovFac-Report/internal/platform/identity"
)

// Stands in for auth.RequireAuth so the report tests don't depend on auth.
func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := identity.WithUser(r.Context(), &identity.User{ID: 7, Name: "Budi", Role: "user"})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	svc, _, _ := newTestService()
	mux := http.NewServeMux()
	NewHandler(svc, slog.New(slog.DiscardHandler)).RegisterRoutes(mux, fakeAuth)
	return mux
}

func multipartBody(t *testing.T, fields map[string][]string, photo []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, values := range fields {
		for _, v := range values {
			if err := mw.WriteField(name, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	if photo != nil {
		part, err := mw.CreateFormFile("photo", "bukti.png")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(photo)
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func post(srv http.Handler, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

var validFields = map[string][]string{
	"title":       {"Lampu jalan mati"},
	"description": {"Sudah seminggu lampu di depan balai RW mati."},
	"hashtags":    {"#lampujalan", "#penerangan"},
}

func TestCreateEndpoint(t *testing.T) {
	srv := newTestServer(t)
	body, ct := multipartBody(t, validFields, pngBytes)

	rec := post(srv, body, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var got struct{ Report reportResponse }
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	r := got.Report
	if r.ID == 0 || r.Status != StatusAdminReview || !strings.HasPrefix(r.PhotoURL, "/uploads/reports/") {
		t.Errorf("unexpected report: %+v", r)
	}
	if len(r.Hashtags) != 2 || r.Hashtags[0] != "lampujalan" {
		t.Errorf("hashtags = %v", r.Hashtags)
	}
}

func TestCreateEndpoint_Errors(t *testing.T) {
	srv := newTestServer(t)

	noPhoto, ct := multipartBody(t, validFields, nil)
	if rec := post(srv, noPhoto, ct); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"photo"`) {
		t.Errorf("missing photo: status = %d, body = %s", rec.Code, rec.Body)
	}

	if rec := post(srv, bytes.NewBufferString(`{"title":"x"}`), "application/json"); rec.Code != http.StatusBadRequest {
		t.Errorf("json body: status = %d, want 400", rec.Code)
	}

	huge, ct := multipartBody(t, validFields, bytes.Repeat([]byte{0xFF}, maxCreateBodyBytes+1))
	if rec := post(srv, huge, ct); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: status = %d, want 413", rec.Code)
	}
}
