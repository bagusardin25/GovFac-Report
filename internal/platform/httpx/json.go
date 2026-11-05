package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const maxBodyBytes = 1 << 20

type errorBody struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, errorBody{Error: message})
}

func ValidationError(w http.ResponseWriter, fields map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, errorBody{Error: "Data tidak valid.", Fields: fields})
}

func ServerError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	logger.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err)
	Error(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
}

// Decode writes the 400 itself, so handlers can just return when it fails.
func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		Error(w, http.StatusBadRequest, "Body request harus berupa JSON yang valid.")
		return false
	}
	return true
}
