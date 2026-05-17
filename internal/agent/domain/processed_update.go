package domain

type ProcessedUpdate struct {
	ID          int64    `json:"id"`
	Description string   `json:"description"`
	TgChatIDs   []int64  `json:"tgChatIds"`
	Priority    Priority `json:"priority"`
}
