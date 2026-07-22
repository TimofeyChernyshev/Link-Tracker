package scrapperhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ServerSuite struct {
	suite.Suite

	ctrl    *gomock.Controller
	service *MockService
	server  *Server
}

func (s *ServerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.service = NewMockService(s.ctrl)

	defaultLimit := 50
	maxLimit := 100
	s.server = NewServer("0", s.service, nil, defaultLimit, maxLimit)
}

func (s *ServerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

func (s *ServerSuite) TestUpdateChat_Register() {
	s.service.EXPECT().RegisterChat(gomock.Any(), int64(1)).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/tg-chat/1", nil)
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(200, rec.Code)
}

func (s *ServerSuite) TestUpdateChat_WrongID() {
	req := httptest.NewRequest(http.MethodPost, "/tg-chat/wrongID", nil)
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(400, rec.Code)
}

func (s *ServerSuite) TestUpdateChat_RegisterReturnError() {
	s.service.EXPECT().RegisterChat(gomock.Any(), int64(2)).Return(errors.New("some error"))

	req := httptest.NewRequest(http.MethodPost, "/tg-chat/2", nil)
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(409, rec.Code)
}

func (s *ServerSuite) TestUpdateChat_Delete() {
	s.service.EXPECT().DeleteChat(gomock.Any(), int64(2)).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/tg-chat/2", nil)
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(200, rec.Code)
}

func (s *ServerSuite) TestUpdateChat_AnotherMethod() {
	req := httptest.NewRequest(http.MethodGet, "/tg-chat/2", nil)
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *ServerSuite) TestLinks_Get() {
	s.service.EXPECT().GetLinks(gomock.Any(), int64(2), 1, 2).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/links?limit=1&offset=2", nil)
	req.Header["Tg-Chat-Id"] = []string{"2"}
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(200, rec.Code)
}

func (s *ServerSuite) TestLinks_GetDefaultValues() {
	s.service.EXPECT().GetLinks(gomock.Any(), int64(2), s.server.defaultLimit, defaultOffset).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/links", nil)
	req.Header["Tg-Chat-Id"] = []string{"2"}
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(200, rec.Code)
}

func (s *ServerSuite) TestLinks_Post() {
	s.service.EXPECT().AddLink(gomock.Any(), int64(2), "https://123", []string{"123"}).Return(domain.Link{ID: 0, URL: "https://123", Tags: []string{"123"}}, nil)

	body := `{"link":"https://123","tags":["123"]}`
	req := httptest.NewRequest(http.MethodPost, "/links", strings.NewReader(body))
	req.Header.Set("Tg-Chat-Id", "2")
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(200, rec.Code)
}

func (s *ServerSuite) TestLinks_Delete() {
	s.service.EXPECT().RemoveLink(gomock.Any(), int64(2), "https://123").Return(domain.Link{ID: 0, URL: "https://123", Tags: []string{"123"}}, nil)

	body := `{"link":"https://123"}`
	req := httptest.NewRequest(http.MethodDelete, "/links", strings.NewReader(body))
	req.Header.Set("Tg-Chat-Id", "2")
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(200, rec.Code)
}

func (s *ServerSuite) TestLinks_BadBody() {
	body := `{"link":"https://asdasd"}`
	req := httptest.NewRequest(http.MethodDelete, "/links", strings.NewReader(body))
	req.Header.Set("Tg-Chat-Id", "asdasd")
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ServerSuite) TestLinks_BadJson() {
	body := `{"link": "}`
	req := httptest.NewRequest(http.MethodDelete, "/links", strings.NewReader(body))
	req.Header.Set("Tg-Chat-Id", "2")
	rec := httptest.NewRecorder()

	s.server.srv.Handler.ServeHTTP(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}
