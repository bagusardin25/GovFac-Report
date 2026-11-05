package main

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/bagusardin25/GovFac-Report/internal/auth"
	"github.com/bagusardin25/GovFac-Report/internal/platform/config"
	"github.com/bagusardin25/GovFac-Report/internal/platform/middleware"
)

// We wire every feature here, repository -> service -> handler, and nowhere else.
func newRouter(cfg config.Config, db *sql.DB, logger *slog.Logger) http.Handler {
	authRepo := auth.NewPostgresRepository(db)
	authService := auth.NewService(authRepo, cfg.SessionTTL)
	authHandler := auth.NewHandler(authService, logger)

	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)

	return middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.Logger(logger),
	)
}
