package scrapperhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	domain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Service interface {
	RegisterChat(chatID int64) error
	DeleteChat(chatID int64) error
	AddLink(chatID int64, url string, tags []string) (domain.Link, error)
	RemoveLink(chatID int64, url string) (domain.Link, error)
	GetLinks(chatID int64) ([]domain.Link, error)
}

type Server struct {
	srv     *http.Server
	service Service
}

func NewServer(port string, service Service) *Server {
	mux := http.NewServeMux()

	server := &Server{
		service: service,
	}

	mux.HandleFunc("/tg-chat/{id}", server.updateChat)
	mux.HandleFunc("/links", server.links)

	server.srv = &http.Server{Handler: mux, Addr: ":" + port}
	return server
}

func (s *Server) Start() error {
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
		s.registerChat(w, id)
	case http.MethodDelete:
		s.deleteChat(w, id)
	default:
		slog.Error("wrong method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) registerChat(w http.ResponseWriter, id int64) {
	err := s.service.RegisterChat(id)
	if err != nil {
		slog.Error("cannot register chat", "id", id, "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteChat(w http.ResponseWriter, id int64) {
	err := s.service.DeleteChat(id)
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
	links, err := s.service.GetLinks(id)
	if err != nil {
		slog.Error("get links error", "error", err)
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	response := ListLinksResponse{Size: int32(len(links))}
	for _, l := range links {
		response.Links = append(response.Links, LinkResponse{ID: l.ID, URL: l.URL, Tags: l.Tags})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("cannot encode response", "response", response, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to encode response")
	}
}

func (s *Server) linksPost(w http.ResponseWriter, r *http.Request, id int64) {
	var addableLink AddLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&addableLink); err != nil {
		slog.Error("cannot parse request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	link, err := s.service.AddLink(id, addableLink.Link, addableLink.Tags)
	if err != nil {
		slog.Error("add link error", "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := LinkResponse{ID: link.ID, URL: link.URL, Tags: link.Tags}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("cannot encode response", "response", response, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to encode response")
	}
}

func (s *Server) linksDelete(w http.ResponseWriter, r *http.Request, id int64) {
	var removableLink RemoveLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&removableLink); err != nil {
		slog.Error("cannot parse request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	link, err := s.service.RemoveLink(id, removableLink.Link)
	if err != nil {
		slog.Error("remove link error", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := LinkResponse{ID: link.ID, URL: link.URL, Tags: link.Tags}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("cannot encode response", "response", response, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to encode response")
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
