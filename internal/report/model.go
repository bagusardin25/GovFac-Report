package report

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	StatusAdminReview = "admin_review"
	StatusApproved    = "approved"
	StatusRejected    = "rejected"
)

type Report struct {
	ID          int64
	UserID      int64
	Title       string
	Description string
	PhotoKey    string
	PhotoURL    string
	Hashtags    []string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Timeline    []StatusChange
}

type StatusChange struct {
	Status string
	Note   string
	At     time.Time
}

type Page struct {
	Reports []*Report
	Page    int
	PerPage int
	Total   int
}

var ErrNotFound = errors.New("report: not found")

const (
	defaultPerPage = 20
	maxPerPage     = 50
)

type CreateInput struct {
	UserID      int64
	Title       string
	Description string
	Hashtags    []string // raw form values, e.g. "#jalanrusak #lampujalan"
	Photo       io.Reader
	PhotoSize   int64
}

type ValidationError map[string]string

func (v ValidationError) Error() string { return "report: invalid input" }

const (
	maxTitleChars       = 120
	maxDescriptionChars = 2000
	maxHashtags         = 5
	maxPhotoBytes       = 5 << 20
)

var photoExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var hashtagPattern = regexp.MustCompile(`^[a-z0-9_]{2,30}$`)

func (in CreateInput) validate() ([]string, ValidationError) {
	errs := ValidationError{}

	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		errs["title"] = "Judul wajib diisi."
	case utf8.RuneCountInString(title) > maxTitleChars:
		errs["title"] = "Judul maksimal 120 karakter."
	}

	description := strings.TrimSpace(in.Description)
	switch {
	case description == "":
		errs["description"] = "Deskripsi wajib diisi."
	case utf8.RuneCountInString(description) > maxDescriptionChars:
		errs["description"] = "Deskripsi maksimal 2000 karakter."
	}

	tags, msg := parseHashtags(in.Hashtags)
	if msg != "" {
		errs["hashtags"] = msg
	}

	switch {
	case in.Photo == nil:
		errs["photo"] = "Foto bukti wajib diunggah."
	case in.PhotoSize > maxPhotoBytes:
		errs["photo"] = "Ukuran foto maksimal 5 MB."
	}

	if len(errs) == 0 {
		return tags, nil
	}
	return nil, errs
}

// We accept tags the way people type them on social media: with or without
// "#", separated by spaces or commas, in any case.
func parseHashtags(raw []string) ([]string, string) {
	seen := make(map[string]bool)
	var tags []string
	for _, value := range raw {
		parts := strings.FieldsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == ',' })
		for _, part := range parts {
			tag := strings.ToLower(strings.TrimLeft(part, "#"))
			if tag == "" {
				continue
			}
			if !hashtagPattern.MatchString(tag) {
				return nil, "Tagar hanya boleh berisi huruf, angka, dan garis bawah (2–30 karakter)."
			}
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	switch {
	case len(tags) == 0:
		return nil, "Tambahkan minimal satu tagar kategori, contoh: #jalanrusak."
	case len(tags) > maxHashtags:
		return nil, "Maksimal 5 tagar."
	}
	return tags, ""
}
