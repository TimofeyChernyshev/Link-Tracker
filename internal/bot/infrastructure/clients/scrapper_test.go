package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type ScrapperClientSuite struct {
	suite.Suite
	client  *ScrapperClient
	server  *httptest.Server
	handler http.HandlerFunc
	ctx     context.Context
}

func (s *ScrapperClientSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ScrapperClientSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

func TestScrapperClientSuite(t *testing.T) {
	suite.Run(t, new(ScrapperClientSuite))
}

func (s *ScrapperClientSuite) TestAddLink_Success() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.assertCommonHeaders(r, http.MethodPost, "/links", 12345)
		s.Equal("application/json", r.Header.Get("Content-Type"))

		var req AddLinkRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		s.NoError(err)
		s.Equal("https://github.com/test", req.Link)
		s.Equal([]string{"tag1", "tag2"}, req.Tags)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LinkResponse{
			Id:   1,
			Url:  "https://github.com/test",
			Tags: []string{"tag1", "tag2"},
		})
	})

	s.startServer(handler)
	err := s.client.AddLink(s.ctx, 12345, "https://github.com/test", []string{"tag1", "tag2"})
	s.NoError(err)
}

func (s *ScrapperClientSuite) TestAddLink_Error() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.errorResponse(w, http.StatusBadRequest, "INVALID_LINK", "Invalid link format")
	})

	s.startServer(handler)
	err := s.client.AddLink(s.ctx, 12345, "invalid", []string{"tag1"})

	s.Error(err)
	s.Contains(err.Error(), "INVALID_LINK")
	s.Contains(err.Error(), "Invalid link format")
}

func (s *ScrapperClientSuite) TestRemoveLink_Success() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.assertCommonHeaders(r, http.MethodDelete, "/links", 12345)

		var req RemoveLinkRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		s.NoError(err)
		s.Equal("https://github.com/test", req.Link)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LinkResponse{
			Id:   1,
			Url:  "https://github.com/test",
			Tags: []string{},
		})
	})

	s.startServer(handler)
	err := s.client.RemoveLink(s.ctx, 12345, "https://github.com/test")
	s.NoError(err)
}

func (s *ScrapperClientSuite) TestGetLinks_Success() {
	expectedLinks := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"tag1"}},
		{URL: "https://github.com/2", Tags: []string{"tag2", "tag3"}},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.assertCommonHeaders(r, http.MethodGet, "/links", 12345)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListLinksResponse{
			Links: []LinkResponse{
				{Id: 1, Url: "https://github.com/1", Tags: []string{"tag1"}},
				{Id: 2, Url: "https://github.com/2", Tags: []string{"tag2", "tag3"}},
			},
			Size: 2,
		})
	})

	s.startServer(handler)
	links, err := s.client.GetLinks(s.ctx, 12345)

	s.NoError(err)
	s.Equal(expectedLinks, links)
}

func (s *ScrapperClientSuite) TestGetLinks_Empty() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.assertCommonHeaders(r, http.MethodGet, "/links", 12345)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListLinksResponse{
			Links: []LinkResponse{},
			Size:  0,
		})
	})

	s.startServer(handler)
	links, err := s.client.GetLinks(s.ctx, 12345)

	s.NoError(err)
	s.Empty(links)
}

func (s *ScrapperClientSuite) TestRegisterChat_Success() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodPost, r.Method)
		s.Equal("/tg-chat/12345", r.URL.Path)
		s.Empty(r.Header.Get(HeaderChatID))

		w.WriteHeader(http.StatusOK)
	})

	s.startServer(handler)
	err := s.client.RegisterChat(s.ctx, 12345)
	s.NoError(err)
}

func (s *ScrapperClientSuite) TestRegisterChat_AlreadyExists() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.errorResponse(w, http.StatusConflict, "CHAT_ALREADY_EXISTS", "Chat already exists")
	})

	s.startServer(handler)
	err := s.client.RegisterChat(s.ctx, 12345)

	s.Error(err)
	s.Contains(err.Error(), "CHAT_ALREADY_EXISTS")
}

func (s *ScrapperClientSuite) TestDeleteChat_Success() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodDelete, r.Method)
		s.Equal("/tg-chat/12345", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	})

	s.startServer(handler)
	err := s.client.DeleteChat(s.ctx, 12345)
	s.NoError(err)
}

func (s *ScrapperClientSuite) TestDeleteChat_NotFound() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.errorResponse(w, http.StatusNotFound, "CHAT_NOT_FOUND", "Chat not found")
	})

	s.startServer(handler)
	err := s.client.DeleteChat(s.ctx, 12345)

	s.Error(err)
	s.Contains(err.Error(), "CHAT_NOT_FOUND")
}

func (s *ScrapperClientSuite) TestTimeout() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	s.startServer(handler)
	s.client.http.Timeout = 50 * time.Millisecond

	err := s.client.RegisterChat(s.ctx, 12345)
	s.Error(err)
}

func (s *ScrapperClientSuite) TestContextCancel() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	s.startServer(handler)

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	err := s.client.RegisterChat(ctx, 12345)
	s.Error(err)
}

func (s *ScrapperClientSuite) TestInvalidJSON() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	})

	s.startServer(handler)
	_, err := s.client.GetLinks(s.ctx, 12345)

	s.Error(err)
	s.Contains(err.Error(), "invalid character")
}

func (s *ScrapperClientSuite) TestConcurrent() {
	var requestCount int32
	var errorCount int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	})

	s.startServer(handler)

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg.Done()
			err := s.client.RegisterChat(s.ctx, int64(id))
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()

	s.Equal(int32(10), atomic.LoadInt32(&requestCount))
	s.Equal(int32(0), atomic.LoadInt32(&errorCount))
}

// startServer создает тестовый сервер с заданным обработчиком
func (s *ScrapperClientSuite) startServer(handler http.HandlerFunc) {
	s.server = httptest.NewServer(handler)
	s.client = NewScrapperClient(s.server.URL)
}

// assertCommonHeaders проверяет базовые заголовки запроса
func (s *ScrapperClientSuite) assertCommonHeaders(r *http.Request, expectedMethod, expectedPath string, expectedChatID int64) {
	s.Equal(expectedMethod, r.Method)
	s.Equal(expectedPath, r.URL.Path)

	if expectedChatID != 0 {
		s.Equal(strconv.FormatInt(expectedChatID, 10), r.Header.Get(HeaderChatID))
	}
}

// errorResponse создает JSON ответ с ошибкой
func (s *ScrapperClientSuite) errorResponse(w http.ResponseWriter, statusCode int, code, description string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiErrorResponse{
		Code:        code,
		Description: description,
	})
}
