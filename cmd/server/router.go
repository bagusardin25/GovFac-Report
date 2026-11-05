package main

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/bagusardin25/GovFac-Report/internal/auth"
	"github.com/bagusardin25/GovFac-Report/internal/notification"
	"github.com/bagusardin25/GovFac-Report/internal/platform/config"
	"github.com/bagusardin25/GovFac-Report/internal/platform/mailer"
	"github.com/bagusardin25/GovFac-Report/internal/platform/middleware"
	"github.com/bagusardin25/GovFac-Report/internal/platform/storage"
	"github.com/bagusardin25/GovFac-Report/internal/report"
)

const uploadsPath = "/uploads/"

// We wire every feature here, repository -> service -> handler, and nowhere else.
func newRouter(cfg config.Config, db *sql.DB, logger *slog.Logger) (http.Handler, error) {
	mail, err := newMailer(cfg.SMTP, logger)
	if err != nil {
		return nil, err
	}
	uploads, err := storage.NewLocal(cfg.UploadDir, uploadsPath)
	if err != nil {
		return nil, err
	}

	notificationRepo := notification.NewPostgresRepository(db)
	notificationService := notification.NewService(notificationRepo, logger)
	notificationHandler := notification.NewHandler(notificationService, logger)

	authRepo := auth.NewPostgresRepository(db)
	authService := auth.NewService(authRepo, mail, notificationService, auth.Options{
		SessionTTL:       cfg.SessionTTL,
		PasswordResetURL: cfg.PasswordResetURL,
	})
	authHandler := auth.NewHandler(authService, logger)

	reportRepo := report.NewPostgresRepository(db)
	reportService := report.NewService(reportRepo, uploads)
	reportHandler := report.NewHandler(reportService, logger)

	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)
	reportHandler.RegisterRoutes(mux, authHandler.RequireAuth)
	notificationHandler.RegisterRoutes(mux, authHandler.RequireAuth)
	mux.Handle("GET "+uploadsPath, http.StripPrefix(uploadsPath, uploads.Handler()))

	return middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.Logger(logger),
	), nil
}

func newMailer(cfg config.SMTP, logger *slog.Logger) (auth.Mailer, error) {
	if cfg.Host == "" {
		logger.Warn("SMTP_HOST not set; emails will be written to the log instead of sent")
		return mailer.NewLog(logger), nil
	}
	return mailer.NewSMTP(cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.From)
}
