package auth

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
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

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type loginResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"token_type"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      userResponse `json:"user"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !httpx.Decode(w, r, &req) {
		return
	}

	u, err := h.svc.Register(r.Context(), RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	var invalid ValidationError
	switch {
	case errors.As(err, &invalid):
		httpx.ValidationError(w, invalid)
	case errors.Is(err, ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, "Email sudah terdaftar.")
	case err != nil:
		httpx.ServerError(w, r, h.logger, err)
	default:
		httpx.JSON(w, http.StatusCreated, map[string]any{"user": toUserResponse(u)})
	}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.Decode(w, r, &req) {
		return
	}

	res, err := h.svc.Login(r.Context(), LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: clientIP(r),
		UserAgent: r.UserAgent(),
	})
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, "Email atau password salah.")
	case err != nil:
		httpx.ServerError(w, r, h.logger, err)
	default:
		httpx.JSON(w, http.StatusOK, loginResponse{
			Token:     res.Token,
			TokenType: "Bearer",
			ExpiresAt: res.ExpiresAt,
			User:      toUserResponse(res.User),
		})
	}
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), bearerToken(r)); err != nil {
		httpx.ServerError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u := identity.FromContext(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]any{"user": userResponse{
		ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role,
	}})
}

func toUserResponse(u *User) userResponse {
	return userResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role}
}

// We take the TCP peer address for now. Once we sit behind a reverse proxy
// this needs to read the proxy's forwarded header instead.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
