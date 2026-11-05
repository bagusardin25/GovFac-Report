package notification

import (
	"net/http"

	"github.com/bagusardin25/GovFac-Report/internal/platform/middleware"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth middleware.Middleware) {
	mux.Handle("GET /api/v1/notifications", requireAuth(http.HandlerFunc(h.listMine)))
}
