package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
)

func createFor(t *testing.T, svc *Service, userID int64, title string) *Report {
	t.Helper()
	in := validInput()
	in.UserID = userID
	in.Title = title
	in.Photo = bytes.NewReader(pngBytes)
	rep, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return rep
}

func TestListMine(t *testing.T) {
	svc, _, _ := newTestService()
	for i := 1; i <= 3; i++ {
		createFor(t, svc, 7, fmt.Sprintf("laporan %d", i))
	}
	createFor(t, svc, 8, "punya orang lain")

	p, err := svc.ListMine(context.Background(), 7, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 3 || len(p.Reports) != 2 || p.Page != 1 || p.PerPage != 2 {
		t.Fatalf("page 1 = total %d, len %d, page %d, per_page %d", p.Total, len(p.Reports), p.Page, p.PerPage)
	}
	if p.Reports[0].Title != "laporan 3" {
		t.Errorf("first item = %q, want newest first", p.Reports[0].Title)
	}
	if p.Reports[0].PhotoURL == "" {
		t.Error("photo url missing in list")
	}

	p2, err := svc.ListMine(context.Background(), 7, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Reports) != 1 || p2.Reports[0].Title != "laporan 1" {
		t.Errorf("page 2 = %d items", len(p2.Reports))
	}
}

func TestListMine_ClampsPaging(t *testing.T) {
	svc, _, _ := newTestService()

	p, err := svc.ListMine(context.Background(), 7, -3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.Page != 1 || p.PerPage != defaultPerPage {
		t.Errorf("defaults: page %d per_page %d", p.Page, p.PerPage)
	}
	if p.Reports == nil {
		t.Error("an empty list should be [], not nil")
	}

	p, _ = svc.ListMine(context.Background(), 7, 1, 1000)
	if p.PerPage != maxPerPage {
		t.Errorf("per_page = %d, want capped at %d", p.PerPage, maxPerPage)
	}
}

func TestGetMine(t *testing.T) {
	svc, _, _ := newTestService()
	mine := createFor(t, svc, 7, "punya saya")
	theirs := createFor(t, svc, 8, "punya orang lain")

	rep, err := svc.GetMine(context.Background(), 7, mine.ID)
	if err != nil {
		t.Fatalf("GetMine: %v", err)
	}
	if len(rep.Timeline) != 1 || rep.Timeline[0].Status != StatusAdminReview {
		t.Errorf("timeline = %+v, want one admin_review entry", rep.Timeline)
	}

	if _, err := svc.GetMine(context.Background(), 7, theirs.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("someone else's report: err = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetMine(context.Background(), 7, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing report: err = %v, want ErrNotFound", err)
	}
}
