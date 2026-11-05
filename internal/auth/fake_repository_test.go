package auth

import (
	"context"
	"sync"
	"time"
)

type fakeRepository struct {
	mu       sync.Mutex
	nextID   int64
	users    map[string]*User          // keyed by normalized email
	sessions map[string]*Session       // keyed by token hash
	resets   map[string]*PasswordReset // keyed by token hash
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		users:    map[string]*User{},
		sessions: map[string]*Session{},
		resets:   map[string]*PasswordReset{},
	}
}

func (f *fakeRepository) ReplacePasswordReset(_ context.Context, pr *PasswordReset) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, existing := range f.resets {
		if existing.UserID == pr.UserID {
			delete(f.resets, hash)
		}
	}
	stored := *pr
	f.resets[pr.TokenHash] = &stored
	return nil
}

func (f *fakeRepository) ResetPassword(_ context.Context, tokenHash, passwordHash string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.resets[tokenHash]
	if !ok || !pr.ExpiresAt.After(now) {
		return ErrResetTokenInvalid
	}
	delete(f.resets, tokenHash)
	for _, u := range f.users {
		if u.ID == pr.UserID {
			u.PasswordHash = passwordHash
		}
	}
	for hash, s := range f.sessions {
		if s.UserID == pr.UserID {
			delete(f.sessions, hash)
		}
	}
	return nil
}

type loginEvent struct {
	userID        int64
	ip, userAgent string
}

type fakeNotifier struct {
	mu     sync.Mutex
	events []loginEvent
}

func (n *fakeNotifier) LoginSucceeded(_ context.Context, userID int64, ip, userAgent string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, loginEvent{userID: userID, ip: ip, userAgent: userAgent})
}

type sentMail struct {
	to, subject, body string
}

type fakeMailer struct {
	sent chan sentMail
}

func newFakeMailer() *fakeMailer {
	return &fakeMailer{sent: make(chan sentMail, 10)}
}

func (m *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	m.sent <- sentMail{to: to, subject: subject, body: body}
	return nil
}

func (f *fakeRepository) CreateUser(_ context.Context, u *User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := normalizeEmail(u.Email)
	if _, exists := f.users[key]; exists {
		return ErrEmailTaken
	}
	f.nextID++
	u.ID = f.nextID
	u.CreatedAt = time.Now()
	stored := *u
	f.users[key] = &stored
	return nil
}

func (f *fakeRepository) FindUserByEmail(_ context.Context, email string) (*User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[normalizeEmail(email)]
	if !ok {
		return nil, ErrUserNotFound
	}
	found := *u
	return &found, nil
}

func (f *fakeRepository) CreateSession(_ context.Context, s *Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := *s
	f.sessions[s.TokenHash] = &stored
	return nil
}

func (f *fakeRepository) FindUserBySession(_ context.Context, tokenHash string, now time.Time) (*User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[tokenHash]
	if !ok || !s.ExpiresAt.After(now) {
		return nil, ErrSessionNotFound
	}
	for _, u := range f.users {
		if u.ID == s.UserID {
			found := *u
			found.PasswordHash = ""
			return &found, nil
		}
	}
	return nil, ErrSessionNotFound
}

func (f *fakeRepository) DeleteSession(_ context.Context, tokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, tokenHash)
	return nil
}
