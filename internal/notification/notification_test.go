package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bagusardin25/GovFac-Report/internal/platform/identity"
)

type fakeRepository struct {
	mu      sync.Mutex
	nextID  int64
	items   []*Notification
	failNow bool
}

func (f *fakeRepository) Create(_ context.Context, n *Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNow {
		return errors.New("db down")
	}
	f.nextID++
	n.ID = f.nextID
	n.CreatedAt = time.Now().Add(time.Duration(f.nextID) * time.Second)
	cp := *n
	f.items = append(f.items, &cp)
	return nil
}

func (f *fakeRepository) ListByUser(_ context.Context, userID int64, limit, offset int) ([]*Notification, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var mine []*Notification
	for i := len(f.items) - 1; i >= 0; i-- {
		if f.items[i].UserID == userID {
			mine = append(mine, f.items[i])
		}
	}
	total := len(mine)
	if offset >= total {
		return []*Notification{}, total, nil
	}
	return mine[offset:min(offset+limit, total)], total, nil
}

const (
	chromeWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
	chromeAndroid = "Mozilla/5.0 (Linux; Android 14; SM-A546E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36"
	safariIPhone  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
	edgeWindows   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0"
	firefoxLinux  = "Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0"
)

func TestDescribeDevice(t *testing.T) {
	tests := map[string]string{
		chromeWindows:           "Chrome di Windows",
		chromeAndroid:           "Chrome di Android",
		safariIPhone:            "Safari di iPhone",
		edgeWindows:             "Edge di Windows",
		firefoxLinux:            "Firefox di Linux",
		"curl/8.9.1":            "curl/8.9.1",
		"":                      "perangkat tidak dikenal",
		"PostmanRuntime/7.42.0": "PostmanRuntime/7.42.0",
	}
	for ua, want := range tests {
		if got := describeDevice(ua); got != want {
			t.Errorf("describeDevice(%q) = %q, want %q", ua, got, want)
		}
	}
}

func newTestService() (*Service, *fakeRepository) {
	repo := &fakeRepository{}
	return NewService(repo, slog.New(slog.DiscardHandler)), repo
}

func TestLoginSucceeded(t *testing.T) {
	svc, repo := newTestService()
	svc.LoginSucceeded(context.Background(), 7, "10.0.0.5", chromeWindows)

	if len(repo.items) != 1 {
		t.Fatalf("got %d notifications, want 1", len(repo.items))
	}
	n := repo.items[0]
	if n.UserID != 7 || n.Type != TypeLogin {
		t.Errorf("unexpected notification: %+v", n)
	}
	if !strings.Contains(n.Message, "Chrome di Windows") || !strings.Contains(n.Message, "10.0.0.5") {
		t.Errorf("message = %q", n.Message)
	}
	var data map[string]string
	if err := json.Unmarshal(n.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["ip"] != "10.0.0.5" || data["device"] != "Chrome di Windows" || data["user_agent"] != chromeWindows {
		t.Errorf("data = %v", data)
	}
}

func TestLoginSucceeded_SwallowsStorageErrors(t *testing.T) {
	svc, repo := newTestService()
	repo.failNow = true
	svc.LoginSucceeded(context.Background(), 7, "10.0.0.5", chromeWindows) // must not panic or block
}

func TestListMine(t *testing.T) {
	svc, _ := newTestService()
	for i := 0; i < 3; i++ {
		svc.LoginSucceeded(context.Background(), 7, fmt.Sprintf("10.0.0.%d", i), chromeWindows)
	}
	svc.LoginSucceeded(context.Background(), 8, "10.9.9.9", chromeAndroid)

	p, err := svc.ListMine(context.Background(), 7, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 3 || len(p.Notifications) != 2 || !strings.Contains(p.Notifications[0].Message, "10.0.0.2") {
		t.Errorf("page = total %d, len %d", p.Total, len(p.Notifications))
	}

	p, _ = svc.ListMine(context.Background(), 7, 0, 999)
	if p.Page != 1 || p.PerPage != maxPerPage {
		t.Errorf("clamping: page %d per_page %d", p.Page, p.PerPage)
	}
}

func TestListEndpoint(t *testing.T) {
	svc, _ := newTestService()
	svc.LoginSucceeded(context.Background(), 7, "10.0.0.5", safariIPhone)
	svc.LoginSucceeded(context.Background(), 8, "10.9.9.9", chromeAndroid)

	mux := http.NewServeMux()
	asUser7 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(identity.WithUser(r.Context(), &identity.User{ID: 7})))
		})
	}
	NewHandler(svc, slog.New(slog.DiscardHandler)).RegisterRoutes(mux, asUser7)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got pageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || len(got.Notifications) != 1 {
		t.Fatalf("user 7 should only see their own notification: %+v", got)
	}
	if n := got.Notifications[0]; n.Type != TypeLogin || !strings.Contains(string(n.Data), `"device":"Safari di iPhone"`) {
		t.Errorf("unexpected item: %+v", n)
	}
}
