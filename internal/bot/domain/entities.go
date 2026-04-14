package domain

// Message - сообщение, присылаемое боту
type Message struct {
	Text      string
	ChatID    int64
	Username  string
	MessageID int
}

// Response - ответ бота
type Response struct {
	Text   string
	ChatID int64
}

type BotCommand struct {
	Name        string
	Description string
}

// Link - ссылка с тегами, котора отслеживается
type Link struct {
	URL  string
	Tags []string
}

// LinkUpdate - обновление ссылки, которое отправляется всем отслеживающим чатам
type LinkUpdate struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}
