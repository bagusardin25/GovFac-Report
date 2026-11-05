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
	// Spread creation times out so ordering by newest is deterministic.
	r.CreatedAt = time.Now().Add(time.Duration(f.nextID) * time.Second)
	r.UpdatedAt = r.CreatedAt
	stored := *r
	stored.Timeline = []StatusChange{{Status: r.Status, At: r.CreatedAt}}
	f.reports = append(f.reports, &stored)
	return nil
}

func (f *fakeRepository) ListByUser(_ context.Context, userID int64, limit, offset int) ([]*Report, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var mine []*Report
	for i := len(f.reports) - 1; i >= 0; i-- { // newest first
		if f.reports[i].UserID == userID {
			cp := *f.reports[i]
			cp.Timeline = nil
			mine = append(mine, &cp)
		}
	}
	total := len(mine)
	if offset >= total {
		return []*Report{}, total, nil
	}
	return mine[offset:min(offset+limit, total)], total, nil
}

func (f *fakeRepository) FindForUser(_ context.Context, id, userID int64) (*Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.reports {
		if r.ID == id && r.UserID == userID {
			cp := *r
			return &cp, nil
		}
	}
	return nil, ErrNotFound
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
