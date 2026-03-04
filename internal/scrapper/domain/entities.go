package domain

import "time"

type Link struct {
	ID        int64
	URL       string
	Tags      []string
	UpdatedAt time.Time
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	ChatIds     []int64
}
