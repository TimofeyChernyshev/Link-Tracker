package bot_server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUpdates_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sender := NewMockService(ctrl)

	s := NewServer(sender, "0")

	reqBody := `{
		"id": 1,
		"url": "https://url",
		"description": "updated",
		"tgChatIds": [10, 20]
	}`

	sender.EXPECT().SendUpdates([]int64{10, 20}, "updated")

	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUpdates_BadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sender := NewMockService(ctrl)

	s := NewServer(sender, "0")

	reqBody := `{
		"id": 
	}`

	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	s.srv.Handler.ServeHTTP(w, req)

	var bodyDecoded APIErrorResponse
	_ = json.NewDecoder(w.Body).Decode(&bodyDecoded)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, APIErrorResponse{Description: "invalid json", Code: http.StatusText(400)}, bodyDecoded)
}
