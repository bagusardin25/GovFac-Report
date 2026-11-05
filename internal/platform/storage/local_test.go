package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocal(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir, "/uploads/")
	if err != nil {
		t.Fatal(err)
	}

	key, err := s.Save(context.Background(), "reports", strings.NewReader("photo-bytes"), ".png")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.HasPrefix(key, "reports/") || !strings.HasSuffix(key, ".png") {
		t.Errorf("key = %q", key)
	}
	if s.URL(key) != "/uploads/"+key {
		t.Errorf("URL = %q", s.URL(key))
	}

	srv := http.StripPrefix("/uploads/", s.Handler())
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	rec := get("/uploads/" + key)
	if body, _ := io.ReadAll(rec.Body); rec.Code != http.StatusOK || string(body) != "photo-bytes" {
		t.Errorf("GET file: status = %d, body = %q", rec.Code, body)
	}
	if rec := get("/uploads/reports/"); rec.Code != http.StatusNotFound {
		t.Errorf("directory listing: status = %d, want 404", rec.Code)
	}
	if rec := get("/uploads/../go.mod"); rec.Code == http.StatusOK {
		t.Error("path traversal should not serve files outside the upload dir")
	}

	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(key))); !os.IsNotExist(err) {
		t.Error("file still exists after Delete")
	}
	if err := s.Delete(context.Background(), key); err != nil {
		t.Errorf("deleting a missing file should be a no-op, got %v", err)
	}
}
