package report

import (
	"net/http"

	"github.com/bagusardin25/GovFac-Report/internal/platform/middleware"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth middleware.Middleware) {
	mux.Handle("POST /api/v1/reports", requireAuth(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/reports/mine", requireAuth(http.HandlerFunc(h.listMine)))
	mux.Handle("GET /api/v1/reports/{id}", requireAuth(http.HandlerFunc(h.getMine)))
}
