package kafkanotifier

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"

type LinkUpdate struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	TgChatIDs   []int64 `json:"tgChatIds"`
	Author      string  `json:"author"`
}

func linkUpdateFromDomain(upd domain.LinkUpdate) *LinkUpdate {
	return &LinkUpdate{
		ID:          upd.ID,
		URL:         upd.URL,
		Description: upd.Description,
		TgChatIDs:   upd.ChatIDs,
		Author:      upd.Author,
	}
}
