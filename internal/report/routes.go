package report

import (
	"net/http"

	"github.com/bagusardin25/GovFac-Report/internal/platform/middleware"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth middleware.Middleware) {
	mux.Handle("POST /api/v1/reports", requireAuth(http.HandlerFunc(h.create)))
}
