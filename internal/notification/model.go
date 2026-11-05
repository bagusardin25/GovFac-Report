package notification

import (
	"encoding/json"
	"time"
)

const TypeLogin = "login"

type Notification struct {
	ID        int64
	UserID    int64
	Type      string
	Title     string
	Message   string
	Data      json.RawMessage
	CreatedAt time.Time
}

type Page struct {
	Notifications []*Notification
	Page          int
	PerPage       int
	Total         int
}

const (
	defaultPerPage = 20
	maxPerPage     = 50
)
