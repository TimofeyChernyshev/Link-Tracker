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

const (
	defaultOffset = 0
)

type Service interface {
	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.Link, error)
	RemoveLink(ctx context.Context, chatID int64, url string) (domain.Link, error)
	GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error)
}

type Server struct {
	srv           *http.Server
	service       Service
	defaultLimit  int
	defaultOffset int
	maxLimit      int
}

func NewServer(port string, service Service, defaultLimit, maxLimit int) *Server {
	mux := http.NewServeMux()

	server := &Server{
		service:       service,
		defaultLimit:  defaultLimit,
		defaultOffset: defaultOffset,
		maxLimit:      maxLimit,
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
		s.registerChat(r.Context(), w, id)
	case http.MethodDelete:
		s.deleteChat(r.Context(), w, id)
	default:
		slog.Error("wrong method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) registerChat(ctx context.Context, w http.ResponseWriter, id int64) {
	err := s.service.RegisterChat(ctx, id)
	if err != nil {
		slog.Error("cannot register chat", "id", id, "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteChat(ctx context.Context, w http.ResponseWriter, id int64) {
	err := s.service.DeleteChat(ctx, id)
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
		s.linksGet(r, w, id)
	case http.MethodPost:
		s.linksPost(r.Context(), w, r, id)
	case http.MethodDelete:
		s.linksDelete(r.Context(), w, r, id)
	default:
		slog.Error("wrong method", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) linksGet(r *http.Request, w http.ResponseWriter, id int64) {
	limit, offset := s.parsePaginationParams(r)

	links, err := s.service.GetLinks(r.Context(), id, limit, offset)
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

func (s *Server) parsePaginationParams(r *http.Request) (limit, offset int) {
	limit = s.defaultLimit
	offset = s.defaultOffset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > s.maxLimit {
				limit = s.maxLimit
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

func (s *Server) linksPost(ctx context.Context, w http.ResponseWriter, r *http.Request, id int64) {
	var addableLink AddLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&addableLink); err != nil {
		slog.Error("cannot parse request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	link, err := s.service.AddLink(ctx, id, addableLink.Link, addableLink.Tags)
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

func (s *Server) linksDelete(ctx context.Context, w http.ResponseWriter, r *http.Request, id int64) {
	var removableLink RemoveLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&removableLink); err != nil {
		slog.Error("cannot parse request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	link, err := s.service.RemoveLink(ctx, id, removableLink.Link)
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
