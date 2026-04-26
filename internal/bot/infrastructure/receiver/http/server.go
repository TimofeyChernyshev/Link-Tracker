package bothttp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Service interface {
	HandleUpdate(chatIDs []int64, desc string) error
}

type Server struct {
	srv *http.Server
}

func NewServer(service Service, port string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/updates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			slog.Error("wrong method", "method", r.Method)
			writeError(w, http.StatusMethodNotAllowed, "method not POST")
			return
		}

		var upd domain.LinkUpdate
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			slog.Error("cannot parse request", "error", err)
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		err := service.HandleUpdate(upd.TgChatIDs, upd.Description)
		if err != nil {
			slog.Error("cannot hadle update", "error", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
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

func (s *Server) Start(_ context.Context) error {
	err := s.srv.ListenAndServe()
	if err != nil {
		return fmt.Errorf("cannot start server: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.srv.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("cannot shutdown server: %w", err)
	}

	return nil
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(APIErrorResponse{
		Description: msg,
		Code:        http.StatusText(code),
	})
}
