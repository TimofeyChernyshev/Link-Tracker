package bot_server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestUpdates_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sender := NewMockSender(ctrl)

	s := NewServer(sender, "0")

	reqBody := `{
		"id": 1,
		"url": "https://url",
		"description": "updated",
		"tgChatIds": [10, 20]
	}`
	sender.EXPECT().SendMessage(&domain.Response{Text: "updated", ChatID: 10})
	sender.EXPECT().SendMessage(&domain.Response{Text: "updated", ChatID: 20})

	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUpdates_BadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sender := NewMockSender(ctrl)

	s := NewServer(sender, "0")

	reqBody := `{
		"id": 
	}`

	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(w, req)

	var bodyDecoded ApiErrorResponse
	_ = json.NewDecoder(w.Body).Decode(&bodyDecoded)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, ApiErrorResponse{Description: "invalid json", Code: http.StatusText(400)}, bodyDecoded)
}
