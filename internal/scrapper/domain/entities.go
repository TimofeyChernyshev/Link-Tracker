package domain

import "time"

type Link struct {
	ID        int64     `db:"id"`
	URL       string    `db:"url"`
	Tags      []string  `db:"tags"`
	UpdatedAt time.Time `db:"updated_at"`
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	ChatIDs     []int64
}

type CheckResult struct {
	Link        Link
	Events      []Event
	Error       error
	ProcessedAt time.Time
}
