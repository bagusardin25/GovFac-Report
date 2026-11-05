package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bagusardin25/GovFac-Report/internal/platform/httpx"
	"github.com/bagusardin25/GovFac-Report/internal/platform/identity"
)

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			unauthorized(w, "Silakan login terlebih dahulu.")
			return
		}

		u, err := h.svc.Authenticate(r.Context(), token)
		if errors.Is(err, ErrSessionNotFound) {
			unauthorized(w, "Sesi tidak valid atau sudah berakhir.")
			return
		}
		if err != nil {
			httpx.ServerError(w, r, h.logger, err)
			return
		}

		ctx := identity.WithUser(r.Context(), &identity.User{
			ID:    u.ID,
			Name:  u.Name,
			Email: u.Email,
			Role:  u.Role,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.Error(w, http.StatusUnauthorized, message)
}
