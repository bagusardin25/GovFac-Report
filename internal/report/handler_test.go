package report

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	srv, _ := newTestServerWithService(t)
	return srv
}

func newTestServerWithService(t *testing.T) (http.Handler, *Service) {
	t.Helper()
	svc, _, _ := newTestService()
	mux := http.NewServeMux()
	NewHandler(svc, slog.New(slog.DiscardHandler)).RegisterRoutes(mux, fakeAuth)
	return mux, svc
}

func get(srv http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
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

func TestListMineEndpoint(t *testing.T) {
	srv, svc := newTestServerWithService(t)
	createFor(t, svc, 7, "satu")
	createFor(t, svc, 7, "dua")
	createFor(t, svc, 8, "bukan punya saya")

	rec := get(srv, "/api/v1/reports/mine?per_page=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got pageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || got.PerPage != 1 || len(got.Reports) != 1 || got.Reports[0].Title != "dua" {
		t.Errorf("unexpected page: %+v", got)
	}
	if got.Reports[0].Timeline != nil {
		t.Error("list items shouldn't carry the timeline")
	}

	if rec := get(srv, "/api/v1/reports/mine?page=abc"); rec.Code != http.StatusOK {
		t.Errorf("bad page param: status = %d, want 200 with defaults", rec.Code)
	}
}

func TestGetMineEndpoint(t *testing.T) {
	srv, svc := newTestServerWithService(t)
	mine := createFor(t, svc, 7, "punya saya")
	theirs := createFor(t, svc, 8, "punya orang lain")

	rec := get(srv, fmt.Sprintf("/api/v1/reports/%d", mine.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("own report: status = %d", rec.Code)
	}
	var got struct{ Report reportResponse }
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Report.Timeline) != 1 || got.Report.Timeline[0].Status != StatusAdminReview {
		t.Errorf("timeline = %+v", got.Report.Timeline)
	}

	for _, path := range []string{fmt.Sprintf("/api/v1/reports/%d", theirs.ID), "/api/v1/reports/9999", "/api/v1/reports/abc"} {
		if rec := get(srv, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
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
