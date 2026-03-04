package domain

import "time"

type Link struct {
	ID        int64
	URL       string
	Tags      []string
	UpdatedAt time.Time
}

type LinkUpdate struct {
	Id          int64
	Url         string
	Description string
	TgChatIds   []int64
}
