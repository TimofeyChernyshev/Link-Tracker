package bot_server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

type Service interface {
	SendUpdates(chatIDs []int64, desc string)
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

		var upd LinkUpdate
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			slog.Error("cannot parse request", "error", err)
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}

		service.SendUpdates(upd.TgChatIDs, upd.Description)

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

	_ = json.NewEncoder(w).Encode(APIErrorResponse{
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
