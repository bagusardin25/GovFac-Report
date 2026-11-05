package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// This is our side of auth.LoginNotifier. A failed insert must never block
// the login itself, so we log it instead of returning it.
func (s *Service) LoginSucceeded(ctx context.Context, userID int64, ip, userAgent string) {
	device := describeDevice(userAgent)
	data, _ := json.Marshal(map[string]string{
		"ip":         ip,
		"device":     device,
		"user_agent": userAgent,
	})

	n := &Notification{
		UserID:  userID,
		Type:    TypeLogin,
		Title:   "Login berhasil",
		Message: fmt.Sprintf("Akun kamu baru saja masuk dari %s (IP %s). Kalau ini bukan kamu, segera ganti password.", device, ip),
		Data:    data,
	}
	if err := s.repo.Create(ctx, n); err != nil {
		s.logger.Error("create login notification", "user_id", userID, "error", err)
	}
}

func (s *Service) ListMine(ctx context.Context, userID int64, page, perPage int) (*Page, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = defaultPerPage
	}
	perPage = min(perPage, maxPerPage)

	list, total, err := s.repo.ListByUser(ctx, userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, err
	}
	return &Page{Notifications: list, Page: page, PerPage: perPage, Total: total}, nil
}
