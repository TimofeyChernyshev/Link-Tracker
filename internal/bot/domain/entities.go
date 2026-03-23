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
