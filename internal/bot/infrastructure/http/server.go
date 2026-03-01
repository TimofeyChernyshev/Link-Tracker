package bot_server

import (
	"context"
	"encoding/json"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Sender interface {
	SendMessage(response *domain.Response)
}

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

type Server struct {
	srv *http.Server
}

func NewServer(bot Sender, port string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/updates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not POST")
			return
		}

		var upd LinkUpdate
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		for _, chatID := range upd.TgChatIds {
			bot.SendMessage(&domain.Response{
				ChatID: chatID,
				Text:   upd.Description,
			})
		}

		w.WriteHeader(http.StatusOK)
	})

	return &Server{
		srv: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(ApiErrorResponse{
		Description: msg,
		Code:        http.StatusText(code),
	})
}

func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
