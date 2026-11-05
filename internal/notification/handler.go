package notification

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bagusardin25/GovFac-Report/internal/platform/httpx"
	"github.com/bagusardin25/GovFac-Report/internal/platform/identity"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

type notificationResponse struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

type pageResponse struct {
	Notifications []notificationResponse `json:"notifications"`
	Page          int                    `json:"page"`
	PerPage       int                    `json:"per_page"`
	Total         int                    `json:"total"`
}

func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	p, err := h.svc.ListMine(r.Context(), identity.FromContext(r.Context()).ID, page, perPage)
	if err != nil {
		httpx.ServerError(w, r, h.logger, err)
		return
	}

	resp := pageResponse{
		Notifications: make([]notificationResponse, 0, len(p.Notifications)),
		Page:          p.Page,
		PerPage:       p.PerPage,
		Total:         p.Total,
	}
	for _, n := range p.Notifications {
		resp.Notifications = append(resp.Notifications, notificationResponse{
			ID:        n.ID,
			Type:      n.Type,
			Title:     n.Title,
			Message:   n.Message,
			Data:      n.Data,
			CreatedAt: n.CreatedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, resp)
}
