package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

type fakeRepository struct {
	mu      sync.Mutex
	nextID  int64
	reports []*Report
	failNow bool
}

func (f *fakeRepository) Create(_ context.Context, r *Report) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNow {
		return errors.New("db down")
	}
	f.nextID++
	r.ID = f.nextID
	r.Status = StatusAdminReview
	r.CreatedAt = time.Now()
	stored := *r
	f.reports = append(f.reports, &stored)
	return nil
}

type fakeStore struct {
	mu    sync.Mutex
	files map[string][]byte
	n     int
}

func newFakeStore() *fakeStore {
	return &fakeStore{files: map[string][]byte{}}
}

func (s *fakeStore) Save(_ context.Context, folder string, r io.Reader, ext string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	key := fmt.Sprintf("%s/photo%d%s", folder, s.n, ext)
	s.files[key] = data
	return key, nil
}

func (s *fakeStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, key)
	return nil
}

func (s *fakeStore) URL(key string) string { return "/uploads/" + key }

// Just enough bytes for http.DetectContentType to recognise each format.
var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 600)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0}, 600)...)
	textBytes = []byte("this is not an image")
)
