package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type PhotoStore interface {
	Save(ctx context.Context, folder string, r io.Reader, ext string) (key string, err error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

type Service struct {
	repo   Repository
	photos PhotoStore
}

func NewService(repo Repository, photos PhotoStore) *Service {
	return &Service{repo: repo, photos: photos}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Report, error) {
	tags, errs := in.validate()
	if errs != nil {
		return nil, errs
	}

	// We sniff the real content instead of trusting the filename or the
	// client's Content-Type.
	head := make([]byte, 512)
	n, err := io.ReadFull(in.Photo, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read photo: %w", err)
	}
	head = head[:n]
	ext, ok := photoExtensions[http.DetectContentType(head)]
	if !ok {
		return nil, ValidationError{"photo": "Foto harus berformat JPG, PNG, atau WebP."}
	}

	key, err := s.photos.Save(ctx, "reports", io.MultiReader(bytes.NewReader(head), in.Photo), ext)
	if err != nil {
		return nil, fmt.Errorf("save photo: %w", err)
	}

	rep := &Report{
		UserID:      in.UserID,
		Title:       strings.TrimSpace(in.Title),
		Description: strings.TrimSpace(in.Description),
		PhotoKey:    key,
		Hashtags:    tags,
	}
	if err := s.repo.Create(ctx, rep); err != nil {
		_ = s.photos.Delete(ctx, key) // don't leave an orphaned file behind
		return nil, err
	}
	rep.PhotoURL = s.photos.URL(key)
	return rep, nil
}
