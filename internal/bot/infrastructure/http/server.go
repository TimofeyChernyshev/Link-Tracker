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

type Server struct {
	srv *http.Server
}

func NewServer(bot Sender, port string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/updates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var upd domain.LinkUpdate
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			w.WriteHeader(http.StatusBadRequest)
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

func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
