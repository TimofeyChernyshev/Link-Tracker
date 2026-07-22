package httpnotifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

var (
	retryCfg = resilience.RetryConfig{
		MaxAttempts:   1,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500, 502, 503, 504},
	}
	cbConfig = resilience.CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit = 100
	timeout   = 5 * time.Second
)

func TestBotClient_SendUpdate_OK(t *testing.T) {
	var received LinkUpdate

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/updates", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)

	c := NewBotClient(ts.URL, resilientClient)

	upd := domain.LinkUpdate{
		ID:          1,
		URL:         "https://github.com/a/b",
		Description: "update",
		ChatIDs:     []int64{10, 20},
	}

	err := c.SendUpdate(context.Background(), upd)
	require.NoError(t, err)

	require.Equal(t, int64(1), received.ID)
	require.Equal(t, upd.URL, received.URL)
	require.Equal(t, upd.Description, received.Description)
	require.Equal(t, upd.ChatIDs, received.TgChatIDs)
}

func TestBotClient_SendUpdate_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_ = json.NewEncoder(w).Encode(APIErrorResponse{
			Code:        "BadRequest",
			Description: "invalid body",
		})
	}))
	defer ts.Close()

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	c := NewBotClient(ts.URL, resilientClient)

	err := c.SendUpdate(context.Background(), domain.LinkUpdate{})
	require.Error(t, err)
}
