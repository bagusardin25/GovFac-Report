package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// We keep uploads on local disk for now. Features only see their own store
// interface, so moving to object storage later won't touch them.
type Local struct {
	dir       string
	urlPrefix string
}

func NewLocal(dir, urlPrefix string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &Local{dir: dir, urlPrefix: urlPrefix}, nil
}

func (s *Local) Save(_ context.Context, folder string, r io.Reader, ext string) (string, error) {
	name := make([]byte, 16)
	if _, err := rand.Read(name); err != nil {
		return "", err
	}
	key := path.Join(folder, hex.EncodeToString(name)+ext)
	full := filepath.Join(s.dir, filepath.FromSlash(key))

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(full)
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(full)
		return "", fmt.Errorf("close file: %w", err)
	}
	return key, nil
}

func (s *Local) Delete(_ context.Context, key string) error {
	err := os.Remove(filepath.Join(s.dir, filepath.FromSlash(key)))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Local) URL(key string) string {
	return s.urlPrefix + key
}

// Handler serves stored files but never directory listings.
func (s *Local) Handler() http.Handler {
	files := http.FileServerFS(os.DirFS(s.dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, r)
	})
}
