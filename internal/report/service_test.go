package report

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func newTestService() (*Service, *fakeRepository, *fakeStore) {
	repo := &fakeRepository{}
	store := newFakeStore()
	return NewService(repo, store), repo, store
}

func validInput() CreateInput {
	return CreateInput{
		UserID:      7,
		Title:       "  Jalan berlubang di Jl. Raya Darmo ",
		Description: "Lubang cukup dalam di lajur kiri.",
		Hashtags:    []string{"#JalanRusak #darmo"},
		Photo:       bytes.NewReader(pngBytes),
		PhotoSize:   int64(len(pngBytes)),
	}
}

func TestCreate(t *testing.T) {
	svc, repo, store := newTestService()

	rep, err := svc.Create(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rep.ID == 0 || rep.UserID != 7 || rep.Status != StatusAdminReview {
		t.Errorf("unexpected report: %+v", rep)
	}
	if rep.Title != "Jalan berlubang di Jl. Raya Darmo" {
		t.Errorf("title not trimmed: %q", rep.Title)
	}
	if !reflect.DeepEqual(rep.Hashtags, []string{"jalanrusak", "darmo"}) {
		t.Errorf("hashtags = %v", rep.Hashtags)
	}
	if !strings.HasSuffix(rep.PhotoKey, ".png") || rep.PhotoURL != "/uploads/"+rep.PhotoKey {
		t.Errorf("photo key/url = %q / %q", rep.PhotoKey, rep.PhotoURL)
	}
	if !bytes.Equal(store.files[rep.PhotoKey], pngBytes) {
		t.Error("stored photo differs from the upload (sniffed bytes lost?)")
	}
	if len(repo.reports) != 1 {
		t.Errorf("repo has %d reports, want 1", len(repo.reports))
	}
}

func TestCreate_DetectsFormatFromContent(t *testing.T) {
	svc, _, _ := newTestService()
	in := validInput()
	in.Photo, in.PhotoSize = bytes.NewReader(jpegBytes), int64(len(jpegBytes))

	rep, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(rep.PhotoKey, ".jpg") {
		t.Errorf("key = %q, want .jpg", rep.PhotoKey)
	}
}

func TestCreate_Validation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*CreateInput)
		field  string
	}{
		{"empty title", func(in *CreateInput) { in.Title = " " }, "title"},
		{"long title", func(in *CreateInput) { in.Title = strings.Repeat("a", 121) }, "title"},
		{"empty description", func(in *CreateInput) { in.Description = "" }, "description"},
		{"long description", func(in *CreateInput) { in.Description = strings.Repeat("a", 2001) }, "description"},
		{"no hashtags", func(in *CreateInput) { in.Hashtags = nil }, "hashtags"},
		{"no photo", func(in *CreateInput) { in.Photo = nil }, "photo"},
		{"photo too big", func(in *CreateInput) { in.PhotoSize = maxPhotoBytes + 1 }, "photo"},
		{"not an image", func(in *CreateInput) { in.Photo = bytes.NewReader(textBytes) }, "photo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, store := newTestService()
			in := validInput()
			tt.modify(&in)

			_, err := svc.Create(context.Background(), in)
			var invalid ValidationError
			if !errors.As(err, &invalid) || invalid[tt.field] == "" {
				t.Fatalf("err = %v, want a ValidationError on %q", err, tt.field)
			}
			if len(repo.reports) != 0 || len(store.files) != 0 {
				t.Error("nothing should be stored for invalid input")
			}
		})
	}
}

func TestCreate_RemovesPhotoWhenInsertFails(t *testing.T) {
	svc, repo, store := newTestService()
	repo.failNow = true

	if _, err := svc.Create(context.Background(), validInput()); err == nil {
		t.Fatal("expected an error")
	}
	if len(store.files) != 0 {
		t.Errorf("orphaned photo left behind: %v", store.files)
	}
}

func TestParseHashtags(t *testing.T) {
	tests := []struct {
		raw     []string
		want    []string
		wantErr bool
	}{
		{[]string{"#jalanrusak #LampuJalan"}, []string{"jalanrusak", "lampujalan"}, false},
		{[]string{"jalanrusak, trotoar", "#got_mampet"}, []string{"jalanrusak", "trotoar", "got_mampet"}, false},
		{[]string{"#banjir #Banjir banjir"}, []string{"banjir"}, false},
		{[]string{"##jalan"}, []string{"jalan"}, false},
		{[]string{""}, nil, true},
		{[]string{"#a"}, nil, true},
		{[]string{"#jalan-rusak"}, nil, true},
		{[]string{"#a1 #a2 #a3 #a4 #a5 #a6"}, nil, true},
	}
	for _, tt := range tests {
		got, msg := parseHashtags(tt.raw)
		if (msg != "") != tt.wantErr {
			t.Errorf("parseHashtags(%q) msg = %q, wantErr %v", tt.raw, msg, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseHashtags(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}
