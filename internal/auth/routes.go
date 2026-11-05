package auth

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.Handle("POST /api/v1/auth/logout", h.RequireAuth(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/v1/auth/me", h.RequireAuth(http.HandlerFunc(h.me)))
	mux.HandleFunc("POST /api/v1/auth/password/forgot", h.forgotPassword)
	mux.HandleFunc("POST /api/v1/auth/password/reset", h.resetPassword)
}
