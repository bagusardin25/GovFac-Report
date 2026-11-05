package report

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bagusardin25/GovFac-Report/internal/platform/httpx"
	"github.com/bagusardin25/GovFac-Report/internal/platform/identity"
)

// Photo limit plus headroom for the text fields and multipart boundaries.
const maxCreateBodyBytes = maxPhotoBytes + 1<<20

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

type reportResponse struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	PhotoURL    string           `json:"photo_url"`
	Hashtags    []string         `json:"hashtags"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Timeline    []statusResponse `json:"timeline,omitempty"`
}

type statusResponse struct {
	Status string    `json:"status"`
	Note   string    `json:"note"`
	At     time.Time `json:"at"`
}

type pageResponse struct {
	Reports []reportResponse `json:"reports"`
	Page    int              `json:"page"`
	PerPage int              `json:"per_page"`
	Total   int              `json:"total"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBodyBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "Ukuran foto maksimal 5 MB.")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "Request harus berupa multipart/form-data.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	in := CreateInput{
		UserID:      identity.FromContext(r.Context()).ID,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Hashtags:    r.MultipartForm.Value["hashtags"],
	}
	file, header, err := r.FormFile("photo")
	switch {
	case errors.Is(err, http.ErrMissingFile):
	case err != nil:
		httpx.Error(w, http.StatusBadRequest, "Foto tidak bisa dibaca.")
		return
	default:
		defer file.Close()
		in.Photo = file
		in.PhotoSize = header.Size
	}

	rep, err := h.svc.Create(r.Context(), in)
	var invalid ValidationError
	switch {
	case errors.As(err, &invalid):
		httpx.ValidationError(w, invalid)
	case err != nil:
		httpx.ServerError(w, r, h.logger, err)
	default:
		httpx.JSON(w, http.StatusCreated, map[string]any{"report": toResponse(rep)})
	}
}

func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	// Bad or missing values fall back to the defaults in the service.
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	p, err := h.svc.ListMine(r.Context(), identity.FromContext(r.Context()).ID, page, perPage)
	if err != nil {
		httpx.ServerError(w, r, h.logger, err)
		return
	}

	resp := pageResponse{Reports: make([]reportResponse, 0, len(p.Reports)), Page: p.Page, PerPage: p.PerPage, Total: p.Total}
	for _, rep := range p.Reports {
		resp.Reports = append(resp.Reports, toResponse(rep))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) getMine(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Laporan tidak ditemukan.")
		return
	}

	rep, err := h.svc.GetMine(r.Context(), identity.FromContext(r.Context()).ID, id)
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "Laporan tidak ditemukan.")
	case err != nil:
		httpx.ServerError(w, r, h.logger, err)
	default:
		httpx.JSON(w, http.StatusOK, map[string]any{"report": toResponse(rep)})
	}
}

func toResponse(r *Report) reportResponse {
	resp := reportResponse{
		ID:          r.ID,
		Title:       r.Title,
		Description: r.Description,
		PhotoURL:    r.PhotoURL,
		Hashtags:    r.Hashtags,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if resp.Hashtags == nil {
		resp.Hashtags = []string{}
	}
	for _, c := range r.Timeline {
		resp.Timeline = append(resp.Timeline, statusResponse{Status: c.Status, Note: c.Note, At: c.At})
	}
	return resp
}
