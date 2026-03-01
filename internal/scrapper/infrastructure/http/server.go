package scrapper_server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

// Storage - контракт хранилища ссылок
type Storage interface {
	RegisterChat(chatId int64) error
	DeleteChat(chatId int64) error
	ChatExists(chatId int64) bool
	AddLink(chatId int64, url string, tags []string) (domain.Link, error)
	RemoveLink(chatId int64, url string) (domain.Link, error)
	GetLinks(chatId int64) []domain.Link
}

type Server struct {
	srv     *http.Server
	storage Storage
}

func NewServer(port string, storage Storage) *Server {
	mux := http.NewServeMux()

	server := &Server{
		storage: storage,
	}

	mux.HandleFunc("/tg-chat/{id}", server.updateChat)
	mux.HandleFunc("/links", server.links)

	server.srv = &http.Server{Handler: mux, Addr: ":" + port}
	return server
}

// updateChat - хендлер для ручки /tg-chat/{id}
func (s *Server) updateChat(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")
	id, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		slog.Error("cannot parse id to int64", "idString", idString, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.updateChatPost(w, id)
	case http.MethodDelete:
		s.updateChatDelete(w, id)
	default:
		slog.Error("wrong method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) updateChatPost(w http.ResponseWriter, id int64) {
	err := s.storage.RegisterChat(id)
	if err != nil {
		slog.Error("cannot register chat", "id", id, "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) updateChatDelete(w http.ResponseWriter, id int64) {
	err := s.storage.DeleteChat(id)
	if err != nil {
		slog.Error("cannot delete chat", "id", id, "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// links - хендлер для ручки /links
func (s *Server) links(w http.ResponseWriter, r *http.Request) {
	idString := r.Header.Get("Tg-Chat-Id")
	id, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		slog.Error("cannot parse id to int64", "idString", idString, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	exists := s.storage.ChatExists(id)
	if !exists {
		slog.Error("chat doesn't exist", "id", id)
		writeError(w, http.StatusNotFound, "chat doesn't exist")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.linksGet(w, id)
	case http.MethodPost:
		s.linksPost(w, r, id)
	case http.MethodDelete:
		s.linksDelete(w, r, id)
	default:
		slog.Error("wrong method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) linksGet(w http.ResponseWriter, id int64) {
	links := s.storage.GetLinks(id)

	response := ListLinksResponse{Size: int32(len(links))}
	for _, l := range links {
		response.Links = append(response.Links, LinkResponse{Id: l.ID, Url: l.URL, Tags: l.Tags})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) linksPost(w http.ResponseWriter, r *http.Request, id int64) {
	var addableLink AddLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&addableLink); err != nil {
		slog.Error("cannot parse request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	link, err := s.storage.AddLink(id, addableLink.Link, addableLink.Tags)
	if err != nil {
		slog.Error("add link error", "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := LinkResponse{Id: link.ID, Url: link.URL, Tags: link.Tags}
	json.NewEncoder(w).Encode(response)
}

func (s *Server) linksDelete(w http.ResponseWriter, r *http.Request, id int64) {
	var removableLink RemoveLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&removableLink); err != nil {
		slog.Error("cannot parse request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	link, err := s.storage.RemoveLink(id, removableLink.Link)
	if err != nil {
		slog.Error("remove link error", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := LinkResponse{Id: link.ID, Url: link.URL, Tags: link.Tags}
	json.NewEncoder(w).Encode(response)
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
