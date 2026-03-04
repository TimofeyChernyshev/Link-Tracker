package bot_server

type LinkUpdate struct {
	Id          int64   `json:"id"`
	Url         string  `json:"url"`
	Description string  `json:"description"`
	TgChatIds   []int64 `json:"tgChatIds"`
}

type ApiErrorResponse struct {
	Description      string   `json:"description"`
	Code             string   `json:"code"`
	ExceptionName    string   `json:"exceptionName,omitempty"`
	ExceptionMessage string   `json:"exceptionMessage,omitempty"`
	Stacktrace       []string `json:"stacktrace,omitempty"`
}
